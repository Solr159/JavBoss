package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// updateLANAccess is initialized before serving requests; nil in container mode.
var updateLANAccess func(enabled bool, save func() error) error

// ServeHTTP serves native TCP listeners and optionally enables live address changes.
// Switching listeners leaves established connections on the same HTTP server.
func ServeHTTP(ctx context.Context, srv *http.Server, listener net.Listener, enabled, allowUpdates bool) error {
	return serveHTTP(ctx, srv, listener, enabled, allowUpdates, net.Listen)
}

func serveHTTP(ctx context.Context, srv *http.Server, listener net.Listener, enabled, allowUpdates bool, listen func(string, string) (net.Listener, error)) error {
	var mu sync.Mutex
	stopped := false
	serveErrors := make(chan error, 1)
	reportError := func(err error) {
		select {
		case serveErrors <- err:
		default:
		}
	}
	start := func(current net.Listener) {
		go func() {
			err := srv.Serve(current)
			mu.Lock()
			defer mu.Unlock()
			// Closing a replaced listener is an expected part of switching addresses.
			if current == listener && !stopped {
				reportError(err)
			}
		}()
	}
	updateLANAccess = nil
	if allowUpdates {
		updateLANAccess = func(nextEnabled bool, save func() error) error {
			mu.Lock()
			defer mu.Unlock()
			if stopped || ctx.Err() != nil {
				return http.ErrServerClosed
			}
			if nextEnabled == enabled {
				return save()
			}
			previousAddress := listener.Addr().String()
			_, port, err := net.SplitHostPort(previousAddress)
			if err != nil {
				return err
			}
			host := "127.0.0.1"
			if nextEnabled {
				host = "0.0.0.0"
			}
			_ = listener.Close()
			next, err := listen("tcp", net.JoinHostPort(host, port))
			if err == nil {
				err = save()
				if err == nil {
					listener = next
					enabled = nextEnabled
					start(next)
					return nil
				}
				_ = next.Close()
			}
			restored, restoreErr := listen("tcp", previousAddress)
			if restoreErr != nil {
				err = errors.Join(fmt.Errorf("apply LAN access: %w", err), fmt.Errorf("restore listener: %w", restoreErr))
				reportError(err)
				return err
			}
			listener = restored
			start(restored)
			return fmt.Errorf("apply LAN access: %w", err)
		}
	}
	start(listener)
	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-serveErrors:
	}
	mu.Lock()
	stopped = true
	_ = listener.Close()
	mu.Unlock()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close()
		return errors.Join(serveErr, err)
	}
	return serveErr
}
