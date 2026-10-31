package update

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

type releaseNotes struct {
	Text string
	HTML string
}

// Read the selected tag's public page, independently of API access or assets.
// Keep both formats for older clients; clients must sanitize HTML before rendering.
func readReleaseNotes(ctx context.Context, client *http.Client, endpoint string) (releaseNotes, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return releaseNotes{}, err
	}
	req.Header.Set("User-Agent", "JavBoss-Updater")
	resp, err := client.Do(req)
	if err != nil {
		return releaseNotes{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return releaseNotes{}, fmt.Errorf("release page: HTTP %d", resp.StatusCode)
	}
	const maxPageSize = 4 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxPageSize+1))
	if err != nil {
		return releaseNotes{}, err
	}
	if len(data) > maxPageSize {
		return releaseNotes{}, fmt.Errorf("release page is too large")
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(data))
	if err != nil {
		return releaseNotes{}, err
	}
	body := doc.Find(`[data-test-selector="body-content"].markdown-body`).First()
	if body.Length() == 0 {
		return releaseNotes{}, fmt.Errorf("release notes not found on release page")
	}
	// Relative GitHub links/images must not resolve against the JavBoss host.
	base, err := url.Parse(endpoint)
	if err != nil {
		return releaseNotes{}, err
	}
	body.Find("[href], [src]").Each(func(_ int, element *goquery.Selection) {
		for _, attr := range []string{"href", "src"} {
			if raw, ok := element.Attr(attr); ok {
				if link, err := url.Parse(raw); err == nil {
					element.SetAttr(attr, base.ResolveReference(link).String())
				} else {
					element.RemoveAttr(attr)
				}
			}
		}
	})
	markup, err := body.Html()
	if err != nil {
		return releaseNotes{}, err
	}
	var text strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
			return
		}
		if node.Type != html.ElementNode {
			return
		}
		switch node.Data {
		case "script", "style", "button", "svg":
			return
		case "br":
			text.WriteByte('\n')
			return
		}
		block := false
		switch node.Data {
		case "div", "p", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "li", "pre", "blockquote", "tr":
			block = true
			text.WriteByte('\n')
		}
		if node.Data == "li" {
			text.WriteString("• ")
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if block {
			text.WriteByte('\n')
		} else if node.Data == "td" || node.Data == "th" {
			text.WriteByte('\t')
		}
	}
	walk(body.Get(0))
	lines := make([]string, 0)
	for _, line := range strings.Split(text.String(), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return releaseNotes{Text: strings.Join(lines, "\n"), HTML: markup}, nil
}
