// Package autosub defines the request/result payloads exchanged with the
// AutoSub gateway.
package autosub

// Video describes a video file to fetch subtitles for. The JSON keys mirror
// the gateway's VideoFile dataclass.
type Video struct {
	Path       string  `json:"path"`
	Name       string  `json:"name"`
	SizeBytes  int64   `json:"size_bytes"`
	ModifiedAt float64 `json:"modified_time"`
	Duration   float64 `json:"duration_seconds,omitempty"`
}
