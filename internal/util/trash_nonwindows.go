//go:build !windows

package util

import "os"

func moveFileToTrash(path string) error {
	return os.Remove(path)
}
