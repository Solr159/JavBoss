package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"javboss/internal/models"
)

func TestOrganizeCodePartsUsesUppercaseDirectories(t *testing.T) {
	code, prefix, ok := organizeCodeParts(" ipx-001 ")
	if !ok {
		t.Fatal("expected valid code")
	}
	if code != "IPX-001" || prefix != "IPX" {
		t.Fatalf("code parts = %q/%q, want IPX/IPX-001", prefix, code)
	}
}

func TestDirectoryProcessModeNormalizesWhitespace(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantMode   string
		wantStatus string
		wantOK     bool
	}{
		{
			name:       "sidecar",
			input:      " sidecar ",
			wantMode:   DirectoryProcessSidecar,
			wantStatus: DirectoryWorkGeneratingSidecar,
			wantOK:     true,
		},
		{
			name:       "organize",
			input:      "\torganize\n",
			wantMode:   DirectoryProcessOrganize,
			wantStatus: DirectoryWorkOrganizing,
			wantOK:     true,
		},
		{
			name:       "organize with sidecar",
			input:      " organize_with_sidecar ",
			wantMode:   DirectoryProcessOrganizeWithSidecar,
			wantStatus: DirectoryWorkOrganizingWithSidecar,
			wantOK:     true,
		},
		{
			name:   "invalid",
			input:  "unknown",
			wantOK: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mode, status, ok := directoryProcessMode(test.input)
			if mode != test.wantMode || status != test.wantStatus || ok != test.wantOK {
				t.Fatalf(
					"directoryProcessMode(%q) = %q, %q, %t; want %q, %q, %t",
					test.input,
					mode,
					status,
					ok,
					test.wantMode,
					test.wantStatus,
					test.wantOK,
				)
			}
		})
	}
}

func TestProcessJavItemOrganizesMediaWithoutRenaming(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "incoming")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	videoName := "ipx001_ch.mp4"
	subtitleName := "ipx001_ch.zh.ass"
	writeTestFile(t, filepath.Join(sourceDir, videoName), []byte("video"))
	writeTestFile(t, filepath.Join(sourceDir, subtitleName), []byte("subtitle"))

	item := models.Jav{
		Code: "ipx-001",
		Videos: []models.Video{{
			Path: filepath.ToSlash(filepath.Join("incoming", videoName)),
		}},
	}
	summary := &DirectoryProcessSummary{}
	processJavItem(
		t.Context(),
		root,
		&item,
		DirectoryProcessOrganize,
		DirectoryProcessLayoutPrefix,
		"",
		summary,
	)

	targetDir := filepath.Join(root, "JAV", "IPX", "IPX-001")
	if _, err := os.Stat(filepath.Join(targetDir, videoName)); err != nil {
		t.Fatalf("organized video missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, subtitleName)); err != nil {
		t.Fatalf("organized subtitle missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sourceDir, videoName)); !os.IsNotExist(err) {
		t.Fatalf("source video still exists or stat failed unexpectedly: %v", err)
	}
	if summary.Moved != 1 || summary.Failed != 0 {
		t.Fatalf("summary = %+v, want one move and no failures", summary)
	}
}

func TestProcessJavItemOrganizesAndWritesJellyfinSidecars(t *testing.T) {
	root := t.TempDir()
	coverDir := t.TempDir()
	videoName := "original-name.mkv"
	writeTestFile(t, filepath.Join(root, videoName), []byte("video"))
	writeTestFile(t, filepath.Join(coverDir, "ipx-001.jpg"), make([]byte, 31*1024))

	uncensored := true
	item := models.Jav{
		Code:         "IPX-001",
		Title:        "测试标题",
		ReleaseUnix:  time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC).Unix(),
		DurationMin:  123,
		IsUncensored: &uncensored,
		Studio:       &models.JavStudio{Name: "测试厂商"},
		Series:       &models.JavSeries{Name: "测试系列"},
		Tags:         []models.JavTag{{Name: "剧情"}},
		Idols:        []models.JavIdol{{Name: "Japanese Name", ChineseName: "中文女优"}},
		Videos:       []models.Video{{Path: videoName}},
	}
	summary := &DirectoryProcessSummary{}
	processJavItem(
		t.Context(),
		root,
		&item,
		DirectoryProcessOrganizeWithSidecar,
		DirectoryProcessLayoutPrefix,
		coverDir,
		summary,
	)

	targetDir := filepath.Join(root, "JAV", "IPX", "IPX-001")
	targetVideo := filepath.Join(targetDir, videoName)
	nfoData, err := os.ReadFile(strings.TrimSuffix(targetVideo, ".mkv") + ".nfo")
	if err != nil {
		t.Fatalf("read generated NFO: %v", err)
	}
	nfo := string(nfoData)
	for _, expected := range []string{
		"<generator>JavBoss</generator>",
		"<title>IPX-001 测试标题</title>",
		"<premiered>2025-01-02</premiered>",
		"<runtime>123</runtime>",
		"<studio>测试厂商</studio>",
		"<name>测试系列</name>",
		"<genre>剧情</genre>",
		"<tag>无码</tag>",
		"<name>中文女优</name>",
	} {
		if !strings.Contains(nfo, expected) {
			t.Fatalf("generated NFO missing %q:\n%s", expected, nfo)
		}
	}
	if _, err := os.Stat(strings.TrimSuffix(targetVideo, ".mkv") + "-poster.jpg"); err != nil {
		t.Fatalf("generated poster missing: %v", err)
	}
	if summary.Moved != 1 || summary.Sidecars != 1 || summary.Failed != 0 {
		t.Fatalf("summary = %+v, want one move, one Sidecar, and no failures", summary)
	}
}

