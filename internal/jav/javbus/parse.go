package javbus

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"javboss/internal/jav/internal/htmlutil"
	"javboss/internal/jav/internal/parseutil"
	"javboss/internal/jav/metadata"
	"javboss/internal/util"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

type javBusCodeRewrite struct {
	inputPrefix   string
	requestPrefix string
}

func javBusLookupCode(code string) (string, *javBusCodeRewrite) {
	code = strings.TrimSpace(code)
	for _, rewrite := range javBusCodeRewrites {
		if javBusCodeHasPrefix(code, rewrite.inputPrefix) {
			r := rewrite
			return rewrite.requestPrefix + code[len(rewrite.inputPrefix):], &r
		}
	}
	return code, nil
}

func normalizeJavBusRewrittenInfo(info *metadata.JavInfo, rewrite *javBusCodeRewrite) {
	if info == nil {
		return
	}
	info.Code = stripJavBusRequestPrefix(info.Code, rewrite)
	info.Title = cleanTitle(stripJavBusRequestPrefix(info.Title, rewrite))
}

func javBusCodeHasPrefix(code, prefix string) bool {
	if len(code) <= len(prefix) || !strings.EqualFold(code[:len(prefix)], prefix) {
		return false
	}
	next := code[len(prefix)]
	return next == '-' || next == '_' || next == ' ' || (next >= '0' && next <= '9')
}

func stripJavBusRequestPrefix(value string, rewrite *javBusCodeRewrite) string {
	value = strings.TrimSpace(value)
	if rewrite == nil || !javBusCodeHasPrefix(value, rewrite.requestPrefix) {
		return value
	}
	addedPrefixLen := len(rewrite.requestPrefix) - len(rewrite.inputPrefix)
	if addedPrefixLen <= 0 || len(value) <= addedPrefixLen {
		return value
	}
	return strings.TrimSpace(value[addedPrefixLen:])
}

func parseJavBusGenreCategories(doc *html.Node, pathPrefix string) []metadata.GenreCategory {
	if doc == nil {
		return nil
	}
	var genres []metadata.GenreCategory
	htmlutil.DocumentSelection(doc).Find(".genre-box").Each(func(_ int, box *goquery.Selection) {
		category := util.SimplifyChineseName(htmlutil.CleanSelectionText(box.PrevAllFiltered("h4").First()))
		if category == "" {
			return
		}
		box.Find("a[href]").Each(func(_ int, link *goquery.Selection) {
			href := htmlutil.SelectionAttr(link, "href")
			if !isJavBusGenreLink(href, pathPrefix) {
				return
			}
			name := strings.TrimSpace(htmlutil.CleanSelectionText(link))
			if name == "" {
				return
			}
			genres = append(genres, metadata.GenreCategory{Name: name, Category: category})
		})
	})
	return genres
}

func isJavBusGenreLink(href, pathPrefix string) bool {
	parsed, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return false
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	prefix := strings.TrimSuffix(pathPrefix, "/") + "/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	id := strings.TrimPrefix(path, prefix)
	return id != "" && !strings.Contains(id, "/")
}

func parseDocument(doc *html.Node) *metadata.JavInfo {
	rawTitle := htmlutil.FirstTextByTag(doc, "h3")
	if rawTitle == "" {
		rawTitle = htmlutil.FirstTextByTag(doc, "title")
	}
	title := cleanTitle(rawTitle)
	code := extractCode(doc)
	// JavBus's label (發行商) is the publisher used as our studio.
	studio := extractJavBusField(doc, "發行商", "发行商", "レーベル", "label")
	series := extractJavBusField(doc, "系列", "series")
	releaseUnix, duration := extractDetails(doc)

	tags := collectGenres(doc)
	actors := collectActors(doc)
	isUncensored := parseJavBusIsUncensored(doc)
	coverURL := parseJavBusCoverURL(doc, "")

	if title == "" && len(tags) == 0 && len(actors) == 0 {
		return nil
	}
	return &metadata.JavInfo{
		Title:        title,
		Code:         code,
		Studio:       studio,
		Series:       series,
		ReleaseUnix:  releaseUnix,
		DurationMin:  duration,
		Tags:         tags,
		Actors:       actors,
		CoverURL:     coverURL,
		SampleImages: parseutil.ParseSampleImages(doc, ""),
		IsUncensored: &isUncensored,
		Provider:     metadata.ProviderJavBus,
	}
}

func parseJavBusIsUncensored(root *html.Node) bool {
	var found bool
	htmlutil.DocumentSelection(root).Find("li.active a").EachWithBreak(func(_ int, link *goquery.Selection) bool {
		href := strings.ToLower(htmlutil.SelectionAttr(link, "href"))
		text := strings.ToLower(htmlutil.CleanSelectionText(link))
		found = strings.Contains(href, "/uncensored") ||
			strings.Contains(text, "無碼") ||
			strings.Contains(text, "无码") ||
			strings.Contains(text, "uncensored")
		return !found
	})
	return found
}

