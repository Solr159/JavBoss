package jav

import (
	"errors"
	"net/url"
	"testing"
	"time"
)

func TestBuildMinnanoAVActressSearchURL(t *testing.T) {
	got, err := buildMinnanoAVActressSearchURL("https://www.minnano-av.com", " 安堂なな ")
	if err != nil {
		t.Fatalf("buildMinnanoAVActressSearchURL: %v", err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse search URL: %v", err)
	}
	if parsed.Path != "/search_result.php" {
		t.Fatalf("path = %q, want /search_result.php", parsed.Path)
	}
	if got := parsed.Query().Get("search_scope"); got != "actress" {
		t.Fatalf("search_scope = %q, want actress", got)
	}
	if got := parsed.Query().Get("search_word"); got != "安堂なな" {
		t.Fatalf("search_word = %q, want 安堂なな", got)
	}
	if got := parsed.Query().Get("search"); got != "Go" {
		t.Fatalf("search = %q, want Go", got)
	}
}

func TestParseMinnanoAVActressInfo(t *testing.T) {
	doc, err := parseHTMLDocument([]byte(`<!doctype html>
<html><body>
<h1>安堂なな<span>あんどうなな / Ando Nana</span></h1>
<div class="act-profile"><table>
<tr><td><span>生年月日</span><p>1995年06月18日 （現在 31歳）ふたご座</p></td></tr>
<tr><td><span>サイズ</span><p>T145 / B81(<a>Dカップ</a>) / W61 / H83 / S</p></td></tr>
</table></div>
</body></html>`))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	info := parseMinnanoAVActressInfo(doc)
	if info == nil {
		t.Fatal("parseMinnanoAVActressInfo returned nil")
	}
	if info.JapaneseName != "安堂なな" {
		t.Fatalf("JapaneseName = %q, want 安堂なな", info.JapaneseName)
	}
	if info.RomanName != "Nana Ando" {
		t.Fatalf("RomanName = %q, want Nana Ando", info.RomanName)
	}
	if info.HeightCM != 145 || info.Bust != 81 || info.Waist != 61 || info.Hips != 83 {
		t.Fatalf("measurements = T%d B%d W%d H%d", info.HeightCM, info.Bust, info.Waist, info.Hips)
	}
	if info.Cup != 4 {
		t.Fatalf("Cup = %d, want 4", info.Cup)
	}
	wantBirthDate := int(time.Date(1995, time.June, 18, 0, 0, 0, 0, time.UTC).Unix())
	if info.BirthDate != wantBirthDate {
		t.Fatalf("BirthDate = %d, want %d", info.BirthDate, wantBirthDate)
	}
}

func TestParseMinnanoAVRomanNameUsesGivenNameFirst(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{value: "ながせゆい / Nagase Yui", want: "Yui Nagase"},
		{value: "あんどうなな / Ando Nana", want: "Nana Ando"},
		{value: "りおん / RION", want: "RION"},
	}

	for _, test := range tests {
		if got := parseMinnanoAVRomanName(test.value); got != test.want {
			t.Errorf("parseMinnanoAVRomanName(%q) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestFindMinnanoAVActressSearchResultURL(t *testing.T) {
	const pageURL = "https://www.minnano-av.com/search_result.php?search_scope=actress"
	const baseURL = "https://www.minnano-av.com"

	t.Run("returns the unique exact match", func(t *testing.T) {
		doc, err := parseHTMLDocument([]byte(`<html><body>
<h2 class="ttl"><a href="actress111.html">安堂奈々</a></h2>
<h2 class="ttl"><a href="actress861218.html?安堂なな"> 安堂なな </a></h2>
</body></html>`))
		if err != nil {
			t.Fatalf("parse fixture: %v", err)
		}
		got := findMinnanoAVActressSearchResultURL(doc, "安堂なな", pageURL, baseURL)
		if got != "https://www.minnano-av.com/actress861218.html" {
			t.Fatalf("result URL = %q", got)
		}
	})

	t.Run("rejects ambiguous exact matches", func(t *testing.T) {
		doc, err := parseHTMLDocument([]byte(`<html><body>
<h2 class="ttl"><a href="actress111.html">安堂なな</a></h2>
<h2 class="ttl"><a href="actress222.html">安堂なな</a></h2>
</body></html>`))
		if err != nil {
			t.Fatalf("parse fixture: %v", err)
		}
		if got := findMinnanoAVActressSearchResultURL(doc, "安堂なな", pageURL, baseURL); got != "" {
			t.Fatalf("result URL = %q, want empty", got)
		}
	})
}

func TestFinalizeMinnanoAVActressInfoRequiresExactName(t *testing.T) {
	const profileURL = "https://www.minnano-av.com/actress861218.html"

	info, err := finalizeMinnanoAVActressInfo("安堂なな", profileURL, &ActressInfo{
		JapaneseName: " 安堂なな ",
		RomanName:    "Ando   Nana",
	})
	if err != nil {
		t.Fatalf("finalizeMinnanoAVActressInfo: %v", err)
	}
	if info.JapaneseName != "安堂なな" || info.RomanName != "Ando Nana" || info.ProfileURL != profileURL {
		t.Fatalf("unexpected finalized info: %#v", info)
	}

	info, err = finalizeMinnanoAVActressInfo("安堂なな", profileURL, &ActressInfo{JapaneseName: "別の女優"})
	if !errors.Is(err, ResourceNotFonud) {
		t.Fatalf("err = %v, want ResourceNotFonud", err)
	}
	if info != nil {
		t.Fatalf("info = %#v, want nil", info)
	}
}
