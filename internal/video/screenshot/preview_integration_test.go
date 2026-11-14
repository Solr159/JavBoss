package screenshot

import (
	"bytes"
	"context"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestFFmpegPreviewSeeksPrecisely(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is required")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	for _, offset := range []int{0, 12} {
		t.Run(strconv.Itoa(offset), func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source.mp4")
			args := []string{"-v", "error", "-f", "lavfi", "-i", "testsrc2=size=160x90:rate=10:duration=24", "-c:v", "mpeg4", "-g", "60", "-bf", "0", "-output_ts_offset", strconv.Itoa(offset), source}
			if out, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
				t.Fatalf("fixture: %v: %s", err, out)
			}
			capture := func(second float64) []byte {
				t.Helper()
				path := filepath.Join(dir, "frame.jpg")
				if err := runFFmpegScreenshotArgs(ctx, ffmpeg, buildFFmpegScreenshotArgs(second, path, source, true), path); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				img, err := jpeg.Decode(bytes.NewReader(data))
				if err != nil || img.Bounds().Dx() != 320 || img.Bounds().Dy() != 180 {
					t.Fatalf("invalid preview: %v", err)
				}
				return data
			}
			exact18, exact20 := capture(18), capture(20)
			if bytes.Equal(exact18, exact20) {
				t.Fatal("precise seek returned the old keyframe")
			}
			// Decode from the beginning as an independent reference for frame 20.
			reference := filepath.Join(dir, "reference.jpg")
			args = []string{"-v", "error", "-filter_threads", "1", "-i", source, "-ss", "20", "-map", "0:v:0", "-frames:v", "1", "-q:v", "2", "-vf", "scale=320:180:force_original_aspect_ratio=decrease", "-threads", "1", "-vsync", "0", reference}
			if out, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
				t.Fatalf("reference: %v: %s", err, out)
			}
			data, err := os.ReadFile(reference)
			if err != nil || !bytes.Equal(exact20, data) {
				t.Fatalf("seek image differs from decoded reference: %v", err)
			}
		})
	}
}
