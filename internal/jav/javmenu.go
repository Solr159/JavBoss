package jav

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	"javboss/internal/common/logging"
	"javboss/internal/util"
)

// javMenu implements lookupProvider.
type javMenu struct{}

var javMenuProvider lookupProvider = javMenu{}

const (
	javMenuBaseURL         = "https://javmenu.com"
	javMenuUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	javMenuRequestInterval = 1500 * time.Millisecond
)

var javMenuRateLimiter = struct {
	sync.Mutex
	next time.Time
}{}

// LookupActressByName implements lookupProvider.
func (javMenu) LookupActressByName(name string) (*ActressInfo, error) {
	return nil, errors.New("javmenu: lookup actress not supported")
}

// LookupActressByCode implements lookupProvider.
func (javMenu) LookupActressByCode(code string) (*ActressInfo, error) {
	return nil, errors.New("javmenu: lookup actress not supported")
}

// LookupActressURLByCodeAndName implements lookupProvider.
func (javMenu) LookupActressURLByCodeAndName(code, name string) (string, error) {
	return "", errors.New("javmenu: lookup actress url not supported")
}

// LookupCoverURLByCode implements lookupProvider.
func (javMenu) LookupCoverURLByCode(code string) (string, error) {
	return "", errors.New("javmenu: lookup cover not supported")
}

// LookupSeriesURLByCode implements lookupProvider.
func (javMenu) LookupSeriesURLByCode(code string) (string, error) {
	return "", errors.New("javmenu: lookup series url not supported")
}

// LookupStudioURLByCode implements lookupProvider.
func (javMenu) LookupStudioURLByCode(code string) (string, error) {
	return "", errors.New("javmenu: lookup studio url not supported")
}

// LookupJavByCode fetches metadata for a given code.
func (javMenu) LookupJavByCode(code string) (*JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	doc, _, err := fetchJavMenuDetailByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	info := parseJavMenuMovieInfo(doc)
	if info == nil {
		return nil, ResourceNotFonud
	}
	if info.Code == "" {
		info.Code = code
	}
	return info, nil
}

func fetchJavMenuDetailByCode(ctx context.Context, code string) (*html.Node, string, error) {
	targetURL := fmt.Sprintf("%s/%s", javMenuBaseURL, url.PathEscape(strings.ToUpper(strings.TrimSpace(code))))
	doc, status, err := fetchJavMenuHTML(ctx, targetURL, javMenuBaseURL)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusNotFound || doc == nil {
		return nil, "", ResourceNotFonud
	}
	return doc, targetURL, nil
}

func fetchJavMenuHTML(ctx context.Context, targetURL, referer string) (*html.Node, int, error) {
	req, err := buildJavMenuRequest(ctx, targetURL, referer)
	if err != nil {
		return nil, 0, err
	}

	logging.Info("javmenu request: %s", targetURL)
	resp, err := doJavMenuRequest(req)
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

	logging.Info("javmenu response status: %s, length: %d bytes", resp.Status, len(body))
	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("javmenu: http %d", resp.StatusCode)
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("javmenu: parse html: %w", err)
	}
	return doc, resp.StatusCode, nil
}

func doJavMenuRequest(req *http.Request) (*http.Response, error) {
	if err := waitForJavMenuRateLimit(req.Context()); err != nil {
		return nil, err
	}
	return util.DoRequest(req)
}

