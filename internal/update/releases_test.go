package update

import (
	"archive/zip"
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestFailedCheckClearsPreviousInstall(t *testing.T) {
	m := newManager(t.Context(), Options{BaseDir: t.TempDir(), Version: "v1.0.0", Release: true})
	m.status.CanInstall = true
	m.selected = asset{Name: "old.zip"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	m.endpoint = server.URL + "/releases/latest"
	m.client = server.Client()
	m.check()
	status := m.Status()
	if status.CanInstall || status.Error == "" || m.selected.Name != "" {
		t.Fatalf("status=%+v selected=%+v", status, m.selected)
	}
}

func TestDownloadFollowsAssetRedirectAfterLatestCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			w.Header().Set("Location", "https://github.com/"+repository+"/releases/tag/v2.1.4")
			w.WriteHeader(http.StatusFound)
		case "/release.zip":
			http.Redirect(w, r, "/asset-content", http.StatusFound)
		case "/asset-content":
			_, _ = w.Write([]byte("archive contents"))
		default:
			t.Errorf("unexpected URL: %s", r.URL)
		}
	}))
	defer server.Close()
	client := server.Client()
	if _, err := latestRelease(t.Context(), client, server.URL+"/releases/latest"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "release.zip")
	if err := downloadArchive(t.Context(), client, asset{URL: server.URL + "/release.zip"}, target, func(int64, int64) {}); err != nil {
		t.Fatal(err)
	}
	requireFile(t, filepath.Dir(target), "release.zip", "archive contents")
}

func TestExtractArchiveRejectsCorruptContents(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.CreateHeader(&zip.FileHeader{Name: "package/file", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("unique archive content")
	if _, err := entry.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	data := buffer.Bytes()
	index := bytes.Index(data, content)
	if index < 0 {
		t.Fatal("missing stored content")
	}
	data[index] ^= 1
	root := t.TempDir()
	archive := putFile(t, root, "corrupt.zip", string(data))
	if err := extractArchive(t.Context(), archive, filepath.Join(root, "payload"), "package"); !errors.Is(err, zip.ErrChecksum) {
		t.Fatalf("expected ZIP checksum failure, got %v", err)
	}
}
