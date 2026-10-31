package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func startDetached(executable string, args []string, dir, readyFile string) (*exec.Cmd, error) {
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}
	log, err := os.OpenFile(filepath.Join(logDir, "update.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	defer log.Close()
	input, err := os.Open(os.DevNull)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	cmd := exec.Command(executable, args...)
	cmd.Dir = dir
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, readyEnvironment+"=") && !strings.HasPrefix(env, "JAVBOSS_BACKGROUND_CHILD=") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	if readyFile != "" {
		cmd.Env = append(cmd.Env, readyEnvironment+"="+readyFile)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = input, log, log
	detachProcess(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}
