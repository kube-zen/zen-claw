package circuit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ErrorThreshold != 0.5 {
		t.Errorf("ErrorThreshold = %v, want 0.5", cfg.ErrorThreshold)
	}
	if cfg.WindowSize != 10 {
		t.Errorf("WindowSize = %d, want 10", cfg.WindowSize)
	}
	if cfg.CooldownDuration != 30*time.Second {
		t.Errorf("CooldownDuration = %v, want 30s", cfg.CooldownDuration)
	}
	if cfg.HalfOpenRequests != 2 {
		t.Errorf("HalfOpenRequests = %d, want 2", cfg.HalfOpenRequests)
	}
}

func TestNewBreaker(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		cfg := Config{
			ErrorThreshold:   0.3,
			WindowSize:       5,
			CooldownDuration: 10 * time.Second,
			HalfOpenRequests: 1,
		}
		b := NewBreaker("test", cfg)

		if b.name != "test" {
			t.Errorf("name = %q, want %q", b.name, "test")
		}
		if b.state != StateClosed {
			t.Errorf("initial state = %v, want %v", b.state, StateClosed)
		}
		if b.errorThreshold != 0.3 {
			t.Errorf("errorThreshold = %v, want 0.3", b.errorThreshold)
		}
	})

	t.Run("with zero config uses defaults", func(t *testing.T) {
		b := NewBreaker("test", Config{})

		if b.windowSize != 10 {
			t.Errorf("windowSize = %d, want 10 (default)", b.windowSize)
		}
	})
}

func TestBreakerStateTransitions(t *testing.T) {
	cfg := Config{
		ErrorThreshold:   0.5, // 50% errors to open
		WindowSize:       4,   // 4 request window
		CooldownDuration: 50 * time.Millisecond,
		HalfOpenRequests: 1,
	}

	t.Run("stays closed on success", func(t *testing.T) {
		// Use higher threshold so initial window doesn't trip
		localCfg := Config{
			ErrorThreshold:   0.9, // 90% errors to open
			WindowSize:       4,
			CooldownDuration: 50 * time.Millisecond,
			HalfOpenRequests: 1,
		}
		b := NewBreaker("test", localCfg)
		ctx := context.Background()

		// Fill window with successes
		for i := 0; i < 5; i++ {
			err := b.Call(ctx, func() error { return nil })
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}

		if b.GetState() != StateClosed {
			t.Errorf("state = %v, want %v", b.GetState(), StateClosed)
		}
	})

	t.Run("opens after threshold failures", func(t *testing.T) {
		b := NewBreaker("test", cfg)
		ctx := context.Background()

		// Fill window with failures (need 50% = 2 out of 4)
		testErr := errors.New("test error")
		for i := 0; i < 4; i++ {
			b.Call(ctx, func() error { return testErr })
		}

		if b.GetState() != StateOpen {
			t.Errorf("state = %v, want %v", b.GetState(), StateOpen)
		}
	})

	t.Run("rejects calls when open", func(t *testing.T) {
		b := NewBreaker("test", cfg)
		ctx := context.Background()

		// Force open
		testErr := errors.New("test error")
		for i := 0; i < 4; i++ {
			b.Call(ctx, func() error { return testErr })
		}

		// Should reject immediately
		err := b.Call(ctx, func() error { return nil })
		if err == nil {
			t.Error("expected error when circuit is open")
		}
	})

	t.Run("transitions to half-open after cooldown", func(t *testing.T) {
		b := NewBreaker("test", cfg)
		ctx := context.Background()

		// Force open
		testErr := errors.New("test error")
		for i := 0; i < 4; i++ {
			b.Call(ctx, func() error { return testErr })
		}

		// Wait for cooldown
		time.Sleep(60 * time.Millisecond)

		// Next call should succeed and transition to half-open
		err := b.Call(ctx, func() error { return nil })
		if err != nil {
			t.Errorf("unexpected error after cooldown: %v", err)
		}

		// Should now be closed (since halfOpenRequests=1)
		if b.GetState() != StateClosed {
			t.Errorf("state = %v, want %v (after recovery)", b.GetState(), StateClosed)
		}
	})
}

func TestBreakerIsAvailable(t *testing.T) {
	cfg := Config{
		ErrorThreshold:   0.5,
		WindowSize:       2,
		CooldownDuration: 50 * time.Millisecond,
		HalfOpenRequests: 1,
	}

	b := NewBreaker("test", cfg)
	ctx := context.Background()

	// Initially available
	if !b.IsAvailable() {
		t.Error("expected available initially")
	}

	// Force open
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		b.Call(ctx, func() error { return testErr })
	}

	// Should be unavailable when open
	if b.IsAvailable() {
		t.Error("expected unavailable when open")
	}

	// Wait for cooldown
	time.Sleep(60 * time.Millisecond)

	// Should be available after cooldown
	if !b.IsAvailable() {
		t.Error("expected available after cooldown")
	}
}

func TestBreakerStats(t *testing.T) {
	// Use high threshold so we can test stats without tripping
	cfg := Config{
		ErrorThreshold:   0.95, // 95% errors to open
		WindowSize:       10,
		CooldownDuration: 30 * time.Second,
		HalfOpenRequests: 2,
	}
	b := NewBreaker("test", cfg)
	ctx := context.Background()

	// Fill window with mostly successes first
	for i := 0; i < 8; i++ {
		b.Call(ctx, func() error { return nil })
	}

	// Then add some failures
	b.Call(ctx, func() error { return errors.New("fail") })
	b.Call(ctx, func() error { return errors.New("fail") })

	state, errRate, failures, successes := b.Stats()

	if state != StateClosed {
		t.Errorf("state = %v, want %v", state, StateClosed)
	}
	if failures < 2 {
		t.Errorf("failures = %d, want >= 2", failures)
	}
	if successes < 8 {
		t.Errorf("successes = %d, want >= 8", successes)
	}
	if errRate < 0 || errRate > 1 {
		t.Errorf("errRate = %v, want 0-1", errRate)
	}
}

func TestManager(t *testing.T) {
	m := NewManager(DefaultConfig())

	t.Run("creates breakers on demand", func(t *testing.T) {
		b1 := m.Get("provider1")
		b2 := m.Get("provider2")

		if b1 == b2 {
			t.Error("expected different breakers for different providers")
		}
	})

	t.Run("returns same breaker for same provider", func(t *testing.T) {
		b1 := m.Get("provider1")
		b2 := m.Get("provider1")

		if b1 != b2 {
			t.Error("expected same breaker for same provider")
		}
	})

	t.Run("all stats returns map of all breakers", func(t *testing.T) {
		m := NewManager(DefaultConfig())
		m.Get("p1")
		m.Get("p2")

		stats := m.AllStats()

		if len(stats) != 2 {
			t.Errorf("expected 2 breakers, got %d", len(stats))
		}
		if _, ok := stats["p1"]; !ok {
			t.Error("missing stats for p1")
		}
		if _, ok := stats["p2"]; !ok {
			t.Error("missing stats for p2")
		}
	})
}

func TestManagerWithZeroConfig(t *testing.T) {
	m := NewManager(Config{})

	b := m.Get("test")
	if b.windowSize != 10 {
		t.Errorf("expected default windowSize 10, got %d", b.windowSize)
	}
}
