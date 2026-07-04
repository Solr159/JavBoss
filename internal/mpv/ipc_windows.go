//go:build windows

package mpv

import (
	"fmt"
	"io"
	"os"
	"time"
)

func dialPlatformMPVIPC(path string, timeout time.Duration) (io.ReadWriteCloser, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		conn, err := os.OpenFile(path, os.O_RDWR, 0)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("dial mpv ipc pipe: %w", lastErr)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
