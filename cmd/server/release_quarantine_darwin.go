package main

import (
	"errors"
	"io/fs"
	"path/filepath"

	"javboss/internal/common/logging"

	"golang.org/x/sys/unix"
)

const quarantineAttribute = "com.apple.quarantine"

// Clear only the download quarantine attribute on installed release files.
// WalkDir and Lremovexattr do not follow symlinks outside the installation.
func clearReleaseQuarantine(baseDir string) error {
	return filepath.WalkDir(baseDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			logging.Error("clear download quarantine failed on %s, skipping: %v", path, walkErr)
			return nil
		}
		// Runtime data can be large and is not part of the downloaded release.
		if entry.IsDir() && filepath.Dir(path) == baseDir && (entry.Name() == "data" || entry.Name() == "logs") {
			return filepath.SkipDir
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if err := unix.Lremovexattr(path, quarantineAttribute); err != nil && !errors.Is(err, unix.ENOATTR) {
			logging.Error("clear download quarantine failed on %s, skipping: %v", path, err)
		}
		return nil
	})
}
