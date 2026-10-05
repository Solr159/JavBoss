package util

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOpenPlaylistPreservesPathsAndOrder(t *testing.T) {
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "中文 #1 & 100%.mp4"), filepath.Join(dir, "second.mp4")}
	var opened string
	err := openPlaylist(paths, dir, func(path string) error {
		opened = path
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		if len(lines) != 3 || lines[0] != "#EXTM3U" {
			t.Fatalf("invalid playlist: %q", content)
		}
		for i, line := range lines[1:] {
			if runtime.GOOS == "windows" {
				if line != paths[i] {
					t.Fatalf("incorrect native path: %q, want %q", line, paths[i])
				}
				continue
			}
			parsed, err := url.Parse(line)
			if err != nil || parsed.Scheme != "file" || parsed.Fragment != "" {
				t.Fatalf("invalid file URL: %q, %v", line, err)
			}
			if strings.TrimPrefix(parsed.Path, "/") != strings.TrimPrefix(filepath.ToSlash(paths[i]), "/") {
				t.Fatalf("incorrect path: %s, want %s", parsed.Path, paths[i])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(opened); err != nil {
		t.Fatalf("playlist removed before asynchronous player could read it: %v", err)
	}
}

func TestPlaylistEntryPath(t *testing.T) {
	for _, tc := range []struct {
		name, goos, path, want string
	}{
		{
			name: "Windows Unicode drive path", goos: "windows",
			path: `F:\Videos\日本語 中文\part 1.mp4`,
			want: `F:\Videos\日本語 中文\part 1.mp4`,
		},
		{
			name: "Windows UNC path and literal URL characters", goos: "windows",
			path: `\\nas\share\中文 #1 & 100% %20.mp4`,
			want: `\\nas\share\中文 #1 & 100% %20.mp4`,
		},
		{
			name: "Linux special characters remain encoded", goos: "linux",
			path: "/videos/part #1 & 100%.mp4",
			want: "file:///videos/part%20%231%20&%20100%25.mp4",
		},
		{
			name: "macOS newline stays inside one entry", goos: "darwin",
			path: "/videos/part\n1.mp4",
			want: "file:///videos/part%0A1.mp4",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := playlistEntryPath(tc.path, tc.goos); got != tc.want {
				t.Fatalf("playlistEntryPath() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestOpenPlaylistCleansUpOnLauncherFailure(t *testing.T) {
	var opened string
	failure := errors.New("launcher failed")
	err := openPlaylist([]string{filepath.Join(t.TempDir(), "movie.mp4")}, t.TempDir(), func(path string) error {
		opened = path
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("expected launcher error, got %v", err)
	}
	if _, err := os.Stat(opened); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed playlist was not cleaned up: %v", err)
	}
}
