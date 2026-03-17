package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	t.Run("acquire token immediately when bucket is full", func(t *testing.T) {
		l := New(5, 1) // capacity 5, rate 1 token per second

		ctx := context.Background()
		err := l.Wait(ctx)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		// Should have 4 tokens left
		if l.Tokens() < 3.9 || l.Tokens() > 4.1 {
			t.Errorf("expected ~4 tokens, got %f", l.Tokens())
		}
	})

	t.Run("wait for token when bucket is empty", func(t *testing.T) {
		l := New(1, 10) // capacity 1, rate 10 tokens per second (0.1s per token)

		ctx := context.Background()

		// First acquire should succeed immediately
		err := l.Wait(ctx)
		if err != nil {
			t.Errorf("expected no error on first acquire, got %v", err)
		}

		// Second acquire should wait
		start := time.Now()
		err = l.Wait(ctx)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("expected no error on second acquire, got %v", err)
		}

		// Should have waited at least 0.05s (half the refill time)
		if elapsed < 50*time.Millisecond {
			t.Errorf("expected to wait for token, but only waited %v", elapsed)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		l := New(1, 0.1) // capacity 1, rate 0.1 tokens per second (10s per token)

		// First acquire should succeed
		ctx := context.Background()
		err := l.Wait(ctx)
		if err != nil {
			t.Errorf("expected no error on first acquire, got %v", err)
		}

		// Second acquire with cancelled context should fail
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err = l.Wait(ctx)
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}

func TestNewNvidiaNIMLimiter(t *testing.T) {
	l := NewNvidiaNIMLimiter()

	// Should have capacity of 1
	if l.capacity != 1 {
		t.Errorf("expected capacity 1, got %f", l.capacity)
	}

	// Should have rate of 1/1.5 = 0.666... tokens per second
	expectedRate := 1.0 / 1.5
	if l.refillRate < expectedRate-0.01 || l.refillRate > expectedRate+0.01 {
		t.Errorf("expected rate ~%f, got %f", expectedRate, l.refillRate)
	}
}

func TestRateLimiter(t *testing.T) {
	t.Run("no rate limiting for unknown providers", func(t *testing.T) {
		rl := NewRateLimiter()
		ctx := context.Background()

		// Should not block for unknown providers
		err := rl.Wait(ctx, "unknown-provider")
		if err != nil {
			t.Errorf("expected no error for unknown provider, got %v", err)
		}
	})

	t.Run("rate limiting for NVIDIA NIM", func(t *testing.T) {
		rl := NewRateLimiter()
		ctx := context.Background()

		// First request should succeed immediately
		err := rl.Wait(ctx, "nvidia-nim")
		if err != nil {
			t.Errorf("expected no error on first request, got %v", err)
		}

		// Second request should wait (or create a new limiter)
		start := time.Now()
		err = rl.Wait(ctx, "nvidia-nim")
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("expected no error on second request, got %v", err)
		}

		// The second request might wait due to rate limiting
		t.Logf("Second request took %v", elapsed)
	})

	t.Run("separate limiters for different providers", func(t *testing.T) {
		rl := NewRateLimiter()
		ctx := context.Background()

		// First request to nvidia-nim should succeed
		err := rl.Wait(ctx, "nvidia-nim")
		if err != nil {
			t.Errorf("expected no error for nvidia-nim, got %v", err)
		}

		// Request to another provider should also succeed immediately
		err = rl.Wait(ctx, "other-provider")
		if err != nil {
			t.Errorf("expected no error for other-provider, got %v", err)
		}
	})
}
