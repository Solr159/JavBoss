package util

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

// TranscodeWorkDirectory is excluded from library scans: it holds temporary
// outputs and original files retained only during the commit/recovery window.
const TranscodeWorkDirectory = ".javboss-transcode"

// BrowserCompatibleVideo uses a conservative common-browser baseline, including
// the actual demuxer and pixel format rather than trusting a filename extension.
func BrowserCompatibleVideo(meta *VideoMetadata) bool {
	if meta == nil || !AssessPlaybackSupport(meta).SupportsDirect {
		return false
	}
	switch meta.Container {
	case "mp4":
		return meta.FormatName == "mov" && (meta.PixelFormat == "yuv420p" || meta.PixelFormat == "yuvj420p")
	case "webm":
		return (meta.FormatName == "matroska" || meta.FormatName == "webm") && meta.PixelFormat == "yuv420p"
	}
	return false
}

type transcodeLog struct{ text string }

func (w *transcodeLog) Write(p []byte) (int, error) {
	w.text += string(p)
	if len(w.text) > 8192 {
		w.text = w.text[len(w.text)-8192:]
	}
	return len(p), nil
}

// TranscodeBrowserMP4 is the software-only entry point. Directory jobs use
// BrowserTranscoder to select hardware automatically. Output must be a new
// temporary path; -n protects any existing file.
func TranscodeBrowserMP4(ctx context.Context, binary, source, output string, progress func(float64, string)) error {
	return transcodeBrowserMP4WithEncoder(ctx, binary, source, output, softwareBrowserEncoder(), nil, progress)
}

func transcodeBrowserMP4Args(source, output string, encoder browserVideoEncoder, meta *VideoMetadata) []string {
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-xerror", "-n"}
	args = append(args, encoder.deviceArgs...)
	args = append(args, "-i", source,
		"-map", "0:v:0", "-map", "0:a?", "-map", "0:s?", "-dn",
		"-map_metadata", "-1")
	args = append(args, encoder.videoArgs(meta)...)
	return append(args,
		"-c:a", "aac", "-profile:a", "aac_low", "-b:a", "192k", "-ac", "2",
		"-c:s", "mov_text",
		"-movflags", "+faststart", "-progress", "pipe:1", "-nostats", "-f", "mp4", output)
}

func transcodeBrowserMP4WithEncoder(ctx context.Context, binary, source, output string, encoder browserVideoEncoder, meta *VideoMetadata, progress func(float64, string)) error {
	cmd := exec.CommandContext(ctx, binary, transcodeBrowserMP4Args(source, output, encoder, meta)...)
	var stderr transcodeLog
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	seconds, speed := 0.0, ""
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), "=")
		if !ok {
			continue
		}
		switch key {
		case "out_time_us":
			if v, err := strconv.ParseFloat(value, 64); err == nil && !math.IsNaN(v) && !math.IsInf(v, 0) {
				seconds = math.Max(0, v/1e6)
			}
		case "speed":
			speed = strings.TrimSpace(value)
		case "progress":
			if progress != nil {
				progress(seconds, speed)
			}
		}
	}
	if scanner.Err() != nil {
		_ = cmd.Process.Kill()
	}
	err = cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(stderr.text))
	}
	return scanner.Err()
}

// ValidateTranscodedVideo fully decodes the output before the source is deleted.
func ValidateTranscodedVideo(ctx context.Context, binary, path string) error {
	cmd := exec.CommandContext(ctx, binary, "-hide_banner", "-loglevel", "error", "-nostdin", "-xerror", "-i", path, "-map", "0:v:0", "-map", "0:a?", "-f", "null", "-")
	var stderr transcodeLog
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("validate converted video: %w: %s", err, strings.TrimSpace(stderr.text))
	}
	return nil
}
