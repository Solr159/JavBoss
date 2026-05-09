package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"pornboss/internal/common"
	"pornboss/internal/common/logging"
	"pornboss/internal/db"
	"pornboss/internal/jav"
)

// StartJavStudioScanner periodically fills missing JAV studio metadata from JavDatabase.
func StartJavStudioScanner(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if err := ScanJavStudios(ctx); err != nil {
				logging.Error("jav studio scan failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// ScanJavStudios scans jav rows with missing studio data and queries JavDatabase for it.
func ScanJavStudios(ctx context.Context) error {
	if common.DB == nil {
		return errors.New("nil db")
	}

	items, err := db.ListJavsMissingStudio(ctx)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	logging.Info("found %d jav rows missing studio info", len(items))

	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return err
		}

		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}

		info, err := jav.LookupJavByCode(code, jav.ProviderJavDatabase)
		if err != nil {
			if !errors.Is(err, jav.ResourceNotFonud) {
				logging.Error("lookup jav studio failed id=%d code=%s err=%v", item.ID, code, err)
			}
			continue
		}

		studio := ""
		if info != nil {
			studio = strings.TrimSpace(info.Studio)
		}
		if studio == "" {
			continue
		}
		if err := db.UpdateJavStudio(ctx, item.ID, studio); err != nil {
			logging.Error("update jav studio failed id=%d code=%s err=%v", item.ID, code, err)
			continue
		}
		logging.Info("jav studio updated id=%d code=%s studio=%s", item.ID, code, studio)
	}
	return nil
}
