package jav

import (
	"encoding/json"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestParseSampleImages(t *testing.T) {
	tests := []struct {
		name     string
		pageURL  string
		html     string
		expected []SampleImage
	}{
		{
			name:    "javbus sample waterfall",
			pageURL: "https://www.javbus.com/IPX-228",
			html: `
				<div id="sample-waterfall">
					<a class="sample-box" href="/pics/sample/ipx228jp-1.jpg">
						<img src="/pics/sample/ipx228-1.jpg">
					</a>
					<a class="sample-box" href="//pics.dmm.co.jp/ipx228jp-2.jpg">
						<img data-src="//pics.dmm.co.jp/ipx228-2.jpg" src="data:image/gif;base64,placeholder">
					</a>
				</div>`,
			expected: []SampleImage{
				{
					ThumbnailURL: "https://www.javbus.com/pics/sample/ipx228-1.jpg",
					DetailURL:    "https://www.javbus.com/pics/sample/ipx228jp-1.jpg",
				},
				{
					ThumbnailURL: "https://pics.dmm.co.jp/ipx228-2.jpg",
					DetailURL:    "https://pics.dmm.co.jp/ipx228jp-2.jpg",
				},
			},
		},
		{
			name:    "javdatabase image gallery",
			pageURL: "https://www.javdatabase.com/movies/IPX-228/",
			html: `
				<div class="image-gallery-section">
					<a href="#" data-image-src="https://pics.dmm.co.jp/ipx228jp-1.jpg">
						<img src="https://pics.dmm.co.jp/ipx228-1.jpg">
					</a>
				</div>`,
			expected: []SampleImage{
				{
					ThumbnailURL: "https://pics.dmm.co.jp/ipx228-1.jpg",
					DetailURL:    "https://pics.dmm.co.jp/ipx228jp-1.jpg",
				},
			},
		},
		{
			name:    "javdb preview images",
			pageURL: "https://javdb.com/v/abc123",
			html: `
				<div class="tile-images preview-images">
					<a class="tile-item" href="https://pics.dmm.co.jp/ipx228jp-1.jpg">
						<img src="https://c0.jdbstatic.com/thumbs/ipx228-1.jpg">
					</a>
					<a class="tile-item" href="/samples/ipx228jp-2.jpg">
						<img data-src="/samples/ipx228-2.jpg" src="data:image/gif;base64,placeholder">
					</a>
				</div>`,
			expected: []SampleImage{
				{
					ThumbnailURL: "https://c0.jdbstatic.com/thumbs/ipx228-1.jpg",
					DetailURL:    "https://pics.dmm.co.jp/ipx228jp-1.jpg",
				},
				{
					ThumbnailURL: "https://javdb.com/samples/ipx228-2.jpg",
					DetailURL:    "https://javdb.com/samples/ipx228jp-2.jpg",
				},
			},
		},
		{
			name:    "javmenu fancybox gallery",
			pageURL: "https://javmenu.com/IPX-228",
			html: `
				<div class="d-flex flex-wrap">
					<a class="tile-item" href="https://c0.jdbstatic.com/samples/kk/kKdRm_l_0.jpg" data-fancybox="gallery">
						<img src="/images/loading.gif" data-src="https://c0.jdbstatic.com/samples/kk/kKdRm_s_0.jpg">
					</a>
					<a class="tile-item" href="/samples/ipx228_l_1.jpg" data-fancybox="gallery">
						<img src="/samples/ipx228_s_1.jpg">
					</a>
				</div>
				<a class="tile-item" href="/recommendation.jpg">
					<img src="/recommendation-thumb.jpg">
				</a>`,
			expected: []SampleImage{
				{
					ThumbnailURL: "https://c0.jdbstatic.com/samples/kk/kKdRm_s_0.jpg",
					DetailURL:    "https://c0.jdbstatic.com/samples/kk/kKdRm_l_0.jpg",
				},
				{
					ThumbnailURL: "https://javmenu.com/samples/ipx228_s_1.jpg",
					DetailURL:    "https://javmenu.com/samples/ipx228_l_1.jpg",
				},
			},
		},
		{
			name:    "ignores images outside gallery",
			pageURL: "https://example.com/movie/ABC-001",
			html: `
				<a href="/cover-large.jpg"><img src="/cover-small.jpg"></a>
				<div id="sample-waterfall"></div>`,
			expected: []SampleImage{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := html.Parse(strings.NewReader(test.html))
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}

			got := parseSampleImages(doc, test.pageURL)
			if len(got) != len(test.expected) {
				t.Fatalf("sample image count = %d, want %d: %#v", len(got), len(test.expected), got)
			}
			for i := range test.expected {
				if got[i] != test.expected[i] {
					t.Fatalf("sample image %d = %#v, want %#v", i, got[i], test.expected[i])
				}
			}
		})
	}
}

func TestSampleImagesFromURLsPairsAvmooSamples(t *testing.T) {
	got := sampleImagesFromURLs(
		[]string{
			"https://jp.netcdn.space/ipx00228-1.jpg",
			"https://jp.netcdn.space/ipx00228-2.jpg",
		},
		[]string{
			"https://jp.netcdn.space/ipx00228jp-1.jpg",
			"https://jp.netcdn.space/ipx00228jp-2.jpg",
		},
		avmooBaseURL,
	)

	if len(got) != 2 {
		t.Fatalf("sample image count = %d, want 2", len(got))
	}
	if got[0].ThumbnailURL != "https://jp.netcdn.space/ipx00228-1.jpg" {
		t.Fatalf("unexpected thumbnail url: %q", got[0].ThumbnailURL)
	}
	if got[0].DetailURL != "https://jp.netcdn.space/ipx00228jp-1.jpg" {
		t.Fatalf("unexpected detail url: %q", got[0].DetailURL)
	}
}

func TestSampleImageJSONFieldNames(t *testing.T) {
	raw, err := json.Marshal(SampleImage{
		ThumbnailURL: "thumbnail",
		DetailURL:    "detail",
	})
	if err != nil {
		t.Fatalf("marshal sample image: %v", err)
	}
	if string(raw) != `{"thumbnail_url":"thumbnail","detail_url":"detail"}` {
		t.Fatalf("unexpected json: %s", raw)
	}
}
