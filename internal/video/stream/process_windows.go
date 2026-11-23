package stream

import (
	"os"

	"github.com/shirou/gopsutil/v4/process"
)

// gopsutil signals by PID on Windows. Keep a separate handle until the
// scheduler releases the child so Windows cannot recycle its PID after Wait.
type processControl struct {
	handle *os.Process
	child  process.Process
}

func openProcessControl(child *os.Process) (*processControl, error) {
	handle, err := os.FindProcess(child.Pid)
	if err != nil {
		return nil, err
	}
	return &processControl{handle: handle, child: process.Process{Pid: int32(child.Pid)}}, nil
}
func (p *processControl) setPaused(pause bool) error {
	if pause {
		return p.child.Suspend()
	}
	return p.child.Resume()
}
func (p *processControl) close() { _ = p.handle.Release() }
