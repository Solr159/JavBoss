package models

import "time"

// Video represents metadata tracked for a single video content entity.
type Video struct {
	ID                int64           `json:"id" gorm:"primaryKey"`
	LocationID        int64           `json:"location_id,omitempty" gorm:"-"`
	DirectoryID       int64           `json:"directory_id" gorm:"-"`
	Path              string          `json:"path" gorm:"-"` // Deprecated: derived from VideoLocation for API compatibility.
	Filename          string          `json:"filename" gorm:"-"`
	Size              int64           `json:"size"`
	ModifiedAt        time.Time       `json:"modified_at" gorm:"-"`
	Fingerprint       string          `json:"fingerprint" gorm:"uniqueIndex"`
	DurationSec       int64           `json:"duration_sec"` // duration in seconds (rounded)
	PlayCount         int64           `json:"play_count" gorm:"not null;default:0"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	JavScrapeOverride string          `json:"jav_scrape_override"`
	Tags              []Tag           `json:"tags,omitempty" gorm:"many2many:video_tag"`
	JavID             *int64          `json:"jav_id" gorm:"-"`
	Jav               *Jav            `json:"jav,omitempty" gorm:"-"`
	DirectoryRef      Directory       `json:"directory,omitempty" gorm:"-"`
	Locations         []VideoLocation `json:"locations,omitempty" gorm:"foreignKey:VideoID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Hidden            bool            `json:"hidden" gorm:"-"`
}

const JavScrapeOverrideSkip = ":skip"
const JavScrapeOverrideManualPrefix = ":manual:"
