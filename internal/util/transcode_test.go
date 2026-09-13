package util

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestValidateTranscodedVideoQuickChecks(t *testing.T) {
	binary, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err := ResolveFFprobePath(); err != nil {
		t.Skip("ffprobe unavailable")
	}
	root := t.TempDir()
	for _, codec := range []string{"libx264", "mpeg4"} {
		cmd := exec.Command(binary, "-hide_banner", "-loglevel", "error", "-nostdin", "-n",
			"-f", "lavfi", "-i", "color=c=black:s=96x64:r=12", "-t", "1",
			"-c:v", codec, "-pix_fmt", "yuv420p", filepath.Join(root, codec+".mp4"))
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("generate sample: %v %s", err, output)
		}
	}
	for name, data := range map[string][]byte{"empty.mp4": nil, "invalid.mp4": []byte("not a video")} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name, file, sourceAudio string
		sourceDuration          float64
		wantError               bool
	}{
		{"valid silent output", "libx264.mp4", "", 1, false},
		{"duration rounding", "libx264.mp4", "", 1.1, false},
		{"missing audio", "libx264.mp4", "aac", 1, true},
		{"shortened output", "libx264.mp4", "", 10, true},
		{"incompatible codec", "mpeg4.mp4", "", 1, true},
		{"empty output", "empty.mp4", "", 1, true},
		{"invalid output", "invalid.mp4", "", 1, true},
		{"missing output", "missing.mp4", "", 1, true},
		{"directory output", ".", "", 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, err := ValidateTranscodedVideo(t.Context(), filepath.Join(root, tt.file), &VideoMetadata{
				AudioCodec: tt.sourceAudio, DurationSeconds: tt.sourceDuration,
			})
			if (err != nil) != tt.wantError {
				t.Fatalf("validation error = %v, want error = %t", err, tt.wantError)
			}
			if err == nil && (meta == nil || !BrowserCompatibleVideo(meta)) {
				t.Fatalf("invalid validated metadata: %+v", meta)
			}
		})
	}
}

func TestBrowserCompatibleVideoChecksActualContainerAndPixelFormat(t *testing.T) {
	tests := []struct {
		name, container, format, video, audio, pixels string
		compatible                                    bool
	}{
		{"mp4", "mp4", "mov", "h264", "aac", "yuv420p", true},
		{"silent", "mp4", "mov", "h264", "", "yuv420p", true},
		{"webm", "webm", "matroska", "vp9", "opus", "yuv420p", true},
		{"10 bit h264", "mp4", "mov", "h264", "aac", "yuv420p10le", false},
		{"disguised transport stream", "mp4", "mpegts", "h264", "aac", "yuv420p", false},
		{"hevc", "mp4", "mov", "hevc", "aac", "yuv420p", false},
		{"ac3", "mp4", "mov", "h264", "ac3", "yuv420p", false},
		{"mkv", "mkv", "matroska", "h264", "aac", "yuv420p", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			meta := &VideoMetadata{Container: test.container, FormatName: test.format, VideoCodec: test.video, AudioCodec: test.audio, PixelFormat: test.pixels}
			if got := BrowserCompatibleVideo(meta); got != test.compatible {
				t.Fatalf("compatible=%v want %v", got, test.compatible)
			}
		})
	}
}
