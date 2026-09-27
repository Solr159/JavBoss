package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func startTestHTTPServer(t *testing.T, listen func(string, string) (net.Listener, error)) (*http.Client, string, context.CancelFunc) {
	t.Helper()
	previousUpdate := updateLANAccess
	t.Cleanup(func() { updateLANAccess = previousUpdate })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	var once sync.Once
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(ready) })
		_, _ = io.WriteString(w, "ok")
	})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveHTTP(ctx, srv, listener, false, true, listen) }()
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 2 * time.Second}
	t.Cleanup(func() {
		client.CloseIdleConnections()
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("serve: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server did not stop")
		}
	})
	url := "http://" + listener.Addr().String()
	// Wait for the first request before using the registered update callback.
	checkHTTPRequest(t, client, url)
	<-ready
	return client, url, cancel
}

func checkHTTPRequest(t *testing.T, client *http.Client, url string) {
	t.Helper()
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("status=%d body=%s err=%v", response.StatusCode, body, err)
	}
}

func TestHTTPServerSwitchesNativeListeners(t *testing.T) {
	addresses := make(chan string, 8)
	client, url, _ := startTestHTTPServer(t, func(network, address string) (net.Listener, error) {
		addresses <- address
		return net.Listen(network, address)
	})
	for _, enabled := range []bool{true, false, true, false} {
		saves := 0
		if err := updateLANAccess(enabled, func() error { saves++; return nil }); err != nil {
			t.Fatal(err)
		}
		address := <-addresses
		host, _, _ := net.SplitHostPort(address)
		if (host == "0.0.0.0") != enabled || saves != 1 {
			t.Fatalf("enabled=%v address=%s saves=%d", enabled, address, saves)
		}
		checkHTTPRequest(t, client, url) // Existing keep-alive connection.
		client.CloseIdleConnections()
		checkHTTPRequest(t, client, url) // New connection on the original port.
	}
	if err := updateLANAccess(false, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(addresses) != 0 {
		t.Fatal("unchanged setting rebound the listener")
	}
}

func TestHTTPServerRollsBackFailedUpdates(t *testing.T) {
	for _, failure := range []string{"bind", "save"} {
		t.Run(failure, func(t *testing.T) {
			client, url, _ := startTestHTTPServer(t, func(network, address string) (net.Listener, error) {
				host, _, _ := net.SplitHostPort(address)
				if failure == "bind" && host == "0.0.0.0" {
					return nil, errors.New("port in use")
				}
				return net.Listen(network, address)
			})
			saved := false
			err := updateLANAccess(true, func() error {
				saved = true
				return errors.New("database unavailable")
			})
			if err == nil || saved != (failure == "save") {
				t.Fatalf("err=%v saved=%v", err, saved)
			}
			client.CloseIdleConnections()
			checkHTTPRequest(t, client, url)
		})
	}
}

func TestHTTPServerRejectsUpdatesAfterCancellation(t *testing.T) {
	_, _, cancel := startTestHTTPServer(t, net.Listen)
	cancel()
	saved := false
	err := updateLANAccess(true, func() error { saved = true; return nil })
	if !errors.Is(err, http.ErrServerClosed) || saved {
		t.Fatalf("err=%v saved=%v", err, saved)
	}
}
