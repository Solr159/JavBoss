package mpv

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"javboss/internal/common/logging"
)

type SubtitleFile struct {
	Path, Title, Language string
	OffsetSec             float64
	Status                string
	Key, Track            string
	Downloaded            bool
}

type SubtitleRequest struct {
	SettingsOnly bool
	Manual       bool
	DownloadOnly bool
	CacheOnly    bool
}

// SettingsOnly reads playback adjustments without searching or downloading.
type SubtitleResolver func(ctx context.Context, request SubtitleRequest) (*SubtitleFile, error)

// SubtitleOffsetSaver persists the offset for the video bound to a playlist entry.
type SubtitleOffsetSaver func(ctx context.Context, offsetMS int64) error

type SubtitleSelectionSaver func(ctx context.Context, track string) error

type subtitleSession struct {
	Generation int64 `json:"generation"`
	Entry      int64 `json:"entry"`
	Loaded     bool  `json:"loaded"`
}

func (e *playlistEvents) subtitleCallback() func() {
	return e.subtitleLoadCallback(false, 0)
}

func (e *playlistEvents) subtitleLoadCallback(manual bool, expectedGeneration int64) func() {
	resolver := e.subtitles[e.entryID]
	if resolver == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	e.subtitleCancel = cancel
	e.subtitleContext = ctx
	entry, endpoint := e.entryID, e.endpoint
	previousSave := e.subtitleSaveDone
	return func() {
		defer cancel()
		if previousSave != nil {
			select {
			case <-previousSave:
			case <-ctx.Done():
				return
			}
		}
		// Lua and JSON IPC listeners receive file-loaded independently. Wait a
		// bounded time for Lua's generation marker before starting network work.
		var session subtitleSession
		ready := false
		for i := 0; i < 20; i++ {
			if ctx.Err() != nil {
				return
			}
			data, err := queryIPCCommand(endpoint, []any{"get_property", "user-data/javboss/subtitle-session"})
			if err == nil && json.Unmarshal(data, &session) == nil && session.Loaded && session.Entry == entry {
				ready = true
				break
			}
			timer := time.NewTimer(25 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		if !ready {
			return
		}
		if manual && session.Generation != expectedGeneration {
			return
		}
		report := func(status string) {
			if err := runIPCCommand(endpoint, []any{"script-message", "javboss-subtitle-status", strconv.FormatInt(session.Generation, 10), strconv.FormatInt(entry, 10), status}); err != nil {
				logging.Error("report playback subtitle status: %v", err)
			}
		}
		failed := func(err error) bool {
			if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
				return true
			}
			if err == nil {
				err = ctx.Err()
			}
			if err != nil {
				logging.Error("resolve playback subtitle: %v", err)
				report("error")
				return true
			}
			return false
		}
		data, err := queryIPCCommand(endpoint, []any{"get_property", "track-list"})
		var tracks []struct {
			Type string `json:"type"`
		}
		if err != nil || json.Unmarshal(data, &tracks) != nil {
			return
		}
		hasSubtitle := false
		for _, track := range tracks {
			if track.Type == "sub" {
				hasSubtitle = true
				break
			}
		}
		// Load the saved delay immediately, even while a subtitle download is pending.
		file, err := resolver(ctx, SubtitleRequest{SettingsOnly: true, Manual: manual})
		if err == nil && file != nil && ctx.Err() == nil {
			err = sendSubtitle(endpoint, session, entry, file)
		}
		if failed(err) {
			return
		}
		// Existing subtitles suppress online downloads, but cached downloads must
		// still be added so the saved selection can be restored.
		cached := SubtitleRequest{DownloadOnly: true, CacheOnly: true}
		request := SubtitleRequest{Manual: manual}
		if hasSubtitle && !manual {
			request = cached
		}
		if manual || (!hasSubtitle && (file == nil || file.Status != "disabled")) {
			report("searching")
		}
		sentSubtitle := false
		for {
			file, err = resolver(ctx, request)
			if failed(err) {
				return
			}
			if file == nil || file.Path == "" {
				if sentSubtitle {
					return
				}
				status := "disabled"
				if file != nil && file.Status != "" {
					status = file.Status
				}
				if hasSubtitle && !manual {
					status = "existing"
				}
				report(status)
				return
			}
			file.Downloaded = file.Downloaded || manual
			if err := sendSubtitle(endpoint, session, entry, file); err != nil {
				logging.Error("load playback subtitle: %v", err)
				report("error")
				return
			}
			sentSubtitle = true
			if !manual && !request.CacheOnly && file.Status == "existing" {
				hasSubtitle = true
				request = cached
				continue
			}
			return
		}
	}
}

// Called under the event lock. Keep one lookup per playback, and validate the
// generation again in the player before applying a manually requested result.
func (e *playlistEvents) subtitleLoadRequest(args []string) func() {
	if len(args) != 3 || !e.loaded {
		return nil
	}
	generation, err := strconv.ParseInt(args[1], 10, 64)
	entry, entryErr := strconv.ParseInt(args[2], 10, 64)
	if err != nil || entryErr != nil || generation <= 0 || entry != e.entryID {
		return nil
	}
	if e.subtitleContext != nil && e.subtitleContext.Err() == nil {
		return nil
	}
	return e.subtitleLoadCallback(true, generation)
}

func sendSubtitle(endpoint string, session subtitleSession, entry int64, file *SubtitleFile) error {
	return runIPCCommand(endpoint, []any{"script-message", "javboss-subtitle-add", strconv.FormatInt(session.Generation, 10), strconv.FormatInt(entry, 10), file.Path, file.Title, file.Language, strconv.FormatFloat(file.OffsetSec, 'f', 3, 64), file.Status, strconv.FormatBool(file.Downloaded), file.Track, file.Key})
}

// Called under the event lock. Capture the original entry's saver before any
// playlist changes. Chain accepted writes in input order so a slow remote save
// cannot overwrite a newer value when the same video is reopened.
func (e *playlistEvents) subtitleSaveCallback(args []string) func() {
	if len(args) != 5 || args[0] != "javboss-subtitle-save" || !e.loaded {
		return nil
	}
	values := make([]int64, 4)
	for i := range values {
		value, err := strconv.ParseInt(args[i+1], 10, 64)
		if err != nil {
			return nil
		}
		values[i] = value
	}
	if values[0] <= 0 || values[1] != e.entryID || values[2] <= 0 || values[3] < -3600000 || values[3] > 3600000 {
		return nil
	}
	save := e.subtitleSavers[e.entryID]
	if save == nil {
		return nil
	}
	previous, done := e.subtitleSaveDone, make(chan struct{})
	e.subtitleSaveDone = done
	endpoint := e.endpoint
	return func() {
		defer close(done)
		if previous != nil {
			<-previous
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		status := "saved"
		if err := save(ctx, values[3]); err != nil {
			logging.Error("save playback subtitle offset: %v", err)
			status = "error"
		}
		// Lua ignores acknowledgements belonging to an older file or edit.
		if err := runIPCCommand(endpoint, []any{"script-message", "javboss-subtitle-saved", args[1], args[2], args[3], status}); err != nil {
			logging.Error("report saved subtitle offset: %v", err)
		}
	}
}

// Serialize visibility saves with offset saves, preserving the captured video.
func (e *playlistEvents) subtitleSelectionCallback(args []string) func() {
	if len(args) != 4 || len(args[3]) > 1024 || !e.loaded {
		return nil
	}
	generation, err := strconv.ParseInt(args[1], 10, 64)
	entry, entryErr := strconv.ParseInt(args[2], 10, 64)
	if err != nil || entryErr != nil || generation <= 0 || entry != e.entryID {
		return nil
	}
	save := e.subtitleSelectionSavers[entry]
	if save == nil {
		return nil
	}
	previous, done := e.subtitleSaveDone, make(chan struct{})
	e.subtitleSaveDone = done
	endpoint := e.endpoint
	return func() {
		defer close(done)
		if previous != nil {
			<-previous
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := save(ctx, args[3]); err != nil {
			logging.Error("save subtitle visibility: %v", err)
			_ = runIPCCommand(endpoint, []any{"script-message", "javboss-subtitle-selection-error", args[1], args[2]})
		}
	}
}
