package jav

import (
	"context"
	"crypto/tls"
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

// javDB implements lookupProvider.
type javDB struct{}

var javDBProvider lookupProvider = javDB{}

const (
	javDBBaseURL   = "https://javdb.com"
	javDBUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

var (
	javDBHTTPClientOnce sync.Once
	javDBHTTPClient     *http.Client
)

// LookupActressByJapaneseName implements lookupProvider.
func (javDB) LookupActressByJapaneseName(name string) (*ActressInfo, error) {
	return nil, errors.New("javdb: lookup actress not supported")
}

// LookupActressByCode implements lookupProvider.
func (javDB) LookupActressByCode(code string) (*ActressInfo, error) {
	return nil, errors.New("javdb: lookup actress not supported")
}

// LookupCoverURLByCode resolves a cover image URL for a movie code.
func (javDB) LookupCoverURLByCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	doc, detailURL, err := fetchJavDBDetailByCode(ctx, code)
	if err != nil {
		return "", err
	}
	coverURL := parseJavDBCoverURL(doc, detailURL)
	if coverURL == "" {
		return "", ResourceNotFonud
	}
	return coverURL, nil
}

// LookupJavByCode fetches metadata for a given code.
func (javDB) LookupJavByCode(code string) (*JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	doc, _, err := fetchJavDBDetailByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	info := parseJavDBMovieInfo(doc)
	if info == nil {
		return nil, ResourceNotFonud
	}
	if info.Code == "" {
		info.Code = code
	}
	return info, nil
}

func fetchJavDBDetailByCode(ctx context.Context, code string) (*html.Node, string, error) {
	searchURL := fmt.Sprintf("%s/search?q=%s&f=all", javDBBaseURL, url.QueryEscape(code))
	searchDoc, status, err := fetchJavDBHTML(ctx, searchURL, javDBBaseURL)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusNotFound || searchDoc == nil {
		return nil, "", ResourceNotFonud
	}

	detailURL := findJavDBSearchResultURL(searchDoc, code, searchURL)
	if detailURL == "" {
		return nil, "", ResourceNotFonud
	}

	detailDoc, status, err := fetchJavDBHTML(ctx, detailURL, searchURL)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusNotFound || detailDoc == nil {
		return nil, "", ResourceNotFonud
	}
	return detailDoc, detailURL, nil
}

func fetchJavDBHTML(ctx context.Context, targetURL, referer string) (*html.Node, int, error) {
	req, err := buildJavDBRequest(ctx, targetURL, referer)
	if err != nil {
		return nil, 0, err
	}

	logging.Info("javdb request: %s", targetURL)
	resp, err := doJavDBRequest(req)
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

	logging.Info("javdb response status: %s, length: %d bytes", resp.Status, len(body))
	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("javdb: http %d", resp.StatusCode)
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("javdb: parse html: %w", err)
	}
	return doc, resp.StatusCode, nil
}

func doJavDBRequest(req *http.Request) (*http.Response, error) {
	return defaultJavDBHTTPClient().Do(req)
}

func defaultJavDBHTTPClient() *http.Client {
	javDBHTTPClientOnce.Do(func() {
		javDBHTTPClient = util.NewHTTPClientWithTransport(15*time.Second, func(t *http.Transport) {
			t.ForceAttemptHTTP2 = true
			t.TLSClientConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
				MaxVersion: tls.VersionTLS13,
				NextProtos: []string{"h2", "http/1.1"},
			}
			t.MaxIdleConns = 200
			t.MaxIdleConnsPerHost = 20
			t.MaxConnsPerHost = 50
		})
	})
	return javDBHTTPClient
}

func buildJavDBRequest(ctx context.Context, targetURL, referer string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", javDBUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cookie", "over18=1")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req, nil
}

func findJavDBSearchResultURL(root *html.Node, code, pageURL string) string {
	list := findJavDBMovieList(root)
	if list == nil {
		return ""
	}

	var detailURL string
	wantCode := normalizeJavDBCode(code)
	for item := list.FirstChild; item != nil; item = item.NextSibling {
		if item.Type != html.ElementNode || item.Data != "div" || !hasClass(item, "item") {
			continue
		}
		itemCode := findJavDBSearchItemCode(item)
		if normalizeJavDBCode(itemCode) != wantCode {
			continue
		}
		if href := firstAnchorHref(item); href != "" {
			detailURL = resolveURL(pageURL, href)
		}
		break
	}
	return detailURL
}

