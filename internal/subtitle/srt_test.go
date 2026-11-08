package subtitle

import (
	"strings"
	"testing"
)

func TestNormalizeSRTAndWebVTT(t *testing.T) {
	raw := []byte("\ufeff9\r\n00:00:01,000 --> 00:00:02,000\r\n<b>Hello</b> &amp; {\\an8}world\r\n")
	got, err := NormalizeSRT(raw)
	if err != nil || string(got) != "1\n00:00:01,000 --> 00:00:02,000\nHello & world\n\n" {
		t.Fatalf("%q %v", got, err)
	}
	vtt, err := WebVTT(got)
	if err != nil || !strings.Contains(string(vtt), "00:00:01.000 --> 00:00:02.000\nHello &amp; world") || !strings.HasPrefix(string(vtt), "WEBVTT\n") {
		t.Fatalf("%q %v", vtt, err)
	}
}

func TestRejectInvalidSubtitles(t *testing.T) {
	for _, raw := range []string{"", "<html>denied</html>", "1\n00:00:02,000 --> 00:00:01,000\ntext", "1\n00:70:00,000 --> 00:80:00,000\ntext", "bad cue", strings.Repeat("x", MaxBytes+1), "\xff", sampleSRT + "\xff", sampleSRT + "\x00", "790\n01:55:55,560 --> 01:\n"} {
		if _, err := NormalizeSRT([]byte(raw)); err == nil {
			t.Fatalf("accepted invalid subtitle length %d", len(raw))
		}
	}
}

func TestMalformedCuesAreOmitted(t *testing.T) {
	for _, tc := range []struct {
		name, cue string
	}{
		{"incomplete block", "bad cue"},
		{"invalid index", "bad\n00:00:03,000 --> 00:00:04,000\nDiscard"},
		{"truncated timestamp", "790\n01:55:55,560 --> 01:\n"},
		{"invalid timestamp", "2\n00:70:00,000 --> 00:80:00,000\nDiscard"},
		{"reversed interval", "2\n00:00:04,000 --> 00:00:03,000\nDiscard"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := tc.cue + "\n\n"
			// Damage at the beginning, middle and end must not discard valid cues.
			raw := []byte(bad + sampleSRT + bad + "99\n00:00:05,000 --> 00:00:06,000\n<b>Keep</b> &amp; {\\an8}clean\n\n" + bad)
			want := sampleSRT + "2\n00:00:05,000 --> 00:00:06,000\nKeep & clean\n\n"
			got, err := NormalizeSRT(raw)
			if err != nil || string(got) != want {
				t.Fatalf("normalized = %q, error = %v", got, err)
			}
			vtt, err := WebVTTWithOffset(raw, 30500)
			wantVTT := "WEBVTT\n\n1\n00:00:31.500 --> 00:00:32.500\nHello\n\n2\n00:00:35.500 --> 00:00:36.500\nKeep &amp; clean\n\n"
			if err != nil || string(vtt) != wantVTT {
				t.Fatalf("VTT = %q, error = %v", vtt, err)
			}
		})
	}
}

func TestZeroDurationTranscriptionCuesAreOmitted(t *testing.T) {
	data, err := NormalizeSRT([]byte(sampleSRT + "2\n00:00:03,000 --> 00:00:03,000\nDuplicate\n\n3\n00:00:04,000 --> 00:00:05,000\n"))
	if err != nil || string(data) != sampleSRT {
		t.Fatalf("%q %v", data, err)
	}
}

func TestWebVTTWithSubtitleOffset(t *testing.T) {
	const raw = "1\n00:59:59,750 --> 01:00:01,250\nHello\n"
	for _, tc := range []struct {
		offset int64
		want   string
	}{
		{0, "00:59:59.750 --> 01:00:01.250"},
		{30500, "01:00:30.250 --> 01:00:31.750"},
		{3600000, "01:59:59.750 --> 02:00:01.250"},
		{-30500, "00:59:29.250 --> 00:59:30.750"},
		{-3600000, "00:00:00.000 --> 00:00:01.250"},
	} {
		data := []byte(raw)
		got, err := WebVTTWithOffset(data, tc.offset)
		if err != nil || !strings.Contains(string(got), tc.want) {
			t.Fatalf("offset %d: %q %v", tc.offset, got, err)
		}
		if string(data) != raw {
			t.Fatal("shared source was mutated")
		}
	}
	for _, offset := range []int64{-3600001, 3600001} {
		if _, err := WebVTTWithOffset([]byte(raw), offset); err == nil {
			t.Fatalf("accepted invalid offset %d", offset)
		}
	}
}

func TestNegativeOffsetClipsOrOmitsCuesBeforeZero(t *testing.T) {
	raw := []byte(sampleSRT + "2\n00:00:02,000 --> 00:00:04,000\nCrossing\n\n3\n00:00:05,000 --> 00:00:06,000\nLater\n")
	got, err := WebVTTWithOffset(raw, -3000)
	want := "WEBVTT\n\n1\n00:00:00.000 --> 00:00:01.000\nCrossing\n\n2\n00:00:02.000 --> 00:00:03.000\nLater\n\n"
	if err != nil || string(got) != want {
		t.Fatalf("negative offset = %q, error = %v", got, err)
	}
	got, err = WebVTTWithOffset(raw, -6000)
	if err != nil || string(got) != "WEBVTT\n\n" {
		t.Fatalf("all cues before zero = %q, error = %v", got, err)
	}
}
