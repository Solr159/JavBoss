package jav

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/util"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

const (
	javBusBaseURL                = "https://www.javbus.com"
	javBusDiscoveryMaxPages      = 100
	javBusDiscoveryRequestTimout = 30 * time.Second
	javBusMagnetResponseMaxBytes = 4 * 1024 * 1024
)

var javBusStarIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

var (
	javBusMagnetGIDPattern = regexp.MustCompile(`(?m)\bvar\s+gid\s*=\s*(\d+)\s*;`)
	javBusMagnetUCPattern  = regexp.MustCompile(`(?m)\bvar\s+uc\s*=\s*(\d+)\s*;`)
	javBusMagnetImgPattern = regexp.MustCompile(`(?m)\bvar\s+img\s*=\s*['"]([^'"]+)['"]\s*;`)
)

var errJavBusVerificationRequired = errors.New("javbus: age verification required")

// JavBusActressSubscription is the validated identity used to refresh an idol
// subscription without relying on a possibly ambiguous display name search.
type JavBusActressSubscription struct {
	Name            string
	ProviderLocator string
}

type JavBusActressWorksOptions struct {
	Offset            int
	Limit             int
	ReleasedNotBefore int64
	ReleasedBefore    int64
}

type JavBusMagnetLink struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Size      string `json:"size"`
	ShareDate string `json:"share_date"`
	HD        bool   `json:"hd"`
	Subtitled bool   `json:"subtitled"`
}

// JavBusDiscoveryItem is the metadata available from a JavBus actress listing.
type JavBusDiscoveryItem struct {
	Code             string             `json:"code"`
	Title            string             `json:"title"`
	ReleaseUnix      int64              `json:"release_unix"`
	DurationMin      int                `json:"duration_min"`
	CoverURL         string             `json:"cover_url"`
	ThumbnailURL     string             `json:"thumbnail_url"`
	DetailURL        string             `json:"detail_url"`
	Actresses        []string           `json:"actresses"`
	Studio           string             `json:"studio"`
	Series           string             `json:"series"`
	Tags             []string           `json:"tags"`
	SampleImages     []SampleImage      `json:"sample_images"`
	IsUncensored     *bool              `json:"is_uncensored,omitempty"`
	Source           string             `json:"source"`
	DetailsFetchedAt *time.Time         `json:"details_fetched_at,omitempty"`
	MagnetLinks      []JavBusMagnetLink `json:"-"`
}

// FetchJavBusDiscoveryItemDetails resolves full metadata from a JavBus movie
// detail page without adding the work to the main JAV catalog.
func FetchJavBusDiscoveryItemDetails(ctx context.Context, code string) (*JavBusDiscoveryItem, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ResourceNotFonud
	}
	lookupCode, rewrite := javBusLookupCode(code)
	doc, detailURL, err := fetchJavBusDocument(ctx, lookupCode)
	if err != nil {
		return nil, err
	}
	if isJavBusVerificationPage(doc) {
		return nil, errJavBusVerificationRequired
	}
	info := parseDocument(doc)
	if info == nil || strings.TrimSpace(info.Code) == "" || strings.TrimSpace(info.Title) == "" {
		return nil, ResourceNotFonud
	}
	info.CoverURL = parseJavBusCoverURL(doc, detailURL)
	info.SampleImages = parseSampleImages(doc, detailURL)
	magnetLinks, err := fetchJavBusMagnetLinks(ctx, doc, detailURL)
	if err != nil {
		return nil, fmt.Errorf("fetch javbus magnet links: %w", err)
	}
	if rewrite != nil {
		normalizeJavBusRewrittenInfo(info, rewrite)
	}
	fetchedAt := time.Now().UTC()
	return &JavBusDiscoveryItem{
		Code:             strings.TrimSpace(info.Code),
		Title:            strings.TrimSpace(info.Title),
		ReleaseUnix:      info.ReleaseUnix,
		DurationMin:      info.DurationMin,
		CoverURL:         strings.TrimSpace(info.CoverURL),
		DetailURL:        detailURL,
		Actresses:        append([]string(nil), info.Actors...),
		Studio:           strings.TrimSpace(info.Studio),
		Series:           strings.TrimSpace(info.Series),
		Tags:             append([]string(nil), info.Tags...),
		SampleImages:     append([]SampleImage(nil), info.SampleImages...),
		IsUncensored:     info.IsUncensored,
		Source:           "javbus",
		DetailsFetchedAt: &fetchedAt,
		MagnetLinks:      magnetLinks,
	}, nil
}

