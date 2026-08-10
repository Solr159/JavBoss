package western

import (
	"path/filepath"
	"regexp"
	"strings"
)

var likelyJAVFilename = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:fc2(?:[-_ ]?ppv)?|1pondo|heyzo|tokyo[-_ ]?hot|caribbeancom|一本道|カリビアンコム)(?:[^a-z0-9]|$)`)

// IsLikelyJAVFilename applies conservative, high-confidence filename rules
// before a video is considered for Western metadata scraping.
func IsLikelyJAVFilename(filename string) bool {
	name := strings.ToLower(filepath.Base(strings.TrimSpace(filename)))
	return name != "" && likelyJAVFilename.MatchString(name)
}
