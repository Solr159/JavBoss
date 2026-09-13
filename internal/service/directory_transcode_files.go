package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"javboss/internal/common"
	"javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/util"
)

type transcodeJournal struct {
	DirectoryID int64  `json:"directory_id"`
	LocationID  int64  `json:"location_id"`
	Source      string `json:"source"`
	Target      string `json:"target"`
	Fingerprint string `json:"fingerprint"`
	Published   bool   `json:"published"`
}

func commitTranscodedFile(ctx context.Context, directory models.Directory, location models.VideoLocation, source, target, stage string, meta *util.VideoMetadata) error {
	output := filepath.Join(stage, "output.mp4")
	file, err := os.OpenFile(output, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if syncErr != nil || closeErr != nil {
		return errors.Join(syncErr, closeErr)
	}
	info, err := os.Stat(output)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(directory.Path, target)
	if err != nil {
		return err
	}
	journal := transcodeJournal{DirectoryID: directory.ID, LocationID: location.ID, Source: location.RelativePath, Target: filepath.ToSlash(relative), Fingerprint: meta.FingerprintV2(info.Size())}
	data, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	journalPath := filepath.Join(stage, "journal.json")
	file, err = os.OpenFile(journalPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	syncErr = file.Sync()
	closeErr = file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		_ = os.Remove(journalPath)
		return err
	}
	// The source only lives here during commit. Successful jobs delete it.
	if err := os.Rename(source, filepath.Join(stage, "original")); err != nil {
		_ = os.Remove(journalPath)
		return err
	}
	if err := publishTranscodeFile(output, target); err != nil {
		// Publication did not succeed, so any target belongs to somebody else.
		if _, checkErr := os.Lstat(source); !errors.Is(checkErr, os.ErrNotExist) {
			return fmt.Errorf("publish failed and source path is occupied; original retained at %s: %w", stage, err)
		}
		if rollbackErr := os.Rename(filepath.Join(stage, "original"), source); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		_ = os.Remove(journalPath)
		return err
	}
	journal.Published = true
	data, err = json.Marshal(journal)
	if err == nil {
		err = writeFileAtomically(journalPath, strings.NewReader(string(data)), 0600)
	}
	if err != nil {
		return errors.Join(err, recoverTranscodeStage(context.Background(), stage))
	}
	var copiedAssets []string
	err = db.CompleteVideoTranscode(ctx, location, journal.Target, journal.Fingerprint, info.Size(), int64(math.Round(meta.DurationSeconds)), info.ModTime().UTC(), func(sourceID, targetID int64) error { return copyTranscodeAssets(sourceID, targetID, &copiedAssets) })
	if err != nil {
		for i := len(copiedAssets) - 1; i >= 0; i-- {
			_ = os.Remove(copiedAssets[i])
		}
		return errors.Join(fmt.Errorf("save converted video: %w", err), recoverTranscodeStage(context.Background(), stage))
	}
	// Recovery consults the committed DB row before removing the source. Keeping
	// this journal until cleanup completes also handles a process exit mid-commit.
	if err := os.Remove(filepath.Join(stage, "original")); err != nil {
		return fmt.Errorf("converted, but source cleanup needs retry: %w", err)
	}
	return os.Remove(journalPath)
}

// Use a hard link for atomic no-clobber publication where supported. FAT/network
// filesystems may require a copy; O_EXCL still prevents overwriting another file.
func publishTranscodeFile(source, target string) error {
	if err := os.Link(source, target); err == nil {
		return nil
	}
	return copyExclusiveFile(source, target)
}

func copyExclusiveFile(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	if err := errors.Join(copyErr, syncErr, closeErr); err != nil {
		_ = os.Remove(target)
		return err
	}
	if err := os.Chtimes(target, info.ModTime(), info.ModTime()); err != nil {
		_ = os.Remove(target)
		return err
	}
	return nil
}

func copyTranscodeAssets(sourceID, targetID int64, copied *[]string) error {
	if common.AppConfig == nil {
		return nil
	}
	root := filepath.Join(filepath.Dir(common.AppConfig.DatabasePath), "video")
	source := filepath.Join(root, strconv.FormatInt(sourceID, 10))
	if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	target := filepath.Join(root, strconv.FormatInt(targetID, 10))
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			err := os.Mkdir(destination, 0755)
			if err == nil {
				*copied = append(*copied, destination)
				return nil
			}
			if errors.Is(err, os.ErrExist) {
				return nil
			}
			return err
		}
		if !entry.Type().IsRegular() {
			return errors.New("video asset is not a regular file")
		}
		if err := copyExclusiveFile(path, destination); err == nil {
			*copied = append(*copied, destination)
		} else if !errors.Is(err, os.ErrExist) {
			return err
		}
		return nil
	})
}

