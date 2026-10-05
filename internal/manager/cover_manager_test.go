package manager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"javboss/internal/jav"
)

func TestSetCoverDownloadHeadersForJavBus(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://www.javbus.com/pics/cover/c85j_b.jpg", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	setCoverDownloadHeaders(req)

	if got := req.Header.Get("Referer"); got != "https://www.javbus.com/" {
		t.Fatalf("Referer = %q, want javbus referer", got)
	}
	if got := req.Header.Get("Cookie"); !strings.Contains(got, "age=verified") {
		t.Fatalf("Cookie = %q, want age verified cookie", got)
	}
	if got := req.Header.Get("User-Agent"); !strings.Contains(got, "Chrome/") {
		t.Fatalf("User-Agent = %q, want browser user agent", got)
	}
}

func TestEnqueueDeduplicatesScheduledCodes(t *testing.T) {
	manager := &CoverManager{
		tasks:     make(chan string, 2),
		scheduled: make(map[string]struct{}),
	}

	manager.Enqueue("ABC-001")
	manager.Enqueue("abc-001")
	manager.Enqueue(" ABC-001 ")
	manager.Enqueue("ABC-002")

	if got := len(manager.tasks); got != 2 {
		t.Fatalf("queued tasks = %d, want 2", got)
	}
	if got := <-manager.tasks; got != "abc-001" {
		t.Fatalf("first task = %q, want normalized abc-001", got)
	}
	if got := <-manager.tasks; got != "abc-002" {
		t.Fatalf("second task = %q, want normalized abc-002", got)
	}

	manager.clearScheduled("ABC-001")
	manager.Enqueue("ABC-001")
	if got := len(manager.tasks); got != 1 {
		t.Fatalf("queued tasks after clear = %d, want 1", got)
	}
}

func TestDownloadCoverRejectsSmallFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte(strings.Repeat("x", int(minValidCoverSizeBytes)-1)))
	}))
	defer server.Close()

	manager := &CoverManager{coverDir: t.TempDir()}
	err := manager.downloadCover(context.Background(), "ABC-001", server.URL+"/small.jpg")
	if !errors.Is(err, errInvalidCover) {
		t.Fatalf("downloadCover error = %v, want errInvalidCover", err)
	}
	if _, err := os.Stat(filepath.Join(manager.coverDir, "abc-001.jpg")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("small cover should not be finalized, stat err=%v", err)
	}
}

func TestDownloadCoverImageSizeLimit(t *testing.T) {
	for _, code := range []string{"ABC-001", "FC2-PPV-1234567"} {
		for _, format := range []string{"jpeg", "png"} {
			for _, encoded := range []bool{false, true} {
				for _, size := range []int{5*1024 - 1, 5 * 1024} {
					t.Run(fmt.Sprintf("%s/%s/encoded=%t/size=%d", code, format, encoded, size), func(t *testing.T) {
						var data bytes.Buffer
						img := image.NewRGBA(image.Rect(0, 0, 240, 320))
						var err error
						if format == "jpeg" {
							err = jpeg.Encode(&data, img, nil)
						} else {
							err = png.Encode(&data, img)
						}
						if err != nil {
							t.Fatal(err)
						}
						if data.Len() > size {
							t.Fatal("fixture exceeds target size")
						}
						data.Write(make([]byte, size-data.Len()))
						payload := data.Bytes()
						if encoded {
							const key byte = 0xad
							payload = make([]byte, data.Len()+1)
							payload[0] = key
							for i, b := range data.Bytes() {
								payload[i+1] = b ^ key
							}
						}
						requests := 0
						server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							requests++
							w.Header().Set("Content-Type", "image/"+format)
							_, _ = w.Write(payload)
						}))
						defer server.Close()
						manager := &CoverManager{coverDir: t.TempDir()}
						err = manager.downloadCover(context.Background(), code, server.URL+"/cover."+format)
						if size < 5*1024 {
							if !errors.Is(err, errInvalidCover) {
								t.Fatalf("downloadCover error = %v, want errInvalidCover", err)
							}
							entries, err := os.ReadDir(manager.coverDir)
							if err != nil || len(entries) != 0 {
								t.Fatalf("rejected cover left files: %v, err=%v", entries, err)
							}
							return
						}
						if err != nil {
							t.Fatal(err)
						}
						path, ok := FindCoverPath(manager.coverDir, code)
						if !ok || !manager.Exists(code) {
							t.Fatal("cover must be discoverable by the manager and API")
						}
						got, err := os.ReadFile(path)
						if err != nil || !bytes.Equal(got, data.Bytes()) {
							t.Fatalf("downloaded image differs from source: %v", err)
						}
						if err := manager.handleTask(context.Background(), code); err != nil || requests != 1 {
							t.Fatalf("existing cover was not reused: requests=%d err=%v", requests, err)
						}
						if err := os.WriteFile(path, data.Bytes()[:5*1024-1], 0o644); err != nil {
							t.Fatal(err)
						}
						if manager.Exists(code) {
							t.Fatal("cover below 5 KiB must not count as an existing image")
						}
					})
				}
			}
		}
	}
}

