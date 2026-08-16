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
	"sync"
	"time"

	"javboss/internal/util"

	"javboss/internal/common/logging"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// javDatabase implements lookupProvider.
type javDatabase struct{}

var javDatabaseProvider lookupProvider = javDatabase{}

var errNoActressLink = errors.New("javdatabase: actress link not found")

const javDatabaseRequestInterval = 500 * time.Millisecond

var javDatabaseRateLimiter = struct {
	sync.Mutex
	next time.Time
}{}

// LookupActressByName implements lookupProvider.
func (javDatabase) LookupActressByName(name string) (*ActressInfo, error) {
	panic("unimplemented")
}

// LookupActressByCode resolves a solo movie code to its actress profile.
func (javDatabase) LookupActressByCode(code string) (*ActressInfo, error) {
	return lookupActressByCode(code)
}

// LookupActressURLByCodeAndName implements lookupProvider.
func (javDatabase) LookupActressURLByCodeAndName(code, name string) (string, error) {
	return "", errors.New("javdatabase: lookup actress url not supported")
}

// LookupSeriesURLByCode implements lookupProvider.
func (javDatabase) LookupSeriesURLByCode(code string) (string, error) {
	return "", errors.New("javdatabase: lookup series url not supported")
}

// LookupStudioURLByCode implements lookupProvider.
func (javDatabase) LookupStudioURLByCode(code string) (string, error) {
	return "", errors.New("javdatabase: lookup studio url not supported")
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
	info.CoverURL = parseJavDatabaseCoverURL(doc, movieURL)
	info.SampleImages = parseSampleImages(doc, movieURL)
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
	resp, err := doJavDatabaseRequest(req)
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

	doc, err := parseHTMLDocument(body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("javdatabase: parse html: %w", err)
	}
	return doc, resp.StatusCode, nil
}

func doJavDatabaseRequest(req *http.Request) (*http.Response, error) {
	if err := waitForJavDatabaseRateLimit(req.Context()); err != nil {
		return nil, err
	}
	return util.DoRequest(req)
}

func waitForJavDatabaseRateLimit(ctx context.Context) error {
	for {
		javDatabaseRateLimiter.Lock()
		now := time.Now()
		if !now.Before(javDatabaseRateLimiter.next) {
			javDatabaseRateLimiter.next = now.Add(javDatabaseRequestInterval)
			javDatabaseRateLimiter.Unlock()
			return nil
		}
		wait := time.Until(javDatabaseRateLimiter.next)
		javDatabaseRateLimiter.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return fmt.Errorf("javdatabase: rate limit wait: %w", ctx.Err())
		case <-timer.C:
		}
	}
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
	documentSelection(root).Find("a").Each(func(_ int, link *goquery.Selection) {
		href := selectionAttr(link, "href")
		if href != "" && looksLikeActressURL(href) {
			if _, exists := seen[href]; !exists {
				seen[href] = struct{}{}
				candidates = append(candidates, actressLinkCandidate{
					href:  href,
					score: scoreActressLink(firstSelectionNode(link), href),
				})
			}
		}
	})

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
	documentSelection(root).Find("p.mb-1").EachWithBreak(func(_ int, paragraph *goquery.Selection) bool {
		node := firstSelectionNode(paragraph)
		if isIdolSection(node) {
			links = collectIdolSectionLinks(node)
			found = true
			if len(links) == 1 {
				link = links[0]
			}
			return false
		}
		return true
	})
	if len(links) > 1 {
		return "", fmt.Errorf("javdatabase: multiple actresses found: %d", len(links))
	}
	if found && len(links) == 0 {
		return "", errNoActressLink
	}
	return link, nil
}

func isIdolSection(n *html.Node) bool {
	bold := documentSelection(n).ChildrenFiltered("b").First()
	if bold.Length() == 0 {
		return false
	}
	label := cleanSelectionText(bold)
	label = strings.TrimSuffix(label, ":")
	label = strings.TrimSuffix(label, "：")
	label = normalizeLabel(label)
	return labelHasAny(label, []string{"idol actress", "idol s actress es", "actress", "actresses", "idol", "idols"})
}

func collectIdolSectionLinks(n *html.Node) []string {
	if n == nil {
		return nil
	}
	return collectAnchorHrefs(n)
}

