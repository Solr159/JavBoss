package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"javboss/internal/common"
	"javboss/internal/common/logging"
	"javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/models"
	"javboss/internal/runtimeconfig"
	"javboss/internal/util"
)

const (
	javLinkWorkerCount = 4 // 增加worker数可能会导致首次扫描目录时jav相关查询接口严重阻塞
	javLinkQueueSize   = 4096
)

// javLinkOutcome classifies why one video location did or did not get JAV metadata.
type javLinkOutcome int

const (
	javLinkOutcomeFailed javLinkOutcome = iota
	javLinkOutcomeAlreadyLinked
	javLinkOutcomeSkippedShort
	javLinkOutcomeSkippedOverride
	javLinkOutcomeNoCode
	javLinkOutcomeNotFound
	javLinkOutcomeLinkedExisting
	javLinkOutcomeResolved
)

func (o javLinkOutcome) label() string {
	switch o {
	case javLinkOutcomeAlreadyLinked:
		return "already_linked"
	case javLinkOutcomeSkippedShort:
		return "skipped_short"
	case javLinkOutcomeSkippedOverride:
		return "skipped_override"
	case javLinkOutcomeNoCode:
		return "no_code"
	case javLinkOutcomeNotFound:
		return "not_found"
	case javLinkOutcomeLinkedExisting:
		return "linked_existing"
	case javLinkOutcomeResolved:
		return "resolved"
	default:
		return "failed"
	}
}

const javLinkSampleLimit = 8

// javLinkStats aggregates one scan batch's JAV link outcomes. A single summary line
// explains why files were not scraped without logging the same skip on every re-scan.
type javLinkStats struct {
	mu              sync.Mutex
	enqueued        int
	fresh           int
	alreadyLinked   int
	skippedShort    int
	skippedOverride int
	noCode          int
	notFound        int
	linkedExisting  int
	resolved        int
	failed          int
	noCodeSamples   []string
	notFoundSamples []string
}

func (s *javLinkStats) recordEnqueued(fresh bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enqueued++
	if fresh {
		s.fresh++
	}
}

func (s *javLinkStats) record(outcome javLinkOutcome, detail string, fresh bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch outcome {
	case javLinkOutcomeAlreadyLinked:
		s.alreadyLinked++
	case javLinkOutcomeSkippedShort:
		s.skippedShort++
	case javLinkOutcomeSkippedOverride:
		s.skippedOverride++
	case javLinkOutcomeNoCode:
		s.noCode++
		if fresh && len(s.noCodeSamples) < javLinkSampleLimit {
			s.noCodeSamples = append(s.noCodeSamples, detail)
		}
	case javLinkOutcomeNotFound:
		s.notFound++
		if fresh && len(s.notFoundSamples) < javLinkSampleLimit {
			s.notFoundSamples = append(s.notFoundSamples, detail)
		}
	case javLinkOutcomeLinkedExisting:
		s.linkedExisting++
	case javLinkOutcomeResolved:
		s.resolved++
	default:
		s.failed++
	}
}

type javLinkStatsSnapshot struct {
	enqueued        int
	fresh           int
	resolved        int
	linkedExisting  int
	alreadyLinked   int
	noCode          int
	notFound        int
	skippedShort    int
	skippedOverride int
	failed          int
	noCodeSamples   []string
	notFoundSamples []string
}

func (s *javLinkStats) snapshot() javLinkStatsSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return javLinkStatsSnapshot{
		enqueued:        s.enqueued,
		fresh:           s.fresh,
		resolved:        s.resolved,
		linkedExisting:  s.linkedExisting,
		alreadyLinked:   s.alreadyLinked,
		noCode:          s.noCode,
		notFound:        s.notFound,
		skippedShort:    s.skippedShort,
		skippedOverride: s.skippedOverride,
		failed:          s.failed,
		noCodeSamples:   append([]string(nil), s.noCodeSamples...),
		notFoundSamples: append([]string(nil), s.notFoundSamples...),
	}
}

type javLinkBatch struct {
	ctx            context.Context
	tasks          chan int64
	seen           map[int64]struct{}
	freshLocations map[int64]struct{}
	stats          javLinkStats
	mu             sync.Mutex
	closed         bool
	workers        sync.WaitGroup
}

func newJavLinkBatch(ctx context.Context) *javLinkBatch {
	if ctx == nil {
		ctx = context.Background()
	}
	batch := &javLinkBatch{
		ctx:            ctx,
		tasks:          make(chan int64, javLinkQueueSize),
		seen:           make(map[int64]struct{}),
		freshLocations: make(map[int64]struct{}),
	}
	for i := 0; i < javLinkWorkerCount; i++ {
		batch.workers.Add(1)
		go batch.worker()
	}
	return batch
}

// Enqueue submits a location that was already known before this scan.
func (b *javLinkBatch) Enqueue(locationID int64) {
	b.enqueue(locationID, false)
}