type javBusMagnetRequestParameters struct {
	GID string
	UC  string
	Img string
}

func fetchJavBusMagnetLinks(ctx context.Context, doc *html.Node, detailURL string) ([]JavBusMagnetLink, error) {
	parameters, ok := parseJavBusMagnetRequestParameters(doc)
	if !ok {
		return []JavBusMagnetLink{}, nil
	}
	target, err := url.Parse(javBusBaseURL + "/ajax/uncledatoolsbyajax.php")
	if err != nil {
		return nil, fmt.Errorf("build magnet URL: %w", err)
	}
	query := target.Query()
	query.Set("gid", parameters.GID)
	query.Set("lang", "zh")
	query.Set("img", parameters.Img)
	query.Set("uc", parameters.UC)
	query.Set("floor", fmt.Sprintf("%d", time.Now().UnixNano()%1000+1))
	target.RawQuery = query.Encode()

	requestCtx, cancel := context.WithTimeout(ctx, javBusDiscoveryRequestTimout)
	defer cancel()
	req, err := buildRequest(requestCtx, target.String())
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html, */*; q=0.01")
	req.Header.Set("Referer", detailURL)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	logging.Info("javbus magnet request: %s", target.Redacted())
	resp, err := doJavBusRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("javbus magnet request returned http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, javBusMagnetResponseMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read javbus magnet response: %w", err)
	}
	if len(body) > javBusMagnetResponseMaxBytes {
		return nil, errors.New("javbus magnet response is too large")
	}
	wrappedBody := []byte("<html><body><table><tbody>" + string(body) + "</tbody></table></body></html>")
	doc, err = parseHTMLDocument(wrappedBody)
	if err != nil {
		return nil, fmt.Errorf("parse javbus magnet response: %w", err)
	}
	return parseJavBusMagnetLinks(doc), nil
}

func parseJavBusMagnetRequestParameters(doc *html.Node) (javBusMagnetRequestParameters, bool) {
	var scripts strings.Builder
	documentSelection(doc).Find("script").Each(func(_ int, script *goquery.Selection) {
		scripts.WriteString(script.Text())
		scripts.WriteByte('\n')
	})
	text := scripts.String()
	parameter := javBusMagnetRequestParameters{
		GID: firstJavBusPatternMatch(javBusMagnetGIDPattern, text),
		UC:  firstJavBusPatternMatch(javBusMagnetUCPattern, text),
		Img: firstJavBusPatternMatch(javBusMagnetImgPattern, text),
	}
	return parameter, parameter.GID != "" && parameter.UC != "" && parameter.Img != ""
}

func firstJavBusPatternMatch(pattern *regexp.Regexp, value string) string {
	match := pattern.FindStringSubmatch(value)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func parseJavBusMagnetLinks(doc *html.Node) []JavBusMagnetLink {
	seen := make(map[string]struct{})
	links := make([]JavBusMagnetLink, 0)
	documentSelection(doc).Find("tr").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 3 {
			return
		}
		anchor := cells.Eq(0).Find(`a[href^="magnet:"]`).First()
		magnetURL := strings.TrimSpace(selectionAttr(anchor, "href"))
		parsed, err := url.Parse(magnetURL)
		if err != nil || !strings.EqualFold(parsed.Scheme, "magnet") ||
			!strings.HasPrefix(strings.ToLower(parsed.Query().Get("xt")), "urn:btih:") {
			return
		}
		if _, exists := seen[magnetURL]; exists {
			return
		}
		seen[magnetURL] = struct{}{}
		name := strings.TrimSpace(parsed.Query().Get("dn"))
		if name == "" {
			name = cleanSelectionText(anchor)
		}
		links = append(links, JavBusMagnetLink{
			Name:      name,
			URL:       magnetURL,
			Size:      cleanSelectionText(cells.Eq(1)),
			ShareDate: cleanSelectionText(cells.Eq(2)),
			HD:        row.Find(`[title*="高清"]`).Length() > 0,
			Subtitled: row.Find(`[title*="字幕"]`).Length() > 0,
		})
	})
	return links
}

// ResolveJavBusActressSubscription verifies that referenceCode resolves to an
// exact solo work and returns the actress identity from that detail page.
func ResolveJavBusActressSubscription(ctx context.Context, referenceCode string) (*JavBusActressSubscription, error) {
	referenceCode = strings.TrimSpace(referenceCode)
	if referenceCode == "" {
		return nil, ResourceNotFonud
	}

	lookupCode, rewrite := javBusLookupCode(referenceCode)
	doc, _, err := fetchJavBusDocument(ctx, lookupCode)
	if err != nil {
		return nil, err
	}
	if isJavBusVerificationPage(doc) {
		return nil, errJavBusVerificationRequired
	}

	info := parseDocument(doc)
	if info == nil || strings.TrimSpace(info.Code) == "" {
		return nil, ResourceNotFonud
	}
	if rewrite != nil {
		normalizeJavBusRewrittenInfo(info, rewrite)
	}
	if normalizeDiscoveryCode(info.Code) != normalizeDiscoveryCode(referenceCode) {
		return nil, ResourceNotFonud
	}

	actors := parseJavBusActressLinks(doc)
	if len(actors) != 1 {
		return nil, ResourceNotFonud
	}
	locator := javBusActressLocator(actors[0].URL)
	if locator == "" {
		return nil, ResourceNotFonud
	}
	return &JavBusActressSubscription{Name: actors[0].Name, ProviderLocator: locator}, nil
}

// FetchJavBusActressWorks fetches a bounded window from a validated JavBus star
// listing. It uses only listing metadata and does not add anything to the
// application's main JAV catalog.
func FetchJavBusActressWorks(ctx context.Context, providerLocator, actressName string, options JavBusActressWorksOptions) ([]JavBusDiscoveryItem, error) {
	providerLocator = javBusActressLocator(providerLocator)
	if providerLocator == "" {
		return nil, ResourceNotFonud
	}
	actressName = normalizeJavBusActressName(actressName)
	if options.Offset < 0 {
		options.Offset = 0
	}
	if options.Limit <= 0 || options.Limit > 100 {
		options.Limit = 10
	}

	nextURL := resolveURL(javBusBaseURL, providerLocator)
	visited := make(map[string]struct{})
	seenCodes := make(map[string]struct{})
	items := make([]JavBusDiscoveryItem, 0, options.Limit)
	itemIndex := 0

	for page := 0; nextURL != "" && page < javBusDiscoveryMaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, ok := visited[nextURL]; ok {
			break
		}
		visited[nextURL] = struct{}{}

		doc, effectiveURL, err := fetchJavBusPage(ctx, nextURL, providerLocator)
		if err != nil {
			return nil, err
		}
		pageItems := parseJavBusDiscoveryItems(doc, effectiveURL, actressName)
		foundItemBeforeLowerBound := false
		for _, item := range pageItems {
			key := normalizeDiscoveryCode(item.Code)
			if key == "" {
				continue
			}
			if _, exists := seenCodes[key]; exists {
				continue
			}
			seenCodes[key] = struct{}{}
			if options.ReleasedNotBefore > 0 {
				if item.ReleaseUnix <= 0 {
					continue
				}
				if item.ReleaseUnix < options.ReleasedNotBefore {
					foundItemBeforeLowerBound = true
					continue
				}
			}
			if options.ReleasedBefore > 0 &&
				(item.ReleaseUnix <= 0 || item.ReleaseUnix >= options.ReleasedBefore) {
				continue
			}
			if itemIndex >= options.Offset {
				items = append(items, item)
				if len(items) == options.Limit {
					return items, nil
				}
			}
			itemIndex++
		}
		// JavBus actress listings are newest-first. Once a page crosses the
		// lower release-date boundary, later pages cannot contain new works.
		if foundItemBeforeLowerBound {
			return items, nil
		}
		nextURL = parseJavBusNextActressPageURL(doc, effectiveURL, providerLocator)
	}
	return items, nil
}

type javBusActressLink struct {
	Name string
	URL  string
}

func parseJavBusActressLinks(root *html.Node) []javBusActressLink {
	section := findMovieSection(root)
	if section == nil {
		section = root
	}
	seen := make(map[string]struct{})
	var result []javBusActressLink
	documentSelection(section).Find(`a[href*="/star/"]`).Each(func(_ int, link *goquery.Selection) {
		name := normalizeJavBusActressName(cleanSelectionText(link))
		href := resolveURL(javBusBaseURL, selectionAttr(link, "href"))
		locator := javBusActressLocator(href)
		if name == "" || locator == "" {
			return
		}
		if _, ok := seen[locator]; ok {
			return
		}
		seen[locator] = struct{}{}
		result = append(result, javBusActressLink{Name: name, URL: href})
	})
	return result
}

// javBusActressLocator returns the canonical, host-independent path for an
// actress listing root. Keeping the complete path preserves JavBus namespaces
// such as /uncensored, where the same star key may identify a different page.
func javBusActressLocator(rawURL string) string {
	locator, isRoot := parseJavBusActressPageLocator(rawURL)
	if !isRoot {
		return ""
	}
	return locator
}

func javBusActressPageLocator(rawURL string) string {
	locator, _ := parseJavBusActressPageLocator(rawURL)
	return locator
}

func parseJavBusActressPageLocator(rawURL string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.User != nil || parsed.Fragment != "" {
		return "", false
	}
	if parsed.IsAbs() {
		if !strings.EqualFold(parsed.Scheme, "https") ||
			!strings.EqualFold(parsed.Hostname(), "www.javbus.com") {
			return "", false
		}
	} else if parsed.Host != "" {
		return "", false
	}

	trimmedPath := strings.TrimSuffix(parsed.Path, "/")
	if trimmedPath == "" || !strings.HasPrefix(trimmedPath, "/") {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(trimmedPath, "/"), "/")
	rootLength := 0
	switch {
	case len(parts) >= 2 && parts[0] == "star":
		rootLength = 2
	case len(parts) >= 3 && parts[0] == "uncensored" && parts[1] == "star":
		rootLength = 3
	default:
		return "", false
	}
	key := parts[rootLength-1]
	if !javBusStarIDPattern.MatchString(key) {
		return "", false
	}
	if len(parts) > rootLength+1 {
		return "", false
	}
	isRoot := len(parts) == rootLength && parsed.RawQuery == ""
	if len(parts) == rootLength+1 {
		page, err := strconv.Atoi(parts[rootLength])
		if err != nil || page <= 0 {
			return "", false
		}
	}
	return "/" + strings.Join(parts[:rootLength], "/"), isRoot
}

func parseJavBusDiscoveryItems(root *html.Node, pageURL, actressName string) []JavBusDiscoveryItem {
	var result []JavBusDiscoveryItem
	documentSelection(root).Find("a.movie-box").Each(func(_ int, card *goquery.Selection) {
		dates := card.Find("date")
		code := strings.TrimSpace(cleanSelectionText(dates.First()))
		releaseText := ""
		if dates.Length() > 1 {
			releaseText = strings.TrimSpace(cleanSelectionText(dates.Eq(1)))
		}
		if code == "" {
			code = strings.TrimSpace(card.AttrOr("data-code", ""))
		}
		if normalizeDiscoveryCode(code) == "" {
			return
		}

		image := card.Find("img").First()
		title := strings.TrimSpace(selectionAttr(image, "title"))
		if title == "" {
			title = strings.TrimSpace(selectionAttr(image, "alt"))
		}
		detailURL := resolveURL(pageURL, selectionAttr(card, "href"))
		coverURL := resolveURL(pageURL, selectionAttr(image, "src"))
		if lazyCover := resolveURL(pageURL, selectionAttr(image, "data-original")); lazyCover != "" {
			coverURL = lazyCover
		}

		var releaseUnix int64
		if release, err := time.Parse("2006-01-02", releaseText); err == nil {
			releaseUnix = release.Unix()
		}
		actresses := []string{}
		if actressName != "" {
			actresses = append(actresses, actressName)
		}
		result = append(result, JavBusDiscoveryItem{
			Code:        strings.TrimSpace(code),
			Title:       title,
			ReleaseUnix: releaseUnix,
			CoverURL:    coverURL,
			DetailURL:   detailURL,
			Actresses:   actresses,
			Source:      "javbus",
		})
	})
	return result
}

func parseJavBusNextActressPageURL(root *html.Node, pageURL, providerLocator string) string {
	var href string
	for _, selector := range []string{`a[rel="next"]`, "li.next a", "a#next"} {
		href = selectionAttr(documentSelection(root).Find(selector).First(), "href")
		if href != "" {
			break
		}
	}
	if href == "" {
		return ""
	}
	next := resolveURL(pageURL, href)
	parsed, err := url.Parse(next)
	if err != nil ||
		!strings.EqualFold(parsed.Scheme, "https") ||
		!strings.EqualFold(parsed.Hostname(), "www.javbus.com") {
		return ""
	}
	if javBusActressPageLocator(next) != providerLocator {
		return ""
	}
	return parsed.String()
}

func fetchJavBusPage(ctx context.Context, targetURL, providerLocator string) (*html.Node, string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, javBusDiscoveryRequestTimout)
	defer cancel()

	req, err := buildRequest(requestCtx, targetURL)
	if err != nil {
		return nil, "", err
	}
	logging.Info("javbus discovery request: %s", targetURL)
	resp, err := doJavBusRequest(req)
	if err != nil {
		if errors.Is(err, util.ErrCachedNotFound) {
			return nil, "", ResourceNotFonud
		}
		return nil, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", ResourceNotFonud
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("javbus discovery: http %d", resp.StatusCode)
	}
	doc, err := parseHTMLDocument(body)
	if err != nil {
		return nil, "", fmt.Errorf("javbus discovery: parse html: %w", err)
	}
	if isJavBusVerificationPage(doc) {
		return nil, "", errJavBusVerificationRequired
	}
	effectiveURL := targetURL
	if resp.Request != nil && resp.Request.URL != nil {
		effectiveURL = resp.Request.URL.String()
	}
	if javBusActressPageLocator(effectiveURL) != providerLocator {
		return nil, "", fmt.Errorf("javbus discovery: actress page redirected outside locator %q", providerLocator)
	}
	return doc, effectiveURL, nil
}

func isJavBusVerificationPage(root *html.Node) bool {
	doc := documentSelection(root)
	title := strings.ToLower(cleanSelectionText(doc.Find("title").First()))
	if strings.Contains(title, "age verification") {
		return true
	}
	return doc.Find(`form[action*="driver-verify"]`).Length() > 0
}

func normalizeJavBusActressName(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func normalizeDiscoveryCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, " ", "")
	return value
}
