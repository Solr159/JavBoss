package avwiki

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"javboss/internal/jav/internal/htmlutil"
	"javboss/internal/jav/internal/parseutil"
	"javboss/internal/jav/metadata"

	"github.com/PuerkitoBio/goquery"
)

var (
	profilePath = regexp.MustCompile(`^/av-actress/[^/]+/$`)
	birthDate   = regexp.MustCompile(`^(\d{4})年\s*(\d{1,2})月\s*(\d{1,2})日$`)
	height      = regexp.MustCompile(`(?i)T\s*(\d{2,3})(?:[^0-9]|$)`)
	bust        = regexp.MustCompile(`(?i)B\s*(\d{2,3})(?:[^0-9]|$)`)
	waist       = regexp.MustCompile(`(?i)W\s*(\d{2,3})(?:[^0-9]|$)`)
	hips        = regexp.MustCompile(`(?i)H\s*(\d{2,3})(?:[^0-9]|$)`)
	cup         = regexp.MustCompile(`(?i)([A-Z])\s*カップ`)
	romanName   = regexp.MustCompile(`^[A-Za-z]+(?:[ -][A-Za-z]+)*$`)
)

func parseActress(tag actressTag, origin string) *metadata.ActressInfo {
	target, err := url.Parse(tag.Link)
	base, baseErr := url.Parse(origin)
	if err != nil || baseErr != nil || target.Scheme != base.Scheme || !strings.EqualFold(target.Host, base.Host) || target.User != nil || !profilePath.MatchString(target.Path) {
		return nil
	}
	target.RawQuery, target.Fragment = "", ""
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(tag.Description))
	if err != nil {
		return nil
	}
	fields := make(map[string]string)
	doc.Find(".actress-data dt").Each(func(_ int, term *goquery.Selection) {
		label := strings.TrimRight(htmlutil.CleanSelectionText(term), "：: ")
		fields[label] = htmlutil.CleanSelectionText(term.NextFiltered("dd"))
	})
	nameField := fields["AV女優名"]
	// Verify the profile itself as well as the API's tag name; do not import a
	// different person's measurements from a malformed or mismatched description.
	name := nameField
	if i := strings.IndexAny(name, "（("); i >= 0 {
		name = name[:i]
	} else if i := strings.Index(name, " - "); i >= 0 {
		name = name[:i]
	}
	if normalizeName(name) == "" || normalizeName(name) != normalizeName(tag.Name) {
		return nil
	}
	info := &metadata.ActressInfo{JapaneseName: normalizeName(tag.Name), ProfileURL: target.String()}
	info.RomanName = parseRomanName(nameField, target.Path)
	if match := birthDate.FindStringSubmatch(fields["生年月日"]); len(match) == 4 {
		year, _ := strconv.Atoi(match[1])
		month, _ := strconv.Atoi(match[2])
		day, _ := strconv.Atoi(match[3])
		date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		if year > 0 && date.Year() == year && int(date.Month()) == month && date.Day() == day {
			info.BirthDate = int(date.Unix())
		}
	}
	info.HeightCM = measurementNumber(height, fields["サイズ"])
	info.Bust = measurementNumber(bust, fields["サイズ"])
	info.Waist = measurementNumber(waist, fields["サイズ"])
	info.Hips = measurementNumber(hips, fields["サイズ"])
	if match := cup.FindStringSubmatch(fields["サイズ"]); len(match) > 1 {
		info.Cup = parseutil.CupLetterToNumber(match[1])
	}
	return info
}

// AV Wiki mixes given-name-first text and family-name-first text. Its profile
// slugs use family-name-first order; match the family name rather than assuming
// that spaces or hyphens identify the order. Keep the spelling from the profile.
func parseRomanName(nameField, profilePath string) string {
	nameField = strings.ReplaceAll(nameField, "）", ")")
	i := strings.LastIndex(nameField, ")")
	if i < 0 {
		return ""
	}
	value := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(nameField[i+1:]), "-–—"))
	value = strings.Join(strings.Fields(value), " ")
	if !romanName.MatchString(value) {
		return ""
	}
	parts := strings.Fields(strings.ReplaceAll(value, "-", " "))
	switch len(parts) {
	case 1:
		// Preserve explicitly capitalized stage names such as RION.
		if parts[0] != strings.ToLower(parts[0]) {
			return parts[0]
		}
	case 2:
		slug := strings.TrimSuffix(strings.TrimPrefix(profilePath, "/av-actress/"), "/")
		slugParts := strings.Split(slug, "-")
		if len(slugParts) != 2 {
			return ""
		}
		if strings.EqualFold(parts[0], slugParts[0]) {
			parts[0], parts[1] = parts[1], parts[0]
		} else if !strings.EqualFold(parts[1], slugParts[0]) {
			// Leave uncertain names for another provider instead of persisting a
			// guessed order. Other profile fields can still be used.
			return ""
		}
	default:
		return ""
	}
	for i, part := range parts {
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}

func measurementNumber(pattern *regexp.Regexp, value string) int {
	match := pattern.FindStringSubmatch(value)
	if len(match) < 2 {
		return 0
	}
	number, _ := strconv.Atoi(match[1])
	return number
}
