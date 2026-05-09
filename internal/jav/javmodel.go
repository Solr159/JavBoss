package jav

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"pornboss/internal/common/logging"
	"pornboss/internal/util"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// javModel implements lookupProvider.
type javModel struct{}

var javModelProvider lookupProvider = javModel{}

// LookupActressByCode implements lookupProvider.
func (javModel) LookupActressByCode(code string) (*ActressInfo, error) {
	return nil, errors.New("javmodel: lookup actress not supported")
}

// LookupCoverURLByCode resolves a cover image URL for a movie code.
func (javModel) LookupCoverURLByCode(code string) (string, error) {
	return "", errors.New("javmodel: lookup cover not supported")
}

// LookupActressByJapaneseName implements lookupProvider.
func (javModel) LookupActressByJapaneseName(name string) (*ActressInfo, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ResourceNotFonud
	}
	logging.Info("javmodel: name -> %s", name)

	base := "https://javmodel.com"
	searchURL := fmt.Sprintf("%s/jav/search.html?q=%s", base, url.QueryEscape(name))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	searchDoc, status, err := fetchJavModelHTML(ctx, searchURL, base)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound || searchDoc == nil {
		return nil, ResourceNotFonud
	}

	card := findJavModelSearchCard(searchDoc)
	if card == nil {
		return nil, ResourceNotFonud
	}
	romanName, href := extractJavModelSearchResult(card)
	romanName = strings.TrimSpace(romanName)

	detailURL := ""
	slug := strings.Join(strings.Fields(romanName), "-")
	if slug != "" {
		detailURL = fmt.Sprintf("%s/jav/%s", base, slug)
	}
	if detailURL == "" && href != "" {
		detailURL = resolveURL(searchURL, href)
	}
	if detailURL == "" {
		return nil, ResourceNotFonud
	}

	detailDoc, status, err := fetchJavModelHTML(ctx, detailURL, searchURL)
	if err != nil {
		return nil, err
	}
	if status == http.StatusFound {
		return nil, ResourceNotFonud
	}
	if status == http.StatusNotFound || detailDoc == nil {
		return nil, ResourceNotFonud
	}

	profile := findJavModelProfileCard(detailDoc)
	if profile == nil {
		return nil, ResourceNotFonud
	}

	info := parseJavModelActressInfo(profile)
	if info == nil {
		return nil, ResourceNotFonud
	}
	if info.RomanName == "" && romanName != "" {
		info.RomanName = romanName
	}
	if info.JapaneseName == "" && containsJapaneseRunes(name) {
		info.JapaneseName = name
	}
	info.ProfileURL = detailURL
	logging.Info("javmodel: found actress profile name=%s roman=%s japanese=%s chinese=%s", name, info.RomanName, info.JapaneseName, info.ChineseName)
	return info, nil
}

// LookupJavByCode implements lookupProvider.
func (javModel) LookupJavByCode(code string) (*JavInfo, error) {
	panic("unimplemented")
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

	doc, err := html.Parse(strings.NewReader(string(body)))
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
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "card") && hasClass(n, "flq-card-blog") {
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

func extractJavModelSearchResult(card *html.Node) (string, string) {
	if card == nil {
		return "", ""
	}
	var roman string
	var href string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if roman != "" || href != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "h5" && hasClass(n, "card-title") && hasClass(n, "h6") {
			if anchor := firstAnchorNode(n); anchor != nil {
				roman = strings.TrimSpace(flattenText(anchor))
				href = strings.TrimSpace(attrValue(anchor, "href"))
				if roman == "" {
					roman = strings.TrimSpace(flattenText(n))
				}
			} else {
				roman = strings.TrimSpace(flattenText(n))
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(card)
	return roman, href
}

func firstAnchorNode(root *html.Node) *html.Node {
	if root == nil {
		return nil
	}
	if root.Type == html.ElementNode && root.Data == "a" {
		return root
	}
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
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

func findJavModelProfileCard(root *html.Node) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "div" &&
			hasClass(n, "col-12") &&
			hasClass(n, "col-lg-7") &&
			hasClass(n, "col-xxl-8") &&
			hasClass(n, "remove-animation") &&
			hasClass(n, "card") {
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

type javModelProfileFields struct {
	BirthDate string
	Height    string
	Bust      string
	Waist     string
	Hips      string
}

func parseJavModelActressInfo(root *html.Node) *ActressInfo {
	if root == nil {
		return nil
	}

	roman := strings.TrimSpace(firstTextByTag(root, "h1"))
	japaneseRaw := strings.TrimSpace(firstTextByTag(root, "h2"))
	japanese, chinese := splitJavModelNames(japaneseRaw)

	fields := extractJavModelProfileFields(root)

	height := parseHeightCM(fields.Height)
	bust := parseHeightCM(fields.Bust)
	waist := parseHeightCM(fields.Waist)
	hips := parseHeightCM(fields.Hips)
	birthDate := parseBirthDateFlexible(fields.BirthDate)

	info := &ActressInfo{
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

func extractJavModelProfileFields(root *html.Node) javModelProfileFields {
	var out javModelProfileFields
	if root == nil {
		return out
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			label, value := extractJavModelRow(n)
			if label != "" && value != "" {
				assignJavModelProfileField(&out, label, value)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

func extractJavModelRow(row *html.Node) (string, string) {
	var cells []*html.Node
	for c := row.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
			cells = append(cells, c)
		}
	}
	if len(cells) < 2 {
		return "", ""
	}
	label := strings.TrimSpace(flattenText(cells[0]))
	value := strings.TrimSpace(flattenText(cells[1]))
	return label, value
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
		if containsJapaneseRunes(part) {
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
	if unix := parseBirthDateUnix(value); unix != 0 {
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
