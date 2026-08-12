//go:build !windows

package manager

import "os"

func replaceFileAtomic(sourcePath string, targetPath string) error {
	return os.Rename(sourcePath, targetPath)
}
