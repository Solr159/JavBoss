package jav

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func resetJavDBRateLimiterForTest() {
	javDBRateLimiter.Lock()
	javDBRateLimiter.next = time.Time{}
	javDBRateLimiter.Unlock()
}

func TestFindJavDBSearchResultURLMatchesFirstExactCode(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<body>
  <div class="movie-list h cols-4 vcols-8">
    <div class="item">
      <a href="/v/kKdRm" class="box">
        <div class="video-title"><strong>IPX-228</strong> Title</div>
      </a>
    </div>
    <div class="item">
      <a href="/v/zKmWJ" class="box">
        <div class="video-title"><strong>IPX-128</strong> Other</div>
      </a>
    </div>
  </div>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	got := findJavDBSearchResultURL(doc, "ipx228", "https://javdb.com/search?q=ipx-228&f=all")
	if got != "https://javdb.com/v/kKdRm" {
		t.Fatalf("unexpected detail url: %q", got)
	}
}

func TestFindSingleJavDBSearchResultURLRejectsAmbiguousExactCodes(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<body>
  <div class="movie-list h cols-4 vcols-8">
    <div class="item">
      <a href="/v/first" class="box">
        <div class="video-title"><strong>IPX-228</strong> First</div>
      </a>
    </div>
    <div class="item">
      <a href="/v/second" class="box">
        <div class="video-title"><strong>IPX228</strong> Second</div>
      </a>
    </div>
  </div>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	got := findSingleJavDBSearchResultURL(doc, "ipx-228", "https://javdb.com/search?q=ipx-228&f=all")
	if got != "" {
		t.Fatalf("ambiguous exact matches should not choose detail url: %q", got)
	}
}

func TestFindSingleJavDBSearchResultURLReturnsUniqueExactCode(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<body>
  <div class="movie-list h cols-4 vcols-8">
    <div class="item">
      <a href="/v/first" class="box">
        <div class="video-title"><strong>IPX-228</strong> First</div>
      </a>
    </div>
    <div class="item">
      <a href="/v/other" class="box">
        <div class="video-title"><strong>IPX-128</strong> Other</div>
      </a>
    </div>
  </div>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	got := findSingleJavDBSearchResultURL(doc, "ipx228", "https://javdb.com/search?q=ipx-228&f=all")
	if got != "https://javdb.com/v/first" {
		t.Fatalf("unexpected detail url: %q", got)
	}
}

func TestParseJavDBMovieInfo(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<head><title> IPX-228 Fallback | JavDB 成人影片數據庫 </title></head>
<body>
  <div class="video-detail">
    <h2 class="title is-4">
      <strong>IPX-228 </strong>
      <strong class="current-title">中年オヤジと制服美少女の汗だく唾液みどろ特濃ベロキス性交 岬ななみ </strong>
    </h2>
    <nav class="panel movie-panel-info">
      <div class="panel-block first-block">
        <strong>番號:</strong>
        <span class="value"><a href="/video_codes/IPX">IPX</a>-228</span>
      </div>
      <div class="panel-block">
        <strong>日期:</strong>
        <span class="value">2018-11-13</span>
      </div>
      <div class="panel-block">
        <strong>時長:</strong>
        <span class="value">170 分鍾</span>
      </div>
      <div class="panel-block">
        <strong>導演:</strong>
        <span class="value"><a href="/directors/6DD">五右衛門</a></span>
      </div>
      <div class="panel-block">
        <strong>片商:</strong>
        <span class="value"><a href="/makers/ZXX">IDEA POCKET</a></span>
      </div>
      <div class="panel-block">
        <strong>發行:</strong>
        <span class="value"><a href="/publishers/8V9">ティッシュ</a></span>
      </div>
      <div class="panel-block">
        <strong>系列:</strong>
        <span class="value"><a href="/series/w54b">中年オヤジ</a></span>
      </div>
      <div class="panel-block">
        <strong>評分:</strong>
        <span class="value">4.41分, 由558人評價</span>
      </div>
      <div class="panel-block">
        <strong>類別:</strong>
        <span class="value"><a href="/tags?c7=28">單體作品</a>, <a href="/tags?c2=5">美少女</a></span>
      </div>
      <div class="panel-block">
        <strong>演員:</strong>
        <span class="value">
          <a href="/actors/QNen">岬ななみ</a><strong class="symbol female">♀</strong>
          <a href="/actors/zXAE">吉村卓</a><strong class="symbol male">♂</strong>
        </span>
      </div>
    </nav>
  </div>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	info := parseJavDBMovieInfo(doc)
	if info == nil {
		t.Fatal("expected info, got nil")
	}
	if info.Provider != ProviderJavDB {
		t.Fatalf("unexpected provider: %s", info.Provider.String())
	}
	if info.Code != "IPX-228" {
		t.Fatalf("unexpected code: %q", info.Code)
	}
	if info.Title != "中年オヤジと制服美少女の汗だく唾液みどろ特濃ベロキス性交 岬ななみ" {
		t.Fatalf("unexpected title: %q", info.Title)
	}

	wantRelease := time.Date(2018, 11, 13, 0, 0, 0, 0, time.UTC).Unix()
	if info.ReleaseUnix != wantRelease {
		t.Fatalf("unexpected release unix: got %d want %d", info.ReleaseUnix, wantRelease)
	}
	if info.DurationMin != 170 {
		t.Fatalf("unexpected duration: %d", info.DurationMin)
	}

	wantTags := []string{"單體作品", "美少女"}
	if len(info.Tags) != len(wantTags) {
		t.Fatalf("unexpected tags length: got %d want %d", len(info.Tags), len(wantTags))
	}
	for i, tag := range wantTags {
		if info.Tags[i] != tag {
			t.Fatalf("unexpected tag at %d: got %q want %q", i, info.Tags[i], tag)
		}
	}

	wantActors := []string{"岬ななみ"}
	if len(info.Actors) != len(wantActors) {
		t.Fatalf("unexpected actors length: got %d want %d", len(info.Actors), len(wantActors))
	}
	for i, actor := range wantActors {
		if info.Actors[i] != actor {
			t.Fatalf("unexpected actor at %d: got %q want %q", i, info.Actors[i], actor)
		}
	}

	fields := extractJavDBMovieFields(doc)
	if fields.Director != "五右衛門" || fields.Maker != "IDEA POCKET" || fields.Publisher != "ティッシュ" || fields.Series != "中年オヤジ" || fields.Rating != "4.41分, 由558人評價" {
		t.Fatalf("unexpected extra fields: %#v", fields)
	}
}

