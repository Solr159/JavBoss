package directory

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"javboss/internal/models"
	"javboss/internal/util"
)

const MaxCleanupFiles = 10000

// CleanupOptions controls candidate categories. Download checks are opt-in because
// incomplete downloads may still be in use and probing can take time.
type CleanupOptions struct {
	IncludeSystem    bool `json:"include_system"`
	IncludeMisc      bool `json:"include_misc"`
	IncludeTorrents  bool `json:"include_torrents"`
	IncludeShortcuts bool `json:"include_shortcuts"`
	IncludeAdImages  bool `json:"include_ad_images"`
	IncludeDownloads bool `json:"include_downloads"`
}

var ErrInvalidCleanupRequest = errors.New("invalid cleanup request")

// CleanupFile is a relative path and the file metadata observed during preview.
// Modified is a string to preserve nanosecond precision in JavaScript clients.
type CleanupFile struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
	Reason   string `json:"reason"`
}

type CleanupIssue struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type CleanupPreview struct {
	RootPath  string         `json:"root_path"`
	Files     []CleanupFile  `json:"files"`
	Issues    []CleanupIssue `json:"issues"`
	Truncated bool           `json:"truncated"`
}

type CleanupResult struct {
	Deleted      []string       `json:"deleted"`
	DeletedBytes int64          `json:"deleted_bytes"`
	Issues       []CleanupIssue `json:"issues"`
}

func cleanupReason(name string, options CleanupOptions) string {
	name = strings.ToLower(name)
	switch name {
	case ".ds_store", "thumbs.db", "ehthumbs.db", "desktop.ini":
		if options.IncludeSystem {
			return "system"
		}
		return ""
	case strings.ToLower(directoryProcessReportName):
		return ""
	}
	if options.IncludeMisc {
		switch strings.ToLower(path.Ext(name)) {
		case ".txt", ".url", ".html", ".htm", ".mht", ".mhtml", ".chm", ".rtf":
			return "misc"
		}
	}
	ext := path.Ext(name)
	if options.IncludeTorrents && ext == ".torrent" {
		return "torrent"
	}
	if options.IncludeShortcuts && ext == ".lnk" {
		return "shortcut"
	}
	if options.IncludeAdImages && isCleanupAdImage(name) {
		return "ad_image"
	}
	if options.IncludeDownloads && isCleanupDownload(name) {
		return "download"
	}
	return ""
}

// reserveCleanup shares the scan reservation with scanning, organizing and path updates.
// It never interrupts an active task.
func (m *Manager) reserveCleanup(ctx context.Context, id int64, status string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.directoryProcessingMu.Lock()
	defer m.directoryProcessingMu.Unlock()
	m.dirScanMu.Lock()
	defer m.dirScanMu.Unlock()
	if m.directoryProcessingStatus[id] != "" || m.dirScanActive[id] != nil {
		return nil, ErrDirectoryWorkInProgress
	}
	session := &directoryScanSession{done: make(chan struct{}), reserve: true}
	m.dirScanActive[id] = session
	m.directoryProcessingStatus[id] = status
	var once sync.Once
	return func() {
		once.Do(func() {
			m.directoryProcessingMu.Lock()
			defer m.directoryProcessingMu.Unlock()
			m.dirScanMu.Lock()
			defer m.dirScanMu.Unlock()
			delete(m.dirScanActive, id)
			delete(m.directoryProcessingStatus, id)
			close(session.done)
		})
	}, nil
}

func PreviewDirectoryCleanup(ctx context.Context, dir models.Directory, options CleanupOptions) (*CleanupPreview, error) {
	return applicationTasks.Load().PreviewDirectoryCleanup(ctx, dir, options)
}

func (m *Manager) PreviewDirectoryCleanup(ctx context.Context, dir models.Directory, options CleanupOptions) (*CleanupPreview, error) {
	release, err := m.reserveCleanup(ctx, dir.ID, "scanning_junk")
	if err != nil {
		return nil, err
	}
	defer release()
	root, err := os.OpenRoot(dir.Path)
	if err != nil {
		return nil, fmt.Errorf("open cleanup directory: %w", err)
	}
	defer root.Close()
	indexed, err := cleanupIndexedPaths(ctx, dir.Path)
	if err != nil {
		return nil, err
	}
	preview := &CleanupPreview{RootPath: dir.Path, Files: []CleanupFile{}, Issues: []CleanupIssue{}}
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			preview.Issues = append(preview.Issues, CleanupIssue{Path: name, Reason: "unreadable"})
			return nil
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		reason := cleanupReason(entry.Name(), options)
		if reason == "" {
			return nil
		}
		if indexed[name] {
			return nil
		}
		if !validCleanupPath(name) {
			preview.Issues = append(preview.Issues, CleanupIssue{Path: name, Reason: "invalid_path"})
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			preview.Issues = append(preview.Issues, CleanupIssue{Path: name, Reason: "unreadable"})
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if reason == "download" {
			eligible, issue := checkCleanupMedia(ctx, filepath.Join(dir.Path, filepath.FromSlash(name)))
			if issue != "" {
				preview.Issues = append(preview.Issues, CleanupIssue{Path: name, Reason: issue})
			}
			if !eligible {
				return nil
			}
		}
		if len(preview.Files) == MaxCleanupFiles {
			preview.Truncated = true
			return fs.SkipAll
		}
		preview.Files = append(preview.Files, CleanupFile{
			Path: name, Size: info.Size(), Modified: info.ModTime().UTC().Format("2006-01-02T15:04:05.999999999Z"), Reason: reason,
		})
		return nil
	})
	return preview, err
}

