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
	"sync"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/util"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// minnanoAV implements lookupProvider for actress profiles from みんなのAV.com.
type minnanoAV struct{}

var minnanoAVProvider lookupProvider = minnanoAV{}

const (
	minnanoAVBaseURL         = "https://www.minnano-av.com"
	minnanoAVRequestInterval = 500 * time.Millisecond
)

var (
	minnanoAVActressPathPattern = regexp.MustCompile(`^/actress\d+\.html$`)
	minnanoAVBirthDatePattern   = regexp.MustCompile(`(\d{4})年\s*(\d{1,2})月\s*(\d{1,2})日`)
	minnanoAVHeightPattern      = regexp.MustCompile(`(?i)T\s*(\d{2,3})`)
	minnanoAVBustPattern        = regexp.MustCompile(`(?i)B\s*(\d{2,3})`)
	minnanoAVWaistPattern       = regexp.MustCompile(`(?i)W\s*(\d{2,3})`)
	minnanoAVHipsPattern        = regexp.MustCompile(`(?i)H\s*(\d{2,3})`)
	minnanoAVCupPattern         = regexp.MustCompile(`(?i)B\s*\d{2,3}\s*\(\s*([A-Z])\s*カップ`)
)

var minnanoAVRateLimiter = struct {
	sync.Mutex
	next time.Time
}{}

// LookupActressByName resolves an exact actress name to a profile.
func (minnanoAV) LookupActressByName(name string) (*ActressInfo, error) {
	name = normalizeMinnanoAVName(name)
	if name == "" {
		return nil, ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return lookupMinnanoAVActressByName(ctx, minnanoAVBaseURL, name)
}

// LookupActressByCode implements lookupProvider.
func (minnanoAV) LookupActressByCode(code string) (*ActressInfo, error) {
	return nil, errors.New("minnanoav: lookup actress by code not supported")
}

// LookupActressURLByCodeAndName implements lookupProvider.
func (minnanoAV) LookupActressURLByCodeAndName(code, name string) (string, error) {
	return "", errors.New("minnanoav: lookup actress url not supported")
}

// LookupJavByCode implements lookupProvider.
func (minnanoAV) LookupJavByCode(code string) (*JavInfo, error) {
	return nil, errors.New("minnanoav: lookup jav not supported")
}

// LookupSeriesURLByCode implements lookupProvider.
func (minnanoAV) LookupSeriesURLByCode(code string) (string, error) {
	return "", errors.New("minnanoav: lookup series url not supported")
}

// LookupStudioURLByCode implements lookupProvider.
func (minnanoAV) LookupStudioURLByCode(code string) (string, error) {
	return "", errors.New("minnanoav: lookup studio url not supported")
}

func lookupMinnanoAVActressByName(ctx context.Context, baseURL, name string) (*ActressInfo, error) {
	searchURL, err := buildMinnanoAVActressSearchURL(baseURL, name)
	if err != nil {
		return nil, err
	}

	doc, status, finalURL, err := fetchMinnanoAVHTML(ctx, searchURL, baseURL)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound || doc == nil {
		return nil, ResourceNotFonud
	}

	if profileURL := canonicalMinnanoAVActressURL(finalURL, baseURL); profileURL != "" {
		if info := parseMinnanoAVActressInfo(doc); info != nil {
			return finalizeMinnanoAVActressInfo(name, profileURL, info, parseMinnanoAVActressAliases(doc)...)
		}
	}

	profileURL := findMinnanoAVActressSearchResultURL(doc, name, searchURL, baseURL)
	if profileURL == "" {
		return nil, ResourceNotFonud
	}

	doc, status, finalURL, err = fetchMinnanoAVHTML(ctx, profileURL, searchURL)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound || doc == nil {
		return nil, ResourceNotFonud
	}
	if canonical := canonicalMinnanoAVActressURL(finalURL, baseURL); canonical != "" {
		profileURL = canonical
	}
	return finalizeMinnanoAVActressInfo(
		name,
		profileURL,
		parseMinnanoAVActressInfo(doc),
		parseMinnanoAVActressAliases(doc)...,
	)
}

func buildMinnanoAVActressSearchURL(baseURL, name string) (string, error) {
	base, err := url.Parse(strings.TrimRight(baseURL, "/") + "/search_result.php")
	if err != nil {
		return "", fmt.Errorf("minnanoav: parse base url: %w", err)
	}
	query := base.Query()
	query.Set("search_scope", "actress")
	query.Set("search_word", normalizeMinnanoAVName(name))
	query.Set("search", "Go")
	base.RawQuery = query.Encode()
	return base.String(), nil
}

func fetchMinnanoAVHTML(ctx context.Context, targetURL, referer string) (*html.Node, int, string, error) {
	req, err := buildMinnanoAVRequest(ctx, targetURL, referer)
	if err != nil {
		return nil, 0, "", err
	}
	if err := waitForMinnanoAVRateLimit(ctx); err != nil {
		return nil, 0, "", err
	}

	logging.Info("minnanoav request: %s", targetURL)
	resp, err := util.DoRequest(req)
	if err != nil {
		if errors.Is(err, util.ErrCachedNotFound) {
			return nil, http.StatusNotFound, targetURL, nil
		}
		return nil, 0, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, responseURL(resp, targetURL), err
	}
	finalURL := responseURL(resp, targetURL)
	logging.Info("minnanoav response status: %s, length: %d bytes target=%s", resp.Status, len(body), finalURL)
	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode, finalURL, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, finalURL, fmt.Errorf("minnanoav: http %d", resp.StatusCode)
	}

	doc, err := parseHTMLDocument(body)
	if err != nil {
		return nil, resp.StatusCode, finalURL, fmt.Errorf("minnanoav: parse html: %w", err)
	}
	return doc, resp.StatusCode, finalURL, nil
}

