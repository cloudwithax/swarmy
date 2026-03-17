// Package ratelimit provides rate limiting functionality for API providers.
package ratelimit

import (
	"context"
	"sync"
	"time"

	"github.com/cloudwithax/swarmy/internal/config"
)

// Limiter implements a token bucket rate limiter.
type Limiter struct {
	tokens     float64
	capacity   float64
	refillRate float64 // tokens per second
	mu         sync.Mutex
	lastRefill time.Time
}

// New creates a new token bucket rate limiter.
// capacity is the maximum number of tokens in the bucket.
// refillRate is the rate at which tokens are added per second.
func New(capacity, refillRate float64) *Limiter {
	return &Limiter{
		tokens:     capacity,
		capacity:   capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// NewNvidiaNIMLimiter creates a rate limiter for NVIDIA NIM.
// NVIDIA NIM allows 40 requests per minute, which is 1 request per 1.5 seconds.
func NewNvidiaNIMLimiter() *Limiter {
	// 40 RPM = 1 request per 1.5 seconds
	// Use capacity of 1 to allow burst of 1, refill at 1/1.5 = 0.666... tokens per second
	return New(1, 1.0/1.5)
}

// Wait blocks until a token is available or the context is cancelled.
// Returns an error if the context is cancelled before a token becomes available.
func (l *Limiter) Wait(ctx context.Context) error {
	for {
		waitTime, ok := l.tryAcquire()
		if ok {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Try again after waiting
		}
	}
}

// tryAcquire attempts to acquire a token from the bucket.
// Returns the time to wait if no token is available, and a bool indicating success.
func (l *Limiter) tryAcquire() (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.lastRefill).Seconds()

	// Refill tokens based on elapsed time
	l.tokens += elapsed * l.refillRate
	if l.tokens > l.capacity {
		l.tokens = l.capacity
	}
	l.lastRefill = now

	if l.tokens >= 1 {
		l.tokens--
		return 0, true
	}

	// Calculate wait time for next token
	waitTime := time.Duration((1 - l.tokens) / l.refillRate * float64(time.Second))
	return waitTime, false
}

// Tokens returns the current number of tokens in the bucket (for testing).
func (l *Limiter) Tokens() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.lastRefill).Seconds()
	tokens := l.tokens + elapsed*l.refillRate
	if tokens > l.capacity {
		tokens = l.capacity
	}
	return tokens
}

// RateLimiter manages rate limiters for multiple providers.
type RateLimiter struct {
	limiters map[string]*Limiter
	mu       sync.RWMutex
}

// NewRateLimiter creates a new multi-provider rate limiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*Limiter),
	}
}

// Wait blocks until a token is available for the given provider.
// It returns immediately if no rate limiting is configured for the provider.
func (r *RateLimiter) Wait(ctx context.Context, providerID string) error {
	r.mu.RLock()
	limiter, exists := r.limiters[providerID]
	r.mu.RUnlock()

	if !exists {
		// Check if this is a provider that needs rate limiting.
		if config.IsNvidiaNIMProvider(providerID) {
			// Create a new limiter for NVIDIA NIM.
			limiter = NewNvidiaNIMLimiter()
			r.mu.Lock()
			r.limiters[providerID] = limiter
			r.mu.Unlock()
		} else {
			// No rate limiting for this provider.
			return nil
		}
	}

	return limiter.Wait(ctx)
}
