package parseutil

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

func ParseDateUnix(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	re := regexp.MustCompile(`\d{4}[-/]\d{2}[-/]\d{2}`)
	match := re.FindString(value)
	if match == "" {
		return 0
	}
	match = strings.ReplaceAll(match, "/", "-")
	t, err := time.Parse("2006-01-02", match)
	if err != nil {
		return 0
	}
	return t.Unix()
}

func ParseBirthDateUnix(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	return int(ParseDateUnix(value))
}

func ParseRuntimeMinutes(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(value)
	if match == "" {
		return 0
	}
	minutes, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}
	return minutes
}

func CupLetterToNumber(value string) int {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return 0
	}
	r := rune(value[0])
	if r < 'A' || r > 'Z' {
		return 0
	}
	return int(r-'A') + 1
}

func ParseHeightCM(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(value)
	if match == "" {
		return 0
	}
	height, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}
	return height
}

func DedupeNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func ContainsJapaneseRunes(value string) bool {
	for _, r := range value {
		switch {
		case r >= 0x3040 && r <= 0x30ff: // Hiragana + Katakana
			return true
		case r >= 0x31f0 && r <= 0x31ff: // Katakana Phonetic Extensions
			return true
		case r >= 0x4e00 && r <= 0x9fff: // CJK Unified Ideographs
			return true
		case r >= 0xff66 && r <= 0xff9d: // Halfwidth Katakana
			return true
		}
	}
	return false
}