func TestDownloadCoverFromURLReplacesExistingCover(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte(strings.Repeat("p", int(minValidCoverSizeBytes))))
	}))
	defer server.Close()

	coverDir := t.TempDir()
	oldPath := filepath.Join(coverDir, "abc-001.jpg")
	if err := os.WriteFile(oldPath, []byte(strings.Repeat("j", int(minValidCoverSizeBytes))), 0o644); err != nil {
		t.Fatalf("write old cover: %v", err)
	}

	if err := DownloadCoverFromURL(context.Background(), coverDir, "ABC-001", server.URL+"/cover"); err != nil {
		t.Fatalf("DownloadCoverFromURL: %v", err)
	}

	if _, err := os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old cover should be removed, stat err=%v", err)
	}
	path, ok := FindCoverPath(coverDir, "ABC-001")
	if !ok {
		t.Fatal("new cover was not found")
	}
	if filepath.Base(path) != "abc-001.png" {
		t.Fatalf("new cover path = %q, want abc-001.png", path)
	}
}

func TestHandleTaskRetriesAfterSmallCover(t *testing.T) {
	var small bytes.Buffer
	if err := jpeg.Encode(&small, image.NewRGBA(image.Rect(0, 0, 120, 90)), nil); err != nil {
		t.Fatal(err)
	}
	if int64(small.Len()) >= minValidCoverSizeBytes {
		t.Fatal("fixture must be smaller than the minimum cover size")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		switch r.URL.Path {
		case "/small.jpg":
			_, _ = w.Write(small.Bytes())
		case "/valid.jpg":
			_, _ = w.Write([]byte(strings.Repeat("y", int(minValidCoverSizeBytes))))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	originalLookup := lookupJavByCode
	calls := map[jav.Provider]int{}
	lookupJavByCode = func(_ context.Context, code string, provider jav.Provider) (*jav.JavInfo, error) {
		calls[provider]++
		switch provider {
		case jav.ProviderJavDatabase:
			return &jav.JavInfo{CoverURL: server.URL + "/small.jpg"}, nil
		case jav.ProviderJavBus:
			return &jav.JavInfo{CoverURL: server.URL + "/valid.jpg"}, nil
		default:
			return nil, jav.ErrNotFound
		}
	}
	t.Cleanup(func() { lookupJavByCode = originalLookup })

	manager := &CoverManager{
		coverDir:  t.TempDir(),
		providers: []jav.Provider{jav.ProviderJavDatabase, jav.ProviderJavBus},
	}
	if err := manager.handleTask(context.Background(), "ABC-001"); err != nil {
		t.Fatalf("handleTask: %v", err)
	}

	if calls[jav.ProviderJavDatabase] != 1 || calls[jav.ProviderJavBus] != 1 {
		t.Fatalf("unexpected provider calls: %#v", calls)
	}
	info, err := os.Stat(filepath.Join(manager.coverDir, "abc-001.jpg"))
	if err != nil {
		t.Fatalf("stat final cover: %v", err)
	}
	if info.Size() != minValidCoverSizeBytes {
		t.Fatalf("final cover size = %d, want %d", info.Size(), minValidCoverSizeBytes)
	}
	if !manager.Exists("ABC-001") {
		t.Fatal("fallback cover must be discoverable")
	}
	if err := manager.handleTask(context.Background(), "ABC-001"); err != nil {
		t.Fatal(err)
	}
	if calls[jav.ProviderJavDatabase] != 1 || calls[jav.ProviderJavBus] != 1 {
		t.Fatalf("existing cover triggered another download: %#v", calls)
	}
}

func TestHandleTaskFC2UsesBuiltInCoverProviders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing.jpg" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		size := minValidCoverSizeBytes
		if r.URL.Path == "/small.jpg" {
			size--
		}
		_, _ = w.Write([]byte(strings.Repeat("x", int(size))))
	}))
	defer server.Close()

	for _, tc := range []struct {
		name     string
		apiPath  string
		apiErr   error
		fallback bool
	}{
		{name: "api cover", apiPath: "/cover.jpg"},
		{name: "api not found", apiErr: jav.ErrNotFound, fallback: true},
		{name: "api failure", apiErr: errors.New("request failed"), fallback: true},
		{name: "no api cover", fallback: true},
		{name: "cover download missing", apiPath: "/missing.jpg", fallback: true},
		{name: "cover too small", apiPath: "/small.jpg", fallback: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			originalLookup := lookupJavByCode
			t.Cleanup(func() { lookupJavByCode = originalLookup })
			var calls []jav.Provider
			lookupJavByCode = func(_ context.Context, code string, provider jav.Provider) (*jav.JavInfo, error) {
				calls = append(calls, provider)
				if code != "fc2-ppv-1234567" {
					t.Fatalf("lookup code = %q", code)
				}
				switch provider {
				case jav.ProviderJavBus, jav.ProviderJavDatabase, jav.ProviderThePornDB:
					return nil, jav.ErrNotFound
				case jav.ProviderJavDBAPI:
					if tc.apiErr != nil {
						return nil, tc.apiErr
					}
					info := &jav.JavInfo{}
					if tc.apiPath != "" {
						info.CoverURL = server.URL + tc.apiPath
					}
					return info, nil
				case jav.ProviderAvsox:
					return &jav.JavInfo{CoverURL: server.URL + "/fallback.jpg"}, nil
				default:
					t.Fatalf("unexpected FC2 cover provider: %s", provider)
					return nil, jav.ErrNotFound
				}
			}
			manager := NewCoverManager(t.TempDir())
			if err := manager.handleTask(context.Background(), " FC2-PPV-1234567 "); err != nil {
				t.Fatal(err)
			}
			wantCalls := []jav.Provider{
				jav.ProviderJavBus, jav.ProviderJavDatabase, jav.ProviderThePornDB, jav.ProviderJavDBAPI,
			}
			if tc.fallback {
				wantCalls = append(wantCalls, jav.ProviderAvsox)
			}
			if !slices.Equal(calls, wantCalls) {
				t.Fatalf("providers = %v, want %v", calls, wantCalls)
			}
			if !manager.Exists("FC2-PPV-1234567") {
				t.Fatal("downloaded FC2 cover was not found")
			}
		})
	}
}

