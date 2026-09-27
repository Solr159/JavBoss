package metadata

// JavInfo holds basic metadata extracted from a JAV metadata provider.
type JavInfo struct {
	Title        string
	ZhTitle      string
	Code         string
	Studio       string
	Series       string
	ReleaseUnix  int64
	DurationMin  int
	Tags         []string
	Actors       []string
	CoverURL     string
	SampleImages []SampleImage
	IsUncensored *bool `json:",omitempty"`
	Provider     Provider
}

// ActressInfo describes actress profile fields returned by metadata providers.
type ActressInfo struct {
	RomanName    string
	JapaneseName string
	ChineseName  string
	HeightCM     int
	Bust         int
	Waist        int
	Hips         int
	BirthDate    int
	Cup          int
	ProfileURL   string
}

// SampleImage contains the thumbnail and full-size URLs for a movie sample image.
type SampleImage struct {
	ThumbnailURL string `json:"thumbnail_url"`
	DetailURL    string `json:"detail_url"`
}

type GenreCategory struct {
	Name     string
	Category string
}
