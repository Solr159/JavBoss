package jav

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"javboss/internal/jav/javdb"
	"javboss/internal/util"
)

type connectivityFunc func(context.Context) (*http.Response, error)

func (connectivityFunc) ConnectivityURL() string { return "https://provider.invalid" }

func (f connectivityFunc) CheckConnectivity(ctx context.Context) (*http.Response, error) {
	return f(ctx)
}

type connectivityBody struct{ closed bool }

func (*connectivityBody) Read([]byte) (int, error) { return 0, io.EOF }
func (b *connectivityBody) Close() error           { b.closed = true; return nil }

func TestConnectivityResults(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		err    error
		want   string
	}{
		{"success", 200, nil, "ok"},
		{"forbidden", 403, nil, "http_error"},
		{"authentication", 401, nil, "http_error"},
		{"missing", 404, nil, "http_error"},
		{"rate limited", 429, nil, "http_error"},
		{"server error", 503, nil, "http_error"},
		{"timeout", 0, &url.Error{Op: "Get", URL: "https://private.invalid", Err: context.DeadlineExceeded}, "timeout"},
		{"canceled", 0, context.Canceled, "canceled"},
		{"dns", 0, &net.DNSError{Err: "lookup failed"}, "dns_error"},
		{"tls", 0, &tls.CertificateVerificationError{Err: errors.New("invalid certificate")}, "tls_error"},
		{"connection", 0, errors.New("proxy credentials must not appear in the result"), "network_error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := &connectivityBody{}
			client := NewClient(map[Provider]any{ProviderJavBus: connectivityFunc(func(ctx context.Context) (*http.Response, error) {
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 20*time.Second {
					t.Fatal("probe must have a bounded deadline")
				}
				return &http.Response{StatusCode: tt.status, Body: body}, tt.err
			})}, nil)
			got, err := client.CheckConnectivity(context.Background(), ProviderJavBus)
			if err != nil || got.Status != tt.want || got.HTTPStatus != tt.status || !body.closed {
				t.Fatalf("result=%+v err=%v body closed=%v", got, err, body.closed)
			}
		})
	}
}

func TestConnectivityProvidersCoverRegistry(t *testing.T) {
	client := NewClient(nil, nil)
	providers := client.ConnectivityProviders()
	if len(providers) != 10 || len(providers) != len(client.providers) {
		t.Fatalf("connectivity must cover all registered providers: %+v", providers)
	}
	for i, provider := range providers {
		if provider.Domain == "" {
			t.Fatalf("provider %s has no connectivity domain", provider.Name)
		}
		if i > 0 && provider.ID <= providers[i-1].ID {
			t.Fatal("provider order must be stable")
		}
	}
	for _, provider := range []Provider{ProviderUnknown, ProviderUser, ProviderManualScrape, Provider(999)} {
		if _, err := client.CheckConnectivity(context.Background(), provider); !errors.Is(err, ErrUnsupportedProvider) {
			t.Fatalf("provider %d: %v", provider, err)
		}
	}
}

func TestConnectivityUsesCurrentProxyAndSignedAPIWithoutCaching(t *testing.T) {
	t.Setenv("JAVBOSS_PROXY_HOST_GATEWAY", "0")
	t.Cleanup(func() { util.SetProxyPort(0) })
	api := javdb.NewAPI(nil, "http://provider.invalid")
	client := NewClient(map[Provider]any{ProviderJavDBAPI: api}, nil)
	if providers := client.ConnectivityProviders(); len(providers) != 1 || providers[0].Domain != "provider.invalid" {
		t.Fatalf("displayed domain must follow the configured API origin: %+v", providers)
	}
	for _, status := range []int{404, 200} {
		calls := 0
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.URL.Host != "provider.invalid" || r.URL.Path != "/api/v2/search" || r.Header.Get("jdsignature") == "" || r.URL.Query().Get("device_uuid") == "" {
				t.Errorf("expected a signed API request through the configured proxy")
			}
			w.WriteHeader(status)
		}))
		t.Cleanup(proxy.Close)
		address := strings.TrimPrefix(proxy.URL, "http://")
		host, portText, err := net.SplitHostPort(address)
		if err != nil {
			t.Fatal(err)
		}
		port, _ := strconv.Atoi(portText)
		util.SetProxy(host, port)
		result, err := client.CheckConnectivity(context.Background(), ProviderJavDBAPI)
		if err != nil || result.HTTPStatus != status || calls != 1 {
			t.Fatalf("status=%d result=%+v calls=%d err=%v", status, result, calls, err)
		}
	}
}

func TestConnectivityHonorsCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := NewClient(nil, nil).CheckConnectivity(ctx, ProviderJavBus)
	if err != nil || result.Status != "canceled" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestConnectivityCacheRetainsResultsAndRefreshesManually(t *testing.T) {
	calls := 0
	client := NewClient(map[Provider]any{ProviderJavBus: connectivityFunc(func(ctx context.Context) (*http.Response, error) {
		calls++
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		status := http.StatusForbidden
		if calls > 1 {
			status = http.StatusOK
		}
		return &http.Response{StatusCode: status, Body: http.NoBody}, nil
	})}, nil)
	if client.ConnectivityProviders()[0].LastResult != nil || calls != 0 {
		t.Fatal("listing unchecked providers must not trigger a check")
	}
	for _, wantStatus := range []int{http.StatusForbidden, http.StatusOK} {
		result, err := client.CheckConnectivity(context.Background(), ProviderJavBus)
		if err != nil || result.HTTPStatus != wantStatus || result.CheckedAt.IsZero() {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		for range 2 {
			cached := client.ConnectivityProviders()[0].LastResult
			if cached == nil || *cached != result {
				t.Fatalf("cached=%+v want=%+v", cached, result)
			}
			cached.Status = "modified copy"
		}
	}
	if calls != 2 {
		t.Fatalf("manual checks must refresh and listing must reuse results: %d requests", calls)
	}
	previous := *client.ConnectivityProviders()[0].LastResult
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = client.CheckConnectivity(ctx, ProviderJavBus)
	if cached := client.ConnectivityProviders()[0].LastResult; cached == nil || *cached != previous {
		t.Fatalf("cancellation replaced the completed result: %+v", cached)
	}
	client.invalidateConnectivityCache()
	if client.ConnectivityProviders()[0].LastResult != nil {
		t.Fatal("invalidated results should be absent")
	}
}

func TestConnectivityCacheDiscardsStaleInFlightResults(t *testing.T) {
	for _, mode := range []string{"newer check", "invalidate", "invalidate and new check"} {
		t.Run(mode, func(t *testing.T) {
			started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			client := NewClient(map[Provider]any{ProviderJavBus: connectivityFunc(func(context.Context) (*http.Response, error) {
				if calls.Add(1) == 1 {
					close(started)
					<-release
					return &http.Response{StatusCode: http.StatusForbidden, Body: http.NoBody}, nil
				}
				return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
			})}, nil)
			go func() {
				defer close(done)
				_, _ = client.CheckConnectivity(context.Background(), ProviderJavBus)
			}()
			<-started
			if mode != "newer check" {
				client.invalidateConnectivityCache()
			}
			if mode != "invalidate" {
				_, _ = client.CheckConnectivity(context.Background(), ProviderJavBus)
			}
			close(release)
			<-done
			cached := client.ConnectivityProviders()[0].LastResult
			if mode == "invalidate" {
				if cached != nil {
					t.Fatalf("old request restored invalidated cache: %+v", cached)
				}
			} else if cached == nil || cached.HTTPStatus != http.StatusOK {
				t.Fatalf("old request replaced the newer result: %+v", cached)
			}
		})
	}
}
