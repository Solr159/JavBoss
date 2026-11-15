package api

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/util"
	"javboss/internal/video/preview"

	"github.com/gin-gonic/gin"
)

func TestVideoPreviewHTTP(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is required for preview integration")
	}
	t.Setenv("JAVBOSS_CONTAINER", "0")
	t.Setenv("JAVBOSS_BUILD_MODE", "development")
	dir := t.TempDir()
	t.Chdir(dir)
	tool := filepath.Join(dir, util.FFmpegToolRelativePath())
	if err := os.MkdirAll(filepath.Dir(tool), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(ffmpeg, tool); err != nil {
		t.Skipf("link fixture ffmpeg: %v", err)
	}
	source := filepath.Join(dir, "video.mp4")
	cmd := exec.CommandContext(t.Context(), ffmpeg, "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=640x360:rate=10:duration=24", "-c:v", "mpeg4", "-g", "60", "-bf", "0", source)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create fixture: %v: %s", err, out)
	}
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	connection, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	deps := newTestDependencies()
	deps.database = database
	deps.bindDatabase(t)
	directory, err := db.Default().CreateDirectory(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	video := &models.Video{DurationSec: 24, Size: stat.Size(), Fingerprint: "preview-http"}
	if err := db.Default().CreateVideo(t.Context(), video); err != nil {
		t.Fatal(err)
	}
	missing, err := db.Default().UpsertVideoLocation(t.Context(), video.ID, directory.ID, "missing.mp4", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	location, err := db.Default().UpsertVideoLocation(t.Context(), video.ID, directory.ID, "video.mp4", stat.ModTime())
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(dir, "cache", "previews")
	preview.Start(t.Context(), cacheDir)
	router := gin.New()
	router.GET("/videos/:id/preview", getVideoPreview)
	base := fmt.Sprintf("/videos/%d/preview?location_id=%d", video.ID, location.ID)
	request := func(path string, status int) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != status {
			t.Fatalf("%s: %d %s", path, response.Code, response.Body.String())
		}
		return response
	}
	first := request(base+"&second=0.5", http.StatusOK)
	image, err := jpeg.Decode(bytes.NewReader(first.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if image.Bounds().Dx() != 320 || image.Bounds().Dy() != 180 {
		t.Fatalf("preview size: %v", image.Bounds())
	}
	if first.Header().Get("Content-Type") != "image/jpeg" || first.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("headers: %v", first.Header())
	}
	second := request(base+"&second=0.9", http.StatusOK)
	if !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Fatal("same second did not reuse image")
	}
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	var images int
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".jpg" {
			images++
		}
	}
	if images != 1 {
		t.Fatalf("cache images: %d", images)
	}
	// Adjacent seconds must have their own precisely captured images, even within one GOP.
	var previous []byte
	for _, second := range []int{18, 19, 20} {
		response := request(fmt.Sprintf("%s&second=%d", base, second), http.StatusOK)
		data := response.Body.Bytes()
		path := filepath.Join(cacheDir, fmt.Sprintf("%d_%d.jpg", video.ID, second))
		if cached, err := os.ReadFile(path); err != nil || !bytes.Equal(cached, data) {
			t.Fatalf("precise screenshot at %d was not cached: %v", second, err)
		}
		if bytes.Equal(previous, data) {
			t.Fatalf("second %d reused the previous frame", second)
		}
		if !bytes.Equal(request(fmt.Sprintf("%s&second=%d.8", base, second), http.StatusOK).Body.Bytes(), data) {
			t.Fatalf("second %d did not reuse its own cache", second)
		}
		previous = data
	}
	for _, value := range []string{"", "invalid", "NaN", "Inf", "-1", "25"} {
		request(base+"&second="+value, http.StatusBadRequest)
	}
	request(fmt.Sprintf("/videos/%d/preview?location_id=%d&second=0", video.ID, missing.ID), http.StatusNotFound)
	request(fmt.Sprintf("/videos/%d/preview?location_id=99999&second=0", video.ID), http.StatusNotFound)
	request(fmt.Sprintf("/videos/%d/preview?location_id=-1&second=0", video.ID), http.StatusBadRequest)
	preview.Start(t.Context(), "")
	response := request(base+"&second=0", http.StatusServiceUnavailable)
	if response.Header().Get("Retry-After") != "3" {
		t.Fatal("missing retry hint")
	}
}
