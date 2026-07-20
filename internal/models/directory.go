package models

import "time"

// Directory represents a root path that can be scanned for videos.
type Directory struct {
	ID                int64     `json:"id" gorm:"primaryKey"`
	Path              string    `json:"path" gorm:"uniqueIndex"`
	Missing           bool      `json:"missing" gorm:"index"`
	IsDelete          bool      `json:"is_delete" gorm:"index"`
	ScannedVideoCount int64     `json:"scanned_video_count" gorm:"->;-:migration"`
	ScrapedVideoCount int64     `json:"scraped_video_count" gorm:"->;-:migration"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
