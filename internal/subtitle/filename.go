package subtitle

import (
	"fmt"
	"regexp"
	"strings"
)

var filenameCode = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,119}$`)
var filenameLanguage = regexp.MustCompile(`^[a-z]{2,3}(?:-[a-z0-9]{2,8})*$`)

// FileName is shared by cache lookup and downloads. Reject path characters
// rather than silently mapping different codes to the same filename.
func FileName(code, language string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	language = strings.ToLower(strings.TrimSpace(language))
	if !filenameCode.MatchString(code) || !filenameLanguage.MatchString(language) {
		return "", fmt.Errorf("invalid subtitle code or language")
	}
	parts := strings.Split(language, "-")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) == 2 {
			parts[i] = strings.ToUpper(parts[i])
		}
		if len(parts[i]) == 4 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return code + "." + strings.Join(parts, "-") + ".srt", nil
}
