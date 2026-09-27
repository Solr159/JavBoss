package htmlutil

import (
	"bytes"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

func DocumentSelection(root *html.Node) *goquery.Selection {
	if root == nil {
		return goquery.NewDocumentFromNode(&html.Node{Type: html.DocumentNode}).Selection
	}
	return goquery.NewDocumentFromNode(root).Selection
}

func FirstSelectionNode(selection *goquery.Selection) *html.Node {
	if selection == nil || selection.Length() == 0 {
		return nil
	}
	return selection.Get(0)
}

func CleanSelectionText(selection *goquery.Selection) string {
	if selection == nil {
		return ""
	}
	return strings.Join(strings.Fields(selection.Text()), " ")
}

func ParseHTMLDocument(body []byte) (*html.Node, error) {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return FirstSelectionNode(document.Selection), nil
}

func FirstTextByTag(root *html.Node, tag string) string {
	return CleanSelectionText(DocumentSelection(root).Find(tag).First())
}

func SelectionTexts(selection *goquery.Selection) []string {
	var values []string
	selection.Each(func(_ int, item *goquery.Selection) {
		if text := CleanSelectionText(item); text != "" {
			values = append(values, text)
		}
	})
	return values
}

func SelectionAttr(selection *goquery.Selection, name string) string {
	if selection == nil {
		return ""
	}
	return strings.TrimSpace(selection.AttrOr(name, ""))
}

func CollectAnchorTexts(root *html.Node) []string {
	if root == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var texts []string
	DocumentSelection(root).Find("a").Each(func(_ int, link *goquery.Selection) {
		text := CleanSelectionText(link)
		if text != "" {
			if _, exists := seen[text]; !exists {
				seen[text] = struct{}{}
				texts = append(texts, text)
			}
		}
	})
	return texts
}
