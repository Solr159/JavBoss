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
)

var javBusProviderKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

var errJavBusVerificationRequired = errors.New("javbus: age verification required")

// JavBusActressSubscription is the validated identity used to refresh an idol
// subscription without relying on a possibly ambiguous display name search.
type JavBusActressSubscription struct {
	Name        string
	ProviderKey string
}

// JavBusDiscoveryItem is the metadata available from a JavBus actress listing.
type JavBusDiscoveryItem struct {
	Code             string        `json:"code"`
	Title            string        `json:"title"`
	ReleaseUnix      int64         `json:"release_unix"`
	DurationMin      int           `json:"duration_min"`
	CoverURL         string        `json:"cover_url"`
	ThumbnailURL     string        `json:"thumbnail_url"`
	DetailURL        string        `json:"detail_url"`
	Actresses        []string      `json:"actresses"`
	Studio           string        `json:"studio"`
	Series           string        `json:"series"`
	Tags             []string      `json:"tags"`
	SampleImages     []SampleImage `json:"sample_images"`
	IsUncensored     *bool         `json:"is_uncensored,omitempty"`
	Source           string        `json:"source"`
	DetailsFetchedAt *time.Time    `json:"details_fetched_at,omitempty"`
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
	}, nil
}

// ValidateJavBusActressSubscription verifies that referenceCode is a solo work
// for name and returns the stable JavBus star key found on that detail page.
func ValidateJavBusActressSubscription(ctx context.Context, name, referenceCode string) (*JavBusActressSubscription, error) {
	name = normalizeJavBusActressName(name)
	referenceCode = strings.TrimSpace(referenceCode)
	if name == "" || referenceCode == "" {
		return nil, ResourceNotFonud
	}

	lookupCode, _ := javBusLookupCode(referenceCode)
	doc, _, err := fetchJavBusDocument(ctx, lookupCode)
	if err != nil {
		return nil, err
	}
	if isJavBusVerificationPage(doc) {
		return nil, errJavBusVerificationRequired
	}

	actors := parseJavBusActressLinks(doc)
	if len(actors) != 1 || normalizeJavBusActressName(actors[0].Name) != name {
		return nil, ResourceNotFonud
	}
	key := javBusStarKey(actors[0].URL)
	if !javBusProviderKeyPattern.MatchString(key) {
		return nil, ResourceNotFonud
	}
	return &JavBusActressSubscription{Name: actors[0].Name, ProviderKey: key}, nil
}

// FetchJavBusActressWorks fetches all available listing pages for a validated
// JavBus star key. It uses only listing metadata and does not add anything to
// the application's main JAV catalog.
func FetchJavBusActressWorks(ctx context.Context, providerKey, actressName string) ([]JavBusDiscoveryItem, error) {
	providerKey = strings.TrimSpace(providerKey)
	if !javBusProviderKeyPattern.MatchString(providerKey) {
		return nil, ResourceNotFonud
	}
	actressName = normalizeJavBusActressName(actressName)

	nextURL := fmt.Sprintf("%s/star/%s", javBusBaseURL, providerKey)
	visited := make(map[string]struct{})
	itemsByCode := make(map[string]JavBusDiscoveryItem)
	order := make([]string, 0)

	for page := 0; nextURL != "" && page < javBusDiscoveryMaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, ok := visited[nextURL]; ok {
			break
		}
		visited[nextURL] = struct{}{}

		doc, err := fetchJavBusPage(ctx, nextURL)
		if err != nil {
			return nil, err
		}
		pageItems := parseJavBusDiscoveryItems(doc, nextURL, actressName)
		for _, item := range pageItems {
			key := normalizeDiscoveryCode(item.Code)
			if key == "" {
				continue
			}
			if _, exists := itemsByCode[key]; !exists {
				order = append(order, key)
			}
			itemsByCode[key] = item
		}
		nextURL = parseJavBusNextActressPageURL(doc, nextURL, providerKey)
	}

	items := make([]JavBusDiscoveryItem, 0, len(order))
	for _, code := range order {
		items = append(items, itemsByCode[code])
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
		key := javBusStarKey(href)
		if name == "" || !javBusProviderKeyPattern.MatchString(key) {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		result = append(result, javBusActressLink{Name: name, URL: href})
	})
	return result
}

func javBusStarKey(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for index := 0; index+1 < len(parts); index++ {
		if strings.EqualFold(parts[index], "star") {
			return strings.TrimSpace(parts[index+1])
		}
	}
	return ""
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

func parseJavBusNextActressPageURL(root *html.Node, pageURL, providerKey string) string {
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
	if javBusStarKey(next) != providerKey {
		return ""
	}
	return parsed.String()
}

func fetchJavBusPage(ctx context.Context, targetURL string) (*html.Node, error) {
	requestCtx, cancel := context.WithTimeout(ctx, javBusDiscoveryRequestTimout)
	defer cancel()

	req, err := buildRequest(requestCtx, targetURL)
	if err != nil {
		return nil, err
	}
	logging.Info("javbus discovery request: %s", targetURL)
	resp, err := doJavBusRequest(req)
	if err != nil {
		if errors.Is(err, util.ErrCachedNotFound) {
			return nil, ResourceNotFonud
		}
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ResourceNotFonud
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("javbus discovery: http %d", resp.StatusCode)
	}
	doc, err := parseHTMLDocument(body)
	if err != nil {
		return nil, fmt.Errorf("javbus discovery: parse html: %w", err)
	}
	if isJavBusVerificationPage(doc) {
		return nil, errJavBusVerificationRequired
	}
	return doc, nil
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
