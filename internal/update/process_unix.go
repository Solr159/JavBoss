//go:build !windows

package update

import (
	"os/exec"
	"syscall"
)

func detachProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
