package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Pace spaces outgoing calls so they stay under rps. Wait is safe for concurrent callers.
type Pace struct {
	interval time.Duration
	mu       sync.Mutex
	next     time.Time
}

func NewPace(rps int) *Pace {
	if rps <= 0 {
		return nil
	}
	return &Pace{interval: time.Second / time.Duration(rps)}
}

func (p *Pace) Wait(ctx context.Context) error {
	if p == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	now := time.Now()
	start := p.next
	if start.Before(now) {
		start = now
	}
	p.next = start.Add(p.interval)
	p.mu.Unlock()

	delay := time.Until(start)
	if delay <= 0 {
		return nil
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
