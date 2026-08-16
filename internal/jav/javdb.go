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

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"

	"javboss/internal/common/logging"
	"javboss/internal/util"
)

// javDB implements lookupProvider.
type javDB struct{}

var javDBProvider lookupProvider = javDB{}

const (
	javDBBaseURL         = "https://javdb.com"
	javDBUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	javDBRequestInterval = 500 * time.Millisecond
)

var (
	javDBHTTPClientOnce sync.Once
	javDBHTTPClient     *http.Client
	javDBRateLimiter    = struct {
		sync.Mutex
		next time.Time
	}{}
)

// LookupActressByName implements lookupProvider.
func (javDB) LookupActressByName(name string) (*ActressInfo, error) {
	return nil, errors.New("javdb: lookup actress not supported")
}

// LookupActressByCode implements lookupProvider.
func (javDB) LookupActressByCode(code string) (*ActressInfo, error) {
	return nil, errors.New("javdb: lookup actress not supported")
}

// LookupActressURLByCodeAndName resolves an actress profile URL from a movie detail page.
func (javDB) LookupActressURLByCodeAndName(code, name string) (string, error) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if code == "" || name == "" {
		return "", ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	doc, detailURL, err := fetchJavDBDetailByCode(ctx, code)
	if err != nil && !errors.Is(err, ResourceNotFonud) {
		return "", err
	}
	if err == nil {
		if actressURL := parseJavDBActressURLByName(doc, name, detailURL); actressURL != "" {
			return actressURL, nil
		}
	}
	return lookupJavDBActressURLByName(ctx, name)
}

// LookupSeriesURLByCode resolves a series detail URL from a movie detail page.
func (javDB) LookupSeriesURLByCode(code string) (string, error) {
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
	seriesURL := parseJavDBSeriesURL(doc, detailURL)
	if seriesURL == "" {
		return "", ResourceNotFonud
	}
	return seriesURL, nil
}

// LookupStudioURLByCode resolves a studio detail URL from a movie detail page.
func (javDB) LookupStudioURLByCode(code string) (string, error) {
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
	studioURL := parseJavDBStudioURL(doc, detailURL)
	if studioURL == "" {
		return "", ResourceNotFonud
	}
	return studioURL, nil
}

// LookupJavByCode fetches metadata for a given code.
func (javDB) LookupJavByCode(code string) (*JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	doc, detailURL, err := fetchJavDBDetailByCode(ctx, code)
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
	info.CoverURL = parseJavDBCoverURL(doc, detailURL)
	info.SampleImages = parseSampleImages(doc, detailURL)
	return info, nil
}

// LookupJavDBURLByCode resolves a movie code to a JavDB detail URL when the
// search results contain exactly one precise code match. Ambiguous or missing
// precise matches return the search URL so the user can choose manually.
func LookupJavDBURLByCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", ResourceNotFonud
	}

	searchURL := javDBSearchURL(code)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	searchDoc, status, err := fetchJavDBHTML(ctx, searchURL, javDBBaseURL)
	if err != nil {
		return "", err
	}
	if status == http.StatusNotFound || searchDoc == nil {
		return searchURL, nil
	}

	detailURL := findSingleJavDBSearchResultURL(searchDoc, code, searchURL)
	if detailURL == "" {
		return searchURL, nil
	}
	return detailURL, nil
}

func fetchJavDBDetailByCode(ctx context.Context, code string) (*html.Node, string, error) {
	searchURL := javDBSearchURL(code)
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

func javDBSearchURL(code string) string {
	return fmt.Sprintf("%s/search?q=%s&f=all", javDBBaseURL, url.QueryEscape(strings.TrimSpace(code)))
}

func javDBActorSearchURL(name string) string {
	return fmt.Sprintf("%s/search?q=%s&f=actor", javDBBaseURL, url.QueryEscape(strings.TrimSpace(name)))
}

func lookupJavDBActressURLByName(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ResourceNotFonud
	}

	searchURL := javDBActorSearchURL(name)
	doc, status, err := fetchJavDBHTML(ctx, searchURL, javDBBaseURL)
	if err != nil {
		return "", err
	}
	if status == http.StatusNotFound || doc == nil {
		return "", ResourceNotFonud
	}

	urls := findJavDBActorSearchResultURLs(doc, name, searchURL)
	switch len(urls) {
	case 0:
		return "", ResourceNotFonud
	case 1:
		return urls[0], nil
	default:
		return searchURL, nil
	}
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

	doc, err := parseHTMLDocument(body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("javdb: parse html: %w", err)
	}
	return doc, resp.StatusCode, nil
}

func doJavDBRequest(req *http.Request) (*http.Response, error) {
	if err := waitForJavDBRateLimit(req.Context()); err != nil {
		return nil, err
	}
	return defaultJavDBHTTPClient().Do(req)
}

