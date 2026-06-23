package models

import "time"

// Jav stores metadata fetched for a given code (may map to multiple videos).
type Jav struct {
	ID            int64      `json:"id" gorm:"primaryKey"`
	Code          string     `json:"code" gorm:"uniqueIndex"`
	Title         string     `json:"title"`
	TitleEn       string     `json:"title_en"`
	StudioID      *int64     `json:"studio_id" gorm:"index"`
	Studio        *JavStudio `json:"studio,omitempty" gorm:"foreignKey:StudioID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	SeriesID      *int64     `json:"series_id" gorm:"index"`
	Series        *JavSeries `json:"series,omitempty" gorm:"foreignKey:SeriesID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	SeriesEnID    *int64     `json:"series_en_id" gorm:"index"`
	SeriesEn      *JavSeries `json:"series_en,omitempty" gorm:"foreignKey:SeriesEnID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	ReleaseUnix   int64      `json:"release_unix"`
	DurationMin   int        `json:"duration_min"`
	FetchedAt     time.Time  `json:"fetched_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	IsUncensored  *bool      `json:"is_uncensored"`
	Tags          []JavTag   `json:"tags,omitempty" gorm:"-"`
	Idols         []JavIdol  `json:"idols,omitempty" gorm:"many2many:jav_idol_map"`
	Videos        []Video    `json:"videos,omitempty" gorm:"-"`
	FavoriteCount int64      `json:"favorite_count" gorm:"-"`
}

type JavStudio struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type JavSeries struct {
	ID        int64      `json:"id" gorm:"primaryKey"`
	Name      string     `json:"name" gorm:"uniqueIndex:idx_jav_series_name_language"`
	IsEnglish bool       `json:"is_english" gorm:"not null;default:0;uniqueIndex:idx_jav_series_name_language"`
	StudioID  *int64     `json:"studio_id" gorm:"index"`
	Studio    *JavStudio `json:"studio,omitempty" gorm:"foreignKey:StudioID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type JavTag struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex:idx_jav_tag_name_user"`
	IsUser    bool      `json:"is_user" gorm:"not null;default:0;uniqueIndex:idx_jav_tag_name_user"`
	Provider  int       `json:"provider" gorm:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type JavIdol struct {
	ID            int64      `json:"id" gorm:"primaryKey"`
	Name          string     `json:"name" gorm:"uniqueIndex:idx_jav_idol_name_language"`
	IsEnglish     bool       `json:"is_english" gorm:"not null;default:0;uniqueIndex:idx_jav_idol_name_language"`
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

type JavIdolFavoriteGroup struct {
	ID         int64     `json:"id" gorm:"primaryKey"`
	EntityType string    `json:"entity_type" gorm:"not null;default:idol;uniqueIndex:idx_jav_favorite_group_type_name;index:idx_jav_favorite_group_type_sort,priority:1"`
	Name       string    `json:"name" gorm:"uniqueIndex:idx_jav_favorite_group_type_name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	SortOrder  int       `json:"sort_order" gorm:"not null;default:0;index:idx_jav_favorite_group_type_sort,priority:2"`
}

func (JavIdolFavoriteGroup) TableName() string {
	return "jav_favorite_group"
}

type JavFavoriteGroup = JavIdolFavoriteGroup

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

type JavIdolFavoriteMap struct {
	JavIdolFavoriteGroupID int64                `gorm:"primaryKey;column:jav_idol_favorite_group_id"`
	JavIdolFavoriteGroup   JavIdolFavoriteGroup `gorm:"foreignKey:JavIdolFavoriteGroupID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	JavIdolID              int64                `gorm:"primaryKey;column:jav_idol_id"`
	CreatedAt              time.Time            `gorm:"autoCreateTime"`
	SortOrder              int                  `gorm:"not null;default:0"`
}

func (JavIdolFavoriteMap) TableName() string {
	return "jav_idol_favorite_map"
}