// Called before a scan can see an interrupted file replacement. No schema or
// long-term backups are needed: the journal is removed once committed or rolled back.
func recoverDirectoryTranscodes(ctx context.Context, directory models.Directory) error {
	return filepath.WalkDir(directory.Path, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() || entry.Name() != util.TranscodeWorkDirectory {
			return nil
		}
		jobs, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, job := range jobs {
			if !job.IsDir() {
				continue
			}
			if err := recoverTranscodeStage(ctx, filepath.Join(path, job.Name())); err != nil {
				return fmt.Errorf("recover conversion %s: %w", path, err)
			}
		}
		_ = os.Remove(path)
		return filepath.SkipDir
	})
}

func recoverTranscodeStage(ctx context.Context, stage string) error {
	data, err := os.ReadFile(filepath.Join(stage, "journal.json"))
	if errors.Is(err, os.ErrNotExist) {
		// Only our known temporary artifacts are cleaned. An original without a
		// journal must be retained for manual recovery.
		if _, err := os.Lstat(filepath.Join(stage, "original")); !errors.Is(err, os.ErrNotExist) {
			return errors.New("original file found without a recovery journal")
		}
		_ = os.Remove(filepath.Join(stage, "output.mp4"))
		_ = os.Remove(stage)
		return nil
	}
	if err != nil {
		return err
	}
	var journal transcodeJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		return err
	}
	directory, err := db.GetDirectory(ctx, journal.DirectoryID)
	if err != nil || directory == nil {
		return errors.New("recovery directory missing")
	}
	source, err := safeDirectoryFilePath(directory.Path, journal.Source)
	if err != nil {
		return err
	}
	target, err := safeDirectoryFilePath(directory.Path, journal.Target)
	if err != nil {
		return err
	}
	if filepath.Dir(source) != filepath.Dir(target) || filepath.Dir(filepath.Dir(stage)) != filepath.Dir(source) {
		return errors.New("recovery journal paths do not match staging directory")
	}
	var location models.VideoLocation
	if err := common.DB.WithContext(ctx).Preload("Video").First(&location, journal.LocationID).Error; err != nil {
		return err
	}
	if location.DirectoryID != directory.ID {
		return errors.New("recovery location changed directories")
	}
	backup := filepath.Join(stage, "original")
	committed := location.RelativePath == journal.Target && location.Video.Fingerprint == journal.Fingerprint
	if committed {
		if err := verifyTranscodeFingerprint(ctx, target, journal.Fingerprint); err != nil {
			return fmt.Errorf("retain original because committed output is unavailable: %w", err)
		}
		if err := os.Remove(backup); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else {
		if location.RelativePath != journal.Source {
			return errors.New("recovery location was moved; original retained")
		}
		if _, err := os.Stat(backup); err == nil {
			if _, err := os.Lstat(target); err == nil {
				outputInfo, outputErr := os.Stat(filepath.Join(stage, "output.mp4"))
				targetInfo, targetErr := os.Lstat(target)
				linked := outputErr == nil && targetErr == nil && os.SameFile(outputInfo, targetInfo)
				if !journal.Published && !linked {
					return errors.New("unconfirmed output publication; original retained")
				}
				// Never remove a file that does not match our validated output.
				if err := verifyTranscodeFingerprint(ctx, target, journal.Fingerprint); err != nil {
					return err
				}
				if err := os.Remove(target); err != nil {
					return err
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if _, err := os.Lstat(source); !errors.Is(err, os.ErrNotExist) {
				return errors.New("source path occupied during rollback")
			}
			if err := os.Rename(backup, source); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.Remove(filepath.Join(stage, "journal.json")); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(stage, "output.mp4"))
	_ = os.Remove(stage)
	return nil
}

func verifyTranscodeFingerprint(ctx context.Context, path, fingerprint string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("converted path is not a regular file")
	}
	meta, err := util.ProbeVideoContext(ctx, path)
	if err != nil {
		return err
	}
	if meta.FingerprintV2(info.Size()) != fingerprint {
		return errors.New("converted file fingerprint changed; original retained")
	}
	return nil
}
