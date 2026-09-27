package avsox

import (
	"fmt"
	"net/url"
	"strings"

	"javboss/internal/jav/internal/avshared"
	"javboss/internal/jav/internal/htmlutil"
	"javboss/internal/jav/internal/parseutil"
	"javboss/internal/jav/metadata"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

type avsoxAPIMovie struct {
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
	Studio      *avsoxAPIStudio `json:"studio"`
	Series      *avsoxAPISeries `json:"series"`
	Genre       []avsoxAPIGenre `json:"genre"`
	Star        []avsoxAPIStar  `json:"star"`
}

type avsoxAPIStudio struct {
	StudioName   string `json:"studioName"`
	StudioNameJA string `json:"studioName_ja"`
	StudioNameEN string `json:"studioName_en"`
	StudioNameCN string `json:"studioName_cn"`
	StudioNameTW string `json:"studioName_tw"`
}

type avsoxAPISeries struct {
	SeriesName   string `json:"seriesName"`
	SeriesNameJA string `json:"seriesName_ja"`
	SeriesNameEN string `json:"seriesName_en"`
	SeriesNameCN string `json:"seriesName_cn"`
	SeriesNameTW string `json:"seriesName_tw"`
}

type avsoxAPIGenre struct {
	GenreName   string `json:"genreName"`
	GenreNameJA string `json:"genreName_ja"`
	GenreNameEN string `json:"genreName_en"`
	GenreNameCN string `json:"genreName_cn"`
	GenreNameTW string `json:"genreName_tw"`
}

type avsoxAPIStar struct {
	StarName   string `json:"starName"`
	StarNameJA string `json:"starName_ja"`
	StarNameEN string `json:"starName_en"`
	StarNameCN string `json:"starName_cn"`
	StarNameTW string `json:"starName_tw"`
}

func avsoxMovieDetailURL(movie *avsoxAPIMovie) string {
	if movie == nil {
		return ""
	}
	movieID := strings.TrimSpace(movie.MovieID)
	if movieID == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/movie/%s", avsoxBaseURL, avsoxAPILanguage, url.PathEscape(movieID))
}

func findAvsoxAPISearchResult(results []avsoxAPIMovie, code string) *avsoxAPIMovie {
	wantCode := normalizeAvsoxCodeForCompare(code)
	for i := range results {
		result := &results[i]
		if normalizeAvsoxCodeForCompare(result.MovieFanHao) != wantCode {
			continue
		}
		if strings.TrimSpace(result.MovieID) == "" {
			continue
		}
		return result
	}
	return nil
}

func avsoxMovieInfoFromAPI(movie *avsoxAPIMovie) *metadata.JavInfo {
	if movie == nil {
		return nil
	}
	isUncensored := true
	info := &metadata.JavInfo{
		Title:        parseutil.FirstNonEmpty(movie.Title, movie.TitleCN, movie.TitleTW, movie.TitleJA, movie.TitleEN),
		Code:         strings.TrimSpace(movie.MovieFanHao),
		ReleaseUnix:  parseutil.ParseDateUnix(movie.ReleaseDate),
		DurationMin:  movie.Length,
		CoverURL:     parseutil.FirstNonEmpty(movie.PosterLarge, movie.PosterSmall),
		IsUncensored: &isUncensored,
		Provider:     metadata.ProviderAvsox,
	}
	if movie.Studio != nil {
		info.Studio = parseutil.FirstNonEmpty(movie.Studio.StudioName, movie.Studio.StudioNameCN, movie.Studio.StudioNameTW, movie.Studio.StudioNameJA, movie.Studio.StudioNameEN)
	}
	if movie.Series != nil {
		info.Series = parseutil.FirstNonEmpty(movie.Series.SeriesName, movie.Series.SeriesNameCN, movie.Series.SeriesNameTW, movie.Series.SeriesNameJA, movie.Series.SeriesNameEN)
	}
	for _, genre := range movie.Genre {
		info.Tags = append(info.Tags, parseutil.FirstNonEmpty(genre.GenreName, genre.GenreNameCN, genre.GenreNameTW, genre.GenreNameJA, genre.GenreNameEN))
	}
	for _, star := range movie.Star {
		info.Actors = append(info.Actors, parseutil.FirstNonEmpty(star.StarName, star.StarNameCN, star.StarNameTW, star.StarNameJA, star.StarNameEN))
	}
	info.Tags = parseutil.DedupeNonEmpty(info.Tags)
	info.Actors = parseutil.DedupeNonEmpty(info.Actors)
	if info.Title == "" && info.Code == "" && info.Studio == "" && info.Series == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func findAvsoxSearchResultURL(root *html.Node, code, pageURL string) string {
	wantCode := normalizeAvsoxCodeForCompare(code)
	var result string
	htmlutil.DocumentSelection(root).Find("#waterfall > div.item").EachWithBreak(func(_ int, item *goquery.Selection) bool {
		if normalizeAvsoxCodeForCompare(findAvsoxSearchItemCode(htmlutil.FirstSelectionNode(item))) == wantCode {
			result = parseutil.ResolveURL(pageURL, htmlutil.SelectionAttr(item.Find("a").First(), "href"))
			return false
		}
		return true
	})
	return result
}

func findAvsoxSearchItemCode(item *html.Node) string {
	var code string
	htmlutil.DocumentSelection(item).Find("date").EachWithBreak(func(_ int, date *goquery.Selection) bool {
		text := htmlutil.CleanSelectionText(date)
		if text != "" && !isAvsoxReleaseDate(text) {
			code = text
			return false
		}
		return true
	})
	return code
}

type avsoxMovieFields struct {
	Title       string
	Code        string
	Studio      string
	Series      string
	ReleaseDate string
	Runtime     string
	Tags        []string
	Actors      []string
}

func parseAvsoxMovieInfo(root *html.Node) *metadata.JavInfo {
	scope := findAvsoxMainContainer(root)
	if scope == nil {
		return nil
	}

	fields := extractAvsoxMovieFields(scope)
	title := cleanAvsoxTitle(strings.TrimSpace(fields.Title), fields.Code)
	if title == "" {
		title = cleanAvsoxMoviePageTitle(strings.TrimSpace(htmlutil.FirstTextByTag(root, "title")), fields.Code)
	}
	isUncensored := true

	info := &metadata.JavInfo{
		Title:        title,
		Code:         strings.TrimSpace(fields.Code),
		Studio:       strings.TrimSpace(fields.Studio),
		Series:       strings.TrimSpace(fields.Series),
		ReleaseUnix:  parseutil.ParseDateUnix(fields.ReleaseDate),
		DurationMin:  parseutil.ParseRuntimeMinutes(fields.Runtime),
		Tags:         parseutil.DedupeNonEmpty(fields.Tags),
		Actors:       parseutil.DedupeNonEmpty(fields.Actors),
		IsUncensored: &isUncensored,
		Provider:     metadata.ProviderAvsox,
	}
	if info.Title == "" && info.Code == "" && info.Studio == "" && info.Series == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func findAvsoxMainContainer(root *html.Node) *html.Node {
	var result *html.Node
	htmlutil.DocumentSelection(root).Find("body > div.container").EachWithBreak(func(_ int, container *goquery.Selection) bool {
		if container.Find("div.movie").Length() > 0 {
			result = htmlutil.FirstSelectionNode(container)
			return false
		}
		return true
	})
	return result
}

func extractAvsoxMovieFields(root *html.Node) avsoxMovieFields {
	var out avsoxMovieFields
	if root == nil {
		return out
	}

	doc := htmlutil.DocumentSelection(root)
	out.Title = htmlutil.CleanSelectionText(doc.Find("h3").First())
	if info := htmlutil.FirstSelectionNode(doc.Find("div.info").First()); info != nil {
		out.Tags = avshared.CollectGenreTexts(info)
		extractAvsoxInfoFields(info, &out)
	}
	out.Actors = htmlutil.SelectionTexts(doc.Find("#avatar-waterfall a"))
	return out
}

func extractAvsoxInfoFields(root *html.Node, out *avsoxMovieFields) {
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
			assignAvsoxMovieField(out, label, value)
		case label != "":
			pendingLabel = label
		case pendingLabel != "":
			assignAvsoxMovieField(out, pendingLabel, parseutil.FirstNonEmpty(htmlutil.CleanSelectionText(paragraph.Find("a").First()), htmlutil.CleanSelectionText(paragraph)))
			pendingLabel = ""
		}
	})
}

