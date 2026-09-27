package ratelimit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestConcurrentRequestsAreSpaced(t *testing.T) {
	const interval = 20 * time.Millisecond
	limiter := New(interval)
	started := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := limiter.Wait(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if elapsed := time.Since(started); elapsed < 2*interval {
		t.Fatalf("three requests started in %s", elapsed)
	}
}

func TestCancellationDoesNotConsumeSlot(t *testing.T) {
	limiter := New(time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := limiter.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if !limiter.next.IsZero() {
		t.Fatal("cancelled request consumed a slot")
	}
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := limiter.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiting request error = %v", err)
	}
}
