//go:build !windows

package mpv

import (
	"io"
	"net"
	"time"
)

func dialPlatformMPVIPC(path string, timeout time.Duration) (io.ReadWriteCloser, error) {
	return net.DialTimeout("unix", path, timeout)
}
