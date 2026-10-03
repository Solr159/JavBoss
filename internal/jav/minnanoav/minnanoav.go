package minnanoav

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
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

// MinnanoAVClient retrieves actress profiles from みんなのAV.com.
type MinnanoAVClient struct {
	httpClient *http.Client
	limiter    *ratelimit.Limiter
}

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

// LookupActressByName resolves an exact actress name to a profile.
func (p *MinnanoAVClient) LookupActressByName(ctx context.Context, name string) (*metadata.ActressInfo, error) {
	name = normalizeMinnanoAVName(name)
	if name == "" {
		return nil, metadata.ErrNotFound
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return p.lookupMinnanoAVActressByName(ctx, minnanoAVBaseURL, name)
}

func (p *MinnanoAVClient) lookupMinnanoAVActressByName(ctx context.Context, baseURL, name string) (*metadata.ActressInfo, error) {
	searchURL, err := buildMinnanoAVActressSearchURL(baseURL, name)
	if err != nil {
		return nil, err
	}

	doc, status, finalURL, err := p.fetchMinnanoAVHTML(ctx, searchURL, baseURL)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound || doc == nil {
		return nil, metadata.ErrNotFound
	}

	if profileURL := canonicalMinnanoAVActressURL(finalURL, baseURL); profileURL != "" {
		if info := parseMinnanoAVActressInfo(doc); info != nil {
			return finalizeMinnanoAVActressInfo(name, profileURL, info, parseMinnanoAVActressAliases(doc)...)
		}
	}

	profileURL := findMinnanoAVActressSearchResultURL(doc, name, searchURL, baseURL)
	if profileURL == "" {
		return nil, metadata.ErrNotFound
	}

	doc, status, finalURL, err = p.fetchMinnanoAVHTML(ctx, profileURL, searchURL)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound || doc == nil {
		return nil, metadata.ErrNotFound
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

func (p *MinnanoAVClient) fetchMinnanoAVHTML(ctx context.Context, targetURL, referer string) (*html.Node, int, string, error) {
	req, err := buildMinnanoAVRequest(ctx, targetURL, referer)
	if err != nil {
		return nil, 0, "", err
	}
	if err := p.limiter.Wait(ctx); err != nil {
		return nil, 0, "", err
	}

	logging.Info("minnanoav request: %s", targetURL)
	resp, err := p.httpClient.Do(req)
	if err != nil {
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

	doc, err := htmlutil.ParseHTMLDocument(body)
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
	htmlutil.DocumentSelection(root).Find("h2.ttl a[href]").Each(func(_ int, link *goquery.Selection) {
		if minnanoAVNameWithoutQualifier(htmlutil.CleanSelectionText(link)) != name {
			return
		}
		resolved := parseutil.ResolveURL(pageURL, htmlutil.SelectionAttr(link, "href"))
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

func parseMinnanoAVActressInfo(root *html.Node) *metadata.ActressInfo {
	if root == nil {
		return nil
	}
	doc := htmlutil.DocumentSelection(root)
	profile := doc.Find("div.act-profile").First()
	if profile.Length() == 0 {
		return nil
	}

	heading := doc.Find("h1").First()
	nameHeading := heading.Clone()
	nameHeading.Find("span").Remove()
	japaneseName := normalizeMinnanoAVName(htmlutil.CleanSelectionText(nameHeading))
	romanName := parseMinnanoAVRomanName(htmlutil.CleanSelectionText(heading.Find("span").First()))

	fields := make(map[string]string)
	profile.Find("tr").Each(func(_ int, row *goquery.Selection) {
		cell := row.ChildrenFiltered("td").First()
		label := htmlutil.CleanSelectionText(cell.ChildrenFiltered("span").First())
		if label == "" {
			return
		}
		value := htmlutil.CleanSelectionText(cell.ChildrenFiltered("p").First())
		if value != "" {
			fields[label] = value
		}
	})

	height, bust, waist, hips, cup := parseMinnanoAVMeasurements(fields["サイズ"])
	info := &metadata.ActressInfo{
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
	htmlutil.DocumentSelection(root).Find("div.act-profile tr").Each(func(_ int, row *goquery.Selection) {
		cell := row.ChildrenFiltered("td").First()
		if htmlutil.CleanSelectionText(cell.ChildrenFiltered("span").First()) != "別名" {
			return
		}
		alias := minnanoAVNameWithoutQualifier(htmlutil.CleanSelectionText(cell.ChildrenFiltered("p").First()))
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
		cup = parseutil.CupLetterToNumber(match[1])
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

func finalizeMinnanoAVActressInfo(name, profileURL string, info *metadata.ActressInfo, aliases ...string) (*metadata.ActressInfo, error) {
	if info == nil {
		return nil, metadata.ErrNotFound
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
		return nil, metadata.ErrNotFound
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

// New creates a provider using the supplied non-nil HTTP client.
func New(httpClient *http.Client) *MinnanoAVClient {
	return &MinnanoAVClient{httpClient: httpClient, limiter: ratelimit.New(minnanoAVRequestInterval)}
}

// OriginURL identifies the origin used for availability checks.
func (p *MinnanoAVClient) OriginURL() string { return minnanoAVBaseURL }
