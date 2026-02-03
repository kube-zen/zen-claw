package circuit

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State string

const (
	StateClosed   State = "closed"    // Normal operation
	StateOpen     State = "open"      // Failing, skip provider
	StateHalfOpen State = "half_open" // Testing recovery
)

// Breaker implements the circuit breaker pattern for provider health
type Breaker struct {
	mu sync.RWMutex

	name          string
	state         State
	failures      int
	successes     int
	lastFailTime  time.Time
	lastStateTime time.Time

	// Config
	errorThreshold   float64       // e.g., 0.5 = 50% error rate to open
	windowSize       int           // Sliding window size (requests)
	cooldownDuration time.Duration // Time before half-open attempt
	halfOpenRequests int           // Requests to test in half-open

	// Metrics - circular buffer for sliding window
	window    []bool // true=success, false=failure
	windowIdx int
}

// Config for circuit breaker
type Config struct {
	ErrorThreshold   float64       // Error rate to trip (0.0-1.0)
	WindowSize       int           // Number of requests to track
	CooldownDuration time.Duration // Wait before retry
	HalfOpenRequests int           // Successes needed to recover
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		ErrorThreshold:   0.5,              // 50% error rate opens circuit
		WindowSize:       10,               // Track last 10 requests
		CooldownDuration: 30 * time.Second, // Wait 30s before retry
		HalfOpenRequests: 2,                // 2 successes to recover
	}
}

// NewBreaker creates a circuit breaker
func NewBreaker(name string, cfg Config) *Breaker {
	if cfg.WindowSize == 0 {
		cfg = DefaultConfig()
	}

	return &Breaker{
		name:             name,
		state:            StateClosed,
		errorThreshold:   cfg.ErrorThreshold,
		windowSize:       cfg.WindowSize,
		cooldownDuration: cfg.CooldownDuration,
		halfOpenRequests: cfg.HalfOpenRequests,
		window:           make([]bool, cfg.WindowSize),
		lastStateTime:    time.Now(),
	}
}

// Call wraps a function with circuit breaker logic
func (b *Breaker) Call(ctx context.Context, fn func() error) error {
	b.mu.Lock()

	// Check if circuit is open
	if b.state == StateOpen {
		if time.Since(b.lastStateTime) >= b.cooldownDuration {
			// Cooldown elapsed, try half-open
			log.Printf("[Circuit] %s: open → half_open", b.name)
			b.state = StateHalfOpen
			b.successes = 0
			b.lastStateTime = time.Now()
		} else {
			b.mu.Unlock()
			return fmt.Errorf("circuit open for %s (retry in %v)", b.name,
				b.cooldownDuration-time.Since(b.lastStateTime))
		}
	}

	currentState := b.state
	b.mu.Unlock()

	// Execute the function
	err := fn()

	// Record result and handle state transitions
	b.mu.Lock()
	defer b.mu.Unlock()

	if err != nil {
		b.recordFailure()
	} else {
		b.recordSuccess()
	}

	switch currentState {
	case StateClosed:
		if b.shouldOpen() {
			log.Printf("[Circuit] %s: closed → open (error rate %.0f%%)", b.name, b.errorRate()*100)
			b.state = StateOpen
			b.lastStateTime = time.Now()
		}

	case StateHalfOpen:
		if err == nil {
			b.successes++
			if b.successes >= b.halfOpenRequests {
				log.Printf("[Circuit] %s: half_open → closed (recovered)", b.name)
				b.state = StateClosed
				b.failures = 0
				b.successes = 0
				b.lastStateTime = time.Now()
				// Reset window
				for i := range b.window {
					b.window[i] = true
				}
			}
		} else {
			log.Printf("[Circuit] %s: half_open → open (test failed)", b.name)
			b.state = StateOpen
			b.lastStateTime = time.Now()
		}
	}

	return err
}

func (b *Breaker) recordSuccess() {
	b.window[b.windowIdx] = true
	b.windowIdx = (b.windowIdx + 1) % b.windowSize
	b.successes++
}

func (b *Breaker) recordFailure() {
	b.window[b.windowIdx] = false
	b.windowIdx = (b.windowIdx + 1) % b.windowSize
	b.failures++
	b.lastFailTime = time.Now()
}

func (b *Breaker) shouldOpen() bool {
	return b.errorRate() >= b.errorThreshold
}

func (b *Breaker) errorRate() float64 {
	failures := 0
	for _, success := range b.window {
		if !success {
			failures++
		}
	}
	return float64(failures) / float64(b.windowSize)
}

// GetState returns current state
func (b *Breaker) GetState() State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// IsAvailable returns true if provider is available
func (b *Breaker) IsAvailable() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.state == StateOpen {
		// Check if cooldown elapsed
		return time.Since(b.lastStateTime) >= b.cooldownDuration
	}
	return true
}

// Stats returns circuit breaker statistics
func (b *Breaker) Stats() (state State, errorRate float64, failures, successes int) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state, b.errorRate(), b.failures, b.successes
}

// Manager manages circuit breakers for multiple providers
type Manager struct {
	mu       sync.RWMutex
	breakers map[string]*Breaker
	config   Config
}

// NewManager creates a circuit breaker manager
func NewManager(cfg Config) *Manager {
	if cfg.WindowSize == 0 {
		cfg = DefaultConfig()
	}
	return &Manager{
		breakers: make(map[string]*Breaker),
		config:   cfg,
	}
}

// Get returns or creates a circuit breaker for a provider
func (m *Manager) Get(name string) *Breaker {
	m.mu.RLock()
	if cb, ok := m.breakers[name]; ok {
		m.mu.RUnlock()
		return cb
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, ok := m.breakers[name]; ok {
		return cb
	}

	cb := NewBreaker(name, m.config)
	m.breakers[name] = cb
	return cb
}

// AllStats returns stats for all breakers
func (m *Manager) AllStats() map[string]map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]map[string]interface{})
	for name, cb := range m.breakers {
		state, errRate, failures, successes := cb.Stats()
		stats[name] = map[string]interface{}{
			"state":      string(state),
			"error_rate": fmt.Sprintf("%.0f%%", errRate*100),
			"failures":   failures,
			"successes":  successes,
			"available":  cb.IsAvailable(),
		}
	}
	return stats
}
