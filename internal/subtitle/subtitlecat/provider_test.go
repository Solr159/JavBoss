package subtitlecat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"javboss/internal/subtitle"
)

func TestSearchRanksByChineseSourceThenGoodRatingThenDownloads(t *testing.T) {
	row := func(id, source, rating, downloads string) string {
		return fmt.Sprintf(`<tr><td><a href="subs/1/%s.html">ABC-123 %s</a> %s</td><td>%s</td><td>125 KB</td><td><span>Downloads</span><span>%s<span> downloads</span></span></td><td>99 languages</td></tr>`, id, id, source, rating, downloads)
	}
	chinese := "(translated from Chinese)"
	good := `<span title="Rated good by users">👍</span>`
	for _, tc := range []struct {
		name, rows string
		want       []string
	}{
		{
			name: "source takes priority over rating and downloads",
			rows: row("japanese", "(translated from Japanese)", good, "9000") + row("chinese", chinese, "", "1"),
			want: []string{"chinese", "japanese"},
		},
		{
			name: "good rating takes priority over downloads",
			rows: row("unrated", chinese, "", "9000") + row("bad", chinese, `<span title="Rated bad by users">👎</span>`, "10000") + row("good", chinese, good, "1"),
			want: []string{"good", "bad", "unrated"},
		},
		{
			name: "downloads sort numerically and support commas",
			rows: row("small", chinese, good, "9") + row("large", chinese, good, "1,200") + row("medium", chinese, good, "22"),
			want: []string{"large", "medium", "small"},
		},
		{
			name: "missing metadata and ties retain page order",
			rows: row("first", "", "", "") + row("second", "", "", "unknown"),
			want: []string{"first", "second"},
		},
		{
			name: "title cannot claim chinese source",
			rows: `<tr><td><a href="subs/1/fake.html">ABC-123 (translated from Chinese) 999 downloads</a></td><td></td></tr>` + row("real", chinese, "", "1"),
			want: []string{"real", "fake"},
		},
		{
			name: "ranking applies to candidates beyond first four",
			rows: row("jp", "(translated from Japanese)", "", "9") + row("zh", chinese, "", "11") + row("en", "(translated from English)", good, "13") + row("best", chinese, good, "22") + row("zhbad", chinese, "👎", "21"),
			want: []string{"best", "zhbad", "zh", "en", "jp"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `<input name="search"><table><tbody>`+tc.rows+`</tbody></table>`)
			}))
			defer server.Close()
			p := New(server.Client())
			p.base, _ = url.Parse(server.URL)
			p.interval = 0
			candidates, err := p.Search(context.Background(), subtitle.Query{Code: "ABC-123"})
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, len(candidates))
			for i, c := range candidates {
				got[i] = c.ID
			}
			want := make([]string, len(tc.want))
			for i, id := range tc.want {
				want[i] = server.URL + "/subs/1/" + id + ".html"
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("candidate order = %v, want %v", got, want)
			}
		})
	}
}

func TestSearchDetailDownloadUsesParsedLinksAndExactCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing user agent")
		}
		switch r.URL.Path {
		case "/index.php":
			if r.URL.Query().Get("search") != "ABC-123" {
				t.Error("incorrect search")
			}
			fmt.Fprint(w, `<input name="search"><a href="subs/1/ABC-123.html">ABC-123</a><a href="subs/1/other.html">ABC-1234</a><a href="https://evil.example/subs/1/x.html">ABC-123</a>`)
		case "/subs/1/ABC-123.html":
			fmt.Fprint(w, `<input name="search"><a href="/subs/99/ABC-123 jp-zh-CN.srt">Download</a><button>Translate</button><a href="https://evil.example/x-zh-CN.srt">Download</a>`)
		case "/subs/99/ABC-123 jp-zh-CN.srt":
			fmt.Fprint(w, "1\n00:00:01,000 --> 00:00:02,000\nHello\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	p := New(server.Client())
	p.base, _ = url.Parse(server.URL)
	p.interval = 0
	cs, err := p.Search(context.Background(), subtitle.Query{Code: "ABC-123"})
	if err != nil || len(cs) != 1 {
		t.Fatalf("candidates=%v err=%v", cs, err)
	}
	tracks, err := p.Tracks(context.Background(), cs[0])
	if err != nil || len(tracks) != 1 || tracks[0].Language != "zh-CN" {
		t.Fatalf("tracks=%v err=%v", tracks, err)
	}
	data, err := p.Download(context.Background(), tracks[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err = subtitle.NormalizeSRT(data); err != nil {
		t.Fatal(err)
	}
}

func TestProviderRejectsChallengesAndCrossOriginRedirects(t *testing.T) {
	for _, redirect := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if redirect {
				http.Redirect(w, r, "https://other.example/file", 302)
			} else {
				fmt.Fprint(w, "<html>challenge</html>")
			}
		}))
		p := New(server.Client())
		p.base, _ = url.Parse(server.URL)
		p.interval = 0
		if _, err := p.Search(context.Background(), subtitle.Query{Code: "ABC-123"}); err == nil {
			t.Fatal("accepted challenge or redirect")
		}
		server.Close()
	}
}

func TestCodePattern(t *testing.T) {
	for _, tc := range []struct {
		code, title string
		match       bool
	}{
		{"ABC-123", "abc123.zh", true}, {"ABC-123", "ABC-1234", false}, {"ABC-123", "XABC-123", false},
		{"FC2-PPV-1234567", "FC2 PPV 1234567", true}, {"ABC-123", "ABC-123 jp", true},
	} {
		if got := codePattern(tc.code).MatchString(tc.title); got != tc.match {
			t.Errorf("%s / %s = %v", tc.code, tc.title, got)
		}
	}
}
