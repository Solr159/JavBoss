package server

import (
	"testing"
	"time"

	"javboss/internal/models"
)

func TestIsScreenshotImageName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "mpv_00-12-34.567.jpg", want: true},
		{name: "mpv_example.PNG", want: true},
		{name: "00-12-34.567.jpg", want: false},
		{name: "../mpv_example.jpg", want: false},
		{name: "nested/mpv_example.jpg", want: false},
		{name: "mpv_example.txt", want: false},
		{name: "", want: false},
	}

	for _, tt := range tests {
		if got := isScreenshotImageName(tt.name); got != tt.want {
			t.Fatalf("isScreenshotImageName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestManualScrapeRequestToJavInfo(t *testing.T) {
	uncensored := true
	duration := 170

	info, err := manualScrapeRequestToJavInfo(videoJavManualScrapeRequest{
		Code:         " ipx-228 ",
		Title:        " Title ",
		Studio:       " Studio ",
		Series:       " Series ",
		ReleaseDate:  "2018-11-13",
		DurationMin:  &duration,
		Tags:         []string{"美少女", "美少女", "", "接吻"},
		Actors:       []string{"岬ななみ", "岬ななみ"},
		CoverURL:     " https://example.test/cover.jpg ",
		IsUncensored: &uncensored,
	})
	if err != nil {
		t.Fatalf("manualScrapeRequestToJavInfo() error = %v", err)
	}

	if info.Code != "IPX-228" {
		t.Fatalf("unexpected code: %q", info.Code)
	}
	if info.Title != "Title" || info.Studio != "Studio" || info.Series != "Series" {
		t.Fatalf("unexpected text fields: %#v", info)
	}
	wantRelease := time.Date(2018, 11, 13, 0, 0, 0, 0, time.UTC).Unix()
	if info.ReleaseUnix != wantRelease {
		t.Fatalf("unexpected release unix: got %d want %d", info.ReleaseUnix, wantRelease)
	}
	if info.DurationMin != 170 {
		t.Fatalf("unexpected duration: %d", info.DurationMin)
	}
	if len(info.Tags) != 2 || info.Tags[0] != "美少女" || info.Tags[1] != "接吻" {
		t.Fatalf("unexpected tags: %#v", info.Tags)
	}
	if len(info.Actors) != 1 || info.Actors[0] != "岬ななみ" {
		t.Fatalf("unexpected actors: %#v", info.Actors)
	}
	if info.CoverURL != "https://example.test/cover.jpg" {
		t.Fatalf("unexpected cover url: %q", info.CoverURL)
	}
	if info.IsUncensored == nil || !*info.IsUncensored {
		t.Fatalf("unexpected uncensored state: %#v", info.IsUncensored)
	}
}

func TestHLSStreamHelpersPreserveLocationID(t *testing.T) {
	video := &models.Video{ID: 42}

	if got, want := buildHLSStreamURL(video, 7), "/videos/42/stream.m3u8?location_id=7"; got != want {
		t.Fatalf("buildHLSStreamURL() = %q, want %q", got, want)
	}
	if got, want := streamCacheKey(42, 7), "42_location_7"; got != want {
		t.Fatalf("streamCacheKey() = %q, want %q", got, want)
	}
	if got, want := buildHLSStreamURL(video, 0), "/videos/42/stream.m3u8"; got != want {
		t.Fatalf("buildHLSStreamURL() without location = %q, want %q", got, want)
	}
	if got, want := streamCacheKey(42, 0), "42"; got != want {
		t.Fatalf("streamCacheKey() without location = %q, want %q", got, want)
	}
}

func TestPlaybackScreenshotName(t *testing.T) {
	tests := []struct {
		second float64
		want   string
	}{
		{second: 0, want: "mpv_00-00-00.jpg"},
		{second: 1.234, want: "mpv_00-00-01.234.jpg"},
		{second: 65, want: "mpv_00-01-05.jpg"},
		{second: 3661.5, want: "mpv_01-01-01.500.jpg"},
	}

	for _, tt := range tests {
		if got := playbackScreenshotName(tt.second); got != tt.want {
			t.Fatalf("playbackScreenshotName(%v) = %q, want %q", tt.second, got, tt.want)
		}
	}
}
