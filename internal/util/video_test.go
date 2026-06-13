package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsVideoRecognizesRMVBRealMediaSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.rmvb")
	if err := os.WriteFile(path, append([]byte(".RMF\x00\x00\x00\x12"), make([]byte, 32)...), 0o644); err != nil {
		t.Fatalf("write rmvb fixture: %v", err)
	}

	if !IsVideo(path) {
		t.Fatal("IsVideo should accept rmvb files with a RealMedia signature")
	}
}

func TestIsVideoRejectsRMVBWithoutRealMediaSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.rmvb")
	if err := os.WriteFile(path, []byte("not a realmedia file"), 0o644); err != nil {
		t.Fatalf("write rmvb fixture: %v", err)
	}

	if IsVideo(path) {
		t.Fatal("IsVideo should reject rmvb files without a RealMedia signature")
	}
}

func TestDetectContainerRecognizesRMVBExtension(t *testing.T) {
	if got := detectContainer("rm", "/videos/sample.rmvb"); got != "rmvb" {
		t.Fatalf("detectContainer() = %q, want %q", got, "rmvb")
	}
}
