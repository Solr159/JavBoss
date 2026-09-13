package util

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBrowserCompatibleMKVChecksAllProbedAudioTracks(t *testing.T) {
	for _, codec := range []string{"aac", "mp3", "dts", "ac3", ""} {
		t.Run(codec, func(t *testing.T) {
			data := fmt.Sprintf(`{"streams":[
				{"codec_type":"video","codec_name":"h264","pix_fmt":"yuv420p"},
				{"codec_type":"audio","codec_name":"aac"},
				{"codec_type":"audio","codec_name":%q}
			],"format":{"format_name":"matroska,webm"}}`, codec)
			meta, err := parseFFprobeOutput([]byte(data), "movie.mkv")
			if err != nil {
				t.Fatal(err)
			}
			want := codec == "aac" || codec == "mp3"
			if got := BrowserCompatibleVideo(meta); got != want {
				t.Fatalf("compatible = %t, want %t for second audio codec %q", got, want, codec)
			}
		})
	}
}

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
		{"mkv", "mkv", "matroska", "h264", "aac", "yuv420p", true},
		{"silent mkv", "mkv", "matroska", "h264", "", "yuv420p", true},
		{"mp3 mkv", "mkv", "matroska", "h264", "mp3", "yuv420p", true},
		{"hevc mkv", "mkv", "matroska", "hevc", "aac", "yuv420p", false},
		{"10 bit mkv", "mkv", "matroska", "h264", "aac", "yuv420p10le", false},
		{"dts mkv", "mkv", "matroska", "h264", "dts", "yuv420p", false},
		{"disguised mkv", "mkv", "mpegts", "h264", "aac", "yuv420p", false},
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
