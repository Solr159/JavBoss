package subtitle

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const MaxBytes = 2 << 20

var timing = regexp.MustCompile(`^(\d{2,3}):([0-5]\d):([0-5]\d)[,.](\d{3})\s+-->\s+(\d{2,3}):([0-5]\d):([0-5]\d)[,.](\d{3})$`)
var markup = regexp.MustCompile(`<[^>]*>|\{[^}]*\}`)

func timestampMS(parts []string) int64 {
	var values [4]int64
	for i := range values {
		values[i], _ = strconv.ParseInt(parts[i], 10, 64)
	}
	return ((values[0]*60+values[1])*60+values[2])*1000 + values[3]
}

// NormalizeSRT omits malformed cues and removes formatting/control markup before
// either player sees untrusted text. File-level size/encoding errors and tracks
// with no usable cues are rejected. Output is UTF-8 SRT with canonical timings.
func NormalizeSRT(data []byte) ([]byte, error) {
	if len(data) == 0 || len(data) > MaxBytes || !utf8.Valid(data) || strings.ContainsRune(string(data), 0) {
		return nil, fmt.Errorf("invalid subtitle size or encoding")
	}
	raw := strings.TrimSpace(strings.TrimPrefix(string(data), "\ufeff"))
	raw = strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	blocks := regexp.MustCompile(`\n[\t ]*\n+`).Split(raw, -1)
	var out strings.Builder
	count := 0
	for _, block := range blocks {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		if len(lines) < 2 {
			continue
		}
		if _, err := strconv.Atoi(strings.TrimSpace(lines[0])); err != nil {
			continue
		}
		m := timing.FindStringSubmatch(strings.TrimSpace(lines[1]))
		if m == nil {
			continue
		}
		start, end := timestampMS(m[1:5]), timestampMS(m[5:9])
		// Reversed/zero-duration intervals and empty cues cannot render.
		if start >= end || len(lines) == 2 {
			continue
		}
		text := markup.ReplaceAllString(html.UnescapeString(strings.Join(lines[2:], "\n")), "")
		text = strings.ReplaceAll(text, "\\", "")
		if strings.TrimSpace(text) == "" {
			continue
		}
		count++
		fmt.Fprintf(&out, "%d\n%s:%s:%s,%s --> %s:%s:%s,%s\n%s\n\n", count, m[1], m[2], m[3], m[4], m[5], m[6], m[7], m[8], text)
	}
	if count == 0 {
		return nil, fmt.Errorf("subtitle has no cues")
	}
	return []byte(out.String()), nil
}

func WebVTT(data []byte) ([]byte, error) {
	return WebVTTWithOffset(data, 0)
}

// WebVTTWithOffset derives a playback response without rewriting the shared SRT.
func WebVTTWithOffset(data []byte, offsetMS int64) ([]byte, error) {
	if offsetMS < -3600000 || offsetMS > 3600000 {
		return nil, fmt.Errorf("invalid subtitle offset")
	}
	normalized, err := NormalizeSRT(data)
	if err != nil {
		return nil, err
	}
	formatTime := func(ms int64) string {
		return fmt.Sprintf("%02d:%02d:%02d.%03d", ms/3600000, (ms/60000)%60, (ms/1000)%60, ms%1000)
	}
	var out strings.Builder
	out.WriteString("WEBVTT\n\n")
	count := 0
	for _, block := range strings.Split(strings.TrimSpace(string(normalized)), "\n\n") {
		lines := strings.Split(block, "\n")
		match := timing.FindStringSubmatch(lines[1])
		start, end := timestampMS(match[1:5])+offsetMS, timestampMS(match[5:9])+offsetMS
		// WebVTT timestamps cannot be negative. Cues already finished before
		// playback starts are omitted; cues crossing zero start at zero.
		if end <= 0 {
			continue
		}
		if start < 0 {
			start = 0
		}
		count++
		// Literal text must not become WebVTT markup or cue delimiters.
		text := html.EscapeString(strings.Join(lines[2:], "\n"))
		fmt.Fprintf(&out, "%d\n%s --> %s\n%s\n\n", count, formatTime(start), formatTime(end), text)
	}
	return []byte(out.String()), nil
}
