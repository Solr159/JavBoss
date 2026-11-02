package directory

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"javboss/internal/db"
	"javboss/internal/util"
)

func isCleanupAdImage(name string) bool {
	switch path.Ext(name) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
	default:
		return false
	}
	stem := strings.TrimSuffix(name, path.Ext(name))
	// Preserve conventional media-library artwork even when a title contains an ad keyword.
	for _, kind := range []string{"poster", "fanart", "thumb", "cover", "folder", "banner", "landscape", "clearart", "logo", "封面", "海报", "海報"} {
		if stem == kind || strings.HasSuffix(stem, "-"+kind) || strings.HasSuffix(stem, "_"+kind) {
			return false
		}
	}
	for _, keyword := range []string{"二维码", "二維碼", "扫码", "掃碼", "最新地址", "地址发布", "地址發佈", "发布器", "發佈器", "宣传", "宣傳", "文宣", "論壇", "论坛", "影视联盟", "影視聯盟", "招聘", "招收", "推广", "推廣", "广告", "廣告", "qr-", "qr_"} {
		if strings.Contains(stem, keyword) {
			return true
		}
	}
	return false
}

func isCleanupDownload(name string) bool {
	switch path.Ext(name) {
	case ".xltd", ".aria2", ".part", ".crdownload":
		return true
	default:
		return false
	}
}

// Protect indexed paths in overlapping library roots, including symlink aliases.
func cleanupIndexedPaths(ctx context.Context, root string) (map[string]bool, error) {
	dirs, err := db.Default().ListDirectories(ctx)
	if err != nil {
		return nil, err
	}
	locations, err := db.Default().AllVideoLocations(ctx)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve cleanup root: %w", err)
	}
	roots := make(map[int64]string, len(dirs))
	for _, dir := range dirs {
		roots[dir.ID] = dir.Path
	}
	resolvedRoots := make(map[int64]string)
	indexed := make(map[string]bool)
	for _, loc := range locations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		base, ok := roots[loc.DirectoryID]
		if !ok {
			continue
		}
		resolvedBase, ok := resolvedRoots[loc.DirectoryID]
		if !ok {
			resolvedBase, err = filepath.EvalSymlinks(base)
			if err != nil {
				// Never silently omit indexed files when their root cannot be resolved.
				return nil, fmt.Errorf("resolve indexed directory %q: %w", base, err)
			}
			resolvedRoots[loc.DirectoryID] = resolvedBase
		}
		rel, err := filepath.Rel(root, filepath.Join(resolvedBase, filepath.FromSlash(loc.RelativePath)))
		if err == nil && filepath.IsLocal(rel) {
			indexed[filepath.ToSlash(rel)] = true
		}
	}
	return indexed, nil
}

func checkCleanupMedia(ctx context.Context, filename string) (bool, string) {
	return checkCleanupMediaWithProbe(ctx, filename, util.ProbeVideoContext)
}

func checkCleanupMediaWithProbe(ctx context.Context, filename string, probe func(context.Context, string) (*util.VideoMetadata, error)) (bool, string) {
	if strings.ToLower(filepath.Ext(filename)) == ".aria2" {
		return true, ""
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := probe(probeCtx, filename)
	if probeCtx.Err() != nil {
		return false, "probe_failed"
	}
	if err == nil {
		return false, "media_file"
	}
	message := strings.ToLower(strings.ReplaceAll(err.Error(), filename, ""))
	for _, failure := range []string{"input/output error", "permission denied", "no such file or directory", "timed out"} {
		if strings.Contains(message, failure) {
			return false, "probe_failed"
		}
	}
	// Only a specific media-format failure is evidence of a cleanup candidate.
	// Missing tools, I/O failures, unsupported codecs and timeouts are never evidence.
	if strings.Contains(message, "moov atom not found") {
		return true, ""
	}
	if strings.Contains(message, "invalid data found when processing input") {
		return true, ""
	}
	return false, "probe_failed"
}
