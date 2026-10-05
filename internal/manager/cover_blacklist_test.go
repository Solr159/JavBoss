package manager

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"javboss/internal/jav"
)

// The fixture contains only the three sampled windows from the real placeholder,
// at offsets 0, 9561, and 19122. Tests do not depend on the runtime data directory.
func placeholderCoverFixture(t *testing.T) []byte {
	t.Helper()
	samples, err := os.ReadFile("testdata/now_printing.samples")
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 768 {
		t.Fatalf("sample fixture length = %d, want 768", len(samples))
	}
	data := make([]byte, 19378)
	for i, offset := range []int{0, 9561, 19122} {
		copy(data[offset:offset+256], samples[i*256:(i+1)*256])
	}
	return data
}

type countingCoverReader struct {
	io.ReaderAt
	bytesRead int
}

func (r *countingCoverReader) ReadAt(p []byte, offset int64) (int, error) {
	n, err := r.ReaderAt.ReadAt(p, offset)
	r.bytesRead += n
	return n, err
}

func TestCoverBlacklistSamplesOnly768Bytes(t *testing.T) {
	data := placeholderCoverFixture(t)
	r := &countingCoverReader{ReaderAt: bytes.NewReader(data)}
	got, err := sampledCoverFingerprint(r, int64(len(data)))
	if err != nil || got != "afc620b085e1833a33b16a37e783938cc5ea9eae8992ecc30f615bc12be2be69" {
		t.Fatalf("sample fingerprint = %q, err=%v", got, err)
	}
	if r.bytesRead != 768 {
		t.Fatalf("read %d bytes, want 768", r.bytesRead)
	}
	// A size not on the blacklist must not even open the file.
	if blocked, err := isBlacklistedCoverFile(filepath.Join(t.TempDir(), "missing.jpg"), 20000); blocked || err != nil {
		t.Fatalf("unknown size: blocked=%t, err=%v", blocked, err)
	}
	if _, err := sampledCoverFingerprint(bytes.NewReader(data[:10000]), int64(len(data))); !errors.Is(err, io.EOF) {
		t.Fatalf("truncated sample error = %v, want EOF", err)
	}
}

func TestFindCoverPathRejectsPlaceholder(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change int
		valid  bool
	}{
		{"placeholder", -1, false},
		{"different header", 100, true},
		{"different center", 9600, true},
		{"different tail", 19200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := placeholderCoverFixture(t)
			if tc.change >= 0 {
				data[tc.change] ^= 0xff
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "mum-184.jpg"), data, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, ok := FindCoverPath(dir, "MUM-184"); ok != tc.valid {
				t.Fatalf("cover found = %t, want %t", ok, tc.valid)
			}
		})
	}
}

func TestDownloadCoverRejectsPlaceholderAndPreservesExistingCover(t *testing.T) {
	data := placeholderCoverFixture(t)
	for _, encoded := range []bool{false, true} {
		name := "plain"
		payload := data
		if encoded {
			name = "encoded"
			payload = make([]byte, len(data)+1)
			payload[0] = 0xad
			for i, b := range data {
				payload[i+1] = b ^ 0xad
			}
		}
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write(payload)
			}))
			defer server.Close()
			dir := t.TempDir()
			existing := bytes.Repeat([]byte("x"), 5120)
			path := filepath.Join(dir, "mum-184.jpg")
			if err := os.WriteFile(path, existing, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := DownloadCoverFromURL(context.Background(), dir, "MUM-184", server.URL+"/cover.jpg"); !errors.Is(err, errInvalidCover) {
				t.Fatalf("download error = %v, want errInvalidCover", err)
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, existing) {
				t.Fatalf("existing cover was changed: %v", err)
			}
			if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("temporary cover remains: %v", err)
			}
		})
	}
}

func TestHandleTaskReplacesPlaceholderUsingFallback(t *testing.T) {
	placeholder := placeholderCoverFixture(t)
	valid := bytes.Repeat([]byte("x"), 5120)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/placeholder.jpg" {
			_, _ = w.Write(placeholder)
			return
		}
		_, _ = w.Write(valid)
	}))
	defer server.Close()
	originalLookup := lookupJavByCode
	t.Cleanup(func() { lookupJavByCode = originalLookup })
	var calls []jav.Provider
	lookupJavByCode = func(_ context.Context, code string, provider jav.Provider) (*jav.JavInfo, error) {
		calls = append(calls, provider)
		if provider == jav.ProviderJavBus {
			return &jav.JavInfo{CoverURL: server.URL + "/placeholder.jpg"}, nil
		}
		return &jav.JavInfo{CoverURL: server.URL + "/valid.jpg"}, nil
	}
	providers := []jav.Provider{jav.ProviderJavBus, jav.ProviderJavDatabase}
	manager := NewCoverManager(t.TempDir())
	path := filepath.Join(manager.coverDir, "mum-184.jpg")
	if err := os.WriteFile(path, placeholder, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := manager.handleTask(context.Background(), "MUM-184"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(calls, providers) {
		t.Fatalf("provider calls = %v, want %v", calls, providers)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, valid) || !manager.Exists("MUM-184") {
		t.Fatalf("placeholder was not replaced with fallback cover: %v", err)
	}
}