func TestProcessJavItemOrganizesBySortedIdols(t *testing.T) {
	root := t.TempDir()
	videoName := "original-name.mp4"
	writeTestFile(t, filepath.Join(root, videoName), []byte("video"))

	item := models.Jav{
		Code: "ipx-001",
		Idols: []models.JavIdol{
			{Name: "Third Idol", ChineseName: "女优3"},
			{Name: "First Idol", ChineseName: "女优1"},
			{Name: "Second Idol", ChineseName: "女优2"},
		},
		Videos: []models.Video{{Path: videoName}},
	}
	summary := &DirectoryProcessSummary{}
	processJavItem(
		t.Context(),
		root,
		&item,
		DirectoryProcessOrganize,
		DirectoryProcessLayoutIdol,
		"",
		summary,
	)

	target := filepath.Join(root, "JAV", "女优1，女优2，女优3", "IPX-001", videoName)
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("idol-organized video missing: %v", err)
	}
	if summary.Moved != 1 || summary.Failed != 0 {
		t.Fatalf("summary = %+v, want one move and no failures", summary)
	}
}

func TestOrganizeIdolComponentFallsBackToUnknownIdol(t *testing.T) {
	if got := organizeIdolComponent(nil); got != directoryUnknownIdol {
		t.Fatalf("organizeIdolComponent(nil) = %q, want %q", got, directoryUnknownIdol)
	}
	if got := organizeIdolComponent([]models.JavIdol{{Name: " ../ "}}); got != directoryUnknownIdol {
		t.Fatalf("unsafe idol component = %q, want %q", got, directoryUnknownIdol)
	}
}

func TestOrganizeIdolComponentDeduplicatesNames(t *testing.T) {
	idols := []models.JavIdol{
		{Name: "Idol B"},
		{Name: "idol b"},
		{Name: "Idol A"},
	}
	if got := organizeIdolComponent(idols); got != "Idol_A，Idol_B" {
		t.Fatalf("organizeIdolComponent() = %q, want %q", got, "Idol_A，Idol_B")
	}
}

func TestOrganizeIdolComponentGroupsMoreThanThreeIdols(t *testing.T) {
	tests := []struct {
		name  string
		idols []models.JavIdol
		want  string
	}{
		{
			name: "three names remain joined",
			idols: []models.JavIdol{
				{Name: "Idol C"},
				{Name: "Idol A"},
				{Name: "Idol B"},
			},
			want: "Idol_A，Idol_B，Idol_C",
		},
		{
			name: "four names use shared folder",
			idols: []models.JavIdol{
				{Name: "Idol D"},
				{Name: "Idol B"},
				{Name: "Idol A"},
				{Name: "Idol C"},
			},
			want: directoryMultipleIdols,
		},
		{
			name: "duplicate names do not exceed limit",
			idols: []models.JavIdol{
				{Name: "Idol C"},
				{Name: "Idol B"},
				{Name: "Idol A"},
				{Name: "idol a"},
			},
			want: "Idol_A，Idol_B，Idol_C",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := organizeIdolComponent(test.idols); got != test.want {
				t.Fatalf("organizeIdolComponent() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestWriteJavSidecarsDoesNotOverwriteUserNFO(t *testing.T) {
	root := t.TempDir()
	videoPath := filepath.Join(root, "movie.mp4")
	nfoPath := filepath.Join(root, "movie.nfo")
	writeTestFile(t, videoPath, []byte("video"))
	const original = "<movie><title>User metadata</title></movie>"
	writeTestFile(t, nfoPath, []byte(original))

	err := writeJavSidecars(videoPath, &models.Jav{Code: "IPX-001"}, "")
	if err == nil {
		t.Fatal("expected existing user NFO conflict")
	}
	data, readErr := os.ReadFile(nfoPath)
	if readErr != nil {
		t.Fatalf("read user NFO: %v", readErr)
	}
	if string(data) != original {
		t.Fatalf("user NFO was modified: %s", data)
	}
}

func TestMoveMediaGroupSkipsExistingTargetWithoutChangingSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "incoming", "same.mp4")
	target := filepath.Join(root, "JAV", "IPX", "IPX-001", "same.mp4")
	writeTestFile(t, source, []byte("source"))
	writeTestFile(t, target, []byte("target"))

	moved, err := moveMediaGroup(source, target)
	if err == nil {
		t.Fatal("expected target conflict")
	}
	if moved {
		t.Fatal("conflicting source should not move")
	}
	sourceData, readErr := os.ReadFile(source)
	if readErr != nil || string(sourceData) != "source" {
		t.Fatalf("source changed after conflict: data=%q err=%v", sourceData, readErr)
	}
	targetData, readErr := os.ReadFile(target)
	if readErr != nil || string(targetData) != "target" {
		t.Fatalf("target changed after conflict: data=%q err=%v", targetData, readErr)
	}
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create test parent directory: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write test file %s: %v", path, err)
	}
}