func TestCoverProviderTimeoutContinuesWithFreshBudget(t *testing.T) {
	for _, stage := range []string{"metadata", "image download"} {
		t.Run(stage, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/slow.jpg" {
					<-r.Context().Done()
					return
				}
				_, _ = w.Write(bytes.Repeat([]byte{'x'}, int(minValidCoverSizeBytes)))
			}))
			defer server.Close()
			originalLookup := lookupJavByCode
			t.Cleanup(func() { lookupJavByCode = originalLookup })
			var calls []jav.Provider
			var firstCtx context.Context
			lookupJavByCode = func(ctx context.Context, code string, provider jav.Provider) (*jav.JavInfo, error) {
				calls = append(calls, provider)
				deadline, ok := ctx.Deadline()
				if remaining := time.Until(deadline); !ok || remaining < 7*time.Second || remaining > 8*time.Second {
					t.Fatalf("provider %s budget = %s, has deadline = %t", provider, remaining, ok)
				}
				if provider == jav.ProviderJavBus {
					firstCtx = ctx
					if stage == "metadata" {
						<-ctx.Done()
						return nil, ctx.Err()
					}
					// Metadata consumes part of the same budget used by the image request.
					time.Sleep(2 * time.Second)
					return &jav.JavInfo{CoverURL: server.URL + "/slow.jpg"}, nil
				}
				if !errors.Is(firstCtx.Err(), context.DeadlineExceeded) {
					t.Fatalf("first provider did not time out: %v", firstCtx.Err())
				}
				return &jav.JavInfo{CoverURL: server.URL + "/cover.jpg"}, nil
			}
			manager := NewCoverManager(t.TempDir())
			manager.providers = []jav.Provider{jav.ProviderJavBus, jav.ProviderJavDBAPI}
			started := time.Now()
			if err := manager.handleTask(t.Context(), "FC2-PPV-1234567"); err != nil {
				t.Fatal(err)
			}
			if elapsed := time.Since(started); elapsed > 10*time.Second {
				t.Fatalf("provider lookup and download did not share an 8 second budget: %s", elapsed)
			}
			if !slices.Equal(calls, manager.providers) || !manager.Exists("FC2-PPV-1234567") {
				t.Fatalf("fallback did not download a cover: providers=%v", calls)
			}
		})
	}
}

func TestCoverProviderStopsWhenParentCanceled(t *testing.T) {
	originalLookup := lookupJavByCode
	t.Cleanup(func() { lookupJavByCode = originalLookup })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	calls := 0
	lookupJavByCode = func(providerCtx context.Context, code string, provider jav.Provider) (*jav.JavInfo, error) {
		calls++
		cancel()
		<-providerCtx.Done()
		return nil, providerCtx.Err()
	}
	manager := NewCoverManager(t.TempDir())
	if err := manager.handleTask(ctx, "ABC-001"); !errors.Is(err, context.Canceled) {
		t.Fatalf("handleTask error = %v, want context canceled", err)
	}
	if calls != 1 {
		t.Fatalf("provider calls = %d, want 1", calls)
	}
}
