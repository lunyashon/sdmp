package ratelimit

import (
	"math"
	"sync"
	"time"
)

const defaultIdle = 2 * time.Minute

// Limiter is an in-process token bucket keyed by caller (source token).
type Limiter struct {
	rps   float64
	burst float64
	idle  time.Duration
	now   func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func New(rps, burst int) *Limiter {
	if rps <= 0 {
		return nil
	}
	if burst <= 0 {
		burst = rps
		if burst > 32 {
			burst = 32
		}
	}
	return &Limiter{
		rps:     float64(rps),
		burst:   float64(burst),
		idle:    defaultIdle,
		now:     time.Now,
		buckets: make(map[string]*bucket),
	}
}

// Allow spends one token for key. retryAfter is set when the request is rejected.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	if l == nil {
		return true, 0
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	} else {
		elapsed := now.Sub(b.last).Seconds()
		if elapsed > 0 {
			b.tokens += elapsed * l.rps
			if b.tokens > l.burst {
				b.tokens = l.burst
			}
			b.last = now
		}
	}
	if b.tokens < 1 {
		wait := time.Duration(math.Ceil((1-b.tokens)/l.rps) * float64(time.Second))
		if wait < time.Second {
			wait = time.Second
		}
		l.evictLocked(now)
		return false, wait
	}
	b.tokens--
	if len(l.buckets) > 256 {
		l.evictLocked(now)
	}
	return true, 0
}

func (l *Limiter) evictLocked(now time.Time) {
	for k, b := range l.buckets {
		if now.Sub(b.last) > l.idle {
			delete(l.buckets, k)
		}
	}
}
