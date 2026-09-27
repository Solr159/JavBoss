package avmoo

import (
	"regexp"
	"strings"

	"javboss/internal/jav/internal/avshared"
	"javboss/internal/jav/internal/htmlutil"
	"javboss/internal/jav/internal/parseutil"
	"javboss/internal/jav/metadata"
	"javboss/internal/util"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

type avmooAPIMovie struct {
	MovieID     string          `json:"movieId"`
	MovieFanHao string          `json:"movieFanHao"`
	Title       string          `json:"title"`
	TitleJA     string          `json:"title_ja"`
	TitleEN     string          `json:"title_en"`
	TitleCN     string          `json:"title_cn"`
	TitleTW     string          `json:"title_tw"`
	ReleaseDate string          `json:"releaseDate"`
	Length      int             `json:"length"`
	PosterSmall string          `json:"posterSmall"`
	PosterLarge string          `json:"posterLarge"`
	SampleSmall []string        `json:"sampleSmall"`
	SampleLarge []string        `json:"sampleLarge"`
	Studio      *avmooAPIStudio `json:"studio"`
	Series      *avmooAPISeries `json:"series"`
	Genre       []avmooAPIGenre `json:"genre"`
	Star        []avmooAPIStar  `json:"star"`
}

type avmooAPIStudio struct {
	StudioName   string `json:"studioName"`
	StudioNameJA string `json:"studioName_ja"`
	StudioNameEN string `json:"studioName_en"`
	StudioNameCN string `json:"studioName_cn"`
	StudioNameTW string `json:"studioName_tw"`
}

type avmooAPISeries struct {
	SeriesName   string `json:"seriesName"`
	SeriesNameJA string `json:"seriesName_ja"`
	SeriesNameEN string `json:"seriesName_en"`
	SeriesNameCN string `json:"seriesName_cn"`
	SeriesNameTW string `json:"seriesName_tw"`
}

type avmooAPIGenre struct {
	GenreName   string `json:"genreName"`
	GenreNameJA string `json:"genreName_ja"`
	GenreNameEN string `json:"genreName_en"`
	GenreNameCN string `json:"genreName_cn"`
	GenreNameTW string `json:"genreName_tw"`
}

type avmooAPIStar struct {
	StarName   string `json:"starName"`
	StarNameJA string `json:"starName_ja"`
	StarNameEN string `json:"starName_en"`
	StarNameCN string `json:"starName_cn"`
	StarNameTW string `json:"starName_tw"`
}

func findAvmooAPISearchResult(results []avmooAPIMovie, code string) *avmooAPIMovie {
	wantCode := normalizeAvmooCode(code)
	for i := range results {
		result := &results[i]
		if normalizeAvmooCode(result.MovieFanHao) != wantCode {
			continue
		}
		if strings.TrimSpace(result.MovieID) == "" {
			continue
		}
		return result
	}
	return nil
}

func avmooMovieInfoFromAPI(movie *avmooAPIMovie) *metadata.JavInfo {
	if movie == nil {
		return nil
	}
	isUncensored := false
	info := &metadata.JavInfo{
		Title:       parseutil.FirstNonEmpty(movie.Title, movie.TitleTW, movie.TitleCN, movie.TitleJA, movie.TitleEN),
		Code:        strings.TrimSpace(movie.MovieFanHao),
		ReleaseUnix: parseutil.ParseDateUnix(movie.ReleaseDate),
		DurationMin: movie.Length,
		CoverURL:    parseutil.FirstNonEmpty(movie.PosterLarge, movie.PosterSmall),
		SampleImages: parseutil.SampleImagesFromURLs(
			movie.SampleSmall,
			movie.SampleLarge,
			avmooBaseURL,
		),
		IsUncensored: &isUncensored,
		Provider:     metadata.ProviderAvmoo,
	}
	if movie.Series != nil {
		info.Series = parseutil.FirstNonEmpty(movie.Series.SeriesName, movie.Series.SeriesNameTW, movie.Series.SeriesNameCN, movie.Series.SeriesNameJA, movie.Series.SeriesNameEN)
	}
	for _, genre := range movie.Genre {
		info.Tags = append(info.Tags, parseutil.FirstNonEmpty(genre.GenreName, genre.GenreNameTW, genre.GenreNameCN, genre.GenreNameJA, genre.GenreNameEN))
	}
	for _, star := range movie.Star {
		info.Actors = append(info.Actors, parseutil.FirstNonEmpty(star.StarName, star.StarNameTW, star.StarNameCN, star.StarNameJA, star.StarNameEN))
	}
	info.Tags = parseutil.DedupeNonEmpty(info.Tags)
	info.Actors = parseutil.DedupeNonEmpty(info.Actors)
	if info.Title == "" && info.Code == "" && info.Series == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func findAvmooSearchResultURL(root *html.Node, code, pageURL string) string {
	wantCode := normalizeAvmooCode(code)
	var result string
	htmlutil.DocumentSelection(root).Find("#waterfall > div.item").EachWithBreak(func(_ int, item *goquery.Selection) bool {
		if normalizeAvmooCode(htmlutil.CleanSelectionText(item.Find("date").First())) == wantCode {
			result = parseutil.ResolveURL(pageURL, htmlutil.SelectionAttr(item.Find("a").First(), "href"))
			return false
		}
		return true
	})
	return result
}

func findAvmooSearchItemCode(item *html.Node) string {
	text := htmlutil.CleanSelectionText(htmlutil.DocumentSelection(item).Find("date").First())
	if util.CodeRe.MatchString(text) {
		return text
	}
	return ""
}

type avmooMovieFields struct {
	Title       string
	Code        string
	Series      string
	ReleaseDate string
	Runtime     string
	Tags        []string
	Actors      []string
}

func parseAvmooMovieInfo(root *html.Node) *metadata.JavInfo {
	scope := findAvmooMainContainer(root)
	if scope == nil {
		return nil
	}

	fields := extractAvmooMovieFields(scope)
	title := strings.TrimSpace(fields.Title)
	if title == "" {
		title = cleanAvmooMoviePageTitle(strings.TrimSpace(htmlutil.FirstTextByTag(root, "title")))
	}

	isUncensored := false
	info := &metadata.JavInfo{
		Title:        title,
		Code:         strings.TrimSpace(fields.Code),
		Series:       strings.TrimSpace(fields.Series),
		ReleaseUnix:  parseutil.ParseDateUnix(fields.ReleaseDate),
		DurationMin:  parseutil.ParseRuntimeMinutes(fields.Runtime),
		Tags:         parseutil.DedupeNonEmpty(fields.Tags),
		Actors:       parseutil.DedupeNonEmpty(fields.Actors),
		CoverURL:     avshared.ParseCoverURL(root, ""),
		SampleImages: parseutil.ParseSampleImages(root, ""),
		IsUncensored: &isUncensored,
		Provider:     metadata.ProviderAvmoo,
	}
	if info.Title == "" && info.Code == "" && info.Series == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func findAvmooMainContainer(root *html.Node) *html.Node {
	var result *html.Node
	htmlutil.DocumentSelection(root).Find("div.container").EachWithBreak(func(_ int, container *goquery.Selection) bool {
		if container.Find("div.movie").Length() > 0 {
			result = htmlutil.FirstSelectionNode(container)
			return false
		}
		return true
	})
	return result
}

func extractAvmooMovieFields(root *html.Node) avmooMovieFields {
	var out avmooMovieFields
	if root == nil {
		return out
	}

	doc := htmlutil.DocumentSelection(root)
	out.Title = cleanAvmooTitle(htmlutil.CleanSelectionText(doc.Find("h3").First()))
	if info := htmlutil.FirstSelectionNode(doc.Find("div.info").First()); info != nil {
		out.Tags = avshared.CollectGenreTexts(info)
		extractAvmooInfoFields(info, &out)
	}
	out.Actors = htmlutil.SelectionTexts(doc.Find("#avatar-waterfall a"))
	return out
}

func extractAvmooInfoFields(root *html.Node, out *avmooMovieFields) {
	pendingLabel := ""
	htmlutil.DocumentSelection(root).Find("p").Each(func(_ int, paragraph *goquery.Selection) {
		labelSelection := paragraph.Find("span.header").First()
		label := htmlutil.CleanSelectionText(labelSelection)
		value := strings.TrimSpace(strings.TrimPrefix(htmlutil.CleanSelectionText(paragraph), label))
		if paragraph.HasClass("header") {
			label = htmlutil.CleanSelectionText(paragraph)
			value = ""
		}
		switch {
		case label != "" && value != "":
			pendingLabel = ""
			assignAvmooMovieField(out, label, value)
		case label != "":
			pendingLabel = label
		case pendingLabel != "":
			assignAvmooMovieField(out, pendingLabel, parseutil.FirstNonEmpty(htmlutil.CleanSelectionText(paragraph.Find("a").First()), htmlutil.CleanSelectionText(paragraph)))
			pendingLabel = ""
		}
	})
}

func extractAvmooParagraphField(p *html.Node) (string, string) {
	paragraph := goquery.NewDocumentFromNode(p).Selection
	if paragraph.HasClass("header") {
		return htmlutil.CleanSelectionText(paragraph), ""
	}
	label := paragraph.ChildrenFiltered("span.header").First()
	labelText := htmlutil.CleanSelectionText(label)
	return labelText, strings.TrimSpace(strings.TrimPrefix(htmlutil.CleanSelectionText(paragraph), labelText))
}

func assignAvmooMovieField(out *avmooMovieFields, label, value string) {
	if out == nil {
		return
	}
	label = avshared.NormalizeLabel(label)
	value = strings.TrimSpace(value)
	if label == "" || value == "" {
		return
	}

	switch label {
	case "識別碼", "识别码", "番號", "番号":
		if out.Code == "" {
			out.Code = value
		}
	case "發行日期", "发行日期", "発売日", "release date":
		if out.ReleaseDate == "" {
			out.ReleaseDate = value
		}
	case "長度", "长度", "時長", "时长", "duration", "runtime":
		if out.Runtime == "" {
			out.Runtime = value
		}
	case "系列", "series":
		if out.Series == "" {
			out.Series = value
		}
	}
}

func cleanAvmooMoviePageTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	title = strings.TrimSuffix(title, "- AVMOO")
	return cleanAvmooTitle(title)
}

func cleanAvmooTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	re := regexp.MustCompile(`(?i)^[a-z]{2,6}[-_ ]?\d{2,5}[a-z]{0,2}\s+`)
	title = re.ReplaceAllString(title, "")
	return strings.TrimSpace(title)
}

func normalizeAvmooCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	var b strings.Builder
	for _, r := range code {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
