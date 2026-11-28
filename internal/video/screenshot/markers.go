package screenshot

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var playbackNamePattern = regexp.MustCompile(`^mpv_([0-9]{2,})-([0-5][0-9])-([0-5][0-9])(?:\.([0-9]{1,3}))?$`)

// PlaybackSecond reads the timestamp shared by MPV and browser screenshots.
// Generated thumbnails (e.g. 128.jpg) and arbitrary image names are excluded.
func PlaybackSecond(name string) (float64, bool) {
	ext := filepath.Ext(name)
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		return 0, false
	}
	parts := playbackNamePattern.FindStringSubmatch(strings.TrimSuffix(name, ext))
	if parts == nil {
		return 0, false
	}
	hours, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return 0, false
	}
	minutes, _ := strconv.Atoi(parts[2])
	seconds, _ := strconv.ParseFloat(parts[3]+"."+parts[4], 64)
	return float64(hours)*3600 + float64(minutes)*60 + seconds, true
}

// ListPlaybackSeconds scans metadata only; it never decodes screenshot images.
func ListPlaybackSeconds(dataDir string, videoID int64) ([]float64, error) {
	seconds := []float64{}
	if dataDir == "" || videoID <= 0 {
		return seconds, nil
	}
	dir := filepath.Join(dataDir, "video", strconv.FormatInt(videoID, 10), "screenshot")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return seconds, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read screenshot markers: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if second, ok := PlaybackSecond(entry.Name()); ok {
			seconds = append(seconds, second)
		}
	}
	slices.Sort(seconds)
	return slices.Compact(seconds), nil
}
