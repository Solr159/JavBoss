package directory

import (
	"context"
	"errors"
	"testing"

	"javboss/internal/models"
	"javboss/internal/util"
)

func TestCleanupExpandedRules(t *testing.T) {
	all := CleanupOptions{IncludeSystem: true, IncludeMisc: true, IncludeTorrents: true, IncludeShortcuts: true, IncludeAdImages: true, IncludeDownloads: true}
	for _, tc := range []struct{ name, reason string }{
		{"notice.MHT", "misc"}, {"notice.mhtml", "misc"}, {"notice.chm", "misc"}, {"notice.rtf", "misc"},
		{"download.TORRENT", "torrent"}, {"movie.mp4 - shortcut.lnk", "shortcut"},
		{"扫码获取最新地址.PNG", "ad_image"}, {"QR-1024.jpg", "ad_image"}, {"影視聯盟.gif", "ad_image"},
		{"宣传图.jpg", "ad_image"}, {"普通截图.jpg", ""}, {"001.jpg", ""}, {"poster.jpg", ""},
		{"广告标题-poster.jpg", ""}, {"广告标题_封面.jpg", ""}, {"宣传图.nfo", ""},
		{"movie.mp4.bt.xltd", "download"}, {"movie.mp4.aria2", "download"}, {"movie.part", "download"}, {"movie.crdownload", "download"},
		{"movie.mp4", ""}, {"movie.mkv", ""}, {"movie.srt", ""}, {"movie.ass", ""},
		{directoryProcessReportName, ""}, {"archive.zip", ""}, {"archive.rar", ""}, {"Desktop.ini", "system"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := cleanupReason(tc.name, all); got != tc.reason {
				t.Fatalf("got %q, want %q", got, tc.reason)
			}
			if got := cleanupReason(tc.name, CleanupOptions{}); got != "" {
				t.Fatalf("disabled category matched: %q", got)
			}
		})
	}
}

func TestCleanupMediaRequiresSpecificEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, probeError, issue string
		eligible                bool
	}{
		{"incomplete MP4 download", "ffprobe: exit status 1: moov atom not found", "", true},
		{"incomplete download", "ffprobe: Invalid data found when processing input", "", true},
		{"playable download", "", "media_file", false},
		{"missing binary", "ffprobe not installed", "probe_failed", false},
		{"read failure", "input/output error", "probe_failed", false},
		{"permission denied", "permission denied", "probe_failed", false},
		{"io error with missing index", "input/output error: moov atom not found", "probe_failed", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eligible, issue := checkCleanupMediaWithProbe(context.Background(), "file.mp4.part", func(ctx context.Context, _ string) (*util.VideoMetadata, error) {
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("probe must have a timeout")
				}
				if tc.probeError != "" {
					return nil, errors.New(tc.probeError)
				}
				return &util.VideoMetadata{}, nil
			})
			if eligible != tc.eligible || issue != tc.issue {
				t.Fatalf("eligible=%v issue=%s", eligible, issue)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	eligible, issue := checkCleanupMediaWithProbe(ctx, "file.mp4.part", func(context.Context, string) (*util.VideoMetadata, error) {
		return nil, errors.New("moov atom not found")
	})
	if eligible || issue != "probe_failed" {
		t.Fatalf("cancelled probe considered evidence: %v %s", eligible, issue)
	}
}

func TestExpandedCleanupPreviewAndDelete(t *testing.T) {
	dir := models.Directory{ID: 1, Path: t.TempDir()}
	m := newCleanupTestDependencies(t).Manager
	for _, name := range []string{"a.torrent", "b.lnk", "c.mht", "二维码.png", "movie.nfo", "poster.jpg", "001.jpg", "archive.zip"} {
		writeCleanupFixture(t, dir.Path, name)
	}
	options := CleanupOptions{IncludeMisc: true, IncludeTorrents: true, IncludeShortcuts: true, IncludeAdImages: true}
	preview, err := m.PreviewDirectoryCleanup(t.Context(), dir, options)
	if err != nil || len(preview.Files) != 4 {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	result, err := m.CleanDirectoryFiles(t.Context(), dir, dir.Path, preview.Files)
	if err != nil || len(result.Deleted) != 4 || len(result.Issues) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	remaining, err := m.PreviewDirectoryCleanup(t.Context(), dir, options)
	if err != nil || len(remaining.Files) != 0 {
		t.Fatalf("remaining=%+v err=%v", remaining, err)
	}
}
