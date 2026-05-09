package jav

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"pornboss/internal/util"
	"regexp"
	"strconv"
	"strings"
	"time"

	"pornboss/internal/common/logging"

	"golang.org/x/net/html"
)

// javBus implements lookupProvider.
type javBus struct{}

var javBusProvider lookupProvider = javBus{}

// LookupActressByJapaneseName implements lookupProvider.
func (javBus) LookupActressByJapaneseName(name string) (*ActressInfo, error) {
	panic("unimplemented")
}

// LookupJavByCode fetches metadata for a given code.
func (javBus) LookupJavByCode(code string) (*JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ResourceNotFonud
	}
	logging.Info("javbus: code -> %s", code)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	info, err := fetchInfo(ctx, code)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}
	if info.Code == "" {
		info.Code = code
	}
	return info, nil
}

// LookupActressByCode resolves a solo movie code to its actress profile.
func (javBus) LookupActressByCode(code string) (*ActressInfo, error) {
	return nil, errors.New("javbus: lookup actress not supported")
}

// LookupCoverURLByCode resolves a cover image URL for a movie code.
func (javBus) LookupCoverURLByCode(code string) (string, error) {
	return "", errors.New("javbus: lookup cover not supported")
}

func fetchInfo(ctx context.Context, code string) (*JavInfo, error) {
	base := "https://www.javbus.com"

	url := fmt.Sprintf("%s/%s", base, code)

	req, err := buildRequest(ctx, url)
	if err != nil {
		return nil, err
	}

	logging.Info("javbus request: %s", url)

	resp, err := util.DoRequest(req)
	// TODO: Should not return here, try curl fallback.
	if err != nil {
		if errors.Is(err, util.ErrCachedNotFound) {
			logging.Info("javbus: cached 404 for %s", url)
			return nil, ResourceNotFonud
		}
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	logging.Info("javbus response status: %s, length: %d bytes", resp.Status, len(body))

	if resp.StatusCode == http.StatusNotFound {
		logging.Info("javbus: %s 404 not found", url)
		return nil, ResourceNotFonud
	}
	if resp.StatusCode != http.StatusOK {
		logging.Info("javbus: non-200 status on %s: %s", url, resp.Status)
		return nil, errors.New("javbus: non-200 response")
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		logging.Error("parse javbus html: %s", err.Error())
		return nil, ResourceNotFonud
	}
	info := parseDocument(doc)
	if info == nil {
		logging.Info("javbus: parseDocument returned nil")
		return nil, ResourceNotFonud
	}
	if info.Code == "" || info.Title == "" {
		logging.Info("javbus: parsed title/code empty (title=%q code=%q)", info.Title, info.Code)
		return nil, ResourceNotFonud
	}
	logging.Info("javbus parsed from %s: title=%q tags=%d actors=%d", url, info.Title, len(info.Tags), len(info.Actors))
	return info, nil
}

func buildRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://www.javbus.com/")
	req.Header.Set("Cookie", "age=verified; existmag=mag")
	return req, nil
}

func parseDocument(doc *html.Node) *JavInfo {
	rawTitle := firstTextByTag(doc, "h3")
	if rawTitle == "" {
		rawTitle = firstTextByTag(doc, "title")
	}
	title := cleanTitle(rawTitle)
	code := extractCode(doc)
	releaseUnix, duration := extractDetails(doc)

	tags := collectGenres(doc)
	actors := collectActors(doc)

	if title == "" && len(tags) == 0 && len(actors) == 0 {
		return nil
	}
	return &JavInfo{
		Title:       title,
		Code:        code,
		ReleaseUnix: releaseUnix,
		DurationMin: duration,
		Tags:        tags,
		Actors:      actors,
		Provider:    ProviderJavBus,
	}
}

