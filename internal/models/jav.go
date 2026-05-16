package models

import "time"

// Jav stores metadata fetched for a given code (may map to multiple videos).
type Jav struct {
	ID          int64      `json:"id" gorm:"primaryKey"`
	Code        string     `json:"code" gorm:"uniqueIndex"`
	Title       string     `json:"title"`
	TitleEn     string     `json:"title_en"`
	StudioID    *int64     `json:"studio_id" gorm:"index"`
	Studio      *JavStudio `json:"studio,omitempty" gorm:"foreignKey:StudioID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	SeriesID    *int64     `json:"series_id" gorm:"index"`
	Series      *JavSeries `json:"series,omitempty" gorm:"foreignKey:SeriesID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	SeriesEnID  *int64     `json:"series_en_id" gorm:"index"`
	SeriesEn    *JavSeries `json:"series_en,omitempty" gorm:"foreignKey:SeriesEnID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	ReleaseUnix int64      `json:"release_unix"`
	DurationMin int        `json:"duration_min"`
	Provider    int        `json:"provider" gorm:"not null;default:0"`
	FetchedAt   time.Time  `json:"fetched_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Tags        []JavTag   `json:"tags,omitempty" gorm:"many2many:jav_tag_map"`
	Idols       []JavIdol  `json:"idols,omitempty" gorm:"many2many:jav_idol_map"`
	Videos      []Video    `json:"videos,omitempty" gorm:"-"`
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
	Name      string    `json:"name" gorm:"uniqueIndex:idx_jav_tag_name_source"`
	Provider  int       `json:"provider" gorm:"not null;default:0;index:idx_jav_tag_provider;uniqueIndex:idx_jav_tag_name_source"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type JavIdol struct {
	ID           int64      `json:"id" gorm:"primaryKey"`
	Name         string     `json:"name" gorm:"uniqueIndex:idx_jav_idol_name_language"`
	IsEnglish    bool       `json:"is_english" gorm:"not null;default:0;uniqueIndex:idx_jav_idol_name_language"`
	RomanName    string     `json:"roman_name"`
	JapaneseName string     `json:"japanese_name"`
	ChineseName  string     `json:"chinese_name"`
	HeightCM     *int       `json:"height_cm"`
	BirthDate    *time.Time `json:"birth_date"`
	Bust         *int       `json:"bust"`
	Waist        *int       `json:"waist"`
	Hips         *int       `json:"hips"`
	Cup          *int       `json:"cup"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Many-to-many join tables.
type JavTagMap struct {
	JavID     int64     `gorm:"primaryKey"`
	Jav       Jav       `gorm:"foreignKey:JavID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	JavTagID  int64     `gorm:"primaryKey"`
	JavTag    JavTag    `gorm:"foreignKey:JavTagID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

type JavIdolMap struct {
	JavID     int64     `gorm:"primaryKey;index:idx_jav_idol_map_jav_idol_id_jav_id,priority:2"`
	Jav       Jav       `gorm:"foreignKey:JavID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	JavIdolID int64     `gorm:"primaryKey;index:idx_jav_idol_map_jav_idol_id_jav_id,priority:1"`
	JavIdol   JavIdol   `gorm:"foreignKey:JavIdolID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}
