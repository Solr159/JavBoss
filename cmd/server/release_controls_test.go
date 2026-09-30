package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestServeWithControlsWaitsForShutdown(t *testing.T) {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	draining := make(chan struct{})
	finishShutdown := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- serveWithControls(stop, func() error {
			<-ctx.Done()
			close(draining)
			<-finishShutdown
			return nil
		}, func() {
			// Closing the UI must stop the server, even without a signal.
		})
	}()
	defer close(finishShutdown)
	select {
	case <-draining:
	case <-time.After(5 * time.Second):
		t.Fatal("closing controls did not stop the server")
	}
	select {
	case err := <-done:
		t.Fatalf("returned before shutdown completed: %v", err)
	default:
	}
	finishShutdown <- struct{}{}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("did not return after shutdown completed")
	}
}

func TestServeWithControlsStopsUIWhenServerReturns(t *testing.T) {
	serveErr := errors.New("listener failed")
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "server failure", err: serveErr},
		{name: "normal server exit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, stop := context.WithCancel(context.Background())
			defer stop()
			err := serveWithControls(stop, func() error {
				return test.err
			}, func() {
				select {
				case <-ctx.Done():
				case <-time.After(5 * time.Second):
					t.Error("server exit did not stop controls")
				}
			})
			if !errors.Is(err, test.err) {
				t.Fatalf("got %v, want %v", err, test.err)
			}
		})
	}
}