func buildMinnanoAVRequest(ctx context.Context, targetURL, referer string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ja-JP,ja;q=0.9,en;q=0.7")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req, nil
}

func waitForMinnanoAVRateLimit(ctx context.Context) error {
	for {
		minnanoAVRateLimiter.Lock()
		now := time.Now()
		if !now.Before(minnanoAVRateLimiter.next) {
			minnanoAVRateLimiter.next = now.Add(minnanoAVRequestInterval)
			minnanoAVRateLimiter.Unlock()
			return nil
		}
		wait := time.Until(minnanoAVRateLimiter.next)
		minnanoAVRateLimiter.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return fmt.Errorf("minnanoav: rate limit wait: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func responseURL(resp *http.Response, fallback string) string {
	if resp != nil && resp.Request != nil && resp.Request.URL != nil {
		return resp.Request.URL.String()
	}
	return fallback
}

func findMinnanoAVActressSearchResultURL(root *html.Node, name, pageURL, baseURL string) string {
	name = normalizeMinnanoAVName(name)
	if root == nil || name == "" {
		return ""
	}

	matches := make(map[string]struct{})
	documentSelection(root).Find("h2.ttl a[href]").Each(func(_ int, link *goquery.Selection) {
		if minnanoAVNameWithoutQualifier(cleanSelectionText(link)) != name {
			return
		}
		resolved := resolveURL(pageURL, selectionAttr(link, "href"))
		if canonical := canonicalMinnanoAVActressURL(resolved, baseURL); canonical != "" {
			matches[canonical] = struct{}{}
		}
	})
	if len(matches) != 1 {
		return ""
	}
	for match := range matches {
		return match
	}
	return ""
}

func canonicalMinnanoAVActressURL(rawURL, baseURL string) string {
	target, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || target.Host == "" {
		return ""
	}
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || !strings.EqualFold(target.Host, base.Host) || !minnanoAVActressPathPattern.MatchString(target.Path) {
		return ""
	}
	target.RawQuery = ""
	target.Fragment = ""
	return target.String()
}

func parseMinnanoAVActressInfo(root *html.Node) *ActressInfo {
	if root == nil {
		return nil
	}
	doc := documentSelection(root)
	profile := doc.Find("div.act-profile").First()
	if profile.Length() == 0 {
		return nil
	}

	heading := doc.Find("h1").First()
	nameHeading := heading.Clone()
	nameHeading.Find("span").Remove()
	japaneseName := normalizeMinnanoAVName(cleanSelectionText(nameHeading))
	romanName := parseMinnanoAVRomanName(cleanSelectionText(heading.Find("span").First()))

	fields := make(map[string]string)
	profile.Find("tr").Each(func(_ int, row *goquery.Selection) {
		cell := row.ChildrenFiltered("td").First()
		label := cleanSelectionText(cell.ChildrenFiltered("span").First())
		if label == "" {
			return
		}
		value := cleanSelectionText(cell.ChildrenFiltered("p").First())
		if value != "" {
			fields[label] = value
		}
	})

	height, bust, waist, hips, cup := parseMinnanoAVMeasurements(fields["サイズ"])
	info := &ActressInfo{
		RomanName:    romanName,
		JapaneseName: japaneseName,
		HeightCM:     height,
		Bust:         bust,
		Waist:        waist,
		Hips:         hips,
		BirthDate:    parseMinnanoAVBirthDate(fields["生年月日"]),
		Cup:          cup,
	}
	if info.JapaneseName == "" {
		return nil
	}
	return info
}

func parseMinnanoAVRomanName(value string) string {
	parts := strings.Split(value, "/")
	if len(parts) < 2 {
		return ""
	}
	nameParts := strings.Fields(parts[len(parts)-1])
	if len(nameParts) < 2 {
		return strings.Join(nameParts, " ")
	}
	return strings.Join(append(nameParts[1:], nameParts[0]), " ")
}

func parseMinnanoAVActressAliases(root *html.Node) []string {
	if root == nil {
		return nil
	}

	aliases := make(map[string]struct{})
	documentSelection(root).Find("div.act-profile tr").Each(func(_ int, row *goquery.Selection) {
		cell := row.ChildrenFiltered("td").First()
		if cleanSelectionText(cell.ChildrenFiltered("span").First()) != "別名" {
			return
		}
		alias := minnanoAVNameWithoutQualifier(cleanSelectionText(cell.ChildrenFiltered("p").First()))
		if alias != "" {
			aliases[alias] = struct{}{}
		}
	})

	result := make([]string, 0, len(aliases))
	for alias := range aliases {
		result = append(result, alias)
	}
	return result
}

func parseMinnanoAVMeasurements(value string) (height, bust, waist, hips, cup int) {
	height = firstMinnanoAVNumber(minnanoAVHeightPattern, value)
	bust = firstMinnanoAVNumber(minnanoAVBustPattern, value)
	waist = firstMinnanoAVNumber(minnanoAVWaistPattern, value)
	hips = firstMinnanoAVNumber(minnanoAVHipsPattern, value)
	if match := minnanoAVCupPattern.FindStringSubmatch(value); len(match) > 1 {
		cup = cupLetterToNumber(match[1])
	}
	return
}

func firstMinnanoAVNumber(pattern *regexp.Regexp, value string) int {
	match := pattern.FindStringSubmatch(value)
	if len(match) < 2 {
		return 0
	}
	number, _ := strconv.Atoi(match[1])
	return number
}

func parseMinnanoAVBirthDate(value string) int {
	match := minnanoAVBirthDatePattern.FindStringSubmatch(value)
	if len(match) != 4 {
		return 0
	}
	year, errYear := strconv.Atoi(match[1])
	month, errMonth := strconv.Atoi(match[2])
	day, errDay := strconv.Atoi(match[3])
	if errYear != nil || errMonth != nil || errDay != nil {
		return 0
	}
	parsed := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if parsed.Year() != year || int(parsed.Month()) != month || parsed.Day() != day {
		return 0
	}
	return int(parsed.Unix())
}

func finalizeMinnanoAVActressInfo(name, profileURL string, info *ActressInfo, aliases ...string) (*ActressInfo, error) {
	if info == nil {
		return nil, ResourceNotFonud
	}
	wantName := normalizeMinnanoAVName(name)
	parsedName := normalizeMinnanoAVName(info.JapaneseName)
	matched := wantName != "" && parsedName != "" && parsedName == wantName
	for _, alias := range aliases {
		if normalizeMinnanoAVName(alias) == wantName {
			matched = true
			break
		}
	}
	if !matched {
		logging.Info("minnanoav: japanese name mismatch input=%s parsed=%s", wantName, parsedName)
		return nil, ResourceNotFonud
	}
	if parsedName != wantName {
		logging.Info("minnanoav: resolved actress alias input=%s parsed=%s", wantName, parsedName)
	}
	info.JapaneseName = parsedName
	info.RomanName = strings.Join(strings.Fields(info.RomanName), " ")
	info.ProfileURL = profileURL
	return info, nil
}

func normalizeMinnanoAVName(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func minnanoAVNameWithoutQualifier(value string) string {
	if index := strings.IndexAny(value, "(（"); index >= 0 {
		value = value[:index]
	}
	return normalizeMinnanoAVName(value)
}
