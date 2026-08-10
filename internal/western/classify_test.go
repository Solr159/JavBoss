package western

import "testing"

func TestIsLikelyJAVFilename(t *testing.T) {
	for _, name := range []string{"FC2-PPV-4360814.mp4", "FC2 4026296.mp4", "1Pondo-123456.mp4", "Tokyo-Hot sample.mkv"} {
		if !IsLikelyJAVFilename(name) {
			t.Fatalf("expected %q to be classified as JAV", name)
		}
	}
	for _, name := range []string{"EvilAngel.Britney.2160p.mp4", "Stacy Cruz scene.mkv"} {
		if IsLikelyJAVFilename(name) {
			t.Fatalf("expected %q not to be classified as JAV", name)
		}
	}
}
