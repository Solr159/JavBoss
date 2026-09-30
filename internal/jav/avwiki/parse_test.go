package avwiki

import (
	"fmt"
	"testing"
	"time"
)

func profileFixture(name, roman, date, size string) string {
	return fmt.Sprintf(`<div class="actress-data"><dl>
<dt>AV女優名<span>：</span></dt><dd>%s（よみ）- %s</dd>
<dt>別名義：</dt><dd>旧名（きゅうめい）</dd>
<dt>生年月日：</dt><dd>%s</dd>
<dt>サイズ：</dt><dd>%s</dd>
</dl></div><div>unrelated T180 B100 W70 H100</div>`, name, roman, date, size)
}

func TestParseActress(t *testing.T) {
	for _, tc := range []struct {
		name, slug, roman, date, size, wantRoman string
		measurements                             [5]int
		wantDate                                 int
	}{
		{"hyphenated", "morisawa-kana", "kana morisawa", "1992年5月9日", "T160-B85-W60-H88", "Kana Morisawa", [5]int{160, 85, 60, 88, 0}, int(time.Date(1992, 5, 9, 0, 0, 0, 0, time.UTC).Unix())},
		{"cup and slug", "kokonoi-sunao", "kokonoi-sunao", "2002年1月30日", "T160 B100(Hカップ) W64 H110", "Sunao Kokonoi", [5]int{160, 100, 64, 110, 8}, int(time.Date(2002, 1, 30, 0, 0, 0, 0, time.UTC).Unix())},
		{"slash and zero padding", "ayase-maria", "maria ayase", "2001年06月10日", "T165 / B86-W58-H88", "Maria Ayase", [5]int{165, 86, 58, 88, 0}, int(time.Date(2001, 6, 10, 0, 0, 0, 0, time.UTC).Unix())},
		{"missing values", "test", "&#8211; &#8211; &#8211;", "不明", "---", "", [5]int{}, 0},
		{"invalid date", "name-test", "test name", "2001年2月29日", "T160 / B80 / W60 / H90", "Test Name", [5]int{160, 80, 60, 90, 0}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tag := actressTag{Name: "女優名", Link: baseURL + "/av-actress/" + tc.slug + "/", Description: profileFixture("女優名", tc.roman, tc.date, tc.size)}
			info := parseActress(tag, baseURL)
			if info == nil {
				t.Fatal("missing profile")
			}
			if info.JapaneseName != tag.Name || info.RomanName != tc.wantRoman || info.ProfileURL != tag.Link || info.BirthDate != tc.wantDate {
				t.Fatalf("unexpected profile: %+v", info)
			}
			if got := [5]int{info.HeightCM, info.Bust, info.Waist, info.Hips, info.Cup}; got != tc.measurements {
				t.Fatalf("measurements = %v, want %v", got, tc.measurements)
			}
		})
	}
}

func TestParseActressRejectsUnrelatedProfiles(t *testing.T) {
	for _, tc := range []struct{ name, link, description string }{
		{"wrong origin", "https://other.test/av-actress/test/", profileFixture("女優名", "test name", "", "")},
		{"movie link", baseURL + "/movie-001/", profileFixture("女優名", "test name", "", "")},
		{"wrong profile", baseURL + "/av-actress/test/", profileFixture("別の女優", "test name", "", "")},
		{"empty profile", baseURL + "/av-actress/test/", ""},
		{"unrelated description", baseURL + "/av-actress/test/", "T160 B90 W60 H90"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if info := parseActress(actressTag{Name: "女優名", Link: tc.link, Description: tc.description}, baseURL); info != nil {
				t.Fatalf("unrelated profile accepted: %+v", info)
			}
		})
	}
}

func TestRomanNameUsesGivenNameFirst(t *testing.T) {
	for _, tc := range []struct{ name, field, slug, want string }{
		{"family first hyphens", "九井スナオ（ここのいすなお）- kokonoi-sunao", "kokonoi-sunao", "Sunao Kokonoi"},
		{"family first spaces", "九井スナオ（ここのいすなお）- Kokonoi Sunao", "kokonoi-sunao", "Sunao Kokonoi"},
		{"already given first", "森沢かな（もりさわかな）- kana morisawa", "morisawa-kana", "Kana Morisawa"},
		{"given first hyphens", "森沢かな（もりさわかな）- kana-morisawa", "morisawa-kana", "Kana Morisawa"},
		{"ascii brackets and whitespace", "天馬ゆい(てんまゆい) -  TENMA   YUI", "tenma-yui", "Yui Tenma"},
		{"keep profile spelling", "乙アリス（おつありす）- arisu otsu", "otsu-alice", "Arisu Otsu"},
		{"preserve capitalized stage name", "RION（りおん）- RION", "rion", "RION"},
		{"capitalize single name", "RION（りおん）- rion", "rion", "Rion"},
		{"unknown family", "名前（なまえ）- test name", "unrelated-person", ""},
		{"ambiguous slug", "名前（なまえ）- test name", "test-name-two", ""},
		{"ambiguous name", "名前（なまえ）- test middle name", "test-name", ""},
		{"missing roman name", "名前（なまえ）- ---", "test-name", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseRomanName(tc.field, "/av-actress/"+tc.slug+"/"); got != tc.want {
				t.Fatalf("parseRomanName(%q) = %q, want %q", tc.field, got, tc.want)
			}
		})
	}
}
