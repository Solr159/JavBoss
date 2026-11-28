package screenshot

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPlaybackSecond(t *testing.T) {
	for _, tc := range []struct {
		name   string
		second float64
		valid  bool
	}{
		{"mpv_00-00-00.jpg", 0, true},
		{"mpv_00-12-34.500.png", 754.5, true},
		{"mpv_01-02-03.001.webp", 3723.001, true},
		{"mpv_100-00-00.1.JPEG", 360000.1, true},
		{"128.jpg", 0, false},
		{"mpv_random.jpg", 0, false},
		{"mpv_00-60-00.jpg", 0, false},
		{"mpv_00-00-60.jpg", 0, false},
		{"mpv_00-00-01.0001.jpg", 0, false},
		{"mpv_00-00-01.jpg.tmp", 0, false},
		{"../mpv_00-00-01.jpg", 0, false},
		{"mpv_999999999999999999999999-00-00.jpg", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := PlaybackSecond(tc.name)
			if got != tc.second || ok != tc.valid {
				t.Fatalf("got (%v, %v), want (%v, %v)", got, ok, tc.second, tc.valid)
			}
		})
	}
}

func TestListPlaybackSecondsTracksFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "video", "7", "screenshot")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"mpv_00-00-02.500.jpg", "mpv_00-00-02.500.png", "mpv_00-00-00.jpg", "128.jpg", "mpv_invalid.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("image"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "mpv_00-00-10.jpg"), 0700); err != nil {
		t.Fatal(err)
	}
	got, err := ListPlaybackSeconds(root, 7)
	if err != nil || !reflect.DeepEqual(got, []float64{0, 2.5}) {
		t.Fatalf("times=%v, err=%v", got, err)
	}
	if err := os.Remove(filepath.Join(dir, "mpv_00-00-00.jpg")); err != nil {
		t.Fatal(err)
	}
	got, err = ListPlaybackSeconds(root, 7)
	if err != nil || !reflect.DeepEqual(got, []float64{2.5}) {
		t.Fatalf("after deletion: times=%v, err=%v", got, err)
	}
	got, err = ListPlaybackSeconds(root, 8)
	if err != nil || len(got) != 0 {
		t.Fatalf("missing directory: times=%v, err=%v", got, err)
	}
}
