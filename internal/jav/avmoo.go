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

	"pornboss/internal/common/logging"
	"pornboss/internal/util"
)

// avmoo implements lookupProvider.
type avmoo struct{}

var avmooProvider lookupProvider = avmoo{}

const (
	avmooBaseURL         = "https://avmoo.shop"
	avmooUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	avmooRequestInterval = 1500 * time.Millisecond
)

var avmooRateLimiter = struct {
	sync.Mutex
	next time.Time
}{}

// LookupActressByJapaneseName implements lookupProvider.
func (avmoo) LookupActressByJapaneseName(name string) (*ActressInfo, error) {
	return nil, errors.New("avmoo: lookup actress not supported")
}

// LookupActressByCode implements lookupProvider.
func (avmoo) LookupActressByCode(code string) (*ActressInfo, error) {
	return nil, errors.New("avmoo: lookup actress not supported")
}

// LookupCoverURLByCode resolves a cover image URL for a movie code.
func (avmoo) LookupCoverURLByCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	doc, detailURL, err := fetchAvmooDetailByCode(ctx, code)
	if err != nil {
		return "", err
	}
	coverURL := parseAvmooCoverURL(doc, detailURL)
	if coverURL == "" {
		return "", ResourceNotFonud
	}
	return coverURL, nil
}

// LookupJavByCode fetches metadata for a given code.
func (avmoo) LookupJavByCode(code string) (*JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	doc, _, err := fetchAvmooDetailByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	info := parseAvmooMovieInfo(doc)
	if info == nil {
		return nil, ResourceNotFonud
	}
	if info.Code == "" {
		info.Code = code
	}
	return info, nil
}

func fetchAvmooDetailByCode(ctx context.Context, code string) (*html.Node, string, error) {
	searchURL := fmt.Sprintf("%s/tw/search/%s", avmooBaseURL, url.PathEscape(code))
	searchDoc, status, err := fetchAvmooHTML(ctx, searchURL, avmooBaseURL)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusNotFound || searchDoc == nil {
		return nil, "", ResourceNotFonud
	}

	detailURL := findAvmooSearchResultURL(searchDoc, code, searchURL)
	if detailURL == "" {
		return nil, "", ResourceNotFonud
	}

	detailDoc, status, err := fetchAvmooHTML(ctx, detailURL, searchURL)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusNotFound || detailDoc == nil {
		return nil, "", ResourceNotFonud
	}
	return detailDoc, detailURL, nil
}

func fetchAvmooHTML(ctx context.Context, targetURL, referer string) (*html.Node, int, error) {
	req, err := buildAvmooRequest(ctx, targetURL, referer)
	if err != nil {
		return nil, 0, err
	}

	logging.Info("avmoo request: %s", targetURL)
	resp, err := doAvmooRequest(req)
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

	logging.Info("avmoo response status: %s, length: %d bytes", resp.Status, len(body))
	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("avmoo: http %d", resp.StatusCode)
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("avmoo: parse html: %w", err)
	}
	return doc, resp.StatusCode, nil
}

func doAvmooRequest(req *http.Request) (*http.Response, error) {
	if err := waitForAvmooRateLimit(req.Context()); err != nil {
		return nil, err
	}
	return util.DoRequest(req)
}

