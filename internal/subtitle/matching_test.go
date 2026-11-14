package subtitle

import "testing"

func TestSplitVersions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		split bool
	}{
		{"ABC-123.mp4", false}, {"abc123.zh-CN.srt", false}, {"ABC-123-1.mp4", true},
		{"ABC-123.2.avsubtitles-zh", true}, {"ABC-123 part2.mp4", true}, {"ABC-123-CD1.mp4", true},
		{"ABC-123.1080p.mp4", false}, {"ABC-123 2026.mp4", false},
	} {
		if got := IsSplit("ABC-123", tc.name); got != tc.split {
			t.Errorf("%s = %v", tc.name, got)
		}
	}
}
