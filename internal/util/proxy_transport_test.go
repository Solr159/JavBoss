package util

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newConnectProxy(t *testing.T) (*url.URL, *atomic.Int32) {
	t.Helper()
	var connections atomic.Int32
	var tunnels sync.WaitGroup
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
			return
		}
		upstream, err := net.DialTimeout("tcp", r.Host, time.Second)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		conn, reader, err := w.(http.Hijacker).Hijack()
		if err != nil {
			upstream.Close()
			return
		}
		connections.Add(1)
		_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
		tunnels.Add(2)
		go func() {
			defer tunnels.Done()
			defer conn.Close()
			defer upstream.Close()
			_, _ = io.Copy(upstream, reader)
		}()
		go func() {
			defer tunnels.Done()
			defer conn.Close()
			defer upstream.Close()
			_, _ = io.Copy(conn, upstream)
		}()
	}))
	t.Cleanup(func() {
		proxy.Close()
		tunnels.Wait()
	})
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	return proxyURL, &connections
}

func TestHTTP2UsesCurrentProxyWithExistingConnections(t *testing.T) {
	t.Setenv("JAVBOSS_PROXY_HOST_GATEWAY", "0")
	t.Cleanup(func() { SetProxyPort(0) })
	release := make(chan struct{})
	var releaseOnce sync.Once
	finish := func() { releaseOnce.Do(func() { close(release) }) }
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hold" {
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-release
		}
		_, _ = io.WriteString(w, "ok")
	}))
	origin.EnableHTTP2 = true
	origin.StartTLS()
	t.Cleanup(origin.Close)
	proxyA, callsA := newConnectProxy(t)
	proxyB, callsB := newConnectProxy(t)
	client := NewHTTPClientWithTransport(3*time.Second, func(transport *http.Transport) {
		transport.ForceAttemptHTTP2 = true
		transport.IdleConnTimeout = time.Second
		transport.TLSClientConfig = origin.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
		transport.Proxy = configuredProxy(func(*http.Request) (*url.URL, error) { return proxyA, nil })
	})
	t.Cleanup(client.CloseIdleConnections)
	t.Cleanup(finish)
	request := func(path string, wantReuse bool) *http.Response {
		t.Helper()
		reused := false
		ctx := httptrace.WithClientTrace(context.Background(), &httptrace.ClientTrace{
			GotConn: func(info httptrace.GotConnInfo) { reused = info.Reused },
		})
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, origin.URL+path, nil)
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if response.ProtoMajor != 2 || response.StatusCode != http.StatusOK || reused != wantReuse {
			response.Body.Close()
			t.Fatalf("protocol=%s status=%d reused=%v wantReuse=%v", response.Proto, response.StatusCode, reused, wantReuse)
		}
		return response
	}
	drain := func(response *http.Response) {
		t.Helper()
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil || string(body) != "ok" {
			t.Fatalf("body=%q err=%v", body, err)
		}
	}
	SetProxySettings(ProxyModeManual, proxyA.Hostname(), proxyA.Port())
	drain(request("/", false))
	// Saving identical settings should preserve connection reuse.
	SetProxySettings(ProxyModeManual, proxyA.Hostname(), proxyA.Port())
	drain(request("/", true))
	held := request("/hold", true)
	t.Cleanup(func() { held.Body.Close() })

	unused, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	badPort := unused.Addr().(*net.TCPAddr).Port
	unused.Close()
	SetProxySettings(ProxyModeManual, "127.0.0.1", strconv.Itoa(badPort))
	response, err := client.Get(origin.URL)
	if response != nil {
		response.Body.Close()
	}
	if err == nil {
		t.Fatal("unreachable proxy succeeded through the previous HTTP/2 connection")
	}
	for _, step := range []struct {
		mode         string
		proxy        *url.URL
		wantA, wantB int32
	}{
		{ProxyModeManual, proxyB, 1, 1},
		{ProxyModeDirect, proxyB, 1, 1},
		{ProxyModeAuto, proxyB, 2, 1},
	} {
		SetProxySettings(step.mode, step.proxy.Hostname(), step.proxy.Port())
		drain(request("/", false))
		if callsA.Load() != step.wantA || callsB.Load() != step.wantB {
			t.Fatalf("mode=%s proxyA=%d proxyB=%d", step.mode, callsA.Load(), callsB.Load())
		}
	}
	// Switching proxies must not interrupt a response already in progress.
	finish()
	drain(held)
}

func TestProxyVersionRebuildsEachClientLazily(t *testing.T) {
	t.Setenv("JAVBOSS_PROXY_HOST_GATEWAY", "0")
	t.Cleanup(func() { SetProxyPort(0) })
	SetProxySettings(ProxyModeDirect, "", "")
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(origin.Close)
	address, _ := url.Parse(origin.URL)
	var builds [2]atomic.Int32
	var clients [2]*http.Client
	for i := range clients {
		clients[i] = NewHTTPClientWithTransport(time.Second, func(transport *http.Transport) {
			builds[i].Add(1)
			transport.Proxy = configuredProxy(nil)
		})
		t.Cleanup(clients[i].CloseIdleConnections)
	}
	requestAll := func(wantBuilds int32) {
		t.Helper()
		var requests sync.WaitGroup
		for _, client := range clients {
			for range 8 {
				requests.Add(1)
				go func() {
					defer requests.Done()
					response, err := client.Get(origin.URL)
					if err != nil {
						t.Error(err)
						return
					}
					_, _ = io.Copy(io.Discard, response.Body)
					response.Body.Close()
				}()
			}
		}
		requests.Wait()
		for i := range clients {
			if got := builds[i].Load(); got != wantBuilds {
				t.Fatalf("client %d built %d times, want %d", i, got, wantBuilds)
			}
		}
	}
	requestAll(1)
	SetProxySettings(ProxyModeManual, address.Hostname(), address.Port())
	for i := range clients {
		if builds[i].Load() != 1 {
			t.Fatal("proxy update rebuilt a client before its next request")
		}
	}
	requestAll(2)
	SetProxySettings(ProxyModeManual, address.Hostname(), address.Port())
	requestAll(2)
	SetProxySettings(ProxyModeAuto, "", "")
	requestAll(3)
}