func waitForAvmooRateLimit(ctx context.Context) error {
	for {
		avmooRateLimiter.Lock()
		now := time.Now()
		if !now.Before(avmooRateLimiter.next) {
			avmooRateLimiter.next = now.Add(avmooRequestInterval)
			avmooRateLimiter.Unlock()
			return nil
		}
		wait := time.Until(avmooRateLimiter.next)
		avmooRateLimiter.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return fmt.Errorf("avmoo: rate limit wait: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func buildAvmooRequest(ctx context.Context, targetURL, referer string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", avmooUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req, nil
}

func findAvmooSearchResultURL(root *html.Node, code, pageURL string) string {
	waterfall := findElementByID(root, "waterfall")
	if waterfall == nil {
		return ""
	}

	wantCode := normalizeAvmooCode(code)
	for item := waterfall.FirstChild; item != nil; item = item.NextSibling {
		if item.Type != html.ElementNode || item.Data != "div" || !hasClass(item, "item") {
			continue
		}
		if normalizeAvmooCode(findAvmooSearchItemCode(item)) != wantCode {
			continue
		}
		if href := firstAnchorHref(item); href != "" {
			return resolveURL(pageURL, href)
		}
	}
	return ""
}

func findAvmooSearchItemCode(item *html.Node) string {
	var code string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if code != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "date" {
			text := strings.TrimSpace(flattenText(n))
			if util.CodeRe.MatchString(text) {
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

type avmooMovieFields struct {
	Title       string
	Code        string
	Series      string
	ReleaseDate string
	Runtime     string
	Tags        []string
	Actors      []string
}

func parseAvmooMovieInfo(root *html.Node) *JavInfo {
	scope := findAvmooMainContainer(root)
	if scope == nil {
		return nil
	}

	fields := extractAvmooMovieFields(scope)
	title := strings.TrimSpace(fields.Title)
	if title == "" {
		title = cleanAvmooMoviePageTitle(strings.TrimSpace(firstTextByTag(root, "title")))
	}

	info := &JavInfo{
		Title:       title,
		Code:        strings.TrimSpace(fields.Code),
		Series:      strings.TrimSpace(fields.Series),
		ReleaseUnix: parseDateUnix(fields.ReleaseDate),
		DurationMin: parseRuntimeMinutes(fields.Runtime),
		Tags:        dedupeNonEmpty(fields.Tags),
		Actors:      dedupeNonEmpty(fields.Actors),
		Provider:    ProviderAvmoo,
	}
	if info.Title == "" && info.Code == "" && info.Series == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func findAvmooMainContainer(root *html.Node) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "container") {
			if findDescendantByClass(n, "div", "movie") != nil {
				found = n
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

func extractAvmooMovieFields(root *html.Node) avmooMovieFields {
	var out avmooMovieFields
	if root == nil {
		return out
	}

	out.Title = cleanAvmooTitle(strings.TrimSpace(firstTextByTag(root, "h3")))
	if info := findDescendantByClass(root, "div", "info"); info != nil {
		out.Tags = collectAvmooGenreTexts(info)
		extractAvmooInfoFields(info, &out)
	}
	if actors := findElementByID(root, "avatar-waterfall"); actors != nil {
		out.Actors = collectAnchorTexts(actors)
	}
	return out
}

func extractAvmooInfoFields(root *html.Node, out *avmooMovieFields) {
	pendingLabel := ""
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "p" {
			label, value := extractAvmooParagraphField(n)
			switch {
			case label != "" && value != "":
				pendingLabel = ""
				assignAvmooMovieField(out, label, value)
			case label != "":
				pendingLabel = label
			case pendingLabel != "":
				assignAvmooMovieField(out, pendingLabel, firstNonEmpty(firstAnchorText(n), flattenText(n)))
				pendingLabel = ""
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
}

func extractAvmooParagraphField(p *html.Node) (string, string) {
	if hasClass(p, "header") {
		return strings.TrimSpace(flattenText(p)), ""
	}
	for c := p.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "span" && hasClass(c, "header") {
			label := strings.TrimSpace(flattenText(c))
			value := collectTextAfterNode(c)
			return label, value
		}
	}
	return "", ""
}

func assignAvmooMovieField(out *avmooMovieFields, label, value string) {
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
	case "發行日期", "发行日期", "発売日", "release date":
		if out.ReleaseDate == "" {
			out.ReleaseDate = value
		}
	case "長度", "长度", "時長", "时长", "duration", "runtime":
		if out.Runtime == "" {
			out.Runtime = value
		}
	case "系列", "series":
		if out.Series == "" {
			out.Series = value
		}
	}
}

func collectAvmooGenreTexts(root *html.Node) []string {
	if root == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var texts []string
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, inGenre bool) {
		in := inGenre || (n.Type == html.ElementNode && n.Data == "span" && hasClass(n, "genre"))
		if n.Type == html.ElementNode && n.Data == "a" && in {
			text := strings.TrimSpace(flattenText(n))
			if text != "" {
				if _, ok := seen[text]; !ok {
					seen[text] = struct{}{}
					texts = append(texts, text)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, in)
		}
	}
	walk(root, false)
	return texts
}

func parseAvmooCoverURL(root *html.Node, pageURL string) string {
	if root == nil {
		return ""
	}

	var cover string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if cover != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "bigImage") {
			if href := strings.TrimSpace(attrValue(n, "href")); href != "" {
				cover = resolveURL(pageURL, href)
				return
			}
		}
		if n.Type == html.ElementNode && n.Data == "img" {
			if src := strings.TrimSpace(attrValue(n, "src")); src != "" && isInsideClass(n, "screencap") {
				cover = resolveURL(pageURL, src)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return strings.TrimSpace(cover)
}

func findElementByID(root *html.Node, id string) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && attrValue(n, "id") == id {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

func findDescendantByClass(root *html.Node, tag, class string) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == tag && hasClass(n, class) {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

func isInsideClass(n *html.Node, class string) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && hasClass(p, class) {
			return true
		}
	}
	return false
}

func collectTextAfterNode(node *html.Node) string {
	var b strings.Builder
	for cur := node.NextSibling; cur != nil; cur = cur.NextSibling {
		if cur.Type == html.ElementNode {
			text := strings.TrimSpace(flattenText(cur))
			if text != "" {
				if b.Len() > 0 {
					b.WriteString(" ")
				}
				b.WriteString(text)
			}
			continue
		}
		if cur.Type == html.TextNode {
			text := strings.TrimSpace(cur.Data)
			if text != "" {
				if b.Len() > 0 {
					b.WriteString(" ")
				}
				b.WriteString(text)
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func cleanAvmooMoviePageTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	title = strings.TrimSuffix(title, "- AVMOO")
	return cleanAvmooTitle(title)
}

func cleanAvmooTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	re := regexp.MustCompile(`(?i)^[a-z]{2,6}[-_ ]?\d{2,5}[a-z]{0,2}\s+`)
	title = re.ReplaceAllString(title, "")
	return strings.TrimSpace(title)
}

func normalizeAvmooLabel(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	label = strings.TrimSuffix(label, ":")
	label = strings.TrimSuffix(label, "：")
	return strings.Join(strings.Fields(label), "")
}

func normalizeAvmooCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	var b strings.Builder
	for _, r := range code {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
