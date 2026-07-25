package jav

import (
	"bytes"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

func documentSelection(root *html.Node) *goquery.Selection {
	if root == nil {
		return goquery.NewDocumentFromNode(&html.Node{Type: html.DocumentNode}).Selection
	}
	return goquery.NewDocumentFromNode(root).Selection
}

func firstSelectionNode(selection *goquery.Selection) *html.Node {
	if selection == nil || selection.Length() == 0 {
		return nil
	}
	return selection.Get(0)
}

func cleanSelectionText(selection *goquery.Selection) string {
	if selection == nil {
		return ""
	}
	return strings.Join(strings.Fields(selection.Text()), " ")
}

func parseHTMLDocument(body []byte) (*html.Node, error) {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return firstSelectionNode(document.Selection), nil
}
