package util

import "strings"

type PlaybackProbeResult struct {
	Container      string `json:"container"`
	VideoCodec     string `json:"video_codec"`
	AudioCodec     string `json:"audio_codec"`
	SupportsDirect bool   `json:"supports_direct"`
}

func ProbePlaybackSupport(path string) (*PlaybackProbeResult, error) {
	meta, err := ProbeVideo(path)
	if err != nil {
		return nil, err
	}
	result := AssessPlaybackSupport(meta)
	return &result, nil
}

func AssessPlaybackSupport(meta *VideoMetadata) PlaybackProbeResult {
	result := PlaybackProbeResult{}
	if meta == nil {
		return result
	}

	// Older probe records stored the extension in Container even when FormatName disagreed.
	result.Container = probedContainer(meta.FormatName, normalizePlaybackValue(meta.Container))
	result.VideoCodec = normalizePlaybackValue(meta.VideoCodec)
	result.AudioCodec = normalizePlaybackValue(meta.AudioCodec)

	audioSafe := result.AudioCodec == "" || result.AudioCodec == "aac" || result.AudioCodec == "mp3"
	webmAudioSafe := result.AudioCodec == "" || result.AudioCodec == "opus" || result.AudioCodec == "vorbis"

	switch result.Container {
	case "mp4":
		result.SupportsDirect = result.VideoCodec == "h264" && audioSafe
	case "webm":
		result.SupportsDirect = (result.VideoCodec == "vp8" || result.VideoCodec == "vp9") && webmAudioSafe
	default:
		result.SupportsDirect = false
	}
	return result
}

func normalizePlaybackValue(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}
