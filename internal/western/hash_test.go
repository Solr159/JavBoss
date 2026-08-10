package western

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenSubtitlesHashIsStable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "video.bin")
	data := make([]byte, openSubtitlesHashChunk+17)
	for index := range data {
		data[index] = byte(index)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := OpenSubtitlesHash(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := OpenSubtitlesHash(path)
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first != second {
		t.Fatalf("hash is not stable: %q %q", first, second)
	}
}
