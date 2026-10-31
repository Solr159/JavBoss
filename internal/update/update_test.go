package update

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func putFile(t *testing.T, root, rel, value string) string {
	t.Helper()
	name := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(value), 0644); err != nil {
		t.Fatal(err)
	}
	return name
}

func requireFile(t *testing.T, root, rel, value string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil || string(data) != value {
		t.Fatalf("%s = %q, %v; want %q", rel, data, err, value)
	}
}

func TestLatestReleaseUsesOnlyValidatedRedirect(t *testing.T) {
	prefix := "https://github.com/" + repository + "/releases/tag/"
	for _, tc := range []struct {
		name, location, want string
		status               int
		bad                  bool
	}{
		{"stable", prefix + "v2.10.0", "v2.10.0", 302, false},
		{"relative", "/" + repository + "/releases/tag/v2.10.0", "v2.10.0", 302, false},
		{"extension", prefix + "browser-extension-v9.0.0", "", 302, true},
		{"prerelease", prefix + "v2.10.0-rc.1", "", 302, true},
		{"foreign host", "https://example.com/" + repository + "/releases/tag/v2.10.0", "", 302, true},
		{"foreign repo", "https://github.com/other/repo/releases/tag/v2.10.0", "", 302, true},
		{"insecure", strings.Replace(prefix, "https:", "http:", 1) + "v2.10.0", "", 302, true},
		{"query", prefix + "v2.10.0?x=1", "", 302, true},
		{"missing location", "", "", 302, true},
		{"no release", "", "", 404, false},
		{"forbidden", "", "", 403, true},
		{"html instead of redirect", "", "", 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != http.MethodHead || r.URL.Path != "/releases/latest" || r.URL.RawQuery != "" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
				}
				w.Header().Set("Location", tc.location)
				w.WriteHeader(tc.status)
			}))
			defer server.Close()
			r, err := latestRelease(t.Context(), server.Client(), server.URL+"/releases/latest")
			if (err != nil) != tc.bad || requests != 1 {
				t.Fatalf("release=%+v requests=%d err=%v", r, requests, err)
			}
			if tc.want != "" && (r == nil || r.Tag != tc.want) {
				t.Fatalf("release=%+v", r)
			}
			if tc.status == 404 && r != nil {
				t.Fatalf("expected no release: %+v", r)
			}
		})
	}
}

func TestReleaseAssetUsesFixedOfficialURL(t *testing.T) {
	for _, platform := range []string{"windows-x86_64", "linux-x86_64", "macos-x86_64", "macos-arm64"} {
		a, err := releaseAsset(&release{Tag: "v2.0.0"}, platform)
		name := "javboss-v2.0.0-" + platform + ".zip"
		if err != nil || a.Name != name || a.URL != "https://github.com/"+repository+"/releases/download/v2.0.0/"+name {
			t.Fatalf("asset=%+v err=%v", a, err)
		}
	}
	for _, tag := range []string{"browser-extension-v2.0.0", "v2.0.0-rc.1", "v02.0.0", "../v2.0.0"} {
		if _, err := releaseAsset(&release{Tag: tag}, "linux-x86_64"); err == nil {
			t.Fatalf("accepted invalid version %s", tag)
		}
	}
	if _, err := releaseAsset(&release{Tag: "v2.0.0"}, "../linux-x86_64"); err == nil {
		t.Fatal("accepted unsupported platform")
	}
}

