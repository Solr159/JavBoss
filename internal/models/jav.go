package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// JavSampleImage stores the thumbnail and full-size URLs for a JAV sample image.
type JavSampleImage struct {
	ThumbnailURL string `json:"thumbnail_url"`
	DetailURL    string `json:"detail_url"`
}

const JavSampleImageNotFound = ":not_found"

// JavSampleImages persists a JAV sample image list as JSON.
type JavSampleImages []JavSampleImage

func NewJavSampleImagesNotFound() JavSampleImages {
	return JavSampleImages{{
		ThumbnailURL: JavSampleImageNotFound,
		DetailURL:    JavSampleImageNotFound,
	}}
}

func (images JavSampleImages) IsNotFound() bool {
	return len(images) == 1 &&
		images[0].ThumbnailURL == JavSampleImageNotFound &&
		images[0].DetailURL == JavSampleImageNotFound
}

func (images JavSampleImages) Value() (driver.Value, error) {
	if images == nil {
		images = JavSampleImages{}
	}
	data, err := json.Marshal(images)
	if err != nil {
		return nil, fmt.Errorf("marshal JAV sample images: %w", err)
	}
	return string(data), nil
}

func (images *JavSampleImages) Scan(value any) error {
	if images == nil {
		return fmt.Errorf("scan JAV sample images into nil receiver")
	}
	var data []byte
	switch typed := value.(type) {
	case nil:
		*images = JavSampleImages{}
		return nil
	case string:
		data = []byte(typed)
	case []byte:
		data = typed
	default:
		return fmt.Errorf("scan JAV sample images from %T", value)
	}
	if raw := strings.TrimSpace(string(data)); raw == "" || raw == "null" {
		*images = JavSampleImages{}
		return nil
	}
	if err := json.Unmarshal(data, images); err != nil {
		return fmt.Errorf("unmarshal JAV sample images: %w", err)
	}
	if *images == nil {
		*images = JavSampleImages{}
	}
	return nil
}

func (images JavSampleImages) MarshalJSON() ([]byte, error) {
	if images == nil {
		return []byte("[]"), nil
	}
	type sampleImagesAlias JavSampleImages
	return json.Marshal(sampleImagesAlias(images))
}

// Jav stores metadata fetched for a given code (may map to multiple videos).
type Jav struct {
	ID             int64           `json:"id" gorm:"primaryKey"`
	Code           string          `json:"code" gorm:"uniqueIndex"`
	Title          string          `json:"title"`
	StudioID       *int64          `json:"studio_id" gorm:"index"`
	Studio         *JavStudio      `json:"studio,omitempty" gorm:"foreignKey:StudioID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	SeriesID       *int64          `json:"series_id" gorm:"index"`
	Series         *JavSeries      `json:"series,omitempty" gorm:"foreignKey:SeriesID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	SeriesEnID     *int64          `json:"-" gorm:"index"`
	SeriesEn       *JavSeries      `json:"-" gorm:"foreignKey:SeriesEnID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	ReleaseUnix    int64           `json:"release_unix"`
	DurationMin    int             `json:"duration_min"`
	FetchedAt      time.Time       `json:"fetched_at"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	IsUncensored   *bool           `json:"is_uncensored"`
	SampleImages   JavSampleImages `json:"sample_images" gorm:"type:text;not null;default:'[]'"`
	FavoriteRating float64         `json:"favorite_rating" gorm:"not null;default:0"`
	Tags           []JavTag        `json:"tags,omitempty" gorm:"-"`
	Idols          []JavIdol       `json:"idols,omitempty" gorm:"many2many:jav_idol_map"`
	Videos         []Video         `json:"videos,omitempty" gorm:"-"`
	FavoriteCount  int64           `json:"favorite_count" gorm:"-"`
}