func waitForJavDBRateLimit(ctx context.Context) error {
	for {
		javDBRateLimiter.Lock()
		now := time.Now()
		if !now.Before(javDBRateLimiter.next) {
			javDBRateLimiter.next = now.Add(javDBRequestInterval)
			javDBRateLimiter.Unlock()
			return nil
		}
		wait := time.Until(javDBRateLimiter.next)
		javDBRateLimiter.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return fmt.Errorf("javdb: rate limit wait: %w", ctx.Err())
		case <-timer.C:
		}
	}
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
	urls := findJavDBSearchResultURLs(root, code, pageURL)
	if len(urls) == 0 {
		return ""
	}
	return urls[0]
}

func findSingleJavDBSearchResultURL(root *html.Node, code, pageURL string) string {
	urls := findJavDBSearchResultURLs(root, code, pageURL)
	if len(urls) != 1 {
		return ""
	}
	return urls[0]
}

func findJavDBSearchResultURLs(root *html.Node, code, pageURL string) []string {
	wantCode := normalizeJavDBCode(code)
	seen := make(map[string]struct{})
	var urls []string
	documentSelection(root).Find("div.movie-list.h.cols-4.vcols-8 > div.item").Each(func(_ int, item *goquery.Selection) {
		itemCode := cleanSelectionText(item.Find("div.video-title strong").First())
		if normalizeJavDBCode(itemCode) != wantCode {
			return
		}
		if href := selectionAttr(item.Find("a").First(), "href"); href != "" {
			detailURL := resolveURL(pageURL, href)
			if detailURL == "" {
				return
			}
			if _, ok := seen[detailURL]; ok {
				return
			}
			seen[detailURL] = struct{}{}
			urls = append(urls, detailURL)
		}
	})
	return urls
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
		Title:        title,
		Code:         strings.TrimSpace(fields.Code),
		Studio:       strings.TrimSpace(fields.Maker),
		Series:       strings.TrimSpace(fields.Series),
		ReleaseUnix:  parseDateUnix(fields.ReleaseDate),
		DurationMin:  parseRuntimeMinutes(fields.Runtime),
		Tags:         dedupeNonEmpty(fields.Tags),
		Actors:       dedupeNonEmpty(fields.Actors),
		CoverURL:     parseJavDBCoverURL(root, ""),
		SampleImages: parseSampleImages(root, ""),
		Provider:     ProviderJavDB,
	}
	if info.Title == "" && info.Code == "" && info.Studio == "" && info.Series == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
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

	documentSelection(panel).Find("div.panel-block").Each(func(_ int, block *goquery.Selection) {
		strong := block.ChildrenFiltered("strong").First()
		if strong.Length() > 0 {
			label := normalizeJavDBLabel(cleanSelectionText(strong))
			assignJavDBMovieField(&out, label, firstSelectionNode(block), firstSelectionNode(strong))
		}
	})
	return out
}

func findJavDBMovieInfoPanel(root *html.Node) *html.Node {
	return firstSelectionNode(documentSelection(root).Find("nav.panel.movie-panel-info").First())
}

func findJavDBCurrentTitle(root *html.Node) string {
	return cleanSelectionText(documentSelection(root).Find("strong.current-title").First())
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
	documentSelection(root).Find("a").Each(func(_ int, link *goquery.Selection) {
		if isJavDBMaleActorLink(firstSelectionNode(link)) {
			return
		}
		text := cleanSelectionText(link)
		if text != "" {
			if _, exists := seen[text]; !exists {
				seen[text] = struct{}{}
				texts = append(texts, text)
			}
		}
	})
	return texts
}

func parseJavDBActressURLByName(root *html.Node, name, pageURL string) string {
	name = strings.TrimSpace(name)
	if root == nil {
		return ""
	}

	actors := collectJavDBActressLinks(root, pageURL)
	if len(actors) == 1 {
		return actors[0].href
	}
	if name == "" {
		return ""
	}
	for _, actor := range actors {
		if actor.text == name {
			return actor.href
		}
	}
	return ""
}

type javDBActressLink struct {
	text string
	href string
}

func collectJavDBActressLinks(root *html.Node, pageURL string) []javDBActressLink {
	if root == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var links []javDBActressLink
	documentSelection(root).Find("a").Each(func(_ int, link *goquery.Selection) {
		node := firstSelectionNode(link)
		if isJavDBMaleActorLink(node) {
			return
		}
		text := cleanSelectionText(link)
		href := selectionAttr(link, "href")
		if text != "" && isJavDBActorURL(href) {
			actorURL := resolveURL(pageURL, href)
			key := text + "\x00" + actorURL
			if actorURL != "" {
				if _, exists := seen[key]; !exists {
					seen[key] = struct{}{}
					links = append(links, javDBActressLink{text: text, href: actorURL})
				}
			}
		}
	})
	return links
}

func findJavDBActorSearchResultURLs(root *html.Node, name, pageURL string) []string {
	name = strings.TrimSpace(name)
	if root == nil || name == "" {
		return nil
	}

	seen := make(map[string]struct{})
	var urls []string
	documentSelection(root).Find("#actors .actor-box, .actors .actor-box").Each(func(_ int, box *goquery.Selection) {
		if href := javDBActorBoxExactHref(firstSelectionNode(box), name); href != "" {
			actorURL := resolveURL(pageURL, href)
			if actorURL != "" {
				if _, exists := seen[actorURL]; !exists {
					seen[actorURL] = struct{}{}
					urls = append(urls, actorURL)
				}
			}
		}
	})
	return urls
}

