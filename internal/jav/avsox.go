package jav

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	"pornboss/internal/common/logging"
	"pornboss/internal/util"
)

// avsox implements lookupProvider.
type avsox struct{}

var avsoxProvider lookupProvider = avsox{}

const (
	avsoxBaseURL         = "https://avsox.click"
	avsoxUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	avsoxRequestInterval = 1500 * time.Millisecond
)

var avsoxRateLimiter = struct {
	sync.Mutex
	next time.Time
}{}

// LookupActressByName implements lookupProvider.
func (avsox) LookupActressByName(name string) (*ActressInfo, error) {
	return nil, errors.New("avsox: lookup actress not supported")
}

// LookupActressByCode implements lookupProvider.
func (avsox) LookupActressByCode(code string) (*ActressInfo, error) {
	return nil, errors.New("avsox: lookup actress not supported")
}

// LookupCoverURLByCode resolves a cover image URL for a movie code.
func (avsox) LookupCoverURLByCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	doc, detailURL, err := fetchAvsoxDetailByCode(ctx, code)
	if err != nil {
		return "", err
	}
	coverURL := parseAvsoxCoverURL(doc, detailURL)
	if coverURL == "" {
		return "", ResourceNotFonud
	}
	return coverURL, nil
}

// LookupJavByCode fetches metadata for a given code.
func (avsox) LookupJavByCode(code string) (*JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	doc, _, err := fetchAvsoxDetailByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	info := parseAvsoxMovieInfo(doc)
	if info == nil {
		return nil, ResourceNotFonud
	}
	if info.Code == "" {
		info.Code = code
	}
	return info, nil
}

func fetchAvsoxDetailByCode(ctx context.Context, code string) (*html.Node, string, error) {
	searchURL := fmt.Sprintf("%s/cn/search/%s", avsoxBaseURL, url.PathEscape(code))
	searchDoc, status, err := fetchAvsoxHTML(ctx, searchURL, avsoxBaseURL)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusNotFound || searchDoc == nil {
		return nil, "", ResourceNotFonud
	}

	detailURL := findAvsoxSearchResultURL(searchDoc, code, searchURL)
	if detailURL == "" {
		return nil, "", ResourceNotFonud
	}

	detailDoc, status, err := fetchAvsoxHTML(ctx, detailURL, searchURL)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusNotFound || detailDoc == nil {
		return nil, "", ResourceNotFonud
	}
	return detailDoc, detailURL, nil
}

func fetchAvsoxHTML(ctx context.Context, targetURL, referer string) (*html.Node, int, error) {
	req, err := buildAvsoxRequest(ctx, targetURL, referer)
	if err != nil {
		return nil, 0, err
	}

	logging.Info("avsox request: %s", targetURL)
	resp, err := doAvsoxRequest(req)
	if err != nil {
		if errors.Is(err, util.ErrCachedNotFound) {
			return nil, http.StatusNotFound, nil
		}
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	logging.Info("avsox response status: %s, length: %d bytes", resp.Status, len(body))
	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("avsox: http %d", resp.StatusCode)
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("avsox: parse html: %w", err)
	}
	return doc, resp.StatusCode, nil
}

func doAvsoxRequest(req *http.Request) (*http.Response, error) {
	if err := waitForAvsoxRateLimit(req.Context()); err != nil {
		return nil, err
	}
	return util.DoRequest(req)
}

