package parseutil

import (
	"strings"

	"javboss/internal/jav/internal/htmlutil"
	"javboss/internal/jav/metadata"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

func ParseSampleImages(root *html.Node, pageURL string) []metadata.SampleImage {
	if root == nil {
		return []metadata.SampleImage{}
	}

	images := make([]metadata.SampleImage, 0)
	seen := make(map[string]struct{})
	htmlutil.DocumentSelection(root).
		Find("#sample-waterfall a, .sample-waterfall a, .image-gallery-section a, .preview-images a.tile-item, a.tile-item[data-fancybox=\"gallery\"]").
		Each(func(_ int, link *goquery.Selection) {
			image := link.Find("img").First()
			thumbnailURL := FirstResolvedSampleURL(
				pageURL,
				htmlutil.SelectionAttr(image, "data-src"),
				htmlutil.SelectionAttr(image, "data-original"),
				htmlutil.SelectionAttr(image, "data-lazy-src"),
				htmlutil.SelectionAttr(image, "src"),
			)
			detailURL := FirstResolvedSampleURL(
				pageURL,
				htmlutil.SelectionAttr(link, "data-image-src"),
				htmlutil.SelectionAttr(link, "data-full"),
				htmlutil.SelectionAttr(link, "data-original"),
				htmlutil.SelectionAttr(link, "href"),
			)
			AppendSampleImage(&images, seen, thumbnailURL, detailURL)
		})
	return images
}

func SampleImagesFromURLs(thumbnailURLs, detailURLs []string, baseURL string) []metadata.SampleImage {
	count := len(thumbnailURLs)
	if len(detailURLs) > count {
		count = len(detailURLs)
	}

	images := make([]metadata.SampleImage, 0, count)
	seen := make(map[string]struct{}, count)
	for i := 0; i < count; i++ {
		var thumbnailURL string
		var detailURL string
		if i < len(thumbnailURLs) {
			thumbnailURL = ResolveSampleImageURL(baseURL, thumbnailURLs[i])
		}
		if i < len(detailURLs) {
			detailURL = ResolveSampleImageURL(baseURL, detailURLs[i])
		}
		AppendSampleImage(&images, seen, thumbnailURL, detailURL)
	}
	return images
}

func AppendSampleImage(images *[]metadata.SampleImage, seen map[string]struct{}, thumbnailURL, detailURL string) {
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
	*images = append(*images, metadata.SampleImage{
		ThumbnailURL: thumbnailURL,
		DetailURL:    detailURL,
	})
}

func FirstResolvedSampleURL(baseURL string, candidates ...string) string {
	for _, candidate := range candidates {
		if resolved := ResolveSampleImageURL(baseURL, candidate); resolved != "" {
			return resolved
		}
	}
	return ""
}

func ResolveSampleImageURL(baseURL, value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if value == "" || value == "#" || strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") {
		return ""
	}
	return ResolveURL(baseURL, value)
}
