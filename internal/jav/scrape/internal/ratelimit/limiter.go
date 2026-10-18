// Package ratelimit spaces request starts for a provider or shared site.
package ratelimit

import (
	"context"
	"sync"
	"time"
)

type Limiter struct {
	mu       sync.Mutex
	next     time.Time
	interval time.Duration
}

func New(interval time.Duration) *Limiter { return &Limiter{interval: interval} }
func (l *Limiter) Wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		l.mu.Lock()
		now := time.Now()
		if !now.Before(l.next) {
			l.next = now.Add(l.interval)
			l.mu.Unlock()
			return nil
		}
		wait := time.Until(l.next)
		l.mu.Unlock()
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
