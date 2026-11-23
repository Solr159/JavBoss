//go:build !windows

package stream

import (
	"os"
	"syscall"
)

// Use the os.Process returned by Start so signaling is synchronized with Wait.
type processControl struct{ child *os.Process }

func openProcessControl(child *os.Process) (*processControl, error) {
	return &processControl{child}, nil
}
func (p *processControl) setPaused(pause bool) error {
	if pause {
		return p.child.Signal(syscall.SIGSTOP)
	}
	return p.child.Signal(syscall.SIGCONT)
}
func (p *processControl) close() {}
