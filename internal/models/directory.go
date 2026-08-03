package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DirectoryScanSummary persists the result of the latest successful directory scan.
type DirectoryScanSummary struct {
	FilesSeen        int   `json:"files_seen,omitempty"`
	Inserted         int   `json:"inserted,omitempty"`
	Updated          int   `json:"updated,omitempty"`
	Removed          int   `json:"removed,omitempty"`
	DurationMS       int64 `json:"duration_ms,omitempty"`
	FinishedAtUnixMS int64 `json:"finished_at_unix_ms,omitempty"`
}

func (summary DirectoryScanSummary) Value() (driver.Value, error) {
	data, err := json.Marshal(summary)
	if err != nil {
		return nil, fmt.Errorf("marshal directory scan summary: %w", err)
	}
	return string(data), nil
}

func (summary *DirectoryScanSummary) Scan(value any) error {
	if summary == nil {
		return fmt.Errorf("scan directory summary into nil receiver")
	}
	var data []byte
	switch typed := value.(type) {
	case nil:
		*summary = DirectoryScanSummary{}
		return nil
	case string:
		data = []byte(typed)
	case []byte:
		data = typed
	default:
		return fmt.Errorf("scan directory summary from %T", value)
	}
	if raw := strings.TrimSpace(string(data)); raw == "" || raw == "null" {
		*summary = DirectoryScanSummary{}
		return nil
	}
	if err := json.Unmarshal(data, summary); err != nil {
		return fmt.Errorf("unmarshal directory scan summary: %w", err)
	}
	return nil
}

// Directory represents a root path that can be scanned for videos.
type Directory struct {
	ID                      int64                `json:"id" gorm:"primaryKey"`
	Path                    string               `json:"path" gorm:"uniqueIndex"`
	Missing                 bool                 `json:"missing" gorm:"index"`
	IsDelete                bool                 `json:"is_delete" gorm:"index"`
	ScannedVideoCount       int64                `json:"scanned_video_count" gorm:"->;-:migration"`
	ScrapedVideoCount       int64                `json:"scraped_video_count" gorm:"->;-:migration"`
	CreatedAt               time.Time            `json:"created_at"`
	UpdatedAt               time.Time            `json:"updated_at"`
	LastScanSummary         DirectoryScanSummary `json:"last_scan_summary" gorm:"type:text;not null;default:'{}'"`
	AutoScanEnabled         bool                 `json:"auto_scan_enabled" gorm:"not null;default:true"`
	AutoScanIntervalMinutes int                  `json:"auto_scan_interval_minutes" gorm:"not null;default:1"`
}
