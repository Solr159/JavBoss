package util

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Accept patterns like "ipx-633", "ipx633", "ipx633_ch", "ipx-714c" (letter suffix ignored).
var CodeRe = regexp.MustCompile(`(?i)([a-z]{2,6})[-_ ]?(\d{2,5})([a-z]{0,2})`)

var (
	fc2PPVCodeRe                = regexp.MustCompile(`(?i)(^|[^a-z0-9])fc2[-_ ]*ppv[-_ ]*([0-9]+)`)
	alphaNumericUncensoredRe    = regexp.MustCompile(`(?i)(^|[^a-z0-9])([a-z]+)(?:\s*([-_ ])\s*)?(\d{2,})([^a-z0-9]|$)`)
	mixedPrefixUncensoredRe     = regexp.MustCompile(`(?i)(^|[^a-z0-9])([a-z0-9]*[a-z][a-z0-9]*\d[a-z0-9]*[a-z][a-z0-9]*)[-_ ](\d{2,})([^a-z0-9]|$)`)
	mixedPrefixCensoredRe       = regexp.MustCompile(`(?i)(^|[^a-z0-9])([a-z][a-z0-9]{1,5})[-_ ](\d{2,5})([a-z]{0,2})([^a-z0-9]|$)`)
	pureNumericUncensoredCodeRe = regexp.MustCompile(`(^|[^0-9])(\d{4,}[-_]\d{2,})([^0-9]|$)`)
	explicitShortCodeRe         = regexp.MustCompile(`(?i)(^|[^a-z0-9])([a-z]{2,6})[-_ ](\d{2})([a-z]{0,2})([^a-z0-9]|$)`)
)

func ExtractCodeFromName(name string) []string {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	var out []string
	seen := make(map[string]struct{})

	appendUniqueCodes(&out, seen, extractCensoredCodesFromName(base))
	appendUniqueCodes(&out, seen, extractUncensoredCodesFromName(base))
	return out
}

func ExtractUncensoredCodesFromName(name string) []string {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	return extractUncensoredCodesFromName(base)
}

func extractFC2PPVCodes(base string) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, match := range fc2PPVCodeRe.FindAllStringSubmatch(base, -1) {
		appendUniqueCode(&out, seen, "FC2-PPV-"+match[2])
	}
	return out
}

func extractUncensoredCodesFromName(base string) []string {
	if codes := extractFC2PPVCodes(base); len(codes) > 0 {
		return codes
	}
	var out []string
	seen := make(map[string]struct{})

	for _, m := range mixedPrefixUncensoredRe.FindAllStringSubmatch(base, -1) {
		if len(m) < 4 {
			continue
		}
		appendUniqueCode(&out, seen, fmt.Sprintf("%s-%s", strings.TrimSpace(m[2]), strings.TrimSpace(m[3])))
	}

	for _, m := range alphaNumericUncensoredRe.FindAllStringSubmatch(base, -1) {
		if len(m) < 5 {
			continue
		}
		prefix := normalizeUncensoredAlphaPrefix(m[2])
		separator := strings.TrimSpace(m[3])
		number := strings.TrimSpace(m[4])
		if separator == "" {
			appendUniqueCode(&out, seen, prefix+number)
		}
		if separator != "" || len(prefix) > 1 {
			appendUniqueCode(&out, seen, fmt.Sprintf("%s-%s", prefix, number))
		}
	}

	for _, m := range pureNumericUncensoredCodeRe.FindAllStringSubmatch(base, -1) {
		if len(m) < 3 {
			continue
		}
		appendUniqueCode(&out, seen, strings.TrimSpace(m[2]))
	}

	for _, m := range explicitShortCodeRe.FindAllStringSubmatch(base, -1) {
		if len(m) < 5 {
			continue
		}
		prefix := strings.ToUpper(strings.TrimSpace(m[2]))
		number := strings.ToUpper(strings.TrimSpace(m[3]))
		suffix := strings.ToUpper(strings.TrimSpace(m[4]))
		base := fmt.Sprintf("%s-%s", prefix, number)
		appendUniqueCode(&out, seen, base)
		if suffix != "" {
			appendUniqueCode(&out, seen, base+suffix)
		}
	}
	return out
}

func normalizeUncensoredAlphaPrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ""
	}
	if len(prefix) == 1 {
		return strings.ToLower(prefix)
	}
	prefix = strings.ToLower(prefix)
	return strings.ToUpper(prefix[:1]) + prefix[1:]
}

func extractCensoredCodesFromName(base string) []string {
	// FC2 belongs to uncensored extraction; generic patterns would produce
	// truncated PPV numbers and unrelated codes from site-name prefixes.
	if fc2PPVCodeRe.MatchString(base) {
		return nil
	}
	var out []string
	seen := make(map[string]struct{})

	matches := CodeRe.FindAllStringSubmatch(base, -1)
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		suffix := strings.ToUpper(strings.TrimSpace(m[3]))
		number := normalizeNumber(m[2])
		base := fmt.Sprintf("%s-%s", strings.ToUpper(m[1]), number)
		appendUniqueCode(&out, seen, base)
		if suffix != "" {
			appendUniqueCode(&out, seen, base+suffix)
		}
	}
	for _, m := range mixedPrefixCensoredRe.FindAllStringSubmatch(base, -1) {
		if len(m) < 5 {
			continue
		}
		suffix := strings.ToUpper(strings.TrimSpace(m[4]))
		number := normalizeNumber(m[3])
		base := fmt.Sprintf("%s-%s", strings.ToUpper(m[2]), number)
		appendUniqueCode(&out, seen, base)
		if suffix != "" {
			appendUniqueCode(&out, seen, base+suffix)
		}
	}
	return out
}

func appendUniqueCodes(out *[]string, seen map[string]struct{}, codes []string) {
	for _, code := range codes {
		appendUniqueCode(out, seen, code)
	}
}

func appendUniqueCode(out *[]string, seen map[string]struct{}, code string) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return
	}
	if _, ok := seen[code]; ok {
		return
	}
	seen[code] = struct{}{}
	*out = append(*out, code)
}

// normalizeNumber trims leading zeros but keeps at least three digits (padding back if needed).
func normalizeNumber(num string) string {
	num = strings.TrimLeft(num, "0")
	if len(num) == 0 {
		num = "0"
	}
	if len(num) < 3 {
		num = fmt.Sprintf("%0*s", 3, num)
	}
	return strings.ToUpper(num)
}
