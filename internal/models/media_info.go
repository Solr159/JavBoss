package models

import (
	"fmt"
	"math"
	"strings"
)

const MediaInfoVersion = 1

// MediaInfo is the persisted result of a successful video probe.
type MediaInfo struct {
	Version         int     `json:"version"`
	Codec           string  `json:"codec"`
	VideoCodec      string  `json:"video_codec"`
	AudioCodec      string  `json:"audio_codec"`
	Container       string  `json:"container"`
	FormatName      string  `json:"format_name"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	FPS             float64 `json:"fps"`
	SampleRate      int     `json:"sample_rate"`
	Channels        int     `json:"channels"`
	DurationSeconds float64 `json:"duration_seconds"`
	FormatBitRate   int64   `json:"format_bit_rate"`
	VideoBitRate    int64   `json:"video_bit_rate"`
	AudioBitRate    int64   `json:"audio_bit_rate"`
}

func (m *MediaInfo) Fingerprint(size int64) string {
	fps := m.FPS
	if fps > 0 {
		fps = math.Round(fps*1000) / 1000
	}
	dur := math.Round(m.DurationSeconds)
	return fmt.Sprintf("%dx%d|%s|%.3f|%d|%d|%.0f|%d",
		m.Width,
		m.Height,
		strings.TrimSpace(m.Codec),
		fps,
		m.SampleRate,
		m.Channels,
		dur,
		size)
}

// FingerprintV2 returns a metadata-only fingerprint with higher granularity.
// Format: widthxheight|bitrate|video_bitrate|audio_bitrate|duration_ms|size
func (m *MediaInfo) FingerprintV2(size int64) string {
	durationMs := int64(math.Round(m.DurationSeconds * 1000))
	return fmt.Sprintf("%dx%d|%d|%d|%d|%d|%d",
		m.Width,
		m.Height,
		m.FormatBitRate,
		m.VideoBitRate,
		m.AudioBitRate,
		durationMs,
		size)
}

// Complete distinguishes unpopulated legacy rows from successful probes, including silent videos.
func (m MediaInfo) Complete() bool { return m.Version == MediaInfoVersion }
