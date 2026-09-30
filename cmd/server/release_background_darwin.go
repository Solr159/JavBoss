package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"javboss/internal/runtimeconfig"
)

const backgroundChildEnv = "JAVBOSS_BACKGROUND_CHILD"

func startReleaseInBackground(baseDir string) (bool, error) {
	if os.Getenv(backgroundChildEnv) == "1" {
		// Consume the marker so external tools launched by JavBoss do not inherit it.
		_ = os.Unsetenv(backgroundChildEnv)
		signal.Ignore(syscall.SIGHUP)
		return false, nil
	}
	if buildMode != "release" || runtimeconfig.ContainerMode() {
		return false, nil
	}
	// Attempt cleanup before re-exec, including bundled tools. Individual failures
	// are logged to a file and skipped.
	_ = clearReleaseQuarantine(baseDir)
	executable, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("resolve executable: %w", err)
	}
	pid, err := launchDetachedRelease(baseDir, executable, os.Args[1:])
	if err != nil {
		return false, err
	}
	if pid != 0 {
		fmt.Println("JavBoss 已在后台运行，可以关闭此终端窗口。")
		fmt.Println("点击屏幕顶部菜单栏中的 JavBoss 图标，可打开页面或退出程序。")
	}
	return true, nil
}

func launchDetachedRelease(baseDir, executable string, args []string) (int, error) {
	logsDir := filepath.Join(baseDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return 0, fmt.Errorf("create logs directory: %w", err)
	}
	output, err := os.OpenFile(filepath.Join(logsDir, "launcher.log"), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open launcher log: %w", err)
	}
	defer output.Close()
	logStart, err := output.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("seek launcher log: %w", err)
	}
	input, err := os.Open(os.DevNull)
	if err != nil {
		return 0, fmt.Errorf("open background stdin: %w", err)
	}
	defer input.Close()

	// Re-exec before initializing AppKit, databases or workers. A new session and
	// redirected descriptors detach the child from the launching terminal.
	cmd := exec.Command(executable, args...)
	cmd.Dir = baseDir
	cmd.Env = append(os.Environ(), backgroundChildEnv+"=1")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = input, output, output
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("launch background process: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case err := <-done:
		// Surface duplicate-instance messages and early failures without waiting
		// for terminal input. The regular application log contains startup errors.
		_, _ = io.Copy(os.Stderr, io.NewSectionReader(output, logStart, 4096))
		if err != nil {
			return 0, fmt.Errorf("background process exited: %w", err)
		}
		return 0, nil
	case <-timer.C:
		return cmd.Process.Pid, nil
	}
}