func firstTextByTag(root *html.Node, tag string) string {
	var text string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if text != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == tag {
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

func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "- JavBus")
	s = strings.TrimSpace(s)

	// Strip leading code like "CPDE-072" or "CPDE072".
	codePrefix := regexp.MustCompile(`(?i)^[a-z]{2,6}[-_ ]?\d{2,5}\s*`)
	s = codePrefix.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func extractCode(root *html.Node) string {
	var code string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if code != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "span" {
			text := strings.TrimSpace(flattenText(n))
			if strings.Contains(text, "識別碼") || strings.Contains(strings.ToLower(text), "id:") {
				for next := n.NextSibling; next != nil; next = next.NextSibling {
					if next.Type != html.ElementNode {
						continue
					}
					if c := strings.TrimSpace(flattenText(next)); c != "" {
						code = c
						return
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return code
}

func extractDetails(root *html.Node) (releaseUnix int64, durationMin int) {
	dateRe := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	durationRe := regexp.MustCompile(`(\d{1,4})\s*(分鐘|分钟|分|分間|min)?`)

	var releaseStr string

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if releaseStr != "" && durationMin != 0 {
			return
		}
		if n.Type == html.ElementNode && n.Data == "p" {
			text := strings.TrimSpace(flattenText(n))
			lower := strings.ToLower(text)
			if releaseStr == "" && (strings.Contains(text, "發行日期") || strings.Contains(text, "発売日") || strings.Contains(lower, "release")) {
				if m := dateRe.FindString(text); m != "" {
					releaseStr = m
				}
			}
			if durationMin == 0 && (strings.Contains(text, "長度") || strings.Contains(text, "時長") || strings.Contains(text, "時間") || strings.Contains(lower, "length") || strings.Contains(lower, "duration")) {
				if m := durationRe.FindStringSubmatch(text); len(m) >= 2 {
					if v, err := strconv.Atoi(strings.TrimSpace(m[1])); err == nil {
						durationMin = v
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if releaseStr != "" {
		if t, err := time.Parse("2006-01-02", releaseStr); err == nil {
			releaseUnix = t.Unix()
		}
	}
	return releaseUnix, durationMin
}

func collectGenres(root *html.Node) []string {
	section := findMovieSection(root)
	if section == nil {
		section = root
	}

	seen := make(map[string]struct{})
	var out []string
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, inGenre bool) {
		in := inGenre || (n.Type == html.ElementNode && n.Data == "span" && hasClass(n, "genre"))
		if n.Type == html.ElementNode && n.Data == "a" && in {
			// 演员链接也出现在 span.genre 下，过滤掉 /star/ 链接避免混入标签
			if href := attrValue(n, "href"); strings.Contains(href, "/star/") {
				// let collectActors handle it
				return
			}
			if t := strings.TrimSpace(flattenText(n)); t != "" {
				if _, ok := seen[t]; !ok {
					seen[t] = struct{}{}
					out = append(out, t)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, in)
		}
	}
	walk(section, false)
	return out
}

func collectActors(root *html.Node) []string {
	section := findMovieSection(root)
	if section == nil {
		section = root
	}

	seen := make(map[string]struct{})
	var out []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := attrValue(n, "href")
			if strings.Contains(href, "/star/") {
				if t := strings.TrimSpace(flattenText(n)); t != "" {
					if _, ok := seen[t]; !ok {
						seen[t] = struct{}{}
						out = append(out, t)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(section)
	return out
}

func attrValue(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, target string) bool {
	for _, attr := range n.Attr {
		if attr.Key != "class" {
			continue
		}
		classes := strings.Fields(attr.Val)
		for _, c := range classes {
			if c == target {
				return true
			}
		}
	}
	return false
}

// addActor deduplicates actor names preferring the longer variant when one contains the other.
func findMovieSection(root *html.Node) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "movie") && hasClass(n, "row") {
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

func flattenText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if cur.Type == html.TextNode {
			b.WriteString(cur.Data)
		}
		for c := cur.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}
