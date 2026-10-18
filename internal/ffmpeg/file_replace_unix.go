//go:build !windows

package ffmpeg

import "os"

func replaceFileAtomic(sourcePath string, targetPath string) error {
	return os.Rename(sourcePath, targetPath)
}