func collectAnchorHrefs(n *html.Node) []string {
	if n == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var hrefs []string
	documentSelection(n).Find("a").Each(func(_ int, link *goquery.Selection) {
		href := selectionAttr(link, "href")
		if href != "" {
			if _, exists := seen[href]; !exists {
				seen[href] = struct{}{}
				hrefs = append(hrefs, href)
			}
		}
	})
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
	text := cleanSelectionText(goquery.NewDocumentFromNode(n).Selection)
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
	if n == nil {
		return false
	}
	found := false
	goquery.NewDocumentFromNode(n).Selection.Parents().Slice(0, maxDepth).EachWithBreak(func(_ int, parent *goquery.Selection) bool {
		text := strings.ToLower(cleanSelectionText(parent))
		for _, k := range keywords {
			if strings.Contains(text, k) {
				found = true
				return false
			}
		}
		return true
	})
	return found
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
		Title:        title,
		Code:         strings.TrimSpace(fields.Code),
		Studio:       strings.TrimSpace(fields.Studio),
		Series:       strings.TrimSpace(fields.Series),
		ReleaseUnix:  parseDateUnix(fields.ReleaseDate),
		DurationMin:  parseRuntimeMinutes(fields.Runtime),
		Tags:         dedupeNonEmpty(fields.Tags),
		Actors:       dedupeNonEmpty(fields.Actors),
		CoverURL:     parseJavDatabaseCoverURL(root, ""),
		SampleImages: parseSampleImages(root, ""),
		Provider:     ProviderJavDatabase,
	}
	if info.Title == "" && info.Code == "" && info.Studio == "" && info.Series == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func parseJavDatabaseCoverURL(root *html.Node, pageURL string) string {
	doc := documentSelection(root)
	for _, candidate := range []string{
		selectionAttr(doc.Find(`meta[property="og:image"]`).First(), "content"),
		selectionAttr(doc.Find("img.poster, img.cover").First(), "src"),
	} {
		if cover := resolveURL(pageURL, candidate); cover != "" {
			return cover
		}
	}
	return ""
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
	Series      string
	ReleaseDate string
	Runtime     string
	Tags        []string
	Actors      []string
}

func findJavDatabaseMovieInfoColumn(root *html.Node) *html.Node {
	doc := documentSelection(root)
	selector := "div.col-md-10.col-lg-10.col-xxl-10.col-8"
	if column := doc.Find("div.movietable " + selector).First(); column.Length() > 0 {
		return firstSelectionNode(column)
	}
	return firstSelectionNode(doc.Find(selector).First())
}

func findDescendantMovieInfoColumn(root *html.Node) *html.Node {
	return firstSelectionNode(documentSelection(root).
		Find("div.col-md-10.col-lg-10.col-xxl-10.col-8").
		First())
}

func extractJavDatabaseMovieFields(root *html.Node) javDatabaseMovieFields {
	var out javDatabaseMovieFields
	if root == nil {
		return out
	}

	documentSelection(root).Find("p.mb-1").Each(func(_ int, line *goquery.Selection) {
		bold := line.ChildrenFiltered("b").First()
		if bold.Length() > 0 {
			label := normalizeLabel(cleanSelectionText(bold))
			assignJavDatabaseMovieField(&out, label, firstSelectionNode(line), firstSelectionNode(bold))
		}
	})
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
	case labelHasAny(label, []string{"series"}):
		if out.Series == "" {
			out.Series = firstNonEmpty(firstAnchorText(line), collectValueAfterBold(bold))
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

	documentSelection(root).Find("b").Each(func(_ int, bold *goquery.Selection) {
		label := normalizeLabel(cleanSelectionText(bold))
		value := collectValueAfterBold(firstSelectionNode(bold))
		if label != "" && value != "" {
			assignProfileField(&out, label, value)
		}
	})
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
	return firstSelectionNode(documentSelection(root).Find("div.entry-content").First())
}

func findIdolName(root *html.Node) string {
	doc := documentSelection(root)
	if name := cleanSelectionText(doc.Find("h1.idol-name, h2.idol-name, h3.idol-name").First()); name != "" {
		return name
	}
	for _, tag := range []string{"h1", "h2", "h3"} {
		if name := cleanSelectionText(doc.Find(tag).First()); name != "" {
			return name
		}
	}
	return ""
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
	if b == nil {
		return ""
	}
	bold := goquery.NewDocumentFromNode(b).Selection
	var valueBuilder strings.Builder
	afterBold := false
	bold.Parent().Contents().EachWithBreak(func(_ int, sibling *goquery.Selection) bool {
		if sibling.Get(0) == b {
			afterBold = true
			return true
		}
		if !afterBold {
			return true
		}
		if sibling.Is("b, br") {
			return false
		}
		valueBuilder.WriteString(sibling.Text())
		return true
	})

	value := strings.TrimSpace(valueBuilder.String())
	value = strings.TrimLeft(value, "-–: ")
	if idx := strings.Index(value, " - "); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}
	return value
}

func collectAnchorTexts(root *html.Node) []string {
	if root == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var texts []string
	documentSelection(root).Find("a").Each(func(_ int, link *goquery.Selection) {
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
	selection := goquery.NewDocumentFromNode(root).Selection
	if goquery.NodeName(selection) == "a" {
		return cleanSelectionText(selection)
	}
	return cleanSelectionText(selection.Find("a").First())
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
