package subtitle

import (
	"regexp"
	"strings"
)

var partMarker = regexp.MustCompile(`(?i)(?:^|[-_. ])(?:cd|disc|disk|part|pt)[-_ .]*[0-9]+(?:[^0-9]|$)`)

// IsSplit identifies explicit parts, including CODE-123-1 / CODE-123.2.
// Full-film subtitles are not automatically applied to these files, and part
// subtitles are not automatically applied to a full film.
func IsSplit(code, name string) bool {
	if partMarker.MatchString(name) {
		return true
	}
	parts := strings.FieldsFunc(code, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	if len(parts) == 0 {
		return false
	}
	for i := range parts {
		parts[i] = regexp.QuoteMeta(parts[i])
	}
	pattern := `(?i)(?:^|[^a-z0-9])` + strings.Join(parts, `[-_ ]?`) + `[-_. ][1-9](?:[^a-z0-9]|$)`
	return regexp.MustCompile(pattern).MatchString(name)
}
