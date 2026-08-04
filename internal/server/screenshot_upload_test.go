package server

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveUploadedScreenshotAcceptsImageAndReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	name := "mpv_00-00-01.250.jpg"
	first := encodeTestScreenshot(t, "jpeg", color.RGBA{R: 255, A: 255})
	info, err := saveUploadedScreenshot(dir, name, bytes.NewReader(first))
	if err != nil {
		t.Fatalf("save first screenshot: %v", err)
	}
	if info.Size() != int64(len(first)) {
		t.Fatalf("saved size = %d, want %d", info.Size(), len(first))
	}

	second := encodeTestScreenshot(t, "jpeg", color.RGBA{B: 255, A: 255})
	if _, err := saveUploadedScreenshot(dir, name, bytes.NewReader(second)); err != nil {
		t.Fatalf("replace screenshot: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read saved screenshot: %v", err)
	}
	if !bytes.Equal(got, second) {
		t.Fatal("saved screenshot was not replaced with the second upload")
	}
}

func TestSaveUploadedScreenshotRejectsExtensionMismatch(t *testing.T) {
	pngData := encodeTestScreenshot(t, "png", color.RGBA{G: 255, A: 255})
	_, err := saveUploadedScreenshot(t.TempDir(), "mpv_00-00-02.jpg", bytes.NewReader(pngData))
	if !errors.Is(err, errScreenshotUploadType) {
		t.Fatalf("error = %v, want errScreenshotUploadType", err)
	}
}

func TestSaveUploadedScreenshotEnforcesSizeLimit(t *testing.T) {
	_, err := saveUploadedScreenshotWithLimit(
		t.TempDir(),
		"mpv_00-00-03.jpg",
		bytes.NewReader([]byte("12345678901")),
		10,
	)
	if !errors.Is(err, errScreenshotUploadTooLarge) {
		t.Fatalf("error = %v, want errScreenshotUploadTooLarge", err)
	}
}

func TestScreenshotUploadTypeMatchesWebP(t *testing.T) {
	header := []byte{'R', 'I', 'F', 'F', 4, 0, 0, 0, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' '}
	if !screenshotUploadTypeMatches("mpv_00-00-04.webp", header) {
		t.Fatal("valid WebP header was rejected")
	}
}

func encodeTestScreenshot(t *testing.T, format string, fill color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, fill)
		}
	}
	var output bytes.Buffer
	var err error
	switch format {
	case "jpeg":
		err = jpeg.Encode(&output, img, &jpeg.Options{Quality: 90})
	case "png":
		err = png.Encode(&output, img)
	default:
		t.Fatalf("unsupported test image format %q", format)
	}
	if err != nil {
		t.Fatalf("encode test screenshot: %v", err)
	}
	return output.Bytes()
}
