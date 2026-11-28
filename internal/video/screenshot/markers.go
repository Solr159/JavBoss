package screenshot

import (
	"fmt"
	"math"
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
	seconds, _ := strconv.Atoi(parts[3])
	millis, _ := strconv.Atoi(parts[4] + strings.Repeat("0", 3-len(parts[4])))
	// Convert once from integer milliseconds so the marker, JSON and delete
	// request all use the same float (e.g. 61.029, not 61.028999999999996).
	totalMillis := hours*3600000 + uint64(minutes)*60000 + uint64(seconds)*1000 + uint64(millis)
	return float64(totalMillis) / 1000, true
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

// PlaybackNamesAt returns all images represented by one deduplicated marker.
func PlaybackNamesAt(dataDir string, videoID int64, second float64) ([]string, error) {
	if dataDir == "" || videoID <= 0 || second < 0 || math.IsNaN(second) || math.IsInf(second, 0) {
		return nil, fmt.Errorf("invalid screenshot marker")
	}
	dir := filepath.Join(dataDir, "video", strconv.FormatInt(videoID, 10), "screenshot")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read screenshot files: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if value, ok := PlaybackSecond(entry.Name()); ok && value == second {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}
