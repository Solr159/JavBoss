package update

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
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

func TestLatestReleaseFiltersAndPaginates(t *testing.T) {
	pages := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		if r.URL.Query().Get("page") == "1" {
			items := make([]release, 100)
			for i := range items {
				items[i] = release{Tag: "browser-extension-v9.0.0"}
			}
			items[1] = release{Tag: "v9.0.0", Draft: true}
			items[2] = release{Tag: "v8.0.0", Prerelease: true}
			items[3] = release{Tag: "v2.9.0"}
			items[4] = release{Tag: "v99.0.0-beta.1"}
			_ = json.NewEncoder(w).Encode(items)
		} else {
			_ = json.NewEncoder(w).Encode([]release{{Tag: "v2.10.0"}, {Tag: "v2.1.0"}})
		}
	}))
	defer server.Close()
	r, err := latestRelease(t.Context(), server.Client(), server.URL)
	if err != nil || r == nil || r.Tag != "v2.10.0" || pages != 2 {
		t.Fatalf("release=%+v, pages=%d, err=%v", r, pages, err)
	}
}

func TestReleaseAssetValidation(t *testing.T) {
	a := asset{Name: "javboss-v2.0.0-linux-x86_64.zip", Size: 42, Digest: "sha256:" + strings.Repeat("a", 64)}
	a.URL = "https://github.com/" + repository + "/releases/download/v2.0.0/" + a.Name
	for _, tc := range []struct {
		name   string
		change func(*asset)
		bad    bool
	}{
		{"valid", func(*asset) {}, false},
		{"missing digest", func(a *asset) { a.Digest = "" }, true},
		{"wrong repo", func(a *asset) { a.URL = strings.Replace(a.URL, repository, "other/repo", 1) }, true},
		{"http", func(a *asset) { a.URL = strings.Replace(a.URL, "https:", "http:", 1) }, true},
		{"wrong arch", func(a *asset) { a.Name = "javboss-v2.0.0-macos-arm64.zip" }, true},
		{"unbounded size", func(a *asset) { a.Size = maxArchiveSize + 1 }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := a
			tc.change(&copy)
			_, err := releaseAsset(&release{Tag: "v2.0.0", Assets: []asset{copy}}, "linux-x86_64")
			if (err != nil) != tc.bad {
				t.Fatalf("error=%v, want bad=%v", err, tc.bad)
			}
		})
	}
}

func TestDownloadRequiresSizeAndSHA256(t *testing.T) {
	content := []byte("release bytes")
	hash := sha256.Sum256(content)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(content) }))
	defer server.Close()
	for _, tc := range []struct {
		name, digest string
		size         int64
		bad          bool
	}{
		{"valid", hex.EncodeToString(hash[:]), int64(len(content)), false},
		{"corrupt", strings.Repeat("0", 64), int64(len(content)), true},
		{"truncated", hex.EncodeToString(hash[:]), int64(len(content) + 1), true},
		{"oversized", hex.EncodeToString(hash[:]), int64(len(content) - 1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var progress int64
			err := downloadArchive(t.Context(), server.Client(), asset{URL: server.URL, Size: tc.size, Digest: "sha256:" + tc.digest}, filepath.Join(t.TempDir(), "release.zip"), func(n int64) { progress = n })
			if (err != nil) != tc.bad {
				t.Fatalf("err=%v", err)
			}
			if !tc.bad && progress != tc.size {
				t.Fatalf("progress=%d", progress)
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stops atomic.Int32
			m := newManager(t.Context(), Options{BaseDir: t.TempDir(), Version: tc.version, Release: tc.release, Container: tc.container, Stop: func() { stops.Add(1) }})
			m.platform = "linux-x86_64"
			name := "javboss-v2.0.0-linux-x86_64.zip"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode([]release{{Tag: "v2.0.0", Assets: []asset{{Name: name, URL: "https://github.com/" + repository + "/releases/download/v2.0.0/" + name, Digest: "sha256:" + strings.Repeat("a", 64), Size: 12}}}})
			}))
			defer server.Close()
			m.endpoint = server.URL
			m.client = server.Client()
			m.check()
			status := m.Status()
			if status.DisabledReason != tc.reason || status.CanInstall != (tc.reason == "") {
				t.Fatalf("status=%+v", status)
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
