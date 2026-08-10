package server

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// rateLimiter is a small keyed token-bucket limiter (per IP, per email, etc.).
// Idle buckets are swept periodically so the map does not grow unbounded under
// a flood of distinct keys.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    rate.Limit
	burst   int
}

type bucket struct {
	lim  *rate.Limiter
	seen time.Time
}

func newRateLimiter(r rate.Limit, burst int) *rateLimiter {
	rl := &rateLimiter{
		buckets: make(map[string]*bucket),
		rate:    r,
		burst:   burst,
	}
	go rl.sweep()
	return rl
}

// Allow reports whether an event for key may proceed now.
func (rl *rateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{lim: rate.NewLimiter(rl.rate, rl.burst)}
		rl.buckets[key] = b
	}
	b.seen = time.Now()
	return b.lim.Allow()
}

func (rl *rateLimiter) sweep() {
	t := time.NewTicker(10 * time.Minute)
	for range t.C {
		cutoff := time.Now().Add(-30 * time.Minute)
		rl.mu.Lock()
		for k, b := range rl.buckets {
			if b.seen.Before(cutoff) {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}