func CleanDirectoryFiles(ctx context.Context, dir models.Directory, rootPath string, files []CleanupFile) (*CleanupResult, error) {
	return applicationTasks.Load().CleanDirectoryFiles(ctx, dir, rootPath, files)
}

func validCleanupPath(name string) bool {
	// Preview paths use io/fs slash separators. Localize rejects traversal and
	// OS-specific ambiguities while preserving literal backslashes on POSIX.
	_, err := filepath.Localize(name)
	return err == nil && name != "."
}

func (m *Manager) CleanDirectoryFiles(ctx context.Context, dir models.Directory, rootPath string, files []CleanupFile) (*CleanupResult, error) {
	if rootPath != dir.Path || len(files) == 0 || len(files) > MaxCleanupFiles {
		return nil, ErrInvalidCleanupRequest
	}
	seen := make(map[string]bool, len(files))
	options := CleanupOptions{IncludeSystem: true, IncludeMisc: true, IncludeTorrents: true, IncludeShortcuts: true, IncludeAdImages: true, IncludeDownloads: true}
	// Validate the entire request before removing anything.
	for _, file := range files {
		if !validCleanupPath(file.Path) || cleanupReason(path.Base(file.Path), options) == "" ||
			file.Modified == "" || file.Size < 0 || seen[file.Path] {
			return nil, ErrInvalidCleanupRequest
		}
		reason := cleanupReason(path.Base(file.Path), options)
		if reason == "download" {
			if file.Reason != reason {
				return nil, ErrInvalidCleanupRequest
			}
		}
		seen[file.Path] = true
	}
	release, err := m.reserveCleanup(ctx, dir.ID, "cleaning_junk")
	if err != nil {
		return nil, err
	}
	defer release()
	root, err := os.OpenRoot(dir.Path)
	if err != nil {
		return nil, fmt.Errorf("open cleanup directory: %w", err)
	}
	defer root.Close()
	// Refresh the index after preview, for every category, before deleting anything.
	indexed, err := cleanupIndexedPaths(ctx, dir.Path)
	if err != nil {
		return nil, err
	}
	result := &CleanupResult{Deleted: []string{}, Issues: []CleanupIssue{}}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			result.Issues = append(result.Issues, CleanupIssue{Path: file.Path, Reason: "cancelled"})
			continue
		}
		if indexed[file.Path] {
			result.Issues = append(result.Issues, CleanupIssue{Path: file.Path, Reason: "indexed_video"})
			continue
		}
		reason := cleanupReason(path.Base(file.Path), options)
		if reason == "download" {
			eligible, issue := checkCleanupMedia(ctx, filepath.Join(dir.Path, filepath.FromSlash(file.Path)))
			if !eligible {
				if issue == "" {
					issue = "media_file"
				}
				result.Issues = append(result.Issues, CleanupIssue{Path: file.Path, Reason: issue})
				continue
			}
		}
		// Open each parent separately and reject symlinks, including links within the root.
		// Use the same platform-specific deletion operation as video cards.
		if reason := removeCleanupFile(root, file); reason != "" {
			result.Issues = append(result.Issues, CleanupIssue{Path: file.Path, Reason: reason})
			continue
		}
		result.Deleted = append(result.Deleted, file.Path)
		result.DeletedBytes += file.Size
	}
	return result, nil
}

func removeCleanupFile(root *os.Root, file CleanupFile) string {
	parts := strings.Split(file.Path, "/")
	parent := root
	for _, part := range parts[:len(parts)-1] {
		info, err := parent.Lstat(part)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "changed"
		}
		next, err := parent.OpenRoot(part)
		if err != nil {
			return "unreadable"
		}
		defer next.Close()
		parent = next
	}
	name := parts[len(parts)-1]
	info, err := parent.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() != file.Size ||
		info.ModTime().UTC().Format("2006-01-02T15:04:05.999999999Z") != file.Modified {
		return "changed"
	}
	fullPath := filepath.Join(root.Name(), filepath.FromSlash(file.Path))
	current, err := os.Lstat(fullPath)
	if err != nil || !os.SameFile(info, current) {
		return "changed"
	}
	if err := util.MoveFileToTrash(fullPath); err != nil {
		return "delete_failed"
	}
	return ""
}
