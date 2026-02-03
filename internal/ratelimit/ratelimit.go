// Package ratelimit provides per-client rate limiting for the gateway.
package ratelimit

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter provides per-client rate limiting using token bucket algorithm.
type Limiter struct {
	mu       sync.RWMutex
	limiters map[string]*clientLimiter
	config   Config
	cleanup  *time.Ticker
	done     chan struct{}
}

// clientLimiter tracks rate limit state for a single client.
type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Config configures the rate limiter.
type Config struct {
	RequestsPerSecond float64       // Max requests per second per client
	BurstSize         int           // Max burst size (tokens)
	CleanupInterval   time.Duration // How often to clean up stale clients
	ClientTTL         time.Duration // How long to keep client state after last request
}

// DefaultConfig returns sensible defaults for rate limiting.
func DefaultConfig() Config {
	return Config{
		RequestsPerSecond: 10,              // 10 req/s per client
		BurstSize:         20,              // Allow bursts up to 20
		CleanupInterval:   5 * time.Minute, // Cleanup every 5 min
		ClientTTL:         30 * time.Minute, // Remove after 30 min inactive
	}
}

// NewLimiter creates a new per-client rate limiter.
func NewLimiter(cfg Config) *Limiter {
	if cfg.RequestsPerSecond <= 0 {
		cfg.RequestsPerSecond = DefaultConfig().RequestsPerSecond
	}
	if cfg.BurstSize <= 0 {
		cfg.BurstSize = DefaultConfig().BurstSize
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = DefaultConfig().CleanupInterval
	}
	if cfg.ClientTTL <= 0 {
		cfg.ClientTTL = DefaultConfig().ClientTTL
	}

	l := &Limiter{
		limiters: make(map[string]*clientLimiter),
		config:   cfg,
		done:     make(chan struct{}),
	}

	// Start cleanup goroutine
	l.cleanup = time.NewTicker(cfg.CleanupInterval)
	go l.cleanupLoop()

	return l
}

// Allow checks if a request from the given client is allowed.
// Returns true if allowed, false if rate limited.
func (l *Limiter) Allow(clientID string) bool {
	cl := l.getOrCreate(clientID)
	return cl.limiter.Allow()
}

// Wait blocks until a request from the given client is allowed.
// Returns an error if the context is canceled.
func (l *Limiter) Wait(ctx context.Context, clientID string) error {
	cl := l.getOrCreate(clientID)
	return cl.limiter.Wait(ctx)
}

// getOrCreate returns existing or creates new limiter for client.
func (l *Limiter) getOrCreate(clientID string) *clientLimiter {
	l.mu.RLock()
	cl, ok := l.limiters[clientID]
	if ok {
		cl.lastSeen = time.Now()
		l.mu.RUnlock()
		return cl
	}
	l.mu.RUnlock()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Double-check after acquiring write lock
	if cl, ok := l.limiters[clientID]; ok {
		cl.lastSeen = time.Now()
		return cl
	}

	cl = &clientLimiter{
		limiter:  rate.NewLimiter(rate.Limit(l.config.RequestsPerSecond), l.config.BurstSize),
		lastSeen: time.Now(),
	}
	l.limiters[clientID] = cl
	return cl
}

// cleanupLoop removes stale client limiters.
func (l *Limiter) cleanupLoop() {
	for {
		select {
		case <-l.cleanup.C:
			l.cleanupStale()
		case <-l.done:
			return
		}
	}
}

// cleanupStale removes limiters that haven't been used recently.
func (l *Limiter) cleanupStale() {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().Add(-l.config.ClientTTL)
	for id, cl := range l.limiters {
		if cl.lastSeen.Before(cutoff) {
			delete(l.limiters, id)
		}
	}
}

// Close stops the cleanup goroutine.
func (l *Limiter) Close() {
	l.cleanup.Stop()
	close(l.done)
}

// Stats returns current rate limiter statistics.
func (l *Limiter) Stats() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return map[string]interface{}{
		"active_clients":      len(l.limiters),
		"requests_per_second": l.config.RequestsPerSecond,
		"burst_size":          l.config.BurstSize,
	}
}

// ClientCount returns the number of tracked clients.
func (l *Limiter) ClientCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.limiters)
}
