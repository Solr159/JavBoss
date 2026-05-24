package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"

	"pornboss/internal/common"
	"pornboss/internal/common/logging"
	"pornboss/internal/db"
	"pornboss/internal/jav"
	"pornboss/internal/util"
)

const (
	javLinkWorkerCount = 4 // 增加worker数可能会导致首次扫描目录时jav相关查询接口严重阻塞
	javLinkQueueSize   = 4096
)

type javLinkBatch struct {
	ctx     context.Context
	tasks   chan int64
	seen    map[int64]struct{}
	mu      sync.Mutex
	closed  bool
	workers sync.WaitGroup
}

func newJavLinkBatch(ctx context.Context) *javLinkBatch {
	if ctx == nil {
		ctx = context.Background()
	}
	batch := &javLinkBatch{
		ctx:   ctx,
		tasks: make(chan int64, javLinkQueueSize),
		seen:  make(map[int64]struct{}),
	}
	for i := 0; i < javLinkWorkerCount; i++ {
		batch.workers.Add(1)
		go batch.worker()
	}
	return batch
}

func (b *javLinkBatch) Enqueue(locationID int64) {
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
	b.mu.Unlock()

	select {
	case b.tasks <- locationID:
	case <-b.ctx.Done():
	}
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
		if err := processVideoLocationJavLink(b.ctx, locationID); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			logging.Error("video location jav link failed location=%d err=%v", locationID, err)
		}
	}
}

func finishJavLinkBatch(batch *javLinkBatch) {
	batch.Wait()
}

func processVideoLocationJavLink(ctx context.Context, locationID int64) error {
	v, err := db.GetVideoForJavScan(ctx, locationID)
	if err != nil || v == nil {
		return err
	}

	if v.JavID != nil {
		return nil
	}
	if v.DurationSec > 0 && v.DurationSec < 900 {
		return nil
	}

	filename := filepath.Base(filepath.FromSlash(v.Filename))
	possibleCodes := util.ExtractCodeFromName(filename)
	if len(possibleCodes) == 0 {
		return nil
	}

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
		return nil
	}

	if linked, err := lookupAndLinkVideoLocationJav(ctx, v, filename, possibleCodes, jav.ProviderJavBus); err != nil || linked {
		return err
	}
	if linked, err := lookupAndLinkVideoLocationJav(ctx, v, filename, possibleCodes, jav.ProviderJavDatabase); err != nil || linked {
		return err
	}
	uncensoredPossibleCodes := util.ExtractUncensoredCodesFromName(filename)
	if linked, err := lookupAndLinkVideoLocationJav(ctx, v, filename, uncensoredPossibleCodes, jav.ProviderAvsox); err != nil || linked {
		return err
	}
	return nil
}

func lookupAndLinkVideoLocationJav(ctx context.Context, v *db.JavScanVideo, filename string, possibleCodes []string, provider jav.Provider) (bool, error) {
	for _, code := range possibleCodes {
		info, err := jav.LookupJavByCode(code, provider)
		if err != nil {
			if errors.Is(err, jav.ResourceNotFonud) {
				continue
			}
			logging.Error("jav lookup failed provider=%s location=%s code=%s err=%v", provider.String(), filename, code, err)
			continue
		}
		if info == nil {
			continue
		}

		if _, err := db.SaveJavInfoAndLinkLocationForVideo(ctx, info, v.LocationID, v.VideoID, v.UpdatedAt); err != nil {
			logging.Error("link video location->jav failed provider=%s location=%s code=%s err=%v", provider.String(), filename, info.Code, err)
		} else {
			logging.Info("link video location->jav success provider=%s location=%s code=%s", provider.String(), filename, info.Code)
			enqueueCover(info.Code)
		}
		return true, nil
	}
	return false, nil
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

func enqueueMissingCovers(ctx context.Context) error {
	mgr := common.CoverManager
	if common.DB == nil || mgr == nil {
		return nil
	}
	codes, err := db.ListJavCodes(ctx)
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
