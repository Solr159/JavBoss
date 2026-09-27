package javmodel

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
	"javboss/internal/jav/internal/htmlutil"
	"javboss/internal/jav/internal/parseutil"
	"javboss/internal/jav/metadata"
	"javboss/internal/util"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// Client retrieves metadata from javmodel.
type Client struct {
}

// LookupActressByName queries javmodel.
func (p *Client) LookupActressByName(ctx context.Context, name string) (*metadata.ActressInfo, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, metadata.ErrNotFound
	}
	logging.Info("javmodel: name -> %s", name)

	base := "https://javmodel.com"
	searchURL := fmt.Sprintf("%s/jav/search.html?q=%s", base, url.QueryEscape(name))

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	searchDoc, status, err := fetchJavModelHTML(ctx, searchURL, base)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound || searchDoc == nil {
		return nil, metadata.ErrNotFound
	}

	card := findJavModelSearchCard(searchDoc)
	if card == nil {
		return nil, metadata.ErrNotFound
	}
	romanName, href := extractJavModelSearchResult(card)
	romanName = strings.TrimSpace(romanName)

	detailURL := ""
	slug := strings.Join(strings.Fields(romanName), "-")
	if slug != "" {
		detailURL = fmt.Sprintf("%s/jav/%s", base, slug)
	}
	if detailURL == "" && href != "" {
		detailURL = parseutil.ResolveURL(searchURL, href)
	}
	if detailURL == "" {
		return nil, metadata.ErrNotFound
	}

	detailDoc, status, err := fetchJavModelHTML(ctx, detailURL, searchURL)
	if err != nil {
		return nil, err
	}
	if status == http.StatusFound {
		return nil, metadata.ErrNotFound
	}
	if status == http.StatusNotFound || detailDoc == nil {
		return nil, metadata.ErrNotFound
	}

	profile := findJavModelProfileCard(detailDoc)
	if profile == nil {
		return nil, metadata.ErrNotFound
	}

	info := parseJavModelActressInfo(profile)
	if info == nil {
		return nil, metadata.ErrNotFound
	}
	info, err = finalizeJavModelActressInfo(name, romanName, detailURL, info)
	if err != nil {
		return nil, err
	}
	logging.Info("javmodel: found actress profile name=%s roman=%s japanese=%s chinese=%s", name, info.RomanName, info.JapaneseName, info.ChineseName)
	return info, nil
}

func fetchJavModelHTML(ctx context.Context, targetURL, referer string) (*html.Node, int, error) {
	req, err := buildJavModelRequest(ctx, targetURL, referer)
	if err != nil {
		return nil, 0, err
	}

	logging.Info("javmodel request: %s", targetURL)
	resp, err := util.DoRequest(req)
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
	logging.Info("javmodel response status: %s, length: %d bytes target=%s", resp.Status, len(body), targetURL)
	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("javmodel: http %d", resp.StatusCode)
	}

	doc, err := htmlutil.ParseHTMLDocument(body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("javmodel: parse html: %w", err)
	}
	return doc, resp.StatusCode, nil
}

func buildJavModelRequest(ctx context.Context, targetURL, referer string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req, nil
}

func findJavModelSearchCard(root *html.Node) *html.Node {
	return htmlutil.FirstSelectionNode(htmlutil.DocumentSelection(root).Find("div.card.flq-card-blog").First())
}

func extractJavModelSearchResult(card *html.Node) (string, string) {
	if card == nil {
		return "", ""
	}
	title := htmlutil.DocumentSelection(card).Find("h5.card-title.h6").First()
	link := title.Find("a").First()
	roman := htmlutil.CleanSelectionText(link)
	if roman == "" {
		roman = htmlutil.CleanSelectionText(title)
	}
	return roman, htmlutil.SelectionAttr(link, "href")
}

func findJavModelProfileCard(root *html.Node) *html.Node {
	return htmlutil.FirstSelectionNode(htmlutil.DocumentSelection(root).
		Find("div.col-12.col-lg-7.col-xxl-8.remove-animation.card").
		First())
}

type javModelProfileFields struct {
	BirthDate string
	Height    string
	Bust      string
	Waist     string
	Hips      string
}

func parseJavModelActressInfo(root *html.Node) *metadata.ActressInfo {
	if root == nil {
		return nil
	}

	roman := strings.TrimSpace(htmlutil.FirstTextByTag(root, "h1"))
	japaneseRaw := strings.TrimSpace(htmlutil.FirstTextByTag(root, "h2"))
	japanese, chinese := splitJavModelNames(japaneseRaw)

	fields := extractJavModelProfileFields(root)

	height := parseutil.ParseHeightCM(fields.Height)
	bust := parseutil.ParseHeightCM(fields.Bust)
	waist := parseutil.ParseHeightCM(fields.Waist)
	hips := parseutil.ParseHeightCM(fields.Hips)
	birthDate := parseBirthDateFlexible(fields.BirthDate)

	info := &metadata.ActressInfo{
		RomanName:    roman,
		JapaneseName: japanese,
		ChineseName:  chinese,
		HeightCM:     height,
		Bust:         bust,
		Waist:        waist,
		Hips:         hips,
		BirthDate:    birthDate,
	}

	return info
}

