//go:build !windows

package mpv

import "os/exec"

func startPlatformPlayer(cmd *exec.Cmd) (wait func() error, release func(), err error) {
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return cmd.Wait, func() {}, nil
}
