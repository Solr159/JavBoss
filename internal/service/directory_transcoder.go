package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"javboss/internal/common"
	"javboss/internal/common/logging"
	"javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/util"
)

const DirectoryProcessTranscode = "transcode"
const DirectoryWorkTranscoding = "transcoding"
const transcodeReportName = "JavBoss-转码报告.txt"

var ErrTranscodeToolsUnavailable = errors.New("ffmpeg or ffprobe unavailable")
var transcodeJobsMu sync.Mutex
var transcodeJobs = map[int64]*directoryTranscodeJob{}

type TranscodeIssue struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// DirectoryTranscodeProgress is polled with GET /directories. Counters refer to
// candidate files; current_percent remains below 100 until validation and commit.
type DirectoryTranscodeProgress struct {
	StartedAtUnixMS       int64            `json:"started_at_unix_ms"`
	Phase                 string           `json:"phase"`
	Total                 int              `json:"total"`
	Processed             int              `json:"processed"`
	Converted             int              `json:"converted"`
	Skipped               int              `json:"skipped"`
	Failed                int              `json:"failed"`
	CurrentFile           string           `json:"current_file"`
	CurrentPercent        float64          `json:"current_percent"`
	DurationSeconds       float64          `json:"duration_seconds"`
	EncodedSeconds        float64          `json:"encoded_seconds"`
	Speed                 string           `json:"speed"`
	Encoder               string           `json:"encoder,omitempty"`
	HardwareAcceleration  bool             `json:"hardware_acceleration"`
	EncoderFallbackReason string           `json:"encoder_fallback_reason,omitempty"`
	HardwareError         string           `json:"hardware_error,omitempty"`
	ElapsedMS             int64            `json:"elapsed_ms"`
	Error                 string           `json:"error,omitempty"`
	Issues                []TranscodeIssue `json:"issues,omitempty"`
}

type directoryTranscodeJob struct {
	mu         sync.Mutex
	progress   DirectoryTranscodeProgress
	started    time.Time
	cancel     context.CancelFunc
	transcoder *util.BrowserTranscoder
}

func (j *directoryTranscodeJob) update(f func(*DirectoryTranscodeProgress)) {
	j.mu.Lock()
	defer j.mu.Unlock()
	f(&j.progress)
	j.progress.ElapsedMS = time.Since(j.started).Milliseconds()
}