func TestDownloadWithoutManifest(t *testing.T) {
	content := "release bytes"
	for _, tc := range []struct {
		name   string
		length int64
		status int
		bad    bool
	}{
		{"known size", int64(len(content)), 200, false},
		{"chunked", -1, 200, false},
		{"truncated", int64(len(content) + 1), 200, true},
		{"over limit", maxArchiveSize + 1, 200, true},
		{"empty", 0, 200, true},
		{"missing asset", -1, 404, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.length >= 0 {
					w.Header().Set("Content-Length", fmt.Sprint(tc.length))
				}
				w.WriteHeader(tc.status)
				w.(http.Flusher).Flush()
				if tc.length != 0 {
					_, _ = w.Write([]byte(content))
				}
			}))
			defer server.Close()
			var downloaded, total int64
			target := filepath.Join(t.TempDir(), "release.zip")
			err := downloadArchive(t.Context(), server.Client(), asset{URL: server.URL}, target, func(n, size int64) { downloaded, total = n, size })
			if (err != nil) != tc.bad {
				t.Fatalf("err=%v", err)
			}
			if !tc.bad {
				wantTotal := tc.length
				if wantTotal < 0 {
					wantTotal = 0
				}
				if downloaded != int64(len(content)) || total != wantTotal {
					t.Fatalf("downloaded=%d total=%d", downloaded, total)
				}
				requireFile(t, filepath.Dir(target), "release.zip", content)
			}
		})
	}
}

func TestExtractArchiveRejectsUnsafeEntries(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		mode       os.FileMode
		bad        bool
	}{
		{"valid", "package/web/dist/index.html", 0644, false},
		{"traversal", "package/../escaped", 0644, true},
		{"absolute", "/package/file", 0644, true},
		{"backslash", `package/..\escaped`, 0644, true},
		{"wrong root", "other/file", 0644, true},
		{"link", "package/link", os.ModeSymlink | 0777, true},
		{"drive", "package/C:/file", 0644, true},
		{"windows parent alias", "package/.. /escaped", 0644, true},
		{"windows device", "package/NUL.txt", 0644, true},
		{"windows trailing dot", "package/file.", 0644, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buffer bytes.Buffer
			writer := zip.NewWriter(&buffer)
			header := &zip.FileHeader{Name: tc.path, Method: zip.Deflate}
			header.SetMode(tc.mode)
			file, err := writer.CreateHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = file.Write([]byte("content"))
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			archive := putFile(t, root, "archive.zip", buffer.String())
			err = extractArchive(t.Context(), archive, filepath.Join(root, "payload"), "package")
			if (err != nil) != tc.bad {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestReplacementPreservesDataAndRollsBackFailures(t *testing.T) {
	for _, failAt := range []int{0, 1, 2, 3, 4, 5, 6} {
		t.Run(fmt.Sprint(failAt), func(t *testing.T) {
			base, payload, backup := t.TempDir(), t.TempDir(), t.TempDir()
			for _, rel := range []string{executableName(), "web/dist/index.html", "modernz/file"} {
				putFile(t, base, rel, "old")
				putFile(t, payload, rel, "new")
			}
			for _, rel := range []string{"config.toml", "data/javboss.db", "data/client/settings.json", "data/tools/ffmpeg"} {
				putFile(t, base, rel, "user")
				putFile(t, payload, rel, "default")
			}
			putFile(t, base, "web/dist/obsolete.js", "old asset")
			calls := 0
			tx := &replacement{journal: filepath.Join(backup, "files.json"), rename: func(a, b string) error {
				calls++
				if calls == failAt {
					return errors.New("simulated locked file")
				}
				return os.Rename(a, b)
			}}
			err := tx.apply(base, payload, backup)
			if (err != nil) != (failAt > 0) {
				t.Fatalf("err=%v", err)
			}
			want := "new"
			if failAt > 0 {
				want = "old"
			}
			for _, rel := range []string{executableName(), "web/dist/index.html", "modernz/file"} {
				requireFile(t, base, rel, want)
			}
			for _, rel := range []string{"config.toml", "data/javboss.db", "data/client/settings.json", "data/tools/ffmpeg"} {
				requireFile(t, base, rel, "user")
			}
			if failAt == 0 {
				if _, err := os.Stat(filepath.Join(base, "web/dist/obsolete.js")); !os.IsNotExist(err) {
					t.Fatal("obsolete frontend survived replacement")
				}
			}
		})
	}
}

func TestDatabaseBackupIncludesWALAndDoesNotMigrate(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "database.db")
	dest := filepath.Join(root, "backup.db")
	db, err := sql.Open("sqlite3", source)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0; CREATE TABLE example (value TEXT); INSERT INTO example VALUES ('saved')"); err != nil {
		t.Fatal(err)
	}
	if err = backupDatabase(source, dest); err != nil {
		t.Fatal(err)
	}
	backup, err := sql.Open("sqlite3", dest)
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var got string
	if err = backup.QueryRow("SELECT value FROM example").Scan(&got); err != nil || got != "saved" {
		t.Fatalf("backup=%q err=%v", got, err)
	}
	var count int
	if err = backup.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='goose_db_version'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("migration unexpectedly run: %d %v", count, err)
	}
}

