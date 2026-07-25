package jav

import (
	"context"
	"errors"
	"fmt"
	"io"
	"javboss/internal/util"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"javboss/internal/common/logging"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// javBus implements lookupProvider.
type javBus struct{}

var javBusProvider lookupProvider = javBus{}

const javBusRequestInterval = 500 * time.Millisecond

var javBusRateLimiter = struct {
	sync.Mutex
	next time.Time
}{}

// LookupActressByName implements lookupProvider.
func (javBus) LookupActressByName(name string) (*ActressInfo, error) {
	panic("unimplemented")
}

// LookupJavByCode fetches metadata for a given code.
func (javBus) LookupJavByCode(code string) (*JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ResourceNotFonud
	}
	lookupCode, rewrite := javBusLookupCode(code)
	logging.Info("javbus: code -> %s", lookupCode)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	info, err := fetchInfo(ctx, lookupCode)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}
	if info.Code == "" {
		info.Code = lookupCode
	}
	if rewrite != nil {
		normalizeJavBusRewrittenInfo(info, rewrite)
	}
	return info, nil
}

// LookupActressByCode resolves a solo movie code to its actress profile.
func (javBus) LookupActressByCode(code string) (*ActressInfo, error) {
	return nil, errors.New("javbus: lookup actress not supported")
}

// LookupActressURLByCodeAndName implements lookupProvider.
func (javBus) LookupActressURLByCodeAndName(code, name string) (string, error) {
	return "", errors.New("javbus: lookup actress url not supported")
}

// LookupCoverURLByCode resolves a cover image URL for a movie code.
func (javBus) LookupCoverURLByCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", ResourceNotFonud
	}
	lookupCode, _ := javBusLookupCode(code)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	doc, pageURL, err := fetchJavBusDocument(ctx, lookupCode)
	if err != nil {
		return "", err
	}
	coverURL := parseJavBusCoverURL(doc, pageURL)
	if coverURL == "" {
		return "", ResourceNotFonud
	}
	return coverURL, nil
}

// LookupSeriesURLByCode implements lookupProvider.
func (javBus) LookupSeriesURLByCode(code string) (string, error) {
	return "", errors.New("javbus: lookup series url not supported")
}

// LookupStudioURLByCode implements lookupProvider.
func (javBus) LookupStudioURLByCode(code string) (string, error) {
	return "", errors.New("javbus: lookup studio url not supported")
}

func fetchInfo(ctx context.Context, code string) (*JavInfo, error) {
	doc, url, err := fetchJavBusDocument(ctx, code)
	if err != nil {
		return nil, err
	}

	info := parseDocument(doc)
	if info == nil {
		logging.Info("javbus: parseDocument returned nil")
		return nil, ResourceNotFonud
	}
	info.CoverURL = parseJavBusCoverURL(doc, url)
	info.SampleImages = parseSampleImages(doc, url)
	if info.Code == "" || info.Title == "" {
		logging.Info("javbus: parsed title/code empty (title=%q code=%q)", info.Title, info.Code)
		return nil, ResourceNotFonud
	}
	logging.Info("javbus parsed from %s: title=%q tags=%d actors=%d", url, info.Title, len(info.Tags), len(info.Actors))
	return info, nil
}

type javBusCodeRewrite struct {
	inputPrefix   string
	requestPrefix string
}

