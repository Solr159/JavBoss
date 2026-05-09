package jav

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"pornboss/internal/util"

	"pornboss/internal/common/logging"

	"golang.org/x/net/html"
)

// javDatabase implements lookupProvider.
type javDatabase struct{}

var javDatabaseProvider lookupProvider = javDatabase{}

var errNoActressLink = errors.New("javdatabase: actress link not found")

// LookupActressByJapaneseName implements lookupProvider.
func (javDatabase) LookupActressByJapaneseName(name string) (*ActressInfo, error) {
	panic("unimplemented")
}

// LookupActressByCode resolves a solo movie code to its actress profile.
func (javDatabase) LookupActressByCode(code string) (*ActressInfo, error) {
	return lookupActressByCode(code)
}

// LookupCoverURLByCode resolves the cover image URL for a given movie code.
func (javDatabase) LookupCoverURLByCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	base := "https://www.javdatabase.com"
	movieURL := fmt.Sprintf("%s/movies/%s", base, code)

	doc, status, err := fetchJavDatabaseHTML(ctx, movieURL, base)
	if err != nil {
		return "", err
	}
	if status == http.StatusNotFound || doc == nil {
		return "", ResourceNotFonud
	}

	coverURL := parseJavDatabaseCoverURL(doc, movieURL)
	if coverURL == "" {
		return "", ResourceNotFonud
	}
	return coverURL, nil
}

// LookupJavByCode fetches metadata for a given code.
func (javDatabase) LookupJavByCode(code string) (*JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ResourceNotFonud
	}

	base := "https://www.javdatabase.com"
	movieURL := fmt.Sprintf("%s/movies/%s/", base, code)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	doc, status, err := fetchJavDatabaseHTML(ctx, movieURL, base)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound || doc == nil {
		return nil, ResourceNotFonud
	}

	info := parseJavDatabaseMovieInfo(doc)
	if info == nil {
		return nil, ResourceNotFonud
	}
	if info.Code != "" && normalizeJavDatabaseCode(info.Code) != normalizeJavDatabaseCode(code) {
		logging.Info("javdatabase: requested code %s resolved to %s", code, info.Code)
		return nil, ResourceNotFonud
	}
	if info.Code == "" {
		info.Code = code
	}
	return info, nil
}

func lookupActressByCode(code string) (*ActressInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ResourceNotFonud
	}

	base := "https://www.javdatabase.com"

	movieURL := fmt.Sprintf("%s/movies/%s", base, code)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	doc, status, err := fetchJavDatabaseHTML(ctx, movieURL, base)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound || doc == nil {
		return nil, ResourceNotFonud
	}

	actressLink, err := findJavDatabaseActressLink(doc)
	if err != nil {
		if errors.Is(err, errNoActressLink) {
			return nil, ResourceNotFonud
		}
		return nil, err
	}
	if actressLink == "" {
		return nil, ResourceNotFonud
	}
	actressURL := resolveURL(movieURL, actressLink)
	if actressURL == "" {
		return nil, ResourceNotFonud
	}

	actressDoc, status, err := fetchJavDatabaseHTML(ctx, actressURL, movieURL)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound || actressDoc == nil {
		return nil, ResourceNotFonud
	}

	info := parseJavDatabaseActressInfo(actressDoc)
	if info == nil {
		return nil, ResourceNotFonud
	}
	info.ProfileURL = actressURL
	return info, nil
}

func fetchJavDatabaseHTML(ctx context.Context, targetURL, referer string) (*html.Node, int, error) {
	req, err := buildJavDatabaseRequest(ctx, targetURL, referer)
	if err != nil {
		return nil, 0, err
	}

	logging.Info("javdatabase request: %s", targetURL)
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

	logging.Info("javdatabase response status: %s, length: %d bytes", resp.Status, len(body))
	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("javdatabase: http %d", resp.StatusCode)
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("javdatabase: parse html: %w", err)
	}
	return doc, resp.StatusCode, nil
}

