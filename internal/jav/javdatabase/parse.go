package javdatabase

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"javboss/internal/jav/internal/htmlutil"
	"javboss/internal/jav/internal/parseutil"
	"javboss/internal/jav/metadata"
	"javboss/internal/util"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

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
	htmlutil.DocumentSelection(root).Find("a").Each(func(_ int, link *goquery.Selection) {
		href := htmlutil.SelectionAttr(link, "href")
		if href != "" && looksLikeActressURL(href) {
			if _, exists := seen[href]; !exists {
				seen[href] = struct{}{}
				candidates = append(candidates, actressLinkCandidate{
					href:  href,
					score: scoreActressLink(htmlutil.FirstSelectionNode(link), href),
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
	htmlutil.DocumentSelection(root).Find("p.mb-1").EachWithBreak(func(_ int, paragraph *goquery.Selection) bool {
		node := htmlutil.FirstSelectionNode(paragraph)
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
	bold := htmlutil.DocumentSelection(n).ChildrenFiltered("b").First()
	if bold.Length() == 0 {
		return false
	}
	label := htmlutil.CleanSelectionText(bold)
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
	htmlutil.DocumentSelection(n).Find("a").Each(func(_ int, link *goquery.Selection) {
		href := htmlutil.SelectionAttr(link, "href")
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
	text := htmlutil.CleanSelectionText(goquery.NewDocumentFromNode(n).Selection)
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
		text := strings.ToLower(htmlutil.CleanSelectionText(parent))
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

func parseJavDatabaseActressInfo(root *html.Node) *metadata.ActressInfo {
	scope := findEntryContent(root)
	if scope == nil {
		scope = root
	}

	roman := strings.TrimSpace(findIdolName(scope))
	japanese := ""
	if parseutil.ContainsJapaneseRunes(roman) {
		japanese = roman
		roman = ""
	}
	roman = cleanIdolName(roman)
	if roman == "" {
		roman = cleanJavDatabaseTitle(strings.TrimSpace(htmlutil.FirstTextByTag(scope, "title")))
	}

	fields := extractJavDatabaseProfileFields(scope)
	if japanese == "" {
		japanese = guessJapaneseName(scope, roman)
	}
	height := parseutil.ParseHeightCM(fields.Height)
	bust, waist, hips := parseMeasurements(fields.Measurements)
	birthDate := parseutil.ParseBirthDateUnix(fields.BirthDate)
	info := &metadata.ActressInfo{
		RomanName:    roman,
		JapaneseName: cleanJapaneseName(parseutil.FirstNonEmpty(fields.JapaneseName, japanese)),
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

func parseJavDatabaseMovieInfo(root *html.Node) *metadata.JavInfo {
	scope := findJavDatabaseMovieInfoColumn(root)
	if scope == nil {
		return nil
	}

	fields := extractJavDatabaseMovieFields(scope)
	title := strings.TrimSpace(fields.Title)
	if title == "" {
		title = cleanJavDatabaseMoviePageTitle(strings.TrimSpace(htmlutil.FirstTextByTag(root, "title")))
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
		CoverURL:     parseJavDatabaseCoverURL(root, ""),
		SampleImages: parseutil.ParseSampleImages(root, ""),
		Provider:     metadata.ProviderJavDatabase,
	}
	if info.Title == "" && info.Code == "" && info.Studio == "" && info.Series == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func parseJavDatabaseCoverURL(root *html.Node, pageURL string) string {
	doc := htmlutil.DocumentSelection(root)
	for _, candidate := range []string{
		htmlutil.SelectionAttr(doc.Find(`meta[property="og:image"]`).First(), "content"),
		htmlutil.SelectionAttr(doc.Find("img.poster, img.cover").First(), "src"),
	} {
		if cover := parseutil.ResolveURL(pageURL, candidate); cover != "" {
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
	doc := htmlutil.DocumentSelection(root)
	selector := "div.col-md-10.col-lg-10.col-xxl-10.col-8"
	if column := doc.Find("div.movietable " + selector).First(); column.Length() > 0 {
		return htmlutil.FirstSelectionNode(column)
	}
	return htmlutil.FirstSelectionNode(doc.Find(selector).First())
}

func findDescendantMovieInfoColumn(root *html.Node) *html.Node {
	return htmlutil.FirstSelectionNode(htmlutil.DocumentSelection(root).
		Find("div.col-md-10.col-lg-10.col-xxl-10.col-8").
		First())
}

func extractJavDatabaseMovieFields(root *html.Node) javDatabaseMovieFields {
	var out javDatabaseMovieFields
	if root == nil {
		return out
	}

	htmlutil.DocumentSelection(root).Find("p.mb-1").Each(func(_ int, line *goquery.Selection) {
		bold := line.ChildrenFiltered("b").First()
		if bold.Length() > 0 {
			label := normalizeLabel(htmlutil.CleanSelectionText(bold))
			assignJavDatabaseMovieField(&out, label, htmlutil.FirstSelectionNode(line), htmlutil.FirstSelectionNode(bold))
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
			out.Studio = parseutil.FirstNonEmpty(firstAnchorText(line), collectValueAfterBold(bold))
		}
	case labelHasAny(label, []string{"series"}):
		if out.Series == "" {
			out.Series = parseutil.FirstNonEmpty(firstAnchorText(line), collectValueAfterBold(bold))
		}
	case labelHasAny(label, []string{"genre", "genres"}):
		if len(out.Tags) == 0 {
			out.Tags = htmlutil.CollectAnchorTexts(line)
		}
	case labelHasAny(label, []string{"idol actress", "idol s actress es", "actress", "actresses", "idol", "idols"}):
		if len(out.Actors) == 0 {
			out.Actors = htmlutil.CollectAnchorTexts(line)
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

	htmlutil.DocumentSelection(root).Find("b").Each(func(_ int, bold *goquery.Selection) {
		label := normalizeLabel(htmlutil.CleanSelectionText(bold))
		value := collectValueAfterBold(htmlutil.FirstSelectionNode(bold))
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

func extractCupFromMeasurements(measurements string) int {
	measurements = strings.TrimSpace(measurements)
	if measurements == "" {
		return 0
	}
	re := regexp.MustCompile(`(?i)\b([A-K])\s*cup\b`)
	if match := re.FindStringSubmatch(measurements); len(match) > 1 {
		return parseutil.CupLetterToNumber(match[1])
	}
	re = regexp.MustCompile(`(?i)\bcup\s*([A-K])\b`)
	if match := re.FindStringSubmatch(measurements); len(match) > 1 {
		return parseutil.CupLetterToNumber(match[1])
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
		return parseutil.CupLetterToNumber(match[1])
	}
	return 0
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
	return htmlutil.FirstSelectionNode(htmlutil.DocumentSelection(root).Find("div.entry-content").First())
}

func findIdolName(root *html.Node) string {
	doc := htmlutil.DocumentSelection(root)
	if name := htmlutil.CleanSelectionText(doc.Find("h1.idol-name, h2.idol-name, h3.idol-name").First()); name != "" {
		return name
	}
	for _, tag := range []string{"h1", "h2", "h3"} {
		if name := htmlutil.CleanSelectionText(doc.Find(tag).First()); name != "" {
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

func firstAnchorText(root *html.Node) string {
	if root == nil {
		return ""
	}
	selection := goquery.NewDocumentFromNode(root).Selection
	if goquery.NodeName(selection) == "a" {
		return htmlutil.CleanSelectionText(selection)
	}
	return htmlutil.CleanSelectionText(selection.Find("a").First())
}

func guessJapaneseName(root *html.Node, roman string) string {
	for _, tag := range []string{"h2", "h3", "h1"} {
		text := strings.TrimSpace(htmlutil.FirstTextByTag(root, tag))
		if text == "" || text == roman {
			continue
		}
		if parseutil.ContainsJapaneseRunes(text) {
			return text
		}
	}
	return ""
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
