package playback

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestReporterRetriesAndRebasesExpiredSessions(t *testing.T) {
	created := make(chan string, 10)
	requests := make(chan string, 10)
	responses := make(chan error, 10)
	count := 0
	r := NewReporter(func(context.Context) (string, error) {
		count++
		id := fmt.Sprint(count)
		created <- id
		return id, nil
	}, func(ctx context.Context, id string, total int64) error {
		requests <- fmt.Sprintf("%s:%d", id, total)
		select {
		case err := <-responses:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	t.Cleanup(func() { r.cancel(); r.Wait() })
	receive := func(ch <-chan string, want string) {
		t.Helper()
		select {
		case got := <-ch:
			if got != want {
				t.Fatalf("got %s want %s", got, want)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timeout waiting for %s", want)
		}
	}
	receive(created, "1")
	r.Update(1000, false)
	receive(requests, "1:1000")
	responses <- errors.New("response lost after commit")
	r.Update(1000, false)
	receive(requests, "1:1000")
	responses <- nil
	r.Update(2000, false)
	receive(requests, "1:2000")
	responses <- ErrExpired
	receive(created, "2")
	r.Update(2500, true)
	receive(requests, "2:500")
	responses <- nil
	r.Wait()
}
