package javdbapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"javboss/internal/jav/scrape/internal/ratelimit"
	"javboss/internal/jav/scrape/metadata"
)

func resetAuthenticationForTest(t *testing.T) {
	t.Helper()
	authenticationFailed.Store(false)
	t.Cleanup(func() { authenticationFailed.Store(false) })
}

func writeAuthTestMovie(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/v2/search" {
		fmt.Fprint(w, `{"success":1,"data":{"movies":[{"id":"m1","number":"ABC-001"}]}}`)
		return
	}
	fmt.Fprint(w, `{"success":1,"data":{"movie":{"number":"ABC-001","origin_title":"Title"}}}`)
}

func TestAuthenticationFailurePausesAllClients(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusUnauthorized} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			resetAuthenticationForTest(t)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(status)
				fmt.Fprint(w, `{"success":0,"action":"JWTVerificationError","data":null}`)
			}))
			defer server.Close()
			first := New(server.Client(), server.URL)
			_, err := first.LookupJavByCode(context.Background(), "ABC-001")
			if !errors.Is(err, errAuthenticationFailed) || errors.Is(err, metadata.ErrNotFound) {
				t.Fatalf("authentication error = %v", err)
			}
			var pending sync.WaitGroup
			for range 10 {
				pending.Add(1)
				go func() {
					defer pending.Done()
					client := New(server.Client(), server.URL)
					_, err := client.LookupStudioURLByCode(context.Background(), "ABC-002")
					if !errors.Is(err, errAuthenticationFailed) {
						t.Errorf("new client bypassed pause: %v", err)
					}
				}()
			}
			pending.Wait()
			if calls.Load() != 1 {
				t.Fatalf("requests = %d, want 1", calls.Load())
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := first.LookupJavByCode(ctx, "ABC-001"); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
		})
	}
}

func TestNonAuthenticationFailuresDoNotPause(t *testing.T) {
	for _, status := range []int{200, 403, 404, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			resetAuthenticationForTest(t)
			calls := 0
			p := newJavDBAPITestProvider(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(status)
				fmt.Fprint(w, `{"success":0,"action":"TemporaryError"}`)
			})
			p.limiter = ratelimit.New(0)
			for range 2 {
				if _, err := p.LookupJavByCode(context.Background(), "ABC-001"); err == nil || errors.Is(err, errAuthenticationFailed) {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			if calls != 2 || authenticationFailed.Load() {
				t.Fatalf("non-auth failure paused requests: calls=%d", calls)
			}
		})
	}
}

func TestInFlightSuccessDoesNotClearAuthenticationFailure(t *testing.T) {
	resetAuthenticationForTest(t)
	p := newJavDBAPITestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/search" {
			pauseAuthentication("JWTVerificationError")
		}
		writeAuthTestMovie(w, r)
	})
	p.limiter = ratelimit.New(0)
	if _, err := p.LookupJavByCode(context.Background(), "ABC-001"); err != nil {
		t.Fatal(err)
	}
	if !authenticationFailed.Load() {
		t.Fatal("in-flight success cleared authentication failure")
	}
}

type limiterWaitContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *limiterWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

func TestQueuedRequestChecksAuthenticationBeforeSending(t *testing.T) {
	resetAuthenticationForTest(t)
	p := newJavDBAPITestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("queued request reached server after authentication failed")
	})
	p.limiter = ratelimit.New(200 * time.Millisecond)
	if err := p.limiter.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx := &limiterWaitContext{Context: context.Background(), waiting: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		_, err := p.request(ctx, "/api/v2/search", nil)
		done <- err
	}()
	select {
	case <-ctx.waiting:
	case <-time.After(time.Second):
		t.Fatal("request did not enter limiter wait")
	}
	pauseAuthentication("JWTVerificationError")
	if err := <-done; !errors.Is(err, errAuthenticationFailed) {
		t.Fatalf("queued request error: %v", err)
	}
}
