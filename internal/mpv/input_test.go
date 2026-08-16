package mpv

import (
	"context"
	"runtime"
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
	if !strings.Contains(content, "keepaspect-window=no\n") {
		t.Fatalf("expected keepaspect-window=no in mpv config, got %q", content)
	}
	if !strings.Contains(content, "save-position-on-quit=yes\n") {
		t.Fatalf("expected save-position-on-quit=yes in mpv config, got %q", content)
	}
	if !strings.Contains(content, "resume-playback=yes\n") {
		t.Fatalf("expected resume-playback=yes in mpv config, got %q", content)
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
	if !strings.Contains(content, "ontop=no\n") {
		t.Fatalf("expected ontop=no in mpv config, got %q", content)
	}
	if !strings.Contains(content, "osd-playing-msg-duration=5000\n") {
		t.Fatalf("expected osd-playing-msg-duration=5000 in mpv config, got %q", content)
	}
	if !strings.Contains(content, "video-align-y=0\n") {
		t.Fatalf("expected video-align-y=0 in mpv config, got %q", content)
	}
	if !strings.Contains(content, "video-margin-ratio-bottom=0.105\n") {
		t.Fatalf("expected video-margin-ratio-bottom=0.105 in mpv config, got %q", content)
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
	if !strings.Contains(content, "ESC write-watch-later-config; stop; set window-minimized yes") {
		t.Fatalf("expected ESC to write watch-later before stop in mpv input config, got %q", content)
	}
	if runtime.GOOS == "darwin" && !strings.Contains(content, `; run /bin/sh "`) {
		t.Fatalf("expected ESC to restore focus on macOS, got %q", content)
	}
}

func TestBuildInputConfContentQuitsWhenReuseWindowDisabled(t *testing.T) {
	openConfigTestDB(t)
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		playerReuseWindowConfigKey: "false",
	}); err != nil {
		t.Fatalf("upsert config: %v", err)
	}

	content, err := buildInputConfContent()
	if err != nil {
		t.Fatalf("buildInputConfContent returned error: %v", err)
	}

	if !strings.Contains(content, "ESC quit\n") {
		t.Fatalf("expected ESC quit in mpv input config, got %q", content)
	}
	if strings.Contains(content, "ESC stop; set window-minimized yes\n") {
		t.Fatalf("expected ESC stop/minimize to be disabled, got %q", content)
	}
}

func TestBuildInputConfContentWritesWatchLaterWhenResumeEnabled(t *testing.T) {
	openConfigTestDB(t)
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		playerResumePlaybackConfigKey: "true",
	}); err != nil {
		t.Fatalf("upsert config: %v", err)
	}

	content, err := buildInputConfContent()
	if err != nil {
		t.Fatalf("buildInputConfContent returned error: %v", err)
	}

	if !strings.Contains(content, "ESC write-watch-later-config; stop; set window-minimized yes") {
		t.Fatalf("expected ESC to write watch-later before stop, got %q", content)
	}
}

func TestBuildInputConfContentSkipsWatchLaterWhenResumeDisabled(t *testing.T) {
	openConfigTestDB(t)
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		playerResumePlaybackConfigKey: "false",
	}); err != nil {
		t.Fatalf("upsert config: %v", err)
	}

	content, err := buildInputConfContent()
	if err != nil {
		t.Fatalf("buildInputConfContent returned error: %v", err)
	}

	if !strings.Contains(content, "ESC stop; set window-minimized yes") {
		t.Fatalf("expected ESC stop/minimize without watch-later, got %q", content)
	}
	if strings.Contains(content, "write-watch-later-config") {
		t.Fatalf("expected watch-later write to be disabled, got %q", content)
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
		"ESC：停止播放并最小化",
		"你可在「设置 → 播放器 → MPV播放器」里关闭此信息显示",
	}
	for _, line := range expected {
		if !strings.Contains(content, line) {
			t.Fatalf("expected %q in mpv hotkey hint, got %q", line, content)
		}
	}
}

func TestBuildStartupHotkeyHintQuitsWhenReuseWindowDisabled(t *testing.T) {
	openConfigTestDB(t)
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		playerReuseWindowConfigKey: "false",
	}); err != nil {
		t.Fatalf("upsert config: %v", err)
	}

	content, err := buildStartupHotkeyHint()
	if err != nil {
		t.Fatalf("buildStartupHotkeyHint returned error: %v", err)
	}

	if !strings.Contains(content, "ESC：退出播放器") {
		t.Fatalf("expected ESC quit hint, got %q", content)
	}
	if strings.Contains(content, "ESC：停止播放并最小化") {
		t.Fatalf("expected ESC stop/minimize hint to be disabled, got %q", content)
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
		playerOntopConfigKey: "true",
	}); err != nil {
		t.Fatalf("upsert config: %v", err)
	}

	content, err := buildConfigContent()
	if err != nil {
		t.Fatalf("buildConfigContent returned error: %v", err)
	}

	if !strings.Contains(content, "ontop=yes\n") {
		t.Fatalf("expected ontop=yes in mpv config, got %q", content)
	}
}

func TestBuildConfigContentEnablesResumePlayback(t *testing.T) {
	openConfigTestDB(t)
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		playerResumePlaybackConfigKey: "true",
	}); err != nil {
		t.Fatalf("upsert config: %v", err)
	}

	content, err := buildConfigContent()
	if err != nil {
		t.Fatalf("buildConfigContent returned error: %v", err)
	}

	if !strings.Contains(content, "save-position-on-quit=yes\n") {
		t.Fatalf("expected save-position-on-quit=yes in mpv config, got %q", content)
	}
}

func TestBuildConfigContentDisablesResumePlaybackLoading(t *testing.T) {
	openConfigTestDB(t)
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		playerResumePlaybackConfigKey: "false",
	}); err != nil {
		t.Fatalf("upsert config: %v", err)
	}

	content, err := buildConfigContent()
	if err != nil {
		t.Fatalf("buildConfigContent returned error: %v", err)
	}

	if !strings.Contains(content, "save-position-on-quit=no\n") {
		t.Fatalf("expected save-position-on-quit=no in mpv config, got %q", content)
	}
	if !strings.Contains(content, "resume-playback=no\n") {
		t.Fatalf("expected resume-playback=no in mpv config, got %q", content)
	}
}

func TestLoadConfiguredPlayerReuseWindowDefaultsToTrue(t *testing.T) {
	prevDB := common.DB
	common.DB = nil
	defer func() {
		common.DB = prevDB
	}()

	if !loadConfiguredPlayerReuseWindow() {
		t.Fatal("expected player reuse window to default to true")
	}
}

func TestLoadConfiguredPlayerReuseWindowCanBeDisabled(t *testing.T) {
	openConfigTestDB(t)
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		playerReuseWindowConfigKey: "false",
	}); err != nil {
		t.Fatalf("upsert config: %v", err)
	}

	if loadConfiguredPlayerReuseWindow() {
		t.Fatal("expected player reuse window to be disabled")
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