var javBusCodeRewrites = []javBusCodeRewrite{
	{inputPrefix: "gana", requestPrefix: "200gana"},
	{inputPrefix: "mium", requestPrefix: "300mium"},
	{inputPrefix: "luxu", requestPrefix: "259luxu"},
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

func normalizeJavBusRewrittenInfo(info *JavInfo, rewrite *javBusCodeRewrite) {
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

func fetchJavBusDocument(ctx context.Context, code string) (*html.Node, string, error) {
	base := "https://www.javbus.com"

	url := fmt.Sprintf("%s/%s", base, code)

	req, err := buildRequest(ctx, url)
	if err != nil {
		return nil, "", err
	}

	logging.Info("javbus request: %s", url)

	resp, err := doJavBusRequest(req)
	// TODO: Should not return here, try curl fallback.
	if err != nil {
		if errors.Is(err, util.ErrCachedNotFound) {
			logging.Info("javbus: cached 404 for %s", url)
			return nil, "", ResourceNotFonud
		}
		return nil, "", err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, "", err
	}

	logging.Info("javbus response status: %s, length: %d bytes", resp.Status, len(body))

	if resp.StatusCode == http.StatusNotFound {
		logging.Info("javbus: %s 404 not found", url)
		return nil, "", ResourceNotFonud
	}
	if resp.StatusCode != http.StatusOK {
		logging.Info("javbus: non-200 status on %s: %s", url, resp.Status)
		return nil, "", errors.New("javbus: non-200 response")
	}

	doc, err := parseHTMLDocument(body)
	if err != nil {
		logging.Error("parse javbus html: %s", err.Error())
		return nil, "", ResourceNotFonud
	}
	return doc, url, nil
}

func doJavBusRequest(req *http.Request) (*http.Response, error) {
	if err := waitForJavBusRateLimit(req.Context()); err != nil {
		return nil, err
	}
	return util.DoRequest(req)
}

func waitForJavBusRateLimit(ctx context.Context) error {
	for {
		javBusRateLimiter.Lock()
		now := time.Now()
		if !now.Before(javBusRateLimiter.next) {
			javBusRateLimiter.next = now.Add(javBusRequestInterval)
			javBusRateLimiter.Unlock()
			return nil
		}
		wait := time.Until(javBusRateLimiter.next)
		javBusRateLimiter.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return fmt.Errorf("javbus: rate limit wait: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func buildRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://www.javbus.com/")
	req.Header.Set("Cookie", "age=verified; existmag=mag")
	return req, nil
}

func parseDocument(doc *html.Node) *JavInfo {
	rawTitle := firstTextByTag(doc, "h3")
	if rawTitle == "" {
		rawTitle = firstTextByTag(doc, "title")
	}
	title := cleanTitle(rawTitle)
	code := extractCode(doc)
	series := extractJavBusField(doc, "系列", "series")
	releaseUnix, duration := extractDetails(doc)

	tags := collectGenres(doc)
	actors := collectActors(doc)
	isUncensored := parseJavBusIsUncensored(doc)
	coverURL := parseJavBusCoverURL(doc, "")

	if title == "" && len(tags) == 0 && len(actors) == 0 {
		return nil
	}
	return &JavInfo{
		Title:        title,
		Code:         code,
		Series:       series,
		ReleaseUnix:  releaseUnix,
		DurationMin:  duration,
		Tags:         tags,
		Actors:       actors,
		CoverURL:     coverURL,
		SampleImages: parseSampleImages(doc, ""),
		IsUncensored: &isUncensored,
		Provider:     ProviderJavBus,
	}
}

func parseJavBusIsUncensored(root *html.Node) bool {
	var found bool
	documentSelection(root).Find("li.active a").EachWithBreak(func(_ int, link *goquery.Selection) bool {
		href := strings.ToLower(selectionAttr(link, "href"))
		text := strings.ToLower(cleanSelectionText(link))
		found = strings.Contains(href, "/uncensored") ||
			strings.Contains(text, "無碼") ||
			strings.Contains(text, "无码") ||
			strings.Contains(text, "uncensored")
		return !found
	})
	return found
}

func parseJavBusCoverURL(root *html.Node, pageURL string) string {
	doc := documentSelection(root)
	for _, candidate := range []string{
		selectionAttr(doc.Find(`meta[property="og:image"]`).First(), "content"),
		selectionAttr(doc.Find("a.bigImage").First(), "href"),
		selectionAttr(doc.Find("img.cover, .bigImage img").First(), "src"),
	} {
		if cover := resolveURL(pageURL, candidate); cover != "" {
			return cover
		}
	}
	return ""
}

func firstTextByTag(root *html.Node, tag string) string {
	return cleanSelectionText(documentSelection(root).Find(tag).First())
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
	documentSelection(root).Find("span").EachWithBreak(func(_ int, label *goquery.Selection) bool {
		labelText := normalizeJavBusFieldLabel(cleanSelectionText(label))
		if !javBusFieldLabelMatches(labelText, normalizedLabels) {
			return true
		}
		value = cleanSelectionText(label.NextAll().First())
		if value == "" {
			line := cleanSelectionText(label.Parent())
			value = strings.TrimSpace(strings.TrimPrefix(line, cleanSelectionText(label)))
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
	documentSelection(root).Find("p").EachWithBreak(func(_ int, paragraph *goquery.Selection) bool {
		text := cleanSelectionText(paragraph)
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
	documentSelection(section).Find("span.genre a").Each(func(_ int, link *goquery.Selection) {
		if strings.Contains(selectionAttr(link, "href"), "/star/") {
			return
		}
		text := cleanSelectionText(link)
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
	documentSelection(section).Find(`a[href*="/star/"]`).Each(func(_ int, link *goquery.Selection) {
		text := cleanSelectionText(link)
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
	return firstSelectionNode(documentSelection(root).Find("div.movie.row").First())
}