func waitForAvsoxRateLimit(ctx context.Context) error {
	for {
		avsoxRateLimiter.Lock()
		now := time.Now()
		if !now.Before(avsoxRateLimiter.next) {
			avsoxRateLimiter.next = now.Add(avsoxRequestInterval)
			avsoxRateLimiter.Unlock()
			return nil
		}
		wait := time.Until(avsoxRateLimiter.next)
		avsoxRateLimiter.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return fmt.Errorf("avsox: rate limit wait: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func buildAvsoxRequest(ctx context.Context, targetURL, referer string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", avsoxUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req, nil
}

func findAvsoxSearchResultURL(root *html.Node, code, pageURL string) string {
	waterfall := findElementByID(root, "waterfall")
	if waterfall == nil {
		return ""
	}

	wantCode := normalizeAvsoxCodeForCompare(code)
	for item := waterfall.FirstChild; item != nil; item = item.NextSibling {
		if item.Type != html.ElementNode || item.Data != "div" || !hasClass(item, "item") {
			continue
		}
		if normalizeAvsoxCodeForCompare(findAvsoxSearchItemCode(item)) != wantCode {
			continue
		}
		if href := firstAnchorHref(item); href != "" {
			return resolveURL(pageURL, href)
		}
	}
	return ""
}

func findAvsoxSearchItemCode(item *html.Node) string {
	var code string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if code != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "date" {
			text := strings.TrimSpace(flattenText(n))
			if text != "" && !isAvsoxReleaseDate(text) {
				code = text
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(item)
	return code
}

type avsoxMovieFields struct {
	Title       string
	Code        string
	Studio      string
	Series      string
	ReleaseDate string
	Runtime     string
	Tags        []string
	Actors      []string
}

func parseAvsoxMovieInfo(root *html.Node) *JavInfo {
	scope := findAvsoxMainContainer(root)
	if scope == nil {
		return nil
	}

	fields := extractAvsoxMovieFields(scope)
	title := cleanAvsoxTitle(strings.TrimSpace(fields.Title), fields.Code)
	if title == "" {
		title = cleanAvsoxMoviePageTitle(strings.TrimSpace(firstTextByTag(root, "title")), fields.Code)
	}
	isUncensored := true

	info := &JavInfo{
		Title:        title,
		Code:         strings.TrimSpace(fields.Code),
		Studio:       strings.TrimSpace(fields.Studio),
		Series:       strings.TrimSpace(fields.Series),
		ReleaseUnix:  parseDateUnix(fields.ReleaseDate),
		DurationMin:  parseRuntimeMinutes(fields.Runtime),
		Tags:         dedupeNonEmpty(fields.Tags),
		Actors:       dedupeNonEmpty(fields.Actors),
		IsUncensored: &isUncensored,
		Provider:     ProviderAvsox,
	}
	if info.Title == "" && info.Code == "" && info.Studio == "" && info.Series == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func findAvsoxMainContainer(root *html.Node) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "body" {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "div" && hasClass(c, "container") && findDescendantByClass(c, "div", "movie") != nil {
					found = c
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

func extractAvsoxMovieFields(root *html.Node) avsoxMovieFields {
	var out avsoxMovieFields
	if root == nil {
		return out
	}

	out.Title = strings.TrimSpace(firstTextByTag(root, "h3"))
	if info := findDescendantByClass(root, "div", "info"); info != nil {
		out.Tags = collectAvmooGenreTexts(info)
		extractAvsoxInfoFields(info, &out)
	}
	if actors := findElementByID(root, "avatar-waterfall"); actors != nil {
		out.Actors = collectAnchorTexts(actors)
	}
	return out
}

func extractAvsoxInfoFields(root *html.Node, out *avsoxMovieFields) {
	pendingLabel := ""
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "p" {
			label, value := extractAvmooParagraphField(n)
			switch {
			case label != "" && value != "":
				pendingLabel = ""
				assignAvsoxMovieField(out, label, value)
			case label != "":
				pendingLabel = label
			case pendingLabel != "":
				assignAvsoxMovieField(out, pendingLabel, firstNonEmpty(firstAnchorText(n), flattenText(n)))
				pendingLabel = ""
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
}

func assignAvsoxMovieField(out *avsoxMovieFields, label, value string) {
	if out == nil {
		return
	}
	label = normalizeAvmooLabel(label)
	value = strings.TrimSpace(value)
	if label == "" || value == "" {
		return
	}

	switch label {
	case "識別碼", "识别码", "番號", "番号":
		if out.Code == "" {
			out.Code = value
		}
	case "發行日期", "发行日期", "發行時間", "发行时间", "発売日", "releasedate":
		if out.ReleaseDate == "" {
			out.ReleaseDate = value
		}
	case "長度", "长度", "時長", "时长", "duration", "runtime":
		if out.Runtime == "" {
			out.Runtime = value
		}
	case "製作商", "制作商", "studio", "メーカー", "メーカー名":
		if out.Studio == "" {
			out.Studio = value
		}
	case "系列", "series":
		if out.Series == "" {
			out.Series = value
		}
	}
}

func parseAvsoxCoverURL(root *html.Node, pageURL string) string {
	return parseAvmooCoverURL(root, pageURL)
}

func cleanAvsoxMoviePageTitle(title, code string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	title = strings.TrimSuffix(title, "- AVSOX")
	return cleanAvsoxTitle(title, code)
}

func cleanAvsoxTitle(title, code string) string {
	title = strings.TrimSpace(title)
	code = strings.TrimSpace(code)
	if title == "" || code == "" {
		return title
	}
	if len(title) >= len(code) && strings.EqualFold(title[:len(code)], code) {
		return strings.TrimSpace(title[len(code):])
	}
	return title
}

func normalizeAvsoxCodeForCompare(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func isAvsoxReleaseDate(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) == len("2006-01-02") && parseDateUnix(value) != 0
}
