package javdb

import (
	"fmt"
	"net/url"
	"strings"

	"javboss/internal/jav/internal/htmlutil"
	"javboss/internal/jav/internal/parseutil"
	"javboss/internal/jav/metadata"
	"javboss/internal/util"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

func javDBSearchURL(code string) string {
	return fmt.Sprintf("%s/search?q=%s&f=all", javDBBaseURL, url.QueryEscape(strings.TrimSpace(code)))
}

func javDBActorSearchURL(name string) string {
	return fmt.Sprintf("%s/search?q=%s&f=actor", javDBBaseURL, url.QueryEscape(strings.TrimSpace(name)))
}

func findJavDBSearchResultURL(root *html.Node, code, pageURL string) string {
	urls := findJavDBSearchResultURLs(root, code, pageURL)
	if len(urls) == 0 {
		return ""
	}
	return urls[0]
}

func findSingleJavDBSearchResultURL(root *html.Node, code, pageURL string) string {
	urls := findJavDBSearchResultURLs(root, code, pageURL)
	if len(urls) != 1 {
		return ""
	}
	return urls[0]
}

func findJavDBSearchResultURLs(root *html.Node, code, pageURL string) []string {
	wantCode := normalizeJavDBCode(code)
	seen := make(map[string]struct{})
	var urls []string
	htmlutil.DocumentSelection(root).Find("div.movie-list.h.cols-4.vcols-8 > div.item").Each(func(_ int, item *goquery.Selection) {
		itemCode := htmlutil.CleanSelectionText(item.Find("div.video-title strong").First())
		if normalizeJavDBCode(itemCode) != wantCode {
			return
		}
		if href := htmlutil.SelectionAttr(item.Find("a").First(), "href"); href != "" {
			detailURL := parseutil.ResolveURL(pageURL, href)
			if detailURL == "" {
				return
			}
			if _, ok := seen[detailURL]; ok {
				return
			}
			seen[detailURL] = struct{}{}
			urls = append(urls, detailURL)
		}
	})
	return urls
}

type javDBMovieFields struct {
	Title       string
	Code        string
	ReleaseDate string
	Runtime     string
	Director    string
	Maker       string
	Publisher   string
	Series      string
	Rating      string
	Tags        []string
	Actors      []string
}

func parseJavDBMovieInfo(root *html.Node) *metadata.JavInfo {
	fields := extractJavDBMovieFields(root)
	title := strings.TrimSpace(fields.Title)
	if title == "" {
		title = cleanJavDBMoviePageTitle(strings.TrimSpace(htmlutil.FirstTextByTag(root, "title")))
	}

	info := &metadata.JavInfo{
		Title:        title,
		Code:         strings.TrimSpace(fields.Code),
		Studio:       strings.TrimSpace(fields.Maker),
		Series:       strings.TrimSpace(fields.Series),
		ReleaseUnix:  parseutil.ParseDateUnix(fields.ReleaseDate),
		DurationMin:  parseutil.ParseRuntimeMinutes(fields.Runtime),
		Tags:         parseutil.DedupeNonEmpty(fields.Tags),
		Actors:       parseutil.DedupeNonEmpty(fields.Actors),
		CoverURL:     parseJavDBCoverURL(root, ""),
		SampleImages: parseutil.ParseSampleImages(root, ""),
		Provider:     metadata.ProviderJavDB,
	}
	if info.Title == "" && info.Code == "" && info.Studio == "" && info.Series == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func extractJavDBMovieFields(root *html.Node) javDBMovieFields {
	var out javDBMovieFields
	if root == nil {
		return out
	}

	if title := findJavDBMovieTitle(root); title != "" {
		out.Title = title
	}

	panel := findJavDBMovieInfoPanel(root)
	if panel == nil {
		return out
	}

	htmlutil.DocumentSelection(panel).Find("div.panel-block").Each(func(_ int, block *goquery.Selection) {
		strong := block.ChildrenFiltered("strong").First()
		if strong.Length() > 0 {
			label := normalizeJavDBLabel(htmlutil.CleanSelectionText(strong))
			assignJavDBMovieField(&out, label, htmlutil.FirstSelectionNode(block), htmlutil.FirstSelectionNode(strong))
		}
	})
	return out
}

func findJavDBMovieInfoPanel(root *html.Node) *html.Node {
	return htmlutil.FirstSelectionNode(htmlutil.DocumentSelection(root).Find("nav.panel.movie-panel-info").First())
}

func findJavDBMovieTitle(root *html.Node) string {
	if title := htmlutil.CleanSelectionText(htmlutil.DocumentSelection(root).Find("span.origin-title").First()); title != "" {
		return title
	}
	return htmlutil.CleanSelectionText(htmlutil.DocumentSelection(root).Find("strong.current-title").First())
}

func assignJavDBMovieField(out *javDBMovieFields, label string, block, strong *html.Node) {
	if out == nil {
		return
	}
	label = normalizeJavDBLabel(label)
	if label == "" {
		return
	}

	value := collectJavDBValue(block, strong)
	switch label {
	case "番号", "番號":
		if out.Code == "" {
			out.Code = value
		}
	case "日期":
		if out.ReleaseDate == "" {
			out.ReleaseDate = value
		}
	case "时长", "時長":
		if out.Runtime == "" {
			out.Runtime = value
		}
	case "导演", "導演":
		if out.Director == "" {
			out.Director = value
		}
	case "片商":
		if out.Maker == "" {
			out.Maker = value
		}
	case "发行", "發行":
		if out.Publisher == "" {
			out.Publisher = value
		}
	case "系列":
		if out.Series == "" {
			out.Series = value
		}
	case "评分", "評分":
		if out.Rating == "" {
			out.Rating = value
		}
	case "类别", "類別":
		if len(out.Tags) == 0 {
			out.Tags = htmlutil.CollectAnchorTexts(block)
		}
	case "演员", "演員":
		if len(out.Actors) == 0 {
			out.Actors = collectJavDBActorTexts(block)
		}
	}
}

func collectJavDBActorTexts(root *html.Node) []string {
	if root == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var texts []string
	htmlutil.DocumentSelection(root).Find("a").Each(func(_ int, link *goquery.Selection) {
		if isJavDBMaleActorLink(htmlutil.FirstSelectionNode(link)) {
			return
		}
		text := htmlutil.CleanSelectionText(link)
		if text != "" {
			if _, exists := seen[text]; !exists {
				seen[text] = struct{}{}
				texts = append(texts, text)
			}
		}
	})
	return texts
}

func parseJavDBActressURLByName(root *html.Node, name, pageURL string) string {
	name = strings.TrimSpace(name)
	if root == nil {
		return ""
	}

	actors := collectJavDBActressLinks(root, pageURL)
	if len(actors) == 1 {
		return actors[0].href
	}
	if name == "" {
		return ""
	}
	for _, actor := range actors {
		if actor.text == name {
			return actor.href
		}
	}
	return ""
}

type javDBActressLink struct {
	text string
	href string
}

func collectJavDBActressLinks(root *html.Node, pageURL string) []javDBActressLink {
	if root == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var links []javDBActressLink
	htmlutil.DocumentSelection(root).Find("a").Each(func(_ int, link *goquery.Selection) {
		node := htmlutil.FirstSelectionNode(link)
		if isJavDBMaleActorLink(node) {
			return
		}
		text := htmlutil.CleanSelectionText(link)
		href := htmlutil.SelectionAttr(link, "href")
		if text != "" && isJavDBActorURL(href) {
			actorURL := parseutil.ResolveURL(pageURL, href)
			key := text + "\x00" + actorURL
			if actorURL != "" {
				if _, exists := seen[key]; !exists {
					seen[key] = struct{}{}
					links = append(links, javDBActressLink{text: text, href: actorURL})
				}
			}
		}
	})
	return links
}

func findJavDBActorSearchResultURLs(root *html.Node, name, pageURL string) []string {
	name = strings.TrimSpace(name)
	if root == nil || name == "" {
		return nil
	}

	seen := make(map[string]struct{})
	var urls []string
	htmlutil.DocumentSelection(root).Find("#actors .actor-box, .actors .actor-box").Each(func(_ int, box *goquery.Selection) {
		if href := javDBActorBoxExactHref(htmlutil.FirstSelectionNode(box), name); href != "" {
			actorURL := parseutil.ResolveURL(pageURL, href)
			if actorURL != "" {
				if _, exists := seen[actorURL]; !exists {
					seen[actorURL] = struct{}{}
					urls = append(urls, actorURL)
				}
			}
		}
	})
	return urls
}

func javDBActorBoxExactHref(root *html.Node, name string) string {
	if root == nil {
		return ""
	}
	name = strings.TrimSpace(name)
	var href string
	htmlutil.DocumentSelection(root).Find("a").EachWithBreak(func(_ int, link *goquery.Selection) bool {
		candidateHref := htmlutil.SelectionAttr(link, "href")
		if candidateHref != "" && isJavDBActorURL(candidateHref) && javDBActorAnchorMatchesName(htmlutil.FirstSelectionNode(link), name) {
			href = candidateHref
			return false
		}
		return true
	})
	return href
}

func javDBActorAnchorMatchesName(anchor *html.Node, name string) bool {
	if anchor == nil || name == "" {
		return false
	}
	selection := goquery.NewDocumentFromNode(anchor).Selection
	if javDBActorTitleHasName(htmlutil.SelectionAttr(selection, "title"), name) {
		return true
	}
	return strings.TrimSpace(firstDescendantTextByTag(anchor, "strong")) == name
}

func javDBActorTitleHasName(title, name string) bool {
	name = strings.TrimSpace(name)
	title = strings.TrimSpace(title)
	if title == "" || name == "" {
		return false
	}
	for _, alias := range strings.FieldsFunc(title, func(r rune) bool {
		return r == ',' || r == '，' || r == '、'
	}) {
		if strings.TrimSpace(alias) == name {
			return true
		}
	}
	return false
}

func parseJavDBSeriesURL(root *html.Node, pageURL string) string {
	return parseJavDBPanelURL(root, pageURL, "系列", isJavDBSeriesURL)
}

func parseJavDBStudioURL(root *html.Node, pageURL string) string {
	return parseJavDBPanelURL(root, pageURL, "片商", isJavDBStudioURL)
}

func parseJavDBPanelURL(root *html.Node, pageURL, wantLabel string, matchHref func(string) bool) string {
	if root == nil {
		return ""
	}

	var matchedURL string
	htmlutil.DocumentSelection(root).Find("div.panel-block").EachWithBreak(func(_ int, block *goquery.Selection) bool {
		if normalizeJavDBLabel(htmlutil.CleanSelectionText(block.ChildrenFiltered("strong").First())) != wantLabel {
			return true
		}
		block.Find("a").EachWithBreak(func(_ int, link *goquery.Selection) bool {
			href := htmlutil.SelectionAttr(link, "href")
			if matchHref(href) {
				matchedURL = parseutil.ResolveURL(pageURL, href)
				return false
			}
			return true
		})
		return matchedURL == ""
	})
	return matchedURL
}

func isJavDBActorURL(href string) bool {
	href = strings.ToLower(strings.TrimSpace(href))
	return strings.Contains(href, "/actors/")
}

func isJavDBSeriesURL(href string) bool {
	href = strings.ToLower(strings.TrimSpace(href))
	return strings.Contains(href, "/series/")
}

func isJavDBStudioURL(href string) bool {
	href = strings.ToLower(strings.TrimSpace(href))
	return strings.Contains(href, "/makers/")
}

func isJavDBMaleActorLink(anchor *html.Node) bool {
	if anchor == nil {
		return false
	}
	next := goquery.NewDocumentFromNode(anchor).Selection.NextAll().First()
	return next.Is("strong.symbol.male") || (goquery.NodeName(next) == "strong" && htmlutil.CleanSelectionText(next) == "♂")
}

func normalizeJavDBLabel(label string) string {
	label = strings.TrimSpace(label)
	label = strings.TrimSuffix(label, ":")
	label = strings.TrimSuffix(label, "：")
	return strings.Join(strings.Fields(label), "")
}

func collectJavDBValue(block, strong *html.Node) string {
	blockSelection := goquery.NewDocumentFromNode(block).Selection
	if value := htmlutil.CleanSelectionText(blockSelection.Find("span.value").First()); value != "" {
		return cleanJavDBValue(value)
	}
	if strong != nil {
		strongText := htmlutil.CleanSelectionText(goquery.NewDocumentFromNode(strong).Selection)
		return cleanJavDBValue(strings.TrimPrefix(htmlutil.CleanSelectionText(blockSelection), strongText))
	}
	return cleanJavDBValue(htmlutil.CleanSelectionText(blockSelection))
}

func firstDescendantTextByTag(root *html.Node, tag string) string {
	if root == nil || tag == "" {
		return ""
	}
	return htmlutil.CleanSelectionText(htmlutil.DocumentSelection(root).Find(tag).First())
}

func cleanJavDBValue(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = strings.Join(strings.Fields(value), " ")
	value = strings.TrimSpace(value)
	value = strings.TrimLeft(value, ":： ")
	return value
}

func parseJavDBCoverURL(root *html.Node, pageURL string) string {
	doc := htmlutil.DocumentSelection(root)
	for _, candidate := range []string{
		htmlutil.SelectionAttr(doc.Find(`meta[property="og:image"]`).First(), "content"),
		htmlutil.SelectionAttr(doc.Find("img.video-cover").First(), "src"),
	} {
		if cover := parseutil.ResolveURL(pageURL, candidate); cover != "" {
			return cover
		}
	}
	return ""
}

func cleanJavDBMoviePageTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	for _, suffix := range []string{"| JavDB 成人影片數據庫", "| JavDB"} {
		title = strings.TrimSpace(strings.TrimSuffix(title, suffix))
	}
	if idx := strings.Index(title, " "); idx > 0 && util.CodeRe.MatchString(title[:idx]) {
		title = strings.TrimSpace(title[idx+1:])
	}
	return title
}

func normalizeJavDBCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	var b strings.Builder
	for _, r := range code {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