func TestManagerCheckNeverStopsApplicationAndDisablesUnsupportedInstall(t *testing.T) {
	for _, tc := range []struct {
		name               string
		release, container bool
		version, reason    string
	}{
		{"desktop", true, false, "v1.0.0", ""},
		{"container", true, true, "v1.0.0", "container"},
		{"development", false, false, "dev", "development"},
		{"already current", true, false, "v2.0.0", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stops atomic.Int32
			m := newManager(t.Context(), Options{BaseDir: t.TempDir(), Version: tc.version, Release: tc.release, Container: tc.container, Stop: func() { stops.Add(1) }})
			m.platform = "linux-x86_64"
			name := "javboss-v2.0.0-linux-x86_64.zip"
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method == http.MethodHead && r.URL.Path == "/releases/latest" {
					w.Header().Set("Location", "https://github.com/"+repository+"/releases/tag/v2.0.0")
					w.WriteHeader(http.StatusFound)
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/releases/tag/v2.0.0" {
					_, _ = w.Write([]byte(`<div data-test-selector="body-content" class="markdown-body"><p>更新说明</p></div>`))
					return
				}
				t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				http.NotFound(w, r)
			}))
			defer server.Close()
			m.endpoint = server.URL + "/releases/latest"
			m.client = server.Client()
			m.check()
			status := m.Status()
			installable := tc.reason == "" && tc.version != "v2.0.0"
			if status.DisabledReason != tc.reason || status.CanInstall != installable {
				t.Fatalf("status=%+v", status)
			}
			if requests != 2 || status.Notes != "更新说明" {
				t.Fatalf("requests=%d notes=%q", requests, status.Notes)
			}
			if installable && m.selected.URL != "https://github.com/"+repository+"/releases/download/v2.0.0/"+name {
				t.Fatalf("wrong download URL: %s", m.selected.URL)
			}
			if stops.Load() != 0 {
				t.Fatal("checking stopped app")
			}
			if tc.reason != "" && m.StartInstall() == nil {
				t.Fatal("unsupported install accepted")
			}
		})
	}
}

func TestBusyOperationsAndHTTPNeverAcceptArbitraryDownloads(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := newManager(ctx, Options{BaseDir: t.TempDir(), Version: "v1.0.0", Release: true})
	m.status.Phase = "downloading"
	if m.StartCheck() == nil || m.StartInstall() == nil {
		t.Fatal("concurrent operation accepted")
	}
	old := active.Swap(m)
	defer active.Store(old)
	m.status.Phase = "idle"
	w := httptest.NewRecorder()
	ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/system/update/install", strings.NewReader(`{"url":"https://attacker/evil","path":"/tmp/evil"}`)), "/system/update")
	if w.Code != http.StatusConflict {
		t.Fatalf("unexpected status=%d", w.Code)
	}
	cancel()
	if !errors.Is(m.StartCheck(), context.Canceled) {
		t.Fatal("started check during shutdown")
	}
}

func TestWaitFileCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()
	if err := waitFile(ctx, filepath.Join(t.TempDir(), "absent"), "ready"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v", err)
	}
}

func TestDatabaseURIEscapesPathsAndWindowsDrives(t *testing.T) {
	for _, path := range []string{"/tmp/数据库 ?#.db", "C:/Users/My Name/数据库.db"} {
		parsed, err := url.Parse(databaseURI(path))
		if err != nil {
			t.Fatal(err)
		}
		if parsed.Host != "" || parsed.Fragment != "" || parsed.Query().Get("mode") != "rw" || strings.TrimPrefix(parsed.Path, "/") != strings.TrimPrefix(path, "/") {
			t.Fatalf("invalid SQLite URI: %s", parsed)
		}
	}
}
