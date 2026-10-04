package playback

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestSessionsRetryOrderingAndExpiry(t *testing.T) {
	now := time.Now()
	var total int64
	fail := false
	s := NewSessions(func(_ context.Context, video, jav, delta int64) error {
		if video != 7 || jav != 9 {
			t.Fatalf("attribution = %d/%d", video, jav)
		}
		if fail {
			return errors.New("database unavailable")
		}
		total += delta
		return nil
	})
	s.now = func() time.Time { return now }
	id, err := s.Create(7, 9)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	for _, tc := range []struct {
		name             string
		checkpoint, want int64
	}{
		{"first", 10000, 10000}, {"lost response retry", 10000, 10000},
		{"next", 20000, 20000}, {"out of order", 5000, 20000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.Report(context.Background(), id, 7, tc.checkpoint); err != nil {
				t.Fatal(err)
			}
			if total != tc.want {
				t.Fatalf("total=%d want=%d", total, tc.want)
			}
		})
	}
	fail = true
	if err := s.Report(context.Background(), id, 7, 30000); err == nil {
		t.Fatal("expected failed commit")
	}
	fail = false
	if err := s.Report(context.Background(), id, 7, 30000); err != nil {
		t.Fatal(err)
	}
	if total != 30000 {
		t.Fatalf("failed write advanced checkpoint: %d", total)
	}
	for _, value := range []int64{-1, 1000000} {
		if err := s.Report(context.Background(), id, 7, value); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid total: %v", err)
		}
	}
	if err := s.Report(context.Background(), id, 8, 30000); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	now = now.Add(SessionTTL)
	if err := s.Report(context.Background(), id, 7, 40000); !errors.Is(err, ErrExpired) {
		t.Fatal(err)
	}
	if err := s.Report(context.Background(), "unknown", 7, 40000); !errors.Is(err, ErrExpired) {
		t.Fatal(err)
	}
	if total != 30000 {
		t.Fatal("expired session was recredited")
	}
}

func TestSessionsConcurrentWindowsAndRetries(t *testing.T) {
	var total int64
	s := NewSessions(func(_ context.Context, _, _, delta int64) error { total += delta; return nil })
	a, _ := s.Create(1, 0)
	b, _ := s.Create(1, 0)
	var wg sync.WaitGroup
	for _, id := range []string{a, b} {
		for i := int64(1); i <= 100; i++ {
			wg.Add(1)
			go func(id string, i int64) {
				defer wg.Done()
				if err := s.Report(context.Background(), id, 1, i); err != nil {
					t.Error(err)
				}
			}(id, i)
		}
	}
	wg.Wait()
	if total != 200 {
		t.Fatalf("total=%d", total)
	}
}
