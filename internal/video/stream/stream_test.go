package stream

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestServeManifestWithoutTranscodingRuntime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a shell executable")
	}
	t.Setenv("JAVBOSS_BUILD_MODE", "development")
	t.Setenv("JAVBOSS_CONTAINER", "")
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join("internal", "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	probe := "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"width\":1920,\"height\":1080}],\"format\":{\"duration\":\"12\"}}'\n"
	if err := os.WriteFile(filepath.Join("internal", "bin", "ffprobe"), []byte(probe), 0755); err != nil {
		t.Fatal(err)
	}
	previous := active.Swap(nil)
	t.Cleanup(func() { active.Store(previous) })
	request := httptest.NewRequest(http.MethodGet, "/videos/1/stream.m3u8?apikey=secret&location_id=9", nil)
	response := httptest.NewRecorder()
	ServeManifest(response, request, "video.mp4", 0, "FULL_HD")
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != MimeHLS {
		t.Fatalf("manifest response = %d %q", response.Code, response.Body.String())
	}
	for _, expected := range []string{"#EXTM3U", "#EXT-X-ENDLIST", "/0.ts?", "apikey=secret", "location_id=9", "resolution=FULL_HD"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("manifest missing %q: %s", expected, response.Body.String())
		}
	}
	if active.Load() != nil {
		t.Fatal("building a manifest initialized the transcoding runtime")
	}
	// Segment generation still requires the configured process manager.
	response = httptest.NewRecorder()
	ServeSegment(response, request, Options{})
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured segment response = %d", response.Code)
	}
}