// EnqueueFresh submits a location discovered or changed by the current scan; only
// these get an individual skip log so periodic re-scans stay quiet.
func (b *javLinkBatch) EnqueueFresh(locationID int64) {
	b.enqueue(locationID, true)
}

func (b *javLinkBatch) enqueue(locationID int64, fresh bool) {
	if b == nil || locationID <= 0 {
		return
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	if _, ok := b.seen[locationID]; ok {
		b.mu.Unlock()
		return
	}
	b.seen[locationID] = struct{}{}
	if fresh {
		if b.freshLocations == nil {
			b.freshLocations = make(map[int64]struct{})
		}
		b.freshLocations[locationID] = struct{}{}
	}
	b.mu.Unlock()

	b.stats.recordEnqueued(fresh)

	progress, _ := b.ctx.Value(directoryScanProgressKey{}).(*directoryScanProgress)
	progress.record(false)

	select {
	case b.tasks <- locationID:
	case <-b.ctx.Done():
	}
}

func (b *javLinkBatch) isFresh(locationID int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.freshLocations == nil {
		return false
	}
	_, ok := b.freshLocations[locationID]
	return ok
}

func (b *javLinkBatch) Wait() {
	if b == nil {
		return
	}

	b.mu.Lock()
	if !b.closed {
		b.closed = true
		close(b.tasks)
	}
	b.mu.Unlock()
	b.workers.Wait()
}

func (b *javLinkBatch) worker() {
	defer b.workers.Done()
	for locationID := range b.tasks {
		if err := b.ctx.Err(); err != nil {
			return
		}
		fresh := b.isFresh(locationID)
		outcome, detail, err := b.processLocation(locationID)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			logging.Error("video location jav link failed location=%d err=%v", locationID, err)
		}
		b.stats.record(outcome, detail, fresh)
		if fresh {
			switch outcome {
			case javLinkOutcomeNoCode, javLinkOutcomeSkippedShort, javLinkOutcomeSkippedOverride, javLinkOutcomeNotFound:
				logging.Info("jav link skip: reason=%s location=%d %s", outcome.label(), locationID, detail)
			case javLinkOutcomeLinkedExisting:
				logging.Info("jav link reused existing metadata: location=%d %s", locationID, detail)
			}
		}
		if progress, _ := b.ctx.Value(directoryScanProgressKey{}).(*directoryScanProgress); progress != nil {
			// Read the persisted result: skipped/unmatched work is not a successful scrape,
			// while files whose metadata was already linked still count in this scan.
			v, err := db.GetVideoForJavScan(b.ctx, locationID)
			if err != nil {
				if b.ctx.Err() == nil {
					logging.Error("read directory scrape progress failed location=%d err=%v", locationID, err)
				}
			} else if v != nil && v.JavID != nil {
				progress.record(true)
			}
		}
	}
}

func finishJavLinkBatch(batch *javLinkBatch) {
	batch.Wait()
}

func (b *javLinkBatch) processLocation(locationID int64) (javLinkOutcome, string, error) {
	ctx := b.ctx
	v, err := db.GetVideoForJavScan(ctx, locationID)
	if err != nil {
		return javLinkOutcomeFailed, "", err
	}
	if v == nil {
		return javLinkOutcomeFailed, "", nil
	}

	filename := filepath.Base(filepath.FromSlash(v.Filename))
	detail := "file=" + filename

	override := normalizeJavScrapeOverride(v.JavScrapeOverride)
	if override == models.JavScrapeOverrideSkip {
		return javLinkOutcomeSkippedOverride, detail, nil
	}

	forcedCode := forcedJavScrapeCode(override)
	if forcedCode == "" {
		if v.JavID != nil {
			return javLinkOutcomeAlreadyLinked, detail, nil
		}
		// Short clips are skipped for JAV scraping in release builds, but kept during
		// development so short test fixtures can still be linked.
		if v.DurationSec > 0 && v.DurationSec < 900 && !runtimeconfig.DevelopmentMode() {
			return javLinkOutcomeSkippedShort, fmt.Sprintf("%s duration=%ds", detail, v.DurationSec), nil
		}
	} else if v.JavID != nil {
		if strings.EqualFold(strings.TrimSpace(v.JavCode), forcedCode) {
			return javLinkOutcomeAlreadyLinked, detail, nil
		}
		if err := db.ClearVideoLocationJavIDForVideo(ctx, v.LocationID, v.VideoID, v.UpdatedAt); err != nil {
			logging.Error("clear video location jav before forced scrape failed location=%d code=%s err=%v", v.LocationID, forcedCode, err)
			return javLinkOutcomeFailed, detail, err
		}
		v.JavID = nil
		v.JavCode = ""
	}

	possibleCodes := javScrapeCodesForVideo(filename, forcedCode)
	if len(possibleCodes) == 0 {
		return javLinkOutcomeNoCode, detail, nil
	}

	if linked := linkExistingJav(ctx, v, possibleCodes); linked {
		return javLinkOutcomeLinkedExisting, detail, nil
	}

	uncensoredPossibleCodes := util.ExtractUncensoredCodesFromName(filename)
	if forcedCode != "" {
		uncensoredPossibleCodes = possibleCodes
	}
	info, err := jav.ResolveJavByCodes(ctx, possibleCodes, uncensoredPossibleCodes)
	if errors.Is(err, jav.ErrNotFound) {
		return javLinkOutcomeNotFound, fmt.Sprintf("%s codes=%s", detail, strings.Join(possibleCodes, ",")), nil
	}
	if err != nil {
		return javLinkOutcomeFailed, detail, err
	}
	if _, err := db.SaveJavInfoAndLinkLocationForVideo(ctx, info, v.LocationID, v.VideoID, v.UpdatedAt); err != nil {
		logging.Error("link video location->jav failed provider=%s location=%s code=%s err=%v", info.Provider.String(), filename, info.Code, err)
		return javLinkOutcomeFailed, detail, nil
	}
	logging.Info("link video location->jav success provider=%s location=%s code=%s", info.Provider.String(), filename, info.Code)
	enqueueCover(info.Code)
	return javLinkOutcomeResolved, detail, nil
}