func DirectoryTranscodeSnapshot(id int64) *DirectoryTranscodeProgress {
	transcodeJobsMu.Lock()
	job := transcodeJobs[id]
	transcodeJobsMu.Unlock()
	if job == nil {
		return nil
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	snapshot := job.progress
	// Bound the API response; the report retains every failure.
	count := min(len(snapshot.Issues), 100)
	snapshot.Issues = append([]TranscodeIssue(nil), snapshot.Issues[:count]...)
	if job.cancel != nil {
		snapshot.ElapsedMS = time.Since(job.started).Milliseconds()
	}
	return &snapshot
}

func CancelDirectoryTranscode(id int64) bool {
	transcodeJobsMu.Lock()
	job := transcodeJobs[id]
	transcodeJobsMu.Unlock()
	if job == nil {
		return false
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.cancel == nil {
		return false
	}
	job.cancel()
	return true
}

func startDirectoryTranscode(ctx context.Context, directory models.Directory) error {
	if common.DB == nil || directory.ID <= 0 || directory.IsDelete {
		return errors.New("invalid directory or database")
	}
	binary, err := util.ResolveFFmpegPath()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTranscodeToolsUnavailable, err)
	}
	if _, err := util.ResolveFFprobePath(); err != nil {
		return fmt.Errorf("%w: %v", ErrTranscodeToolsUnavailable, err)
	}
	directoryProcessingMu.Lock()
	if directoryProcessingStatus[directory.ID] != "" {
		directoryProcessingMu.Unlock()
		return ErrDirectoryWorkInProgress
	}
	directoryProcessingStatus[directory.ID] = DirectoryWorkTranscoding
	directoryProcessingMu.Unlock()
	release, err := reserveTranscodeDirectories(ctx, directory)
	if err != nil {
		setDirectoryProcessingStatus(directory.ID, "")
		return err
	}
	// Recheck after reservation in case an edit completed while startup was waiting.
	current, err := db.GetDirectory(ctx, directory.ID)
	if err != nil || current == nil || current.IsDelete || current.Path != directory.Path {
		release()
		setDirectoryProcessingStatus(directory.ID, "")
		return errors.New("directory changed while starting conversion")
	}
	jobCtx, cancel := context.WithCancel(context.Background())
	job := &directoryTranscodeJob{started: time.Now(), cancel: cancel, progress: DirectoryTranscodeProgress{Phase: "discovering"}}
	job.progress.StartedAtUnixMS = job.started.UnixMilli()
	transcodeJobsMu.Lock()
	transcodeJobs[directory.ID] = job
	transcodeJobsMu.Unlock()
	go func() {
		defer release()
		defer setDirectoryProcessingStatus(directory.ID, "")
		defer cancel()
		err := runDirectoryTranscode(jobCtx, directory, binary, job)
		job.update(func(p *DirectoryTranscodeProgress) {
			p.Phase = "completed"
			p.CurrentFile, p.Speed = "", ""
			if err != nil {
				p.Phase, p.Error = "failed", err.Error()
				if errors.Is(err, context.Canceled) {
					p.Phase = "cancelled"
					p.Error = ""
				}
			}
		})
		job.mu.Lock()
		job.cancel = nil
		job.mu.Unlock()
		if err != nil {
			logging.Error("directory transcode ended id=%d err=%v", directory.ID, err)
		}
		if reportErr := writeTranscodeReport(directory.Path, job); reportErr != nil {
			job.update(func(p *DirectoryTranscodeProgress) { p.Error = fmt.Sprintf("write transcode report: %v", reportErr) })
		}
	}()
	return nil
}

// Overlapping registered roots describe the same physical files. Reserve their
// scans too so they cannot observe files between publication and database commit.
func reserveTranscodeDirectories(ctx context.Context, directory models.Directory) (func(), error) {
	dirs, err := db.ListActiveDirectories(ctx)
	if err != nil {
		return nil, err
	}
	var releases []func()
	release := func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}
	for _, dir := range dirs {
		a, b := directory.Path, dir.Path
		if resolved, err := filepath.EvalSymlinks(a); err == nil {
			a = resolved
		}
		if resolved, err := filepath.EvalSymlinks(b); err == nil {
			b = resolved
		}
		if !transcodePathContains(a, b) && !transcodePathContains(b, a) {
			continue
		}
		reserved, err := CancelAndReserveDirectoryScan(ctx, dir.ID)
		if err != nil {
			release()
			return nil, err
		}
		releases = append(releases, reserved)
	}
	return release, nil
}

