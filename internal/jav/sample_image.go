package jav

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// SampleImage contains the thumbnail and full-size URLs for a movie sample image.
type SampleImage struct {
	ThumbnailURL string `json:"thumbnail_url"`
	DetailURL    string `json:"detail_url"`
}

func parseSampleImages(root *html.Node, pageURL string) []SampleImage {
	if root == nil {
		return []SampleImage{}
	}

	images := make([]SampleImage, 0)
	seen := make(map[string]struct{})
	documentSelection(root).
		Find("#sample-waterfall a, .sample-waterfall a, .image-gallery-section a, .preview-images a.tile-item, a.tile-item[data-fancybox=\"gallery\"]").
		Each(func(_ int, link *goquery.Selection) {
			image := link.Find("img").First()
			thumbnailURL := firstResolvedSampleURL(
				pageURL,
				selectionAttr(image, "data-src"),
				selectionAttr(image, "data-original"),
				selectionAttr(image, "data-lazy-src"),
				selectionAttr(image, "src"),
			)
			detailURL := firstResolvedSampleURL(
				pageURL,
				selectionAttr(link, "data-image-src"),
				selectionAttr(link, "data-full"),
				selectionAttr(link, "data-original"),
				selectionAttr(link, "href"),
			)
			appendSampleImage(&images, seen, thumbnailURL, detailURL)
		})
	return images
}

func sampleImagesFromURLs(thumbnailURLs, detailURLs []string, baseURL string) []SampleImage {
	count := len(thumbnailURLs)
	if len(detailURLs) > count {
		count = len(detailURLs)
	}

	images := make([]SampleImage, 0, count)
	seen := make(map[string]struct{}, count)
	for i := 0; i < count; i++ {
		var thumbnailURL string
		var detailURL string
		if i < len(thumbnailURLs) {
			thumbnailURL = resolveSampleImageURL(baseURL, thumbnailURLs[i])
		}
		if i < len(detailURLs) {
			detailURL = resolveSampleImageURL(baseURL, detailURLs[i])
		}
		appendSampleImage(&images, seen, thumbnailURL, detailURL)
	}
	return images
}

func appendSampleImage(images *[]SampleImage, seen map[string]struct{}, thumbnailURL, detailURL string) {
	if thumbnailURL == "" {
		thumbnailURL = detailURL
	}
	if detailURL == "" {
		detailURL = thumbnailURL
	}
	if thumbnailURL == "" {
		return
	}

	key := thumbnailURL + "\x00" + detailURL
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	*images = append(*images, SampleImage{
		ThumbnailURL: thumbnailURL,
		DetailURL:    detailURL,
	})
}

func firstResolvedSampleURL(baseURL string, candidates ...string) string {
	for _, candidate := range candidates {
		if resolved := resolveSampleImageURL(baseURL, candidate); resolved != "" {
			return resolved
		}
	}
	return ""
}

func resolveSampleImageURL(baseURL, value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if value == "" || value == "#" || strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") {
		return ""
	}
	return resolveURL(baseURL, value)
}

func selectionAttr(selection *goquery.Selection, name string) string {
	if selection == nil {
		return ""
	}
	return strings.TrimSpace(selection.AttrOr(name, ""))
}
