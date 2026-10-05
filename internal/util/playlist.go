package util

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OpenPlaylist hands one UTF-8 M3U playlist to the system's associated player.
// Keep the file available after OpenFile returns: desktop launchers are asynchronous.
func OpenPlaylist(paths []string) error {
	return openPlaylist(paths, os.TempDir(), OpenFile)
}

func openPlaylist(paths []string, tempDir string, open func(string) error) error {
	if len(paths) == 0 {
		return fmt.Errorf("empty playlist")
	}
	var content strings.Builder
	content.WriteString("#EXTM3U\n")
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve playlist path: %w", err)
		}
		filePath := filepath.ToSlash(absolute)
		if !strings.HasPrefix(filePath, "/") {
			filePath = "/" + filePath
		}
		content.WriteString((&url.URL{Scheme: "file", Path: filePath}).String())
		content.WriteByte('\n')
	}
	dir := filepath.Join(tempDir, "javboss-playlists")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create playlist directory: %w", err)
	}
	// Bound the cache without removing a playlist a recently launched player may read.
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "playlist-") && strings.HasSuffix(entry.Name(), ".m3u8") {
			if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > 7*24*time.Hour {
				_ = os.Remove(filepath.Join(dir, entry.Name()))
			}
		}
	}
	file, err := os.CreateTemp(dir, "playlist-*.m3u8")
	if err != nil {
		return fmt.Errorf("create playlist: %w", err)
	}
	_, writeErr := file.WriteString(content.String())
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(file.Name())
		if writeErr != nil {
			return fmt.Errorf("write playlist: %w", writeErr)
		}
		return fmt.Errorf("close playlist: %w", closeErr)
	}
	if err := open(file.Name()); err != nil {
		_ = os.Remove(file.Name())
		return fmt.Errorf("open playlist: %w", err)
	}
	return nil
}
