//go:build !darwin

package main

func startReleaseInBackground(baseDir string) (bool, error) {
	return false, nil
}