func TestParseJavDBActressURLByName(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<body>
  <nav class="panel movie-panel-info">
    <div class="panel-block">
      <strong>演員:</strong>
      <span class="value">
        <a href="/actors/QNen">岬ななみ</a><strong class="symbol female">♀</strong>
        <a href="/actors/zXAE">吉村卓</a><strong class="symbol male">♂</strong>
      </span>
    </div>
  </nav>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	got := parseJavDBActressURLByName(doc, "岬ななみ", "https://javdb.com/v/kKdRm")
	if got != "https://javdb.com/actors/QNen" {
		t.Fatalf("unexpected actress url: %q", got)
	}

	got = parseJavDBActressURLByName(doc, "吉村卓", "https://javdb.com/v/kKdRm")
	if got != "https://javdb.com/actors/QNen" {
		t.Fatalf("single actress url should be used as alias, got %q", got)
	}
}

func TestParseJavDBActressURLByNameRequiresMatchForMultipleActresses(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<body>
  <nav class="panel movie-panel-info">
    <div class="panel-block">
      <strong>演員:</strong>
      <span class="value">
        <a href="/actors/first">第一女優</a><strong class="symbol female">♀</strong>
        <a href="/actors/second">第二女優</a><strong class="symbol female">♀</strong>
      </span>
    </div>
  </nav>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	got := parseJavDBActressURLByName(doc, "別の女優", "https://javdb.com/v/kKdRm")
	if got != "" {
		t.Fatalf("unexpected unmatched actress url: %q", got)
	}

	got = parseJavDBActressURLByName(doc, "第二女優", "https://javdb.com/v/kKdRm")
	if got != "https://javdb.com/actors/second" {
		t.Fatalf("unexpected matched actress url: %q", got)
	}
}

func TestParseJavDBActressURLByNameUsesSingleActressAsAlias(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<body>
  <nav class="panel movie-panel-info">
    <div class="panel-block">
      <strong>演員:</strong>
      <span class="value">
        <a href="/actors/A1JK">別名の女優</a><strong class="symbol female">♀</strong>
      </span>
    </div>
  </nav>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	got := parseJavDBActressURLByName(doc, "美月アンジェリア", "https://javdb.com/v/56Kdp")
	if got != "https://javdb.com/actors/A1JK" {
		t.Fatalf("unexpected actress url: %q", got)
	}
}