type JavStudio struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type JavStudioAlias struct {
	ID          int64     `json:"id" gorm:"primaryKey"`
	JavStudioID int64     `json:"jav_studio_id" gorm:"not null;index"`
	JavStudio   JavStudio `json:"-" gorm:"foreignKey:JavStudioID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Alias       string    `json:"alias" gorm:"not null;uniqueIndex"`
	CreatedAt   time.Time `json:"created_at"`
}

type JavSeries struct {
	ID        int64      `json:"id" gorm:"primaryKey"`
	Name      string     `json:"name" gorm:"uniqueIndex:idx_jav_series_name_language"`
	IsEnglish bool       `json:"-" gorm:"not null;default:0;uniqueIndex:idx_jav_series_name_language"`
	StudioID  *int64     `json:"studio_id" gorm:"index"`
	Studio    *JavStudio `json:"studio,omitempty" gorm:"foreignKey:StudioID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type JavTag struct {
	ID             int64           `json:"id" gorm:"primaryKey"`
	Name           string          `json:"name" gorm:"uniqueIndex:idx_jav_tag_name_user"`
	SimplifiedName string          `json:"simplified_name,omitempty" gorm:"-"`
	IsUser         bool            `json:"is_user" gorm:"not null;default:0;uniqueIndex:idx_jav_tag_name_user"`
	Provider       int             `json:"provider" gorm:"-"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	CategoryID     *int64          `json:"category_id,omitempty" gorm:"index"`
	Category       *JavTagCategory `json:"category,omitempty" gorm:"foreignKey:CategoryID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
}

type JavTagCategory struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"not null;uniqueIndex"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	SortOrder int       `json:"sort_order" gorm:"not null;default:0"`
}

type JavIdol struct {
	ID            int64      `json:"id" gorm:"primaryKey"`
	Name          string     `json:"name" gorm:"uniqueIndex"`
	RomanName     string     `json:"roman_name"`
	JapaneseName  string     `json:"japanese_name"`
	ChineseName   string     `json:"chinese_name"`
	HeightCM      *int       `json:"height_cm"`
	BirthDate     *time.Time `json:"birth_date"`
	Bust          *int       `json:"bust"`
	Waist         *int       `json:"waist"`
	Hips          *int       `json:"hips"`
	Cup           *int       `json:"cup"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	CoverJavID    *int64     `json:"cover_jav_id" gorm:"index"`
	CoverCropLeft float64    `json:"cover_crop_left" gorm:"not null;default:0.53"`
}

type JavIdolAlias struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	JavIdolID int64     `json:"jav_idol_id" gorm:"not null;index"`
	JavIdol   JavIdol   `json:"-" gorm:"foreignKey:JavIdolID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Alias     string    `json:"alias" gorm:"not null;uniqueIndex"`
	CreatedAt time.Time `json:"created_at"`
}

type JavFavoriteGroup struct {
	ID         int64     `json:"id" gorm:"primaryKey"`
	EntityType string    `json:"entity_type" gorm:"not null;default:idol;uniqueIndex:idx_jav_favorite_group_type_name;index:idx_jav_favorite_group_type_sort,priority:1"`
	Name       string    `json:"name" gorm:"uniqueIndex:idx_jav_favorite_group_type_name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	SortOrder  int       `json:"sort_order" gorm:"not null;default:0;index:idx_jav_favorite_group_type_sort,priority:2"`
}

func (JavFavoriteGroup) TableName() string {
	return "jav_favorite_group"
}

type JavIdolFavoriteGroup = JavFavoriteGroup

type JavFavoriteMap struct {
	JavFavoriteGroupID int64            `gorm:"primaryKey;index:idx_jav_favorite_map_entity_type_entity_id_group_id,priority:3"`
	JavFavoriteGroup   JavFavoriteGroup `gorm:"foreignKey:JavFavoriteGroupID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	EntityType         string           `gorm:"not null;default:idol;index:idx_jav_favorite_map_entity_type_entity_id_group_id,priority:1"`
	EntityID           int64            `gorm:"primaryKey;index:idx_jav_favorite_map_entity_type_entity_id_group_id,priority:2"`
	CreatedAt          time.Time        `gorm:"autoCreateTime"`
	SortOrder          int              `gorm:"not null;default:0;index"`
}

// Many-to-many join tables.
type JavTagMap struct {
	JavID     int64     `gorm:"primaryKey"`
	Jav       Jav       `gorm:"foreignKey:JavID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	JavTagID  int64     `gorm:"primaryKey"`
	JavTag    JavTag    `gorm:"foreignKey:JavTagID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Provider  int       `gorm:"primaryKey;not null;default:0;index"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

type JavIdolMap struct {
	JavID     int64     `gorm:"primaryKey;index:idx_jav_idol_map_jav_idol_id_jav_id,priority:2"`
	Jav       Jav       `gorm:"foreignKey:JavID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	JavIdolID int64     `gorm:"primaryKey;index:idx_jav_idol_map_jav_idol_id_jav_id,priority:1"`
	JavIdol   JavIdol   `gorm:"foreignKey:JavIdolID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}
