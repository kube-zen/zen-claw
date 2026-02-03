package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.RequestsPerSecond <= 0 {
		t.Error("RequestsPerSecond should be > 0")
	}
	if cfg.BurstSize <= 0 {
		t.Error("BurstSize should be > 0")
	}
	if cfg.CleanupInterval <= 0 {
		t.Error("CleanupInterval should be > 0")
	}
	if cfg.ClientTTL <= 0 {
		t.Error("ClientTTL should be > 0")
	}
}

func TestNewLimiter(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		cfg := Config{
			RequestsPerSecond: 5,
			BurstSize:         10,
			CleanupInterval:   time.Minute,
			ClientTTL:         time.Hour,
		}
		l := NewLimiter(cfg)
		defer l.Close()

		if l.config.RequestsPerSecond != 5 {
			t.Errorf("RequestsPerSecond = %v, want 5", l.config.RequestsPerSecond)
		}
	})

	t.Run("with zero config uses defaults", func(t *testing.T) {
		l := NewLimiter(Config{})
		defer l.Close()

		if l.config.RequestsPerSecond != 10 {
			t.Errorf("RequestsPerSecond = %v, want 10 (default)", l.config.RequestsPerSecond)
		}
	})
}

func TestLimiterAllow(t *testing.T) {
	cfg := Config{
		RequestsPerSecond: 100, // High rate for testing
		BurstSize:         5,
		CleanupInterval:   time.Hour,
		ClientTTL:         time.Hour,
	}
	l := NewLimiter(cfg)
	defer l.Close()

	t.Run("allows burst", func(t *testing.T) {
		// Should allow up to burst size
		for i := 0; i < 5; i++ {
			if !l.Allow("client1") {
				t.Errorf("request %d should be allowed", i)
			}
		}
	})

	t.Run("limits after burst", func(t *testing.T) {
		// New client with low rate
		cfg2 := Config{
			RequestsPerSecond: 1,
			BurstSize:         2,
			CleanupInterval:   time.Hour,
			ClientTTL:         time.Hour,
		}
		l2 := NewLimiter(cfg2)
		defer l2.Close()

		// Consume burst
		l2.Allow("client")
		l2.Allow("client")

		// Next should be rate limited
		if l2.Allow("client") {
			t.Error("should be rate limited after burst")
		}
	})

	t.Run("different clients have separate limits", func(t *testing.T) {
		cfg2 := Config{
			RequestsPerSecond: 1,
			BurstSize:         2,
			CleanupInterval:   time.Hour,
			ClientTTL:         time.Hour,
		}
		l2 := NewLimiter(cfg2)
		defer l2.Close()

		// Exhaust client1's burst
		l2.Allow("client1")
		l2.Allow("client1")

		// client2 should still have its burst
		if !l2.Allow("client2") {
			t.Error("client2 should be allowed (has its own limit)")
		}
	})
}

func TestLimiterWait(t *testing.T) {
	cfg := Config{
		RequestsPerSecond: 100,
		BurstSize:         2,
		CleanupInterval:   time.Hour,
		ClientTTL:         time.Hour,
	}
	l := NewLimiter(cfg)
	defer l.Close()

	t.Run("returns immediately when allowed", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := l.Wait(ctx, "waiter")
		if err != nil {
			t.Errorf("Wait() error = %v", err)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		cfg2 := Config{
			RequestsPerSecond: 0.1, // Very slow
			BurstSize:         1,
			CleanupInterval:   time.Hour,
			ClientTTL:         time.Hour,
		}
		l2 := NewLimiter(cfg2)
		defer l2.Close()

		// Consume burst
		l2.Allow("slow")

		// Next wait should be canceled
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		err := l2.Wait(ctx, "slow")
		if err == nil {
			t.Error("Wait() should return error on context timeout")
		}
	})
}

func TestLimiterConcurrency(t *testing.T) {
	cfg := Config{
		RequestsPerSecond: 1000,
		BurstSize:         100,
		CleanupInterval:   time.Hour,
		ClientTTL:         time.Hour,
	}
	l := NewLimiter(cfg)
	defer l.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				l.Allow("concurrent")
			}
		}(i)
	}
	wg.Wait()

	// Should not panic or deadlock
}

func TestLimiterStats(t *testing.T) {
	cfg := Config{
		RequestsPerSecond: 5,
		BurstSize:         10,
		CleanupInterval:   time.Hour,
		ClientTTL:         time.Hour,
	}
	l := NewLimiter(cfg)
	defer l.Close()

	// Create some clients
	l.Allow("a")
	l.Allow("b")
	l.Allow("c")

	stats := l.Stats()

	if stats["active_clients"] != 3 {
		t.Errorf("active_clients = %v, want 3", stats["active_clients"])
	}
	if stats["requests_per_second"] != 5.0 {
		t.Errorf("requests_per_second = %v, want 5", stats["requests_per_second"])
	}
}

func TestLimiterClientCount(t *testing.T) {
	l := NewLimiter(DefaultConfig())
	defer l.Close()

	if l.ClientCount() != 0 {
		t.Errorf("ClientCount() = %d, want 0", l.ClientCount())
	}

	l.Allow("a")
	l.Allow("b")

	if l.ClientCount() != 2 {
		t.Errorf("ClientCount() = %d, want 2", l.ClientCount())
	}
}

func TestLimiterCleanup(t *testing.T) {
	cfg := Config{
		RequestsPerSecond: 10,
		BurstSize:         10,
		CleanupInterval:   10 * time.Millisecond,
		ClientTTL:         20 * time.Millisecond,
	}
	l := NewLimiter(cfg)
	defer l.Close()

	l.Allow("temp")

	if l.ClientCount() != 1 {
		t.Error("expected 1 client")
	}

	// Wait for cleanup
	time.Sleep(50 * time.Millisecond)

	if l.ClientCount() != 0 {
		t.Errorf("expected 0 clients after cleanup, got %d", l.ClientCount())
	}
}