func assignAvsoxMovieField(out *avsoxMovieFields, label, value string) {
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
	case "發行日期", "发行日期", "發行時間", "发行时间", "発売日", "releasedate":
		if out.ReleaseDate == "" {
			out.ReleaseDate = value
		}
	case "長度", "长度", "時長", "时长", "duration", "runtime":
		if out.Runtime == "" {
			out.Runtime = value
		}
	case "製作商", "制作商", "studio", "メーカー", "メーカー名":
		if out.Studio == "" {
			out.Studio = value
		}
	case "系列", "series":
		if out.Series == "" {
			out.Series = value
		}
	}
}

func parseAvsoxCoverURL(root *html.Node, pageURL string) string {
	return avshared.ParseCoverURL(root, pageURL)
}

func cleanAvsoxMoviePageTitle(title, code string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	title = strings.TrimSuffix(title, "- AVSOX")
	return cleanAvsoxTitle(title, code)
}

func cleanAvsoxTitle(title, code string) string {
	title = strings.TrimSpace(title)
	code = strings.TrimSpace(code)
	if title == "" || code == "" {
		return title
	}
	if len(title) >= len(code) && strings.EqualFold(title[:len(code)], code) {
		return strings.TrimSpace(title[len(code):])
	}
	return title
}

func normalizeAvsoxCodeForCompare(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func isAvsoxReleaseDate(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) == len("2006-01-02") && parseutil.ParseDateUnix(value) != 0
}
