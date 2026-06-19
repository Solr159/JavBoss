package util

import (
	"fmt"
	"javboss/internal/common/logging"
	"os/exec"
)

// OpenFile opens a file with the system default application.
func OpenFile(path string) error {
	if handled, err := openFileDirect(path); handled {
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}
		return nil
	}
	cmd, err := buildOpenCommand(path, false)
	if err != nil {
		return err
	}
	if err := startCommand(cmd, "open file"); err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	return nil
}

// RevealFile opens the containing folder and highlights the file when supported.
func RevealFile(path string) error {
	cmd, err := buildOpenCommand(path, true)
	if err != nil {
		return err
	}
	if err := startCommand(cmd, "reveal file"); err != nil {
		return fmt.Errorf("reveal file: %w", err)
	}
	return nil
}

func startCommand(cmd *exec.Cmd, label string) error {
	logging.Info("%s command: %v", label, cmd.Args)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			logging.Error("%s command exited with error: %v", label, err)
		}
	}()
	return nil
}
