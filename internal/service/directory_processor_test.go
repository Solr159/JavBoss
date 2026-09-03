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

func TestProcessJavItemOrganizesByCompleteCode(t *testing.T) {
	root := t.TempDir()
	videoName := "original-name.mp4"
	writeTestFile(t, filepath.Join(root, videoName), []byte("video"))

	item := models.Jav{
		Code:   "ipx-001",
		Videos: []models.Video{{Path: videoName}},
	}
	summary := &DirectoryProcessSummary{}
	processJavItem(
		t.Context(),
		root,
		&item,
		DirectoryProcessOrganize,
		DirectoryProcessLayoutCode,
		"",
		summary,
	)

	target := filepath.Join(root, "JAV", "IPX-001", videoName)
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("complete-code organized video missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "JAV", "IPX")); !os.IsNotExist(err) {
		t.Fatalf("prefix directory unexpectedly exists: %v", err)
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

func TestCleanupEmptySourceDirectoriesOnlyRemovesMovedSourceAncestors(t *testing.T) {
	root := t.TempDir()
	removeSource := filepath.Join(root, "remove", "leaf", "first.mp4")
	keepSource := filepath.Join(root, "keep", "leaf", "second.mp4")
	unrelatedEmpty := filepath.Join(root, "unrelated-empty")
	writeTestFile(t, removeSource, []byte("first"))
	writeTestFile(t, keepSource, []byte("second"))
	writeTestFile(t, filepath.Join(root, "keep", "leaf", ".DS_Store"), []byte("keep"))
	if err := os.MkdirAll(unrelatedEmpty, 0o755); err != nil {
		t.Fatalf("create unrelated empty directory: %v", err)
	}

	summary := &DirectoryProcessSummary{}
	for _, item := range []models.Jav{
		{Code: "IPX-001", Videos: []models.Video{{Path: "remove/leaf/first.mp4"}}},
		{Code: "IPX-002", Videos: []models.Video{{Path: "keep/leaf/second.mp4"}}},
	} {
		processJavItem(
			t.Context(),
			root,
			&item,
			DirectoryProcessOrganize,
			DirectoryProcessLayoutPrefix,
			"",
			summary,
		)
	}
	cleanupEmptySourceDirectories(root, summary)

	if summary.EmptyDirectoriesRemoved != 2 || len(summary.DirectoryCleanupFailures) != 0 {
		t.Fatalf("cleanup summary = %+v, want two removed directories and no failures", summary)
	}
	if _, err := os.Stat(filepath.Join(root, "remove")); !os.IsNotExist(err) {
		t.Fatalf("empty moved-source ancestors should be removed: %v", err)
	}
	for _, path := range []string{
		root,
		filepath.Join(root, "keep", "leaf"),
		unrelatedEmpty,
	} {
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Fatalf("directory should remain %s: info=%v err=%v", path, info, err)
		}
	}
}

func TestCleanupEmptySourceDirectoriesDoesNotFollowSymlinks(t *testing.T) {
	root := t.TempDir()
	externalRoot := t.TempDir()
	externalLeaf := filepath.Join(externalRoot, "leaf")
	if err := os.MkdirAll(externalLeaf, 0o755); err != nil {
		t.Fatalf("create external directory: %v", err)
	}
	link := filepath.Join(root, "linked")
	if err := os.Symlink(externalRoot, link); err != nil {
		t.Skipf("create directory symlink: %v", err)
	}

	summary := &DirectoryProcessSummary{
		emptyDirectoryCandidates: []string{filepath.Join(link, "leaf")},
	}
	cleanupEmptySourceDirectories(root, summary)

	if summary.EmptyDirectoriesRemoved != 0 || len(summary.DirectoryCleanupFailures) == 0 {
		t.Fatalf("cleanup summary = %+v, want a recorded symlink safety skip", summary)
	}
	if info, err := os.Stat(externalLeaf); err != nil || !info.IsDir() {
		t.Fatalf("external directory should remain: info=%v err=%v", info, err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("source symlink should remain: info=%v err=%v", info, err)
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

func TestProcessJavItemRecordsMoveFailureThatRemainsAtSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "incoming", "same.mp4")
	target := filepath.Join(root, "JAV", "IPX", "IPX-001", "same.mp4")
	writeTestFile(t, source, []byte("source"))
	writeTestFile(t, target, []byte("target"))

	item := models.Jav{
		Code:   "IPX-001",
		Videos: []models.Video{{Path: "incoming/same.mp4"}},
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

	if summary.Failed != 1 || len(summary.MoveFailures) != 1 || len(summary.SidecarFailures) != 0 {
		t.Fatalf("summary = %+v, want one move failure only", summary)
	}
	failure := summary.MoveFailures[0]
	if failure.Code != "IPX-001" || failure.SourcePath != "incoming/same.mp4" ||
		failure.TargetPath != "JAV/IPX/IPX-001/same.mp4" ||
		failure.Reason != "目标位置已存在同名文件" {
		t.Fatalf("move failure = %+v", failure)
	}
	if data, err := os.ReadFile(source); err != nil || string(data) != "source" {
		t.Fatalf("failed source should remain unchanged: data=%q err=%v", data, err)
	}
}

func TestProcessJavItemSeparatesSidecarFailureFromMoveFailure(t *testing.T) {
	root := t.TempDir()
	videoName := "movie.mp4"
	writeTestFile(t, filepath.Join(root, "incoming", videoName), []byte("video"))
	writeTestFile(
		t,
		filepath.Join(root, "incoming", "movie.nfo"),
		[]byte("<movie><title>User metadata</title></movie>"),
	)

	item := models.Jav{
		Code:   "IPX-001",
		Videos: []models.Video{{Path: filepath.ToSlash(filepath.Join("incoming", videoName))}},
	}
	summary := &DirectoryProcessSummary{}
	processJavItem(
		t.Context(),
		root,
		&item,
		DirectoryProcessOrganizeWithSidecar,
		DirectoryProcessLayoutPrefix,
		"",
		summary,
	)

	if summary.Moved != 1 || summary.Failed != 1 || len(summary.MoveFailures) != 0 ||
		len(summary.SidecarFailures) != 1 {
		t.Fatalf("summary = %+v, want a successful move and one Sidecar failure", summary)
	}
	failure := summary.SidecarFailures[0]
	if failure.SourcePath != "JAV/IPX/IPX-001/movie.mp4" ||
		failure.Reason != "已有非 JavBoss 管理的 NFO，未覆盖该文件" {
		t.Fatalf("Sidecar failure = %+v", failure)
	}
	if _, err := os.Stat(filepath.Join(root, "JAV", "IPX", "IPX-001", videoName)); err != nil {
		t.Fatalf("video should have moved before Sidecar failure: %v", err)
	}
}

func TestWriteDirectoryProcessReportOverwritesPreviousReport(t *testing.T) {
	root := t.TempDir()
	reportPath := filepath.Join(root, directoryProcessReportName)
	writeTestFile(t, reportPath, []byte("stale failure"))

	startedAt := time.Date(2026, time.September, 2, 14, 30, 0, 0, time.Local)
	finishedAt := startedAt.Add(18 * time.Second)
	summary := &DirectoryProcessSummary{
		Locations:               5,
		Moved:                   2,
		AlreadyOrganized:        1,
		Sidecars:                1,
		Skipped:                 1,
		Failed:                  2,
		EmptyDirectoriesRemoved: 2,
		MoveFailures: []DirectoryProcessIssue{{
			Code:       "IPX-001",
			SourcePath: "incoming/IPX-001.mp4",
			TargetPath: "JAV/IPX/IPX-001/IPX-001.mp4",
			Reason:     "目标位置已存在同名文件",
		}},
		SkippedItems: []DirectoryProcessIssue{{
			SourcePath: "incoming/unknown.mp4",
			Reason:     "番号为空或无法生成安全的目录名",
		}},
		SidecarFailures: []DirectoryProcessIssue{{
			Code:       "IPX-002",
			SourcePath: "JAV/IPX/IPX-002/IPX-002.mp4",
			Reason:     "已有非 JavBoss 管理的 NFO，未覆盖该文件",
		}},
		DirectoryCleanupFailures: []DirectoryProcessIssue{{
			SourcePath: "incoming/locked",
			Reason:     "没有足够的文件或目录访问权限",
		}},
	}

	if err := writeDirectoryProcessReport(
		root,
		DirectoryProcessOrganizeWithSidecar,
		DirectoryProcessLayoutPrefix,
		summary,
		nil,
		startedAt,
		finishedAt,
	); err != nil {
		t.Fatalf("write directory processing report: %v", err)
	}

	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read directory processing report: %v", err)
	}
	report := string(data)
	for _, expected := range []string{
		"JavBoss 目录整理报告",
		"开始时间：2026-09-02 14:30:00",
		"完成时间：2026-09-02 14:30:18",
		"处理模式：整理并生成 NFO 和封面",
		"整理方式：按番号前缀（JAV/前缀/番号）",
		"参与处理视频：5",
		"成功移动：2",
		"已经位于目标位置：1",
		"未满足整理条件：1",
		"移动失败并留在原处：1",
		"NFO/封面生成失败：1",
		"已删除空目录：2",
		"空目录清理失败：1",
		"源文件：incoming/IPX-001.mp4",
		"目标文件：JAV/IPX/IPX-001/IPX-001.mp4",
		"源文件：incoming/unknown.mp4",
		"源文件：JAV/IPX/IPX-002/IPX-002.mp4",
		"目录：incoming/locked",
	} {
		if !strings.Contains(report, expected) {
			t.Fatalf("processing report missing %q:\n%s", expected, report)
		}
	}
	if strings.Contains(report, "stale failure") {
		t.Fatalf("previous report content was not replaced:\n%s", report)
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
