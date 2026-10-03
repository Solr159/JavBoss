package javmenu

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/jav/internal/htmlutil"
	"javboss/internal/jav/internal/parseutil"
	"javboss/internal/jav/internal/ratelimit"
	"javboss/internal/jav/metadata"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// JavMenuClient retrieves metadata from javmenu.
type JavMenuClient struct {
	httpClient *http.Client
	limiter    *ratelimit.Limiter
}

const (
	javMenuBaseURL         = "https://javmenu.com"
	javMenuUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	javMenuRequestInterval = 1500 * time.Millisecond
)

// LookupJavByCode fetches metadata for a given code.
func (p *JavMenuClient) LookupJavByCode(ctx context.Context, code string) (*metadata.JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, metadata.ErrNotFound
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	doc, detailURL, err := p.fetchJavMenuDetailByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	info := parseJavMenuMovieInfo(doc)
	if info == nil {
		return nil, metadata.ErrNotFound
	}
	if info.Code == "" {
		info.Code = code
	}
	info.SampleImages = parseutil.ParseSampleImages(doc, detailURL)
	return info, nil
}

func (p *JavMenuClient) fetchJavMenuDetailByCode(ctx context.Context, code string) (*html.Node, string, error) {
	targetURL := fmt.Sprintf("%s/%s", javMenuBaseURL, url.PathEscape(strings.ToUpper(strings.TrimSpace(code))))
	doc, status, err := p.fetchJavMenuHTML(ctx, targetURL, javMenuBaseURL)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusNotFound || doc == nil {
		return nil, "", metadata.ErrNotFound
	}
	return doc, targetURL, nil
}

func (p *JavMenuClient) fetchJavMenuHTML(ctx context.Context, targetURL, referer string) (*html.Node, int, error) {
	req, err := buildJavMenuRequest(ctx, targetURL, referer)
	if err != nil {
		return nil, 0, err
	}

	logging.Info("javmenu request: %s", targetURL)
	resp, err := p.doJavMenuRequest(req)
	if err != nil {
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

	doc, err := htmlutil.ParseHTMLDocument(body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("javmenu: parse html: %w", err)
	}
	return doc, resp.StatusCode, nil
}

func (p *JavMenuClient) doJavMenuRequest(req *http.Request) (*http.Response, error) {
	if err := p.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	return p.httpClient.Do(req)
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

func parseJavMenuMovieInfo(root *html.Node) *metadata.JavInfo {
	cardBody := findJavMenuInfoCardBody(root)
	if cardBody == nil {
		return nil
	}

	fields := extractJavMenuMovieFields(cardBody)
	title := cleanJavMenuTitle(strings.TrimSpace(htmlutil.FirstTextByTag(root, "h1")), fields.Code)
	if title == "" {
		title = cleanJavMenuTitle(strings.TrimSpace(htmlutil.FirstTextByTag(root, "title")), fields.Code)
	}

	info := &metadata.JavInfo{
		Title:        title,
		Code:         strings.TrimSpace(fields.Code),
		Studio:       strings.TrimSpace(fields.Studio),
		Series:       strings.TrimSpace(fields.Series),
		ReleaseUnix:  parseutil.ParseDateUnix(fields.ReleaseDate),
		DurationMin:  parseutil.ParseRuntimeMinutes(fields.Runtime),
		Tags:         parseutil.DedupeNonEmpty(fields.Tags),
		Actors:       parseutil.DedupeNonEmpty(fields.Actors),
		SampleImages: parseutil.ParseSampleImages(root, ""),
		Provider:     metadata.ProviderJavMenu,
	}
	if info.Title == "" && info.Code == "" && info.Studio == "" && info.Series == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func findJavMenuInfoCardBody(root *html.Node) *html.Node {
	var result *html.Node
	htmlutil.DocumentSelection(root).Find("div.card.rounded div.card-body").EachWithBreak(func(_ int, body *goquery.Selection) bool {
		if strings.Contains(body.Text(), "影片資料") {
			result = htmlutil.FirstSelectionNode(body)
			return false
		}
		return true
	})
	return result
}

func extractJavMenuMovieFields(cardBody *html.Node) javMenuMovieFields {
	var out javMenuMovieFields
	if cardBody == nil {
		return out
	}

	htmlutil.DocumentSelection(cardBody).ChildrenFiltered("div").Each(func(_ int, row *goquery.Selection) {
		labelSelection := row.ChildrenFiltered("span").First()
		label := normalizeJavMenuLabel(htmlutil.CleanSelectionText(labelSelection))
		if label == "" {
			return
		}
		value := strings.TrimSpace(strings.TrimPrefix(htmlutil.CleanSelectionText(row), htmlutil.CleanSelectionText(labelSelection)))
		anchorText := htmlutil.CleanSelectionText(row.Find("a").First())
		switch label {
		case "番號", "番号", "識別碼", "识别码":
			out.Code = cleanJavMenuCode(value)
		case "發佈於", "发布于", "發行日期", "发行日期", "発売日", "release date":
			out.ReleaseDate = parseutil.FirstNonEmpty(out.ReleaseDate, value)
		case "時長", "时长", "長度", "长度", "duration", "runtime":
			out.Runtime = parseutil.FirstNonEmpty(out.Runtime, value)
		case "出版", "發行", "发行", "片商", "製作商", "制作商", "studio", "maker", "publisher":
			out.Studio = parseutil.FirstNonEmpty(out.Studio, parseutil.FirstNonEmpty(anchorText, value))
		case "系列", "series":
			out.Series = parseutil.FirstNonEmpty(out.Series, parseutil.FirstNonEmpty(anchorText, value))
		case "類別", "类别", "主題", "主题", "genre", "genres", "tags":
			out.Tags = append(out.Tags, htmlutil.SelectionTexts(row.Find("a.genre"))...)
		case "女優", "女优", "演員", "演员", "actress", "actor", "actors":
			out.Actors = append(out.Actors, htmlutil.SelectionTexts(row.Find("a.actress"))...)
		}
	})
	return out
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

// New creates a provider using the supplied non-nil HTTP client.
func New(httpClient *http.Client) *JavMenuClient {
	return &JavMenuClient{httpClient: httpClient, limiter: ratelimit.New(javMenuRequestInterval)}
}

// OriginURL identifies the origin used for availability checks.
func (p *JavMenuClient) OriginURL() string { return javMenuBaseURL }