func buildJavDatabaseRequest(ctx context.Context, targetURL, referer string) (*http.Request, error) {
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

type actressLinkCandidate struct {
	href  string
	score int
}

func findJavDatabaseActressLink(root *html.Node) (string, error) {
	link, err := findActressLinkFromIdolSection(root)
	if err != nil {
		return "", err
	}
	if link != "" {
		return link, nil
	}

	var candidates []actressLinkCandidate
	seen := make(map[string]struct{})

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := strings.TrimSpace(attrValue(n, "href"))
			if href != "" && looksLikeActressURL(href) {
				if _, ok := seen[href]; !ok {
					seen[href] = struct{}{}
					score := scoreActressLink(n, href)
					candidates = append(candidates, actressLinkCandidate{href: href, score: score})
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)

	if len(candidates) == 0 {
		return "", nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	return candidates[0].href, nil
}

func findActressLinkFromIdolSection(root *html.Node) (string, error) {
	var link string
	var links []string
	found := false
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found {
			return
		}
		if n.Type == html.ElementNode && n.Data == "p" && hasClass(n, "mb-1") {
			if isIdolSection(n) {
				links = collectIdolSectionLinks(n)
				found = true
				if len(links) > 1 {
					return
				}
				if len(links) == 1 {
					link = links[0]
				}
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if len(links) > 1 {
		return "", fmt.Errorf("javdatabase: multiple actresses found: %d", len(links))
	}
	if found && len(links) == 0 {
		return "", errNoActressLink
	}
	return link, nil
}

func isIdolSection(n *html.Node) bool {
	text := strings.ToLower(strings.TrimSpace(flattenText(n)))
	if text == "" {
		return false
	}
	if strings.Contains(text, "idol(s)/actress(es)") {
		return true
	}
	return strings.Contains(text, "idol") && strings.Contains(text, "actress")
}

func collectIdolSectionLinks(n *html.Node) []string {
	if n == nil {
		return nil
	}
	if b := findBoldByLabel(n, "idol(s)/actress(es)"); b != nil {
		return collectAnchorHrefsAfterBold(b)
	}
	return collectAnchorHrefs(n)
}

func findBoldByLabel(root *html.Node, label string) *html.Node {
	label = strings.ToLower(strings.TrimSpace(label))
	if root == nil || label == "" {
		return nil
	}
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "b" {
			text := strings.ToLower(strings.TrimSpace(flattenText(n)))
			if strings.Contains(text, label) {
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

func collectAnchorHrefsAfterBold(b *html.Node) []string {
	if b == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var hrefs []string
	for cur := b.NextSibling; cur != nil; cur = cur.NextSibling {
		if cur.Type == html.ElementNode && (cur.Data == "b" || cur.Data == "br") {
			break
		}
		for _, href := range collectAnchorHrefs(cur) {
			if href == "" {
				continue
			}
			if _, ok := seen[href]; ok {
				continue
			}
			seen[href] = struct{}{}
			hrefs = append(hrefs, href)
		}
	}
	return hrefs
}

func collectAnchorHrefs(n *html.Node) []string {
	if n == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var hrefs []string
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if cur.Type == html.ElementNode && cur.Data == "a" {
			href := strings.TrimSpace(attrValue(cur, "href"))
			if href != "" {
				if _, ok := seen[href]; !ok {
					seen[href] = struct{}{}
					hrefs = append(hrefs, href)
				}
			}
		}
		for c := cur.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return hrefs
}

func looksLikeActressURL(href string) bool {
	if href == "" || strings.HasPrefix(href, "#") {
		return false
	}
	lower := strings.ToLower(href)
	if strings.Contains(lower, "/movies/") {
		return false
	}
	for _, token := range []string{"/models/", "/model/", "/idols/", "/idol/", "/actress", "/actresses", "/actor", "/actors", "/stars/", "/star/"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func scoreActressLink(n *html.Node, href string) int {
	text := strings.TrimSpace(flattenText(n))
	lower := strings.ToLower(href)
	score := 1
	if strings.Contains(lower, "/models/") || strings.Contains(lower, "/model/") {
		score += 3
	}
	if strings.Contains(lower, "/idols/") || strings.Contains(lower, "/idol/") {
		score += 3
	}
	if strings.Contains(lower, "/actress") || strings.Contains(lower, "/actors") || strings.Contains(lower, "/stars/") {
		score += 2
	}
	if text != "" {
		score++
		if util.CodeRe.MatchString(text) {
			score -= 2
		}
	}
	if hasAncestorKeyword(n, []string{"actress", "actresses", "cast", "starring", "stars", "actor"}, 4) {
		score += 2
	}
	return score
}

func hasAncestorKeyword(n *html.Node, keywords []string, maxDepth int) bool {
	depth := 0
	for p := n.Parent; p != nil && depth < maxDepth; p = p.Parent {
		text := strings.ToLower(flattenText(p))
		for _, k := range keywords {
			if strings.Contains(text, k) {
				return true
			}
		}
		depth++
	}
	return false
}

func parseJavDatabaseActressInfo(root *html.Node) *ActressInfo {
	scope := findEntryContent(root)
	if scope == nil {
		scope = root
	}

	roman := strings.TrimSpace(findIdolName(scope))
	japanese := ""
	if containsJapaneseRunes(roman) {
		japanese = roman
		roman = ""
	}
	roman = cleanIdolName(roman)
	if roman == "" {
		roman = cleanJavDatabaseTitle(strings.TrimSpace(firstTextByTag(scope, "title")))
	}

	fields := extractJavDatabaseProfileFields(scope)
	if japanese == "" {
		japanese = guessJapaneseName(scope, roman)
	}
	height := parseHeightCM(fields.Height)
	bust, waist, hips := parseMeasurements(fields.Measurements)
	birthDate := parseBirthDateUnix(fields.BirthDate)
	info := &ActressInfo{
		RomanName:    roman,
		JapaneseName: cleanJapaneseName(firstNonEmpty(fields.JapaneseName, japanese)),
		HeightCM:     height,
		Bust:         bust,
		Waist:        waist,
		Hips:         hips,
		BirthDate:    birthDate,
		Cup:          parseCupValue(fields.Cup),
	}
	if info.Cup == 0 && fields.Measurements != "" {
		info.Cup = extractCupFromMeasurements(fields.Measurements)
	}

	if info.RomanName == "" && info.JapaneseName == "" && info.HeightCM == 0 && info.BirthDate == 0 && info.Bust == 0 && info.Waist == 0 && info.Hips == 0 && info.Cup == 0 {
		return nil
	}
	return info
}

func parseJavDatabaseMovieInfo(root *html.Node) *JavInfo {
	scope := findJavDatabaseMovieInfoColumn(root)
	if scope == nil {
		return nil
	}

	fields := extractJavDatabaseMovieFields(scope)
	title := strings.TrimSpace(fields.Title)
	if title == "" {
		title = cleanJavDatabaseMoviePageTitle(strings.TrimSpace(firstTextByTag(root, "title")))
	}

	info := &JavInfo{
		Title:       title,
		Code:        strings.TrimSpace(fields.Code),
		Studio:      strings.TrimSpace(fields.Studio),
		ReleaseUnix: parseDateUnix(fields.ReleaseDate),
		DurationMin: parseRuntimeMinutes(fields.Runtime),
		Tags:        dedupeNonEmpty(fields.Tags),
		Actors:      dedupeNonEmpty(fields.Actors),
		Provider:    ProviderJavDatabase,
	}
	if info.Title == "" && info.Code == "" && info.Studio == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func parseJavDatabaseCoverURL(root *html.Node, pageURL string) string {
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
		if n.Type == html.ElementNode && n.Data == "img" {
			if hasClass(n, "poster") || hasClass(n, "cover") {
				src := strings.TrimSpace(attrValue(n, "src"))
				if src != "" {
					cover = resolveURL(pageURL, src)
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return strings.TrimSpace(cover)
}

func normalizeJavDatabaseCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	var b strings.Builder
	for _, r := range code {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type javDatabaseMovieFields struct {
	Title       string
	Code        string
	Studio      string
	ReleaseDate string
	Runtime     string
	Tags        []string
	Actors      []string
}

func findJavDatabaseMovieInfoColumn(root *html.Node) *html.Node {
	if root == nil {
		return nil
	}

	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "movietable") {
			if column := findDescendantMovieInfoColumn(n); column != nil {
				found = column
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if found != nil {
		return found
	}
	return findDescendantMovieInfoColumn(root)
}

func findDescendantMovieInfoColumn(root *html.Node) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "div" &&
			hasClass(n, "col-md-10") &&
			hasClass(n, "col-lg-10") &&
			hasClass(n, "col-xxl-10") &&
			hasClass(n, "col-8") {
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

func extractJavDatabaseMovieFields(root *html.Node) javDatabaseMovieFields {
	var out javDatabaseMovieFields
	if root == nil {
		return out
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "p" && hasClass(n, "mb-1") {
			if bold := firstChildElementByTag(n, "b"); bold != nil {
				label := strings.TrimSpace(flattenText(bold))
				label = strings.TrimSuffix(label, ":")
				label = strings.TrimSuffix(label, "：")
				assignJavDatabaseMovieField(&out, label, n, bold)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

func assignJavDatabaseMovieField(out *javDatabaseMovieFields, label string, line, bold *html.Node) {
	if out == nil {
		return
	}

	label = normalizeLabel(label)
	if label == "" {
		return
	}

	switch {
	case labelHasAny(label, []string{"title"}):
		if out.Title == "" {
			out.Title = strings.TrimSpace(collectValueAfterBold(bold))
		}
	case labelHasAny(label, []string{"dvd id", "code", "movie id"}):
		if out.Code == "" {
			out.Code = strings.TrimSpace(collectValueAfterBold(bold))
		}
	case labelHasAny(label, []string{"release date", "released", "date"}):
		if out.ReleaseDate == "" {
			out.ReleaseDate = strings.TrimSpace(collectValueAfterBold(bold))
		}
	case labelHasAny(label, []string{"runtime", "duration"}):
		if out.Runtime == "" {
			out.Runtime = strings.TrimSpace(collectValueAfterBold(bold))
		}
	case labelHasAny(label, []string{"studio", "studios"}):
		if out.Studio == "" {
			out.Studio = firstNonEmpty(firstAnchorText(line), collectValueAfterBold(bold))
		}
	case labelHasAny(label, []string{"genre", "genres"}):
		if len(out.Tags) == 0 {
			out.Tags = collectAnchorTexts(line)
		}
	case labelHasAny(label, []string{"idol actress", "idol s actress es", "actress", "actresses", "idol", "idols"}):
		if len(out.Actors) == 0 {
			out.Actors = collectAnchorTexts(line)
		}
	}
}

type javDatabaseProfileFields struct {
	JapaneseName string
	Height       string
	BirthDate    string
	Measurements string
	Cup          string
}

func extractJavDatabaseProfileFields(root *html.Node) javDatabaseProfileFields {
	var out javDatabaseProfileFields
	if root == nil {
		return out
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "b":
				label := strings.TrimSpace(flattenText(n))
				label = strings.TrimSuffix(label, ":")
				label = strings.TrimSuffix(label, "：")
				if label != "" {
					value := strings.TrimSpace(collectValueAfterBold(n))
					if value != "" {
						assignProfileField(&out, label, value)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

func assignProfileField(out *javDatabaseProfileFields, label, value string) {
	if out == nil {
		return
	}
	label = normalizeLabel(label)
	value = strings.TrimSpace(value)
	if label == "" || value == "" {
		return
	}

	switch {
	case labelHasAny(label, []string{"japanese name", "name japanese", "native name", "japanese", "jp"}):
		if out.JapaneseName == "" {
			out.JapaneseName = value
		}
	case labelHasAny(label, []string{"height", "height cm", "height centimeter"}):
		if out.Height == "" {
			out.Height = value
		}
	case labelHasAny(label, []string{"dob", "birthdate", "birth date", "birthday", "born", "date of birth"}):
		if out.BirthDate == "" {
			out.BirthDate = value
		}
	case labelHasAny(label, []string{"measurements", "bust waist hips", "bust waist hip", "bust/waist/hips", "bwh", "b w h"}):
		if out.Measurements == "" {
			out.Measurements = value
		}
	case labelHasAny(label, []string{"cup", "cup size"}):
		if out.Cup == "" {
			out.Cup = value
		}
	}
}

func normalizeLabel(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	label = strings.ReplaceAll(label, "：", ":")
	replacer := strings.NewReplacer("-", " ", "_", " ", "/", " ", "(", " ", ")", " ", "[", " ", "]", " ", ".", " ", ",", " ")
	label = replacer.Replace(label)
	label = strings.Join(strings.Fields(label), " ")
	return label
}

func labelHasAny(label string, tokens []string) bool {
	for _, token := range tokens {
		if strings.Contains(label, token) {
			return true
		}
	}
	return false
}

func parseDateUnix(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	re := regexp.MustCompile(`\d{4}[-/]\d{2}[-/]\d{2}`)
	match := re.FindString(value)
	if match == "" {
		return 0
	}
	match = strings.ReplaceAll(match, "/", "-")
	t, err := time.Parse("2006-01-02", match)
	if err != nil {
		return 0
	}
	return t.Unix()
}

func parseBirthDateUnix(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	return int(parseDateUnix(value))
}

func parseRuntimeMinutes(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(value)
	if match == "" {
		return 0
	}
	minutes, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}
	return minutes
}

func extractCupFromMeasurements(measurements string) int {
	measurements = strings.TrimSpace(measurements)
	if measurements == "" {
		return 0
	}
	re := regexp.MustCompile(`(?i)\b([A-K])\s*cup\b`)
	if match := re.FindStringSubmatch(measurements); len(match) > 1 {
		return cupLetterToNumber(match[1])
	}
	re = regexp.MustCompile(`(?i)\bcup\s*([A-K])\b`)
	if match := re.FindStringSubmatch(measurements); len(match) > 1 {
		return cupLetterToNumber(match[1])
	}
	return 0
}

func parseCupValue(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	re := regexp.MustCompile(`(?i)\b([A-K])\b`)
	if match := re.FindStringSubmatch(value); len(match) > 1 {
		return cupLetterToNumber(match[1])
	}
	return 0
}

func cupLetterToNumber(value string) int {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return 0
	}
	r := rune(value[0])
	if r < 'A' || r > 'Z' {
		return 0
	}
	return int(r-'A') + 1
}

func parseHeightCM(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(value)
	if match == "" {
		return 0
	}
	height, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}
	return height
}

func parseMeasurements(value string) (int, int, int) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, 0, 0
	}
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(value, -1)
	if len(matches) < 3 {
		return 0, 0, 0
	}
	bust, err := strconv.Atoi(matches[0])
	if err != nil {
		return 0, 0, 0
	}
	waist, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, 0, 0
	}
	hips, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, 0, 0
	}
	return bust, waist, hips
}

func findEntryContent(root *html.Node) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "entry-content") {
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

func findIdolName(root *html.Node) string {
	var name string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if name != "" {
			return
		}
		if n.Type == html.ElementNode && (n.Data == "h1" || n.Data == "h2" || n.Data == "h3") {
			if hasClass(n, "idol-name") {
				name = strings.TrimSpace(flattenText(n))
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if name != "" {
		return name
	}
	if t := strings.TrimSpace(firstTextByTag(root, "h1")); t != "" {
		return t
	}
	if t := strings.TrimSpace(firstTextByTag(root, "h2")); t != "" {
		return t
	}
	return strings.TrimSpace(firstTextByTag(root, "h3"))
}

func cleanIdolName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if idx := strings.Index(value, " - "); idx >= 0 {
		value = value[:idx]
	}
	for _, suffix := range []string{"JAV Profile", "- JAV Profile"} {
		value = strings.TrimSpace(strings.TrimSuffix(value, suffix))
	}
	return strings.TrimSpace(value)
}

func cleanJapaneseName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	value = strings.Trim(value, " -")
	value = strings.Trim(value, "–—")
	return strings.TrimSpace(value)
}

func collectValueAfterBold(b *html.Node) string {
	for cur := b.NextSibling; cur != nil; cur = cur.NextSibling {
		if cur.Type == html.ElementNode && (cur.Data == "b" || cur.Data == "br") {
			break
		}
		if text := firstAnchorText(cur); text != "" {
			return text
		}
	}

	var bld strings.Builder
	for cur := b.NextSibling; cur != nil; cur = cur.NextSibling {
		if cur.Type == html.ElementNode {
			if cur.Data == "b" || cur.Data == "br" {
				break
			}
			text := strings.TrimSpace(flattenText(cur))
			if text != "" {
				bld.WriteString(text)
			}
			continue
		}
		if cur.Type == html.TextNode {
			bld.WriteString(cur.Data)
		}
	}
	value := strings.TrimSpace(bld.String())
	value = strings.TrimLeft(value, "-–: ")
	if idx := strings.Index(value, " - "); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}
	return value
}

func firstChildElementByTag(root *html.Node, tag string) *html.Node {
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			return c
		}
	}
	return nil
}

func collectAnchorTexts(root *html.Node) []string {
	if root == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var texts []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
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

func dedupeNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func firstAnchorText(root *html.Node) string {
	if root == nil {
		return ""
	}
	if root.Type == html.ElementNode && root.Data == "a" {
		return strings.TrimSpace(flattenText(root))
	}
	var text string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if text != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func guessJapaneseName(root *html.Node, roman string) string {
	for _, tag := range []string{"h2", "h3", "h1"} {
		text := strings.TrimSpace(firstTextByTag(root, tag))
		if text == "" || text == roman {
			continue
		}
		if containsJapaneseRunes(text) {
			return text
		}
	}
	return ""
}

func containsJapaneseRunes(value string) bool {
	for _, r := range value {
		switch {
		case r >= 0x3040 && r <= 0x30ff: // Hiragana + Katakana
			return true
		case r >= 0x31f0 && r <= 0x31ff: // Katakana Phonetic Extensions
			return true
		case r >= 0x4e00 && r <= 0x9fff: // CJK Unified Ideographs
			return true
		case r >= 0xff66 && r <= 0xff9d: // Halfwidth Katakana
			return true
		}
	}
	return false
}

func cleanJavDatabaseTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	for _, suffix := range []string{"- JAVDatabase", "- JavDatabase", "- JavDatabase.com", "- JAVDatabase.com"} {
		title = strings.TrimSuffix(title, suffix)
	}
	return strings.TrimSpace(title)
}

func cleanJavDatabaseMoviePageTitle(title string) string {
	title = cleanJavDatabaseTitle(title)
	if title == "" {
		return ""
	}
	if idx := strings.LastIndex(title, " - "); idx >= 0 {
		title = strings.TrimSpace(title[idx+3:])
	}
	if strings.EqualFold(title, "JAV Database") {
		return ""
	}
	return title
}

func resolveURL(baseURL, href string) string {
	if href == "" {
		return ""
	}
	reference, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if reference.IsAbs() {
		return reference.String()
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return base.ResolveReference(reference).String()
}
