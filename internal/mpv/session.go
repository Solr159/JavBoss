package mpv

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	sessionDirOnce sync.Once
	sessionDirPath string
	sessionDirErr  error
)

func sessionDir() (string, error) {
	sessionDirOnce.Do(func() {
		dir, err := os.MkdirTemp("", fmt.Sprintf("javboss-mpv-%d-", os.Getpid()))
		if err != nil {
			sessionDirErr = fmt.Errorf("create mpv session dir: %w", err)
			return
		}
		sessionDirPath = dir
	})
	return sessionDirPath, sessionDirErr
}

func sessionPath(elem ...string) (string, error) {
	dir, err := sessionDir()
	if err != nil {
		return "", err
	}
	parts := append([]string{dir}, elem...)
	return filepath.Join(parts...), nil
}