func finalizeJavModelActressInfo(name, romanName, detailURL string, info *metadata.ActressInfo) (*metadata.ActressInfo, error) {
	if info == nil {
		return nil, metadata.ErrNotFound
	}
	name = strings.TrimSpace(name)
	if info.RomanName == "" && romanName != "" {
		info.RomanName = romanName
	}
	japaneseName := strings.TrimSpace(info.JapaneseName)
	if japaneseName == "" {
		logging.Info("javmodel: missing japanese name in profile input=%s roman=%s", name, info.RomanName)
		return nil, metadata.ErrNotFound
	}
	if japaneseName != name {
		logging.Info("javmodel: japanese name mismatch input=%s parsed=%s roman=%s", name, info.JapaneseName, info.RomanName)
		return nil, metadata.ErrNotFound
	}
	info.JapaneseName = japaneseName
	info.ProfileURL = detailURL
	return info, nil
}

func extractJavModelProfileFields(root *html.Node) javModelProfileFields {
	var out javModelProfileFields
	if root == nil {
		return out
	}

	htmlutil.DocumentSelection(root).Find("tr").Each(func(_ int, row *goquery.Selection) {
		cells := row.ChildrenFiltered("th, td")
		if cells.Length() >= 2 {
			assignJavModelProfileField(
				&out,
				htmlutil.CleanSelectionText(cells.Eq(0)),
				htmlutil.CleanSelectionText(cells.Eq(1)),
			)
		}
	})
	return out
}

func assignJavModelProfileField(out *javModelProfileFields, label, value string) {
	if out == nil {
		return
	}
	label = strings.TrimSpace(label)
	value = strings.TrimSpace(value)
	if label == "" || value == "" {
		return
	}

	switch label {
	case "Birthday":
		if out.BirthDate == "" {
			out.BirthDate = value
		}
	case "Height":
		if out.Height == "" {
			out.Height = value
		}
	case "Breast":
		if out.Bust == "" {
			out.Bust = value
		}
	case "Waist":
		if out.Waist == "" {
			out.Waist = value
		}
	case "Hips":
		if out.Hips == "" {
			out.Hips = value
		}
	}
}

func splitJavModelNames(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	parts := splitJavModelNameVariants(raw)
	if len(parts) == 0 {
		return raw, ""
	}
	var japanese string
	for _, part := range parts {
		if parseutil.ContainsJapaneseRunes(part) {
			japanese = part
			break
		}
	}
	if japanese == "" {
		japanese = parts[0]
	}
	var chinese string
	for _, part := range parts {
		if part != "" && part != japanese {
			chinese = part
			break
		}
	}
	return japanese, chinese
}

func splitJavModelNameVariants(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	re := regexp.MustCompile(`\s*[-–—]\s*`)
	parts := re.Split(raw, -1)
	var out []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return []string{raw}
	}
	return out
}

func parseBirthDateFlexible(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if unix := parseutil.ParseBirthDateUnix(value); unix != 0 {
		return unix
	}
	re := regexp.MustCompile(`\d{1,2}/\d{1,2}/\d{4}`)
	if match := re.FindString(value); match != "" {
		parts := strings.Split(match, "/")
		if len(parts) == 3 {
			first, _ := strconv.Atoi(parts[0])
			layout := "01/02/2006"
			if first > 12 {
				layout = "02/01/2006"
			}
			if t, err := time.Parse(layout, match); err == nil {
				return int(t.Unix())
			}
		}
		if t, err := time.Parse("01/02/2006", match); err == nil {
			return int(t.Unix())
		}
	}
	return 0
}

// New creates an independent provider client.
func New() *Client { return &Client{} }

// CheckConnectivity requests the site using its normal headers and transport, without lookup caching.
// The caller owns the response body.
func (p *Client) CheckConnectivity(ctx context.Context) (*http.Response, error) {
	req, err := buildJavModelRequest(ctx, p.ConnectivityURL()+"/", p.ConnectivityURL())
	if err != nil {
		return nil, err
	}
	return util.DefaultHTTPClient().Do(req)
}

// ConnectivityURL identifies the origin used for connectivity checks.
func (p *Client) ConnectivityURL() string { return "https://javmodel.com" }