func waitForJavMenuRateLimit(ctx context.Context) error {
	for {
		javMenuRateLimiter.Lock()
		now := time.Now()
		if !now.Before(javMenuRateLimiter.next) {
			javMenuRateLimiter.next = now.Add(javMenuRequestInterval)
			javMenuRateLimiter.Unlock()
			return nil
		}
		wait := time.Until(javMenuRateLimiter.next)
		javMenuRateLimiter.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return fmt.Errorf("javmenu: rate limit wait: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func buildJavMenuRequest(ctx context.Context, targetURL, referer string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", javMenuUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req, nil
}

type javMenuMovieFields struct {
	Title       string
	Code        string
	Studio      string
	Series      string
	ReleaseDate string
	Runtime     string
	Tags        []string
	Actors      []string
}

func parseJavMenuMovieInfo(root *html.Node) *JavInfo {
	cardBody := findJavMenuInfoCardBody(root)
	if cardBody == nil {
		return nil
	}

	fields := extractJavMenuMovieFields(cardBody)
	title := cleanJavMenuTitle(strings.TrimSpace(firstTextByTag(root, "h1")), fields.Code)
	if title == "" {
		title = cleanJavMenuTitle(strings.TrimSpace(firstTextByTag(root, "title")), fields.Code)
	}

	info := &JavInfo{
		Title:       title,
		Code:        strings.TrimSpace(fields.Code),
		Studio:      strings.TrimSpace(fields.Studio),
		Series:      strings.TrimSpace(fields.Series),
		ReleaseUnix: parseDateUnix(fields.ReleaseDate),
		DurationMin: parseRuntimeMinutes(fields.Runtime),
		Tags:        dedupeNonEmpty(fields.Tags),
		Actors:      dedupeNonEmpty(fields.Actors),
		Provider:    ProviderJavMenu,
	}
	if info.Title == "" && info.Code == "" && info.Studio == "" && info.Series == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func findJavMenuInfoCardBody(root *html.Node) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "card") && hasClass(n, "rounded") {
			body := findDescendantByClass(n, "div", "card-body")
			if body != nil && strings.Contains(flattenText(body), "影片資料") {
				found = body
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

func extractJavMenuMovieFields(cardBody *html.Node) javMenuMovieFields {
	var out javMenuMovieFields
	if cardBody == nil {
		return out
	}

	for row := cardBody.FirstChild; row != nil; row = row.NextSibling {
		if row.Type != html.ElementNode || row.Data != "div" {
			continue
		}
		labelNode := firstJavMenuFieldLabelNode(row)
		if labelNode == nil {
			continue
		}
		label := normalizeJavMenuLabel(flattenText(labelNode))
		if label == "" {
			continue
		}
		switch label {
		case "番號", "番号", "識別碼", "识别码":
			out.Code = cleanJavMenuCode(collectTextAfterNode(labelNode))
		case "發佈於", "发布于", "發行日期", "发行日期", "発売日", "release date":
			out.ReleaseDate = firstNonEmpty(out.ReleaseDate, collectTextAfterNode(labelNode))
		case "時長", "时长", "長度", "长度", "duration", "runtime":
			out.Runtime = firstNonEmpty(out.Runtime, collectTextAfterNode(labelNode))
		case "出版", "發行", "发行", "片商", "製作商", "制作商", "studio", "maker", "publisher":
			out.Studio = firstNonEmpty(out.Studio, firstNonEmpty(firstAnchorText(row), collectTextAfterNode(labelNode)))
		case "系列", "series":
			out.Series = firstNonEmpty(out.Series, firstNonEmpty(firstAnchorText(row), collectTextAfterNode(labelNode)))
		case "類別", "类别", "主題", "主题", "genre", "genres", "tags":
			out.Tags = append(out.Tags, collectJavMenuAnchorTextsByClass(row, "genre")...)
		case "女優", "女优", "演員", "演员", "actress", "actor", "actors":
			out.Actors = append(out.Actors, collectJavMenuAnchorTextsByClass(row, "actress")...)
		}
	}
	return out
}

func firstJavMenuFieldLabelNode(row *html.Node) *html.Node {
	for c := row.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "span" {
			return c
		}
	}
	return nil
}

func collectJavMenuAnchorTextsByClass(root *html.Node, class string) []string {
	var values []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, class) {
			if text := strings.TrimSpace(flattenText(n)); text != "" {
				values = append(values, text)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return values
}

func cleanJavMenuTitle(title, code string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	title = strings.TrimSuffix(title, "免費AV在線看")
	title = strings.TrimSuffix(title, "免费AV在线看")
	title = strings.TrimSpace(title)
	if code != "" {
		title = strings.TrimSpace(strings.TrimPrefix(title, code))
	}
	re := regexp.MustCompile(`(?i)^[a-z]{2,8}[-_ ]?\d{2,6}[a-z]{0,3}\s+`)
	title = re.ReplaceAllString(title, "")
	if idx := strings.Index(title, " | "); idx >= 0 {
		title = strings.TrimSpace(title[:idx])
	}
	return strings.TrimSpace(title)
}

func cleanJavMenuCode(code string) string {
	code = strings.TrimSpace(code)
	code = strings.ReplaceAll(code, "\u00a0", " ")
	code = strings.Join(strings.Fields(code), "")
	code = strings.ReplaceAll(code, "－", "-")
	code = strings.ReplaceAll(code, "ー", "-")
	return strings.TrimSpace(code)
}

func normalizeJavMenuLabel(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	label = strings.TrimSuffix(label, ":")
	label = strings.TrimSuffix(label, "：")
	return strings.Join(strings.Fields(label), "")
}