// logSummary writes one JAV link summary for a finished directory scan batch.
func (b *javLinkBatch) logSummary(directory models.Directory) {
	if b == nil {
		return
	}
	s := b.stats.snapshot()
	if s.enqueued == 0 {
		return
	}
	logging.Info(
		"sync directory jav link summary: id=%d path=%s submitted=%d fresh=%d resolved=%d linked_existing=%d already_linked=%d no_code=%d not_found=%d skipped_short=%d skipped_override=%d failed=%d",
		directory.ID, directory.Path, s.enqueued, s.fresh, s.resolved, s.linkedExisting,
		s.alreadyLinked, s.noCode, s.notFound, s.skippedShort, s.skippedOverride, s.failed,
	)
	if len(s.noCodeSamples) > 0 {
		logging.Info("sync directory jav link no-code files: id=%d files=%s", directory.ID, strings.Join(s.noCodeSamples, "; "))
	}
	if len(s.notFoundSamples) > 0 {
		logging.Info("sync directory jav link not-found files: id=%d files=%s", directory.ID, strings.Join(s.notFoundSamples, "; "))
	}
}

func linkExistingJav(ctx context.Context, v *db.JavScanVideo, possibleCodes []string) bool {
	for _, code := range possibleCodes {
		existJav, err := db.GetJavByCode(ctx, code)
		if err != nil {
			logging.Error("jav lookup existing failed location=%d code=%s err=%v", v.LocationID, code, err)
			continue
		}
		if existJav == nil {
			continue
		}
		if err := db.SetVideoLocationJavIDForVideo(ctx, v.LocationID, v.VideoID, existJav.ID, v.UpdatedAt); err != nil {
			logging.Error("set video location jav failed location=%d code=%s err=%v", v.LocationID, code, err)
		} else {
			enqueueCover(existJav.Code)
		}
		return true
	}
	return false
}

func javScrapeCodesForVideo(filename, forcedCode string) []string {
	forcedCode = strings.TrimSpace(forcedCode)
	if forcedCode != "" {
		return []string{forcedCode}
	}
	return util.ExtractCodeFromName(filename)
}

func normalizeJavScrapeOverride(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.EqualFold(raw, models.JavScrapeOverrideSkip) {
		return models.JavScrapeOverrideSkip
	}
	if strings.HasPrefix(strings.ToLower(raw), models.JavScrapeOverrideManualPrefix) {
		code := strings.TrimSpace(raw[len(models.JavScrapeOverrideManualPrefix):])
		if code == "" {
			return ""
		}
		return models.JavScrapeOverrideManualPrefix + strings.ToUpper(code)
	}
	return strings.ToUpper(raw)
}

func forcedJavScrapeCode(override string) string {
	override = normalizeJavScrapeOverride(override)
	if override == "" || override == models.JavScrapeOverrideSkip {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(override), models.JavScrapeOverrideManualPrefix) {
		return strings.TrimSpace(override[len(models.JavScrapeOverrideManualPrefix):])
	}
	return override
}

func enqueueCover(code string) {
	mgr := common.CoverManager
	if mgr == nil {
		return
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return
	}
	mgr.Enqueue(code)
}

// enqueueMissingCoversForDirectory 只补充指定目录中已关联 JAV 的缺失封面。
func enqueueMissingCoversForDirectory(ctx context.Context, directoryID int64) error {
	mgr := common.CoverManager
	if common.DB == nil || mgr == nil {
		return nil
	}
	codes, err := db.ListJavCodesForDirectory(ctx, directoryID)
	if err != nil {
		return err
	}
	for _, c := range codes {
		code := strings.TrimSpace(c)
		if code == "" {
			continue
		}
		if mgr.Exists(code) {
			continue
		}
		mgr.Enqueue(code)
	}
	return nil
}
