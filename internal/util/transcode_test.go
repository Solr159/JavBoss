package util

import "testing"

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