func TestFindJavDBActorSearchResultURLsExactName(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<body>
  <div id="actors" class="actors">
    <div class="box actor-box">
      <a href="/actors/A1JK" title="美月アンジェリア">
        <figure class="image"></figure>
        <strong>美月アンジェリア</strong>
      </a>
    </div>
    <div class="box actor-box">
      <a href="/actors/other" title="美月アンジェリア別名">
        <strong>美月アンジェリア別名</strong>
      </a>
    </div>
  </div>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	got := findJavDBActorSearchResultURLs(doc, "美月アンジェリア", "https://javdb.com/search?q=x&f=actor")
	if len(got) != 1 || got[0] != "https://javdb.com/actors/A1JK" {
		t.Fatalf("unexpected actor urls: %#v", got)
	}
}

func TestFindJavDBActorSearchResultURLsMatchesTitleAliasList(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<body>
  <div id="actors" class="actors">
    <div class="box actor-box">
      <a href="/actors/7qzR" title="満月ひかり, 初芽里奈, 桃井りん, 葵かな, 新見リナ, 桃居りん, 初芽里菜, 初咲里奈">
        <figure class="image"></figure>
        <strong>満月ひかり</strong>
      </a>
    </div>
  </div>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	got := findJavDBActorSearchResultURLs(doc, "初芽里奈", "https://javdb.com/search?q=x&f=actor")
	if len(got) != 1 || got[0] != "https://javdb.com/actors/7qzR" {
		t.Fatalf("unexpected actor urls: %#v", got)
	}
}

func TestFindJavDBActorSearchResultURLsReturnsAllExactMatches(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<body>
  <div id="actors" class="actors">
    <div class="box actor-box">
      <a href="/actors/A1JK" title="美月アンジェリア"><strong>美月アンジェリア</strong></a>
    </div>
    <div class="box actor-box">
      <a href="/actors/RGp8" title="美月アンジェリア"><strong>美月アンジェリア</strong></a>
    </div>
  </div>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	got := findJavDBActorSearchResultURLs(doc, "美月アンジェリア", "https://javdb.com/search?q=x&f=actor")
	want := []string{"https://javdb.com/actors/A1JK", "https://javdb.com/actors/RGp8"}
	if len(got) != len(want) {
		t.Fatalf("unexpected actor url count: got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected actor url at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestParseJavDBSeriesURL(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<body>
  <nav class="panel movie-panel-info">
    <div class="panel-block">
      <strong>系列:</strong>
      <span class="value"><a href="/series/w54b">中年オヤジ</a></span>
    </div>
  </nav>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	got := parseJavDBSeriesURL(doc, "https://javdb.com/v/kKdRm")
	if got != "https://javdb.com/series/w54b" {
		t.Fatalf("unexpected series url: %q", got)
	}
}

func TestParseJavDBStudioURL(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<body>
  <nav class="panel movie-panel-info">
    <div class="panel-block">
      <strong>片商:</strong>
      <span class="value"><a href="/makers/ZXX">IDEA POCKET</a></span>
    </div>
  </nav>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	got := parseJavDBStudioURL(doc, "https://javdb.com/v/kKdRm")
	if got != "https://javdb.com/makers/ZXX" {
		t.Fatalf("unexpected studio url: %q", got)
	}
}

func TestParseJavDBCoverURL(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<body>
  <img src="https://c0.jdbstatic.com/covers/kk/kKdRm.jpg" class="video-cover">
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	got := parseJavDBCoverURL(doc, "https://javdb.com/v/kKdRm")
	if got != "https://c0.jdbstatic.com/covers/kk/kKdRm.jpg" {
		t.Fatalf("unexpected cover url: %q", got)
	}
}

func TestJavDBRateLimiterSpacesRequests(t *testing.T) {
	resetJavDBRateLimiterForTest()
	t.Cleanup(resetJavDBRateLimiterForTest)

	start := time.Now()
	for i := 0; i < 3; i++ {
		if err := waitForJavDBRateLimit(context.Background()); err != nil {
			t.Fatalf("waitForJavDBRateLimit() request %d: %v", i+1, err)
		}
	}

	if elapsed := time.Since(start); elapsed < (2*javDBRequestInterval - 50*time.Millisecond) {
		t.Fatalf("rate limiter allowed 3 requests in %s", elapsed)
	}
}

func TestJavDBRateLimiterHonorsContext(t *testing.T) {
	resetJavDBRateLimiterForTest()
	t.Cleanup(resetJavDBRateLimiterForTest)

	javDBRateLimiter.Lock()
	javDBRateLimiter.next = time.Now().Add(time.Hour)
	javDBRateLimiter.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := waitForJavDBRateLimit(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waitForJavDBRateLimit() err = %v, want context deadline exceeded", err)
	}
}