func javDBActorBoxExactHref(root *html.Node, name string) string {
	if root == nil {
		return ""
	}
	name = strings.TrimSpace(name)
	var href string
	documentSelection(root).Find("a").EachWithBreak(func(_ int, link *goquery.Selection) bool {
		candidateHref := selectionAttr(link, "href")
		if candidateHref != "" && isJavDBActorURL(candidateHref) && javDBActorAnchorMatchesName(firstSelectionNode(link), name) {
			href = candidateHref
			return false
		}
		return true
	})
	return href
}

func javDBActorAnchorMatchesName(anchor *html.Node, name string) bool {
	if anchor == nil || name == "" {
		return false
	}
	selection := goquery.NewDocumentFromNode(anchor).Selection
	if javDBActorTitleHasName(selectionAttr(selection, "title"), name) {
		return true
	}
	return strings.TrimSpace(firstDescendantTextByTag(anchor, "strong")) == name
}

func javDBActorTitleHasName(title, name string) bool {
	name = strings.TrimSpace(name)
	title = strings.TrimSpace(title)
	if title == "" || name == "" {
		return false
	}
	for _, alias := range strings.FieldsFunc(title, func(r rune) bool {
		return r == ',' || r == '，' || r == '、'
	}) {
		if strings.TrimSpace(alias) == name {
			return true
		}
	}
	return false
}

func parseJavDBSeriesURL(root *html.Node, pageURL string) string {
	return parseJavDBPanelURL(root, pageURL, "系列", isJavDBSeriesURL)
}

func parseJavDBStudioURL(root *html.Node, pageURL string) string {
	return parseJavDBPanelURL(root, pageURL, "片商", isJavDBStudioURL)
}

func parseJavDBPanelURL(root *html.Node, pageURL, wantLabel string, matchHref func(string) bool) string {
	if root == nil {
		return ""
	}

	var matchedURL string
	documentSelection(root).Find("div.panel-block").EachWithBreak(func(_ int, block *goquery.Selection) bool {
		if normalizeJavDBLabel(cleanSelectionText(block.ChildrenFiltered("strong").First())) != wantLabel {
			return true
		}
		block.Find("a").EachWithBreak(func(_ int, link *goquery.Selection) bool {
			href := selectionAttr(link, "href")
			if matchHref(href) {
				matchedURL = resolveURL(pageURL, href)
				return false
			}
			return true
		})
		return matchedURL == ""
	})
	return matchedURL
}

func isJavDBActorURL(href string) bool {
	href = strings.ToLower(strings.TrimSpace(href))
	return strings.Contains(href, "/actors/")
}

func isJavDBSeriesURL(href string) bool {
	href = strings.ToLower(strings.TrimSpace(href))
	return strings.Contains(href, "/series/")
}

func isJavDBStudioURL(href string) bool {
	href = strings.ToLower(strings.TrimSpace(href))
	return strings.Contains(href, "/makers/")
}

func isJavDBMaleActorLink(anchor *html.Node) bool {
	if anchor == nil {
		return false
	}
	next := goquery.NewDocumentFromNode(anchor).Selection.NextAll().First()
	return next.Is("strong.symbol.male") || (goquery.NodeName(next) == "strong" && cleanSelectionText(next) == "♂")
}

func normalizeJavDBLabel(label string) string {
	label = strings.TrimSpace(label)
	label = strings.TrimSuffix(label, ":")
	label = strings.TrimSuffix(label, "：")
	return strings.Join(strings.Fields(label), "")
}

func collectJavDBValue(block, strong *html.Node) string {
	blockSelection := goquery.NewDocumentFromNode(block).Selection
	if value := cleanSelectionText(blockSelection.Find("span.value").First()); value != "" {
		return cleanJavDBValue(value)
	}
	if strong != nil {
		strongText := cleanSelectionText(goquery.NewDocumentFromNode(strong).Selection)
		return cleanJavDBValue(strings.TrimPrefix(cleanSelectionText(blockSelection), strongText))
	}
	return cleanJavDBValue(cleanSelectionText(blockSelection))
}

func firstDescendantTextByTag(root *html.Node, tag string) string {
	if root == nil || tag == "" {
		return ""
	}
	return cleanSelectionText(documentSelection(root).Find(tag).First())
}

func cleanJavDBValue(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = strings.Join(strings.Fields(value), " ")
	value = strings.TrimSpace(value)
	value = strings.TrimLeft(value, ":： ")
	return value
}

func parseJavDBCoverURL(root *html.Node, pageURL string) string {
	doc := documentSelection(root)
	for _, candidate := range []string{
		selectionAttr(doc.Find(`meta[property="og:image"]`).First(), "content"),
		selectionAttr(doc.Find("img.video-cover").First(), "src"),
	} {
		if cover := resolveURL(pageURL, candidate); cover != "" {
			return cover
		}
	}
	return ""
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