func findJavDBMovieList(root *html.Node) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "div" &&
			hasClass(n, "movie-list") &&
			hasClass(n, "h") &&
			hasClass(n, "cols-4") &&
			hasClass(n, "vcols-8") {
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

func findJavDBSearchItemCode(item *html.Node) string {
	var code string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if code != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "video-title") {
			if strong := firstChildElementByTag(n, "strong"); strong != nil {
				code = strings.TrimSpace(flattenText(strong))
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

func firstAnchorHref(root *html.Node) string {
	if root == nil {
		return ""
	}
	if root.Type == html.ElementNode && root.Data == "a" {
		return strings.TrimSpace(attrValue(root, "href"))
	}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if href := firstAnchorHref(c); href != "" {
			return href
		}
	}
	return ""
}

type javDBMovieFields struct {
	Title       string
	Code        string
	ReleaseDate string
	Runtime     string
	Director    string
	Maker       string
	Publisher   string
	Series      string
	Rating      string
	Tags        []string
	Actors      []string
}

func parseJavDBMovieInfo(root *html.Node) *JavInfo {
	fields := extractJavDBMovieFields(root)
	title := strings.TrimSpace(fields.Title)
	if title == "" {
		title = cleanJavDBMoviePageTitle(strings.TrimSpace(firstTextByTag(root, "title")))
	}

	info := &JavInfo{
		Title:       title,
		Code:        strings.TrimSpace(fields.Code),
		ReleaseUnix: parseDateUnix(fields.ReleaseDate),
		DurationMin: parseRuntimeMinutes(fields.Runtime),
		Tags:        dedupeNonEmpty(fields.Tags),
		Actors:      dedupeNonEmpty(fields.Actors),
		Provider:    ProviderJavDB,
	}
	if info.Title == "" && info.Code == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func extractJavDBMovieFields(root *html.Node) javDBMovieFields {
	var out javDBMovieFields
	if root == nil {
		return out
	}

	if title := findJavDBCurrentTitle(root); title != "" {
		out.Title = title
	}

	panel := findJavDBMovieInfoPanel(root)
	if panel == nil {
		return out
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "panel-block") {
			if strong := firstChildElementByTag(n, "strong"); strong != nil {
				label := strings.TrimSpace(flattenText(strong))
				label = strings.TrimSuffix(label, ":")
				label = strings.TrimSuffix(label, "：")
				assignJavDBMovieField(&out, label, n, strong)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(panel)
	return out
}

func findJavDBMovieInfoPanel(root *html.Node) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "nav" && hasClass(n, "panel") && hasClass(n, "movie-panel-info") {
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

func findJavDBCurrentTitle(root *html.Node) string {
	var title string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if title != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "strong" && hasClass(n, "current-title") {
			title = strings.TrimSpace(flattenText(n))
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return title
}

func assignJavDBMovieField(out *javDBMovieFields, label string, block, strong *html.Node) {
	if out == nil {
		return
	}
	label = normalizeJavDBLabel(label)
	if label == "" {
		return
	}

	value := collectJavDBValue(block, strong)
	switch label {
	case "番号", "番號":
		if out.Code == "" {
			out.Code = value
		}
	case "日期":
		if out.ReleaseDate == "" {
			out.ReleaseDate = value
		}
	case "时长", "時長":
		if out.Runtime == "" {
			out.Runtime = value
		}
	case "导演", "導演":
		if out.Director == "" {
			out.Director = value
		}
	case "片商":
		if out.Maker == "" {
			out.Maker = value
		}
	case "发行", "發行":
		if out.Publisher == "" {
			out.Publisher = value
		}
	case "系列":
		if out.Series == "" {
			out.Series = value
		}
	case "评分", "評分":
		if out.Rating == "" {
			out.Rating = value
		}
	case "类别", "類別":
		if len(out.Tags) == 0 {
			out.Tags = collectAnchorTexts(block)
		}
	case "演员", "演員":
		if len(out.Actors) == 0 {
			out.Actors = collectJavDBActorTexts(block)
		}
	}
}

func collectJavDBActorTexts(root *html.Node) []string {
	if root == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var texts []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			if isJavDBMaleActorLink(n) {
				return
			}
			text := strings.TrimSpace(flattenText(n))
			if text != "" {
				if _, ok := seen[text]; !ok {
					seen[text] = struct{}{}
					texts = append(texts, text)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return texts
}

func isJavDBMaleActorLink(anchor *html.Node) bool {
	for cur := anchor.NextSibling; cur != nil; cur = cur.NextSibling {
		text := strings.TrimSpace(flattenText(cur))
		if cur.Type == html.TextNode {
			if text == "" {
				continue
			}
			return false
		}
		if cur.Type != html.ElementNode {
			continue
		}
		if cur.Data == "strong" && hasClass(cur, "symbol") && hasClass(cur, "male") {
			return true
		}
		if cur.Data == "strong" && text == "♂" {
			return true
		}
		return false
	}
	return false
}

func normalizeJavDBLabel(label string) string {
	label = strings.TrimSpace(label)
	label = strings.TrimSuffix(label, ":")
	label = strings.TrimSuffix(label, "：")
	return strings.Join(strings.Fields(label), "")
}

func collectJavDBValue(block, strong *html.Node) string {
	if value := firstDescendantTextByClass(block, "span", "value"); value != "" {
		return cleanJavDBValue(value)
	}
	if strong != nil {
		return cleanJavDBValue(collectValueAfterBold(strong))
	}
	return cleanJavDBValue(flattenText(block))
}

func firstDescendantTextByClass(root *html.Node, tag, class string) string {
	if root == nil {
		return ""
	}
	var text string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if text != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == tag && hasClass(n, class) {
			text = strings.TrimSpace(flattenText(n))
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return text
}

func cleanJavDBValue(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = strings.Join(strings.Fields(value), " ")
	value = strings.TrimSpace(value)
	value = strings.TrimLeft(value, ":： ")
	return value
}

func parseJavDBCoverURL(root *html.Node, pageURL string) string {
	if root == nil {
		return ""
	}

	var cover string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if cover != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "meta" {
			prop := strings.ToLower(strings.TrimSpace(attrValue(n, "property")))
			content := strings.TrimSpace(attrValue(n, "content"))
			if prop == "og:image" && content != "" {
				cover = resolveURL(pageURL, content)
				return
			}
		}
		if n.Type == html.ElementNode && n.Data == "img" && hasClass(n, "video-cover") {
			if src := strings.TrimSpace(attrValue(n, "src")); src != "" {
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

func cleanJavDBMoviePageTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	for _, suffix := range []string{"| JavDB 成人影片數據庫", "| JavDB"} {
		title = strings.TrimSpace(strings.TrimSuffix(title, suffix))
	}
	if idx := strings.Index(title, " "); idx > 0 && util.CodeRe.MatchString(title[:idx]) {
		title = strings.TrimSpace(title[idx+1:])
	}
	return title
}

func normalizeJavDBCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	var b strings.Builder
	for _, r := range code {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
