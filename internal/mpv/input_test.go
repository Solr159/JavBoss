package mpv

import (
	"context"
	"strings"
	"testing"

	"javboss/internal/common"
	dbpkg "javboss/internal/db"
)

func TestBuildConfigContentIncludesRequiredDefaults(t *testing.T) {
	prevDB := common.DB
	common.DB = nil
	defer func() {
		common.DB = prevDB
	}()

	content, err := buildConfigContent()
	if err != nil {
		t.Fatalf("buildConfigContent returned error: %v", err)
	}

	if !strings.Contains(content, "keep-open=yes\n") {
		t.Fatalf("expected keep-open=yes in mpv config, got %q", content)
	}
	if !strings.Contains(content, "osc=no\n") {
		t.Fatalf("expected osc=no in mpv config, got %q", content)
	}
	if !strings.Contains(content, "input-default-bindings=no\n") {
		t.Fatalf("expected input-default-bindings=no in mpv config, got %q", content)
	}
	if !strings.Contains(content, "auto-window-resize=no\n") {
		t.Fatalf("expected auto-window-resize=no in fixed-size mpv config, got %q", content)
	}
	if !strings.Contains(content, "ontop=yes\n") {
		t.Fatalf("expected ontop=yes in mpv config, got %q", content)
	}
	if !strings.Contains(content, "osd-playing-msg-duration=5000\n") {
		t.Fatalf("expected osd-playing-msg-duration=5000 in mpv config, got %q", content)
	}
	if !strings.Contains(content, "video-align-y=1\n") {
		t.Fatalf("expected video-align-y=1 in mpv config, got %q", content)
	}
	if !strings.Contains(content, "video-margin-ratio-bottom=0.125\n") {
		t.Fatalf("expected video-margin-ratio-bottom=0.125 in mpv config, got %q", content)
	}
	if !strings.Contains(content, "watch-later-options-remove=sub-pos,osd-margin-y\n") {
		t.Fatalf("expected watch-later overrides in mpv config, got %q", content)
	}
	if !strings.Contains(content, "geometry=80%x80%+50%+50%\n") {
		t.Fatalf("expected centered default geometry in mpv config, got %q", content)
	}
}

func TestBuildInputConfContentIncludesDefaultScreenshotKey(t *testing.T) {
	prevDB := common.DB
	common.DB = nil
	defer func() {
		common.DB = prevDB
	}()

	content, err := buildInputConfContent()
	if err != nil {
		t.Fatalf("buildInputConfContent returned error: %v", err)
	}

	if !strings.Contains(content, "e screenshot\n") {
		t.Fatalf("expected e screenshot in mpv input config, got %q", content)
	}
	if !strings.Contains(content, "q no-osd add volume -5\n") {
		t.Fatalf("expected q no-osd volume down in mpv input config, got %q", content)
	}
	if !strings.Contains(content, "w no-osd add volume 5\n") {
		t.Fatalf("expected w no-osd volume up in mpv input config, got %q", content)
	}
	if !strings.Contains(content, "SPACE cycle pause\n") {
		t.Fatalf("expected SPACE cycle pause in mpv input config, got %q", content)
	}
	if !strings.Contains(content, "ESC quit\n") {
		t.Fatalf("expected ESC quit in mpv input config, got %q", content)
	}
}

func TestBuildStartupHotkeyHintIncludesDefaultHotkeys(t *testing.T) {
	prevDB := common.DB
	common.DB = nil
	defer func() {
		common.DB = prevDB
	}()

	content, err := buildStartupHotkeyHint()
	if err != nil {
		t.Fatalf("buildStartupHotkeyHint returned error: %v", err)
	}

	expected := []string{
		"a：进度 -1 秒",
		"x：进度 +5 秒",
		"q：音量 -5%",
		"w：音量 +5%",
		"e：截图",
		"空格：暂停/继续",
		"ESC：退出",
		"你可在「全局设置 → MPV播放器 → 基础设置」里关闭此信息显示",
	}
	for _, line := range expected {
		if !strings.Contains(content, line) {
			t.Fatalf("expected %q in mpv hotkey hint, got %q", line, content)
		}
	}
}

func TestBuildStartupHotkeyHintCanBeDisabled(t *testing.T) {
	openConfigTestDB(t)
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		playerShowHotkeyHintConfigKey: "false",
	}); err != nil {
		t.Fatalf("upsert config: %v", err)
	}

	content, err := buildStartupHotkeyHint()
	if err != nil {
		t.Fatalf("buildStartupHotkeyHint returned error: %v", err)
	}

	if content != "" {
		t.Fatalf("expected disabled hotkey hint to be empty, got %q", content)
	}
}

func TestBuildConfigContentRespectsConfiguredOntop(t *testing.T) {
	openConfigTestDB(t)
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		playerOntopConfigKey: "false",
	}); err != nil {
		t.Fatalf("upsert config: %v", err)
	}

	content, err := buildConfigContent()
	if err != nil {
		t.Fatalf("buildConfigContent returned error: %v", err)
	}

	if !strings.Contains(content, "ontop=no\n") {
		t.Fatalf("expected ontop=no in mpv config, got %q", content)
	}
}

func TestBuildConfigContentCentersConfiguredWindowSize(t *testing.T) {
	openConfigTestDB(t)
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		playerWindowWidthConfigKey:  "80",
		playerWindowHeightConfigKey: "60",
	}); err != nil {
		t.Fatalf("upsert config: %v", err)
	}

	content, err := buildConfigContent()
	if err != nil {
		t.Fatalf("buildConfigContent returned error: %v", err)
	}

	if !strings.Contains(content, "geometry=80%x60%+50%+50%\n") {
		t.Fatalf("expected centered configured geometry in mpv config, got %q", content)
	}
}

func TestBuildConfigContentUsesOnlyAutofitForAutomaticWindowSize(t *testing.T) {
	openConfigTestDB(t)
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		playerWindowUseAutofitConfigKey: "true",
	}); err != nil {
		t.Fatalf("upsert config: %v", err)
	}

	content, err := buildConfigContent()
	if err != nil {
		t.Fatalf("buildConfigContent returned error: %v", err)
	}

	if !strings.Contains(content, "autofit=80%x80%\n") {
		t.Fatalf("expected default autofit size in mpv config, got %q", content)
	}
	if strings.Contains(content, "auto-window-resize=no\n") {
		t.Fatalf("expected autofit mpv config to leave automatic window resize enabled, got %q", content)
	}
	if strings.Contains(content, "geometry=") {
		t.Fatalf("expected autofit mpv config to omit fixed geometry, got %q", content)
	}
}

func openConfigTestDB(t *testing.T) {
	t.Helper()

	prevDB := common.DB
	db, err := dbpkg.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	common.DB = db
	t.Cleanup(func() {
		common.DB = prevDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

}