func parseJavBusCoverURL(root *html.Node, pageURL string) string {
	doc := htmlutil.DocumentSelection(root)
	for _, candidate := range []string{
		htmlutil.SelectionAttr(doc.Find(`meta[property="og:image"]`).First(), "content"),
		htmlutil.SelectionAttr(doc.Find("a.bigImage").First(), "href"),
		htmlutil.SelectionAttr(doc.Find("img.cover, .bigImage img").First(), "src"),
	} {
		if cover := parseutil.ResolveURL(pageURL, candidate); cover != "" {
			return cover
		}
	}
	return ""
}

func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "- JavBus")
	s = strings.TrimSpace(s)

	// Strip leading code like "CPDE-072" or "CPDE072".
	codePrefix := regexp.MustCompile(`(?i)^[a-z]{2,6}[-_ ]?\d{2,5}\s*`)
	s = codePrefix.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func extractCode(root *html.Node) string {
	return extractJavBusField(root, "識別碼", "id:")
}

func extractJavBusField(root *html.Node, labels ...string) string {
	normalizedLabels := make([]string, 0, len(labels))
	for _, label := range labels {
		label = normalizeJavBusFieldLabel(label)
		if label != "" {
			normalizedLabels = append(normalizedLabels, label)
		}
	}
	if len(normalizedLabels) == 0 {
		return ""
	}

	var value string
	htmlutil.DocumentSelection(root).Find("span").EachWithBreak(func(_ int, label *goquery.Selection) bool {
		labelText := normalizeJavBusFieldLabel(htmlutil.CleanSelectionText(label))
		if !javBusFieldLabelMatches(labelText, normalizedLabels) {
			return true
		}
		value = htmlutil.CleanSelectionText(label.NextAll().First())
		if value == "" {
			line := htmlutil.CleanSelectionText(label.Parent())
			value = strings.TrimSpace(strings.TrimPrefix(line, htmlutil.CleanSelectionText(label)))
		}
		return value == ""
	})
	return value
}

func normalizeJavBusFieldLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, ":")
	value = strings.TrimSuffix(value, "：")
	return strings.TrimSpace(value)
}

func javBusFieldLabelMatches(text string, labels []string) bool {
	for _, label := range labels {
		if text == label || strings.Contains(text, label) {
			return true
		}
	}
	return false
}

func extractDetails(root *html.Node) (releaseUnix int64, durationMin int) {
	dateRe := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	durationRe := regexp.MustCompile(`(\d{1,4})\s*(分鐘|分钟|分|分間|min)?`)

	var releaseStr string
	htmlutil.DocumentSelection(root).Find("p").EachWithBreak(func(_ int, paragraph *goquery.Selection) bool {
		text := htmlutil.CleanSelectionText(paragraph)
		lower := strings.ToLower(text)
		if releaseStr == "" && (strings.Contains(text, "發行日期") || strings.Contains(text, "発売日") || strings.Contains(lower, "release")) {
			releaseStr = dateRe.FindString(text)
		}
		if durationMin == 0 && (strings.Contains(text, "長度") || strings.Contains(text, "時長") || strings.Contains(text, "時間") || strings.Contains(lower, "length") || strings.Contains(lower, "duration")) {
			if match := durationRe.FindStringSubmatch(text); len(match) >= 2 {
				if value, err := strconv.Atoi(strings.TrimSpace(match[1])); err == nil {
					durationMin = value
				}
			}
		}
		return releaseStr == "" || durationMin == 0
	})
	if releaseStr != "" {
		if t, err := time.Parse("2006-01-02", releaseStr); err == nil {
			releaseUnix = t.Unix()
		}
	}
	return releaseUnix, durationMin
}

func collectGenres(root *html.Node) []string {
	section := findMovieSection(root)
	if section == nil {
		section = root
	}

	seen := make(map[string]struct{})
	var out []string
	htmlutil.DocumentSelection(section).Find("span.genre a").Each(func(_ int, link *goquery.Selection) {
		if strings.Contains(htmlutil.SelectionAttr(link, "href"), "/star/") {
			return
		}
		text := htmlutil.CleanSelectionText(link)
		if text != "" {
			if _, exists := seen[text]; !exists {
				seen[text] = struct{}{}
				out = append(out, text)
			}
		}
	})
	return out
}

func collectActors(root *html.Node) []string {
	section := findMovieSection(root)
	if section == nil {
		section = root
	}

	seen := make(map[string]struct{})
	var out []string
	htmlutil.DocumentSelection(section).Find(`a[href*="/star/"]`).Each(func(_ int, link *goquery.Selection) {
		text := htmlutil.CleanSelectionText(link)
		if text != "" {
			if _, exists := seen[text]; !exists {
				seen[text] = struct{}{}
				out = append(out, text)
			}
		}
	})
	return out
}

func findMovieSection(root *html.Node) *html.Node {
	return htmlutil.FirstSelectionNode(htmlutil.DocumentSelection(root).Find("div.movie.row").First())
}
