package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// StringList stores a string slice as JSON in SQLite.
type StringList []string

func (values StringList) Value() (driver.Value, error) {
	if values == nil {
		values = StringList{}
	}
	data, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("marshal string list: %w", err)
	}
	return string(data), nil
}

func (values *StringList) Scan(value any) error {
	if values == nil {
		return errors.New("scan string list into nil receiver")
	}
	var data []byte
	switch typed := value.(type) {
	case nil:
		*values = StringList{}
		return nil
	case string:
		data = []byte(typed)
	case []byte:
		data = typed
	default:
		return fmt.Errorf("scan string list from %T", value)
	}
	if strings.TrimSpace(string(data)) == "" || strings.TrimSpace(string(data)) == "null" {
		*values = StringList{}
		return nil
	}
	if err := json.Unmarshal(data, values); err != nil {
		return fmt.Errorf("unmarshal string list: %w", err)
	}
	if *values == nil {
		*values = StringList{}
	}
	return nil
}

// WesternMetadata stores Emby-style metadata for non-JAV scenes and movies.
type WesternMetadata struct {
	VideoID       int64      `json:"video_id" gorm:"primaryKey"`
	Title         string     `json:"title"`
	OriginalTitle string     `json:"original_title"`
	ContentType   string     `json:"content_type" gorm:"not null;default:scene;index"`
	Studio        string     `json:"studio" gorm:"index"`
	Description   string     `json:"description"`
	ReleaseDate   string     `json:"release_date"`
	Source        string     `json:"source" gorm:"not null;index"`
	SourceID      string     `json:"source_id"`
	SourceURL     string     `json:"source_url"`
	CoverURL      string     `json:"cover_url"`
	Performers    StringList `json:"performers" gorm:"type:text;not null;default:'[]'"`
	Genres        StringList `json:"genres" gorm:"type:text;not null;default:'[]'"`
	Labels        StringList `json:"labels" gorm:"type:text;not null;default:'[]'"`
	FetchedAt     time.Time  `json:"fetched_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	MatchStatus   string     `json:"match_status" gorm:"not null;default:matched;index"`
}