func transcodePathContains(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func runDirectoryTranscode(ctx context.Context, directory models.Directory, binary string, job *directoryTranscodeJob) error {
	if err := recoverDirectoryTranscodes(ctx, directory); err != nil {
		return err
	}
	locations, err := db.VideoLocationsByDirectory(ctx, directory.ID)
	if err != nil {
		return err
	}
	byPath := make(map[string]models.VideoLocation, len(locations))
	for _, location := range locations {
		byPath[location.RelativePath] = location
	}
	var paths []string
	err = filepath.WalkDir(directory.Path, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == util.TranscodeWorkDirectory {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() || !util.IsVideoCandidate(path) {
			return nil
		}
		paths = append(paths, path)
		job.update(func(p *DirectoryTranscodeProgress) { p.Total = len(paths) })
		return nil
	})
	if err != nil {
		return fmt.Errorf("enumerate directory: %w", err)
	}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(directory.Path, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		job.update(func(p *DirectoryTranscodeProgress) {
			p.Phase, p.CurrentFile = "probing", relative
			p.CurrentPercent, p.DurationSeconds, p.EncodedSeconds, p.Speed = 0, 0, 0, ""
		})
		converted, err := transcodeDirectoryFile(ctx, directory, binary, relative, byPath[relative], job)
		if errors.Is(err, context.Canceled) {
			return err
		}
		job.update(func(p *DirectoryTranscodeProgress) {
			p.Processed++
			if err != nil {
				p.Failed++
				p.Issues = append(p.Issues, TranscodeIssue{Path: relative, Error: err.Error()})
			} else if converted {
				p.Converted++
			} else {
				p.Skipped++
			}
		})
	}
	return nil
}

func transcodeDirectoryFile(ctx context.Context, directory models.Directory, binary, relative string, location models.VideoLocation, job *directoryTranscodeJob) (bool, error) {
	source, err := safeDirectoryFilePath(directory.Path, relative)
	if err != nil {
		return false, err
	}
	info, err := os.Lstat(source)
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, errors.New("source is not a regular file")
	}
	probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	meta, err := util.ProbeVideoContext(probeCtx, source)
	cancel()
	if err != nil {
		return false, err
	}
	if util.BrowserCompatibleVideo(meta) {
		return false, nil
	}
	targetRelative := strings.TrimSuffix(relative, filepath.Ext(relative)) + ".mp4"
	target, err := safeDirectoryFilePath(directory.Path, targetRelative)
	if err != nil {
		return false, err
	}
	if source != target {
		targetInfo, err := os.Lstat(target)
		if err == nil && strings.EqualFold(source, target) && os.SameFile(info, targetInfo) {
			target, targetRelative = source, relative
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("target already exists or is inaccessible: %s", targetRelative)
		}
	}
	fingerprint := meta.FingerprintV2(info.Size())
	if location.ID == 0 {
		video, err := db.GetVideoByFingerprint(ctx, fingerprint)
		if err != nil {
			return false, err
		}
		if video == nil {
			video = &models.Video{Fingerprint: fingerprint, Size: info.Size(), DurationSec: int64(math.Round(meta.DurationSeconds))}
			if err := db.CreateVideo(ctx, video); err != nil {
				video, err = db.GetVideoByFingerprint(ctx, fingerprint)
				if err != nil || video == nil {
					return false, errors.New("cannot register source video")
				}
			}
		}
		saved, err := db.UpsertVideoLocation(ctx, video.ID, directory.ID, relative, info.ModTime().UTC())
		if err != nil {
			return false, err
		}
		location = *saved
	} else if location.Video.Fingerprint != fingerprint {
		return false, errors.New("source changed since its last scan; scan the directory and retry")
	}
	// Stage beside the source to support directories containing mounted filesystems.
	workRoot := filepath.Join(filepath.Dir(source), util.TranscodeWorkDirectory)
	if err := os.Mkdir(workRoot, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return false, err
	}
	workInfo, err := os.Lstat(workRoot)
	if err != nil || !workInfo.IsDir() {
		return false, errors.New("invalid transcode work directory")
	}
	stage, err := os.MkdirTemp(workRoot, "job-")
	if err != nil {
		return false, err
	}
	keepStage := false
	defer func() {
		if !keepStage {
			_ = os.RemoveAll(stage)
			_ = os.Remove(workRoot)
		}
	}()
	output := filepath.Join(stage, "output.mp4")
	job.update(func(p *DirectoryTranscodeProgress) {
		p.Phase = "selecting_encoder"
		p.DurationSeconds = meta.DurationSeconds
	})
	if job.transcoder == nil {
		job.transcoder = util.NewBrowserTranscoder(binary)
	}
	var outputMeta *util.VideoMetadata
	err = job.transcoder.Transcode(ctx, source, output, util.BrowserTranscodeOptions{
		Metadata: meta,
		Progress: func(seconds float64, speed string) {
			job.update(func(p *DirectoryTranscodeProgress) {
				p.EncodedSeconds, p.Speed = seconds, speed
				if meta.DurationSeconds > 0 {
					p.CurrentPercent = math.Min(99, 100*seconds/meta.DurationSeconds)
				}
			})
		},
		EncoderChanged: func(status util.BrowserEncoderStatus) {
			changed := false
			job.update(func(p *DirectoryTranscodeProgress) {
				changed = p.Encoder != status.Name || p.EncoderFallbackReason != status.FallbackReason
				p.Encoder, p.HardwareAcceleration = status.Name, status.Hardware
				p.EncoderFallbackReason, p.HardwareError = status.FallbackReason, status.Detail
				p.Phase, p.CurrentPercent, p.EncodedSeconds, p.Speed = "transcoding", 0, 0, ""
			})
			if changed {
				logging.Info("directory transcode encoder id=%d encoder=%s hardware=%t fallback=%s detail=%s", directory.ID, status.Name, status.Hardware, status.FallbackReason, status.Detail)
			}
		},
		Validate: func(ctx context.Context, output string) error {
			job.update(func(p *DirectoryTranscodeProgress) { p.Phase = "verifying" })
			var probeErr error
			outputMeta, probeErr = util.ProbeVideoContext(ctx, output)
			if probeErr != nil {
				return probeErr
			}
			if !util.BrowserCompatibleVideo(outputMeta) || outputMeta.DurationSeconds <= 0 || (meta.AudioCodec != "" && outputMeta.AudioCodec == "") {
				return errors.New("converted file failed browser compatibility validation")
			}
			if meta.DurationSeconds > 0 && math.Abs(outputMeta.DurationSeconds-meta.DurationSeconds) > math.Max(2, meta.DurationSeconds*0.01) {
				return errors.New("converted duration differs from source")
			}
			return util.ValidateTranscodedVideo(ctx, binary, output)
		},
	})
	if err != nil {
		return false, err
	}
	after, err := os.Lstat(source)
	if err != nil || !os.SameFile(info, after) || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return false, errors.New("source changed during conversion")
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := os.Chmod(output, info.Mode().Perm()); err != nil {
		return false, err
	}
	job.update(func(p *DirectoryTranscodeProgress) { p.Phase = "finalizing" })
	// Finish the short filesystem/DB commit even if cancellation arrives now.
	commitCtx, commitCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer commitCancel()
	err = commitTranscodedFile(commitCtx, directory, location, source, target, stage, outputMeta)
	if err != nil {
		// A journal means rollback/cleanup still needs recovery; never remove its source.
		_, journalErr := os.Stat(filepath.Join(stage, "journal.json"))
		keepStage = journalErr == nil
		return false, err
	}
	job.update(func(p *DirectoryTranscodeProgress) { p.CurrentPercent = 100 })
	return true, nil
}

func writeTranscodeReport(root string, job *directoryTranscodeJob) error {
	job.mu.Lock()
	snapshot := job.progress
	job.mu.Unlock()
	var report strings.Builder
	fmt.Fprintf(&report, "编码器 / Encoder: %s\n硬件编码 / Hardware encoding: %t\n回退原因 / Fallback: %s\n%s\n", snapshot.Encoder, snapshot.HardwareAcceleration, snapshot.EncoderFallbackReason, snapshot.HardwareError)
	fmt.Fprintf(&report, "JavBoss 转码报告 / Transcode report\n状态 / Status: %s\n总数 / Total: %d\n成功 / Converted: %d\n兼容已跳过 / Already compatible: %d\n失败 / Failed: %d\n%s\n", snapshot.Phase, snapshot.Total, snapshot.Converted, snapshot.Skipped, snapshot.Failed, snapshot.Error)
	for _, issue := range snapshot.Issues {
		fmt.Fprintf(&report, "\n%s\n%s\n", issue.Path, issue.Error)
	}
	return writeFileAtomically(filepath.Join(root, transcodeReportName), strings.NewReader(report.String()), 0644)
}
