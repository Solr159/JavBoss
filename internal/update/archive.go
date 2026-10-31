package update

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

const maxArchiveSize int64 = 2 << 30
const maxExpandedSize int64 = 4 << 30

type progressWriter struct {
	io.Writer
	report func(int64)
	count  int64
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	w.count += int64(n)
	w.report(w.count)
	return n, err
}

func downloadArchive(ctx context.Context, client *http.Client, a asset, target string, progress func(int64, int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download release: HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxArchiveSize {
		return fmt.Errorf("release archive exceeds size limit")
	}
	total := resp.ContentLength
	if total < 0 {
		total = 0
	}
	progress(0, total)
	f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	w := &progressWriter{Writer: f, report: func(n int64) { progress(n, total) }}
	n, err := io.Copy(w, io.LimitReader(resp.Body, maxArchiveSize+1))
	if err != nil {
		return fmt.Errorf("download release: %w", err)
	}
	if n == 0 || n > maxArchiveSize {
		return fmt.Errorf("invalid release archive size: %d", n)
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return fmt.Errorf("release size mismatch: got %d, want %d", n, resp.ContentLength)
	}
	return f.Sync()
}

// Never extract links or allow a ZIP entry to escape the staging directory.
// Packaged config.toml is deliberately excluded from installation below.
func extractArchive(ctx context.Context, archive, dest string, allowedRoots ...string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("open release archive: %w", err)
	}
	defer r.Close()
	if len(r.File) > 50000 {
		return fmt.Errorf("too many archive entries")
	}
	// Accept either release layout, but never combine multiple roots in one ZIP.
	var root string
	if len(r.File) > 0 {
		root, _, _ = strings.Cut(r.File[0].Name, "/")
		if !slices.Contains(allowedRoots, root) {
			return fmt.Errorf("invalid archive root %q", root)
		}
	}
	seen := make(map[string]bool)
	var expanded int64
	for _, entry := range r.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := strings.TrimSuffix(entry.Name, "/")
		if strings.ContainsAny(name, `\:`) || path.Clean(name) != name ||
			(name != root && !strings.HasPrefix(name, root+"/")) {
			return fmt.Errorf("invalid archive path %q", entry.Name)
		}
		for _, component := range strings.Split(name, "/") {
			if !portableArchiveName(component) {
				return fmt.Errorf("unsupported archive path %q", entry.Name)
			}
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(name, root), "/")
		key := strings.ToLower(rel)
		if seen[key] {
			return fmt.Errorf("duplicate archive path %q", entry.Name)
		}
		seen[key] = true
		mode := entry.Mode()
		if !mode.IsRegular() && !mode.IsDir() {
			return fmt.Errorf("unsupported archive entry %q", entry.Name)
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if mode.IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if entry.UncompressedSize64 > uint64(maxExpandedSize-expanded) {
			return fmt.Errorf("release archive is too large")
		}
		expanded += int64(entry.UncompressedSize64)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := extractFile(entry, target); err != nil {
			return err
		}
	}
	return nil
}

// Windows strips trailing spaces/dots and recognizes device names even with an
// extension. Reject these on every platform so an archive cannot alias another
// path (in particular a parent directory) when installed on Windows.
func portableArchiveName(name string) bool {
	if name == "" || strings.TrimRight(name, ". ") != name {
		return false
	}
	for _, c := range name {
		if c < 32 || strings.ContainsRune(`<>"|?*`, c) {
			return false
		}
	}
	base, _, _ := strings.Cut(strings.ToUpper(name), ".")
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return false
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		return false
	}
	return true
}

func extractFile(entry *zip.File, target string) error {
	src, err := entry.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	mode := os.FileMode(0644)
	if entry.Mode().Perm()&0111 != 0 {
		mode = 0755
	}
	dst, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(dst, io.LimitReader(src, int64(entry.UncompressedSize64)+1))
	closeErr := dst.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func executableName() string {
	if platformOS() == "windows" {
		return "javboss.exe"
	}
	return "javboss"
}

func validatePayload(root string) error {
	for _, rel := range []string{executableName(), "web/dist/index.html"} {
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("release is missing %s", rel)
		}
	}
	return os.Chmod(filepath.Join(root, executableName()), 0755)
}
