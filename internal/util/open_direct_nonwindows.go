//go:build !windows

package util

func openFileDirect(path string) (bool, error) {
	return false, nil
}
