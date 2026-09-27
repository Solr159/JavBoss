package avshared

import (
	"strings"

	"javboss/internal/jav/internal/htmlutil"
	"javboss/internal/jav/internal/parseutil"

	"golang.org/x/net/html"
)

func CollectGenreTexts(root *html.Node) []string {
	if root == nil {
		return nil
	}

	return parseutil.DedupeNonEmpty(htmlutil.SelectionTexts(htmlutil.DocumentSelection(root).Find("span.genre a")))
}

func ParseCoverURL(root *html.Node, pageURL string) string {
	doc := htmlutil.DocumentSelection(root)
	for _, candidate := range []string{
		htmlutil.SelectionAttr(doc.Find("a.bigImage").First(), "href"),
		htmlutil.SelectionAttr(doc.Find(".screencap img").First(), "src"),
	} {
		if cover := parseutil.ResolveURL(pageURL, candidate); cover != "" {
			return cover
		}
	}
	return ""
}

func NormalizeLabel(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	label = strings.TrimSuffix(label, ":")
	label = strings.TrimSuffix(label, "：")
	return strings.Join(strings.Fields(label), "")
}
