package jav

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func resetAvmooRateLimiterForTest() {
	avmooRateLimiter.Lock()
	avmooRateLimiter.next = time.Time{}
	avmooRateLimiter.Unlock()
}

func TestFindAvmooSearchResultURLMatchesExactCode(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<body>
  <div class="container-fluid">
    <div class="row">
      <div id="waterfall">
        <div class="item">
          <a class="movie-box" href="//avmoo.shop/tw/movie/ee973af572422a79">
            <div class="photo-info"><span>Title<br><date>IPX-123</date> / <date>2018-03-31</date></span></div>
          </a>
        </div>
        <div class="item">
          <a class="movie-box" href="//avmoo.shop/tw/movie/e7bb56c0b0512dc7">
            <div class="photo-info"><span>Other<br><date>IPX-059</date> / <date>2017-12-09</date></span></div>
          </a>
        </div>
      </div>
    </div>
  </div>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	got := findAvmooSearchResultURL(doc, "ipx123", "https://avmoo.shop/tw/search/ipx-123")
	if got != "https://avmoo.shop/tw/movie/ee973af572422a79" {
		t.Fatalf("unexpected detail url: %q", got)
	}
}

func TestParseAvmooMovieInfoFromFixture(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(avmooDetailFixture))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	info := parseAvmooMovieInfo(doc)
	if info == nil {
		t.Fatal("expected info, got nil")
	}
	if info.Provider != ProviderAvmoo {
		t.Fatalf("unexpected provider: %s", info.Provider.String())
	}
	if info.Code != "IPX-228" {
		t.Fatalf("unexpected code: %q", info.Code)
	}
	if info.Title != "中年オヤジと制服美少女の汗だく唾液みどろ特濃ベロキス性交 岬ななみ" {
		t.Fatalf("unexpected title: %q", info.Title)
	}
	if info.Series != "中年オヤジと制服美少女の汗だく唾液みどろ特濃ベロキス性交" {
		t.Fatalf("unexpected series: %q", info.Series)
	}

	wantRelease := time.Date(2018, 11, 10, 0, 0, 0, 0, time.UTC).Unix()
	if info.ReleaseUnix != wantRelease {
		t.Fatalf("unexpected release unix: got %d want %d", info.ReleaseUnix, wantRelease)
	}
	if info.DurationMin != 171 {
		t.Fatalf("unexpected duration: %d", info.DurationMin)
	}

	wantTags := []string{"校服", "單體作品", "DMM獨家", "花癡", "美少女", "數位馬賽克", "高畫質", "接吻", "流汗"}
	if len(info.Tags) != len(wantTags) {
		t.Fatalf("unexpected tags length: got %d want %d %#v", len(info.Tags), len(wantTags), info.Tags)
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
}

func TestParseAvmooCoverURLFromFixture(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(avmooDetailFixture))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	got := parseAvmooCoverURL(doc, "https://avmoo.shop/tw/movie/1a27d5e9cb82f32f")
	if got != "https://jp.netcdn.space/digital/video/ipx00228/ipx00228pl.jpg" {
		t.Fatalf("unexpected cover url: %q", got)
	}
}

func TestAvmooRateLimiterSpacesRequests(t *testing.T) {
	resetAvmooRateLimiterForTest()
	t.Cleanup(resetAvmooRateLimiterForTest)

	start := time.Now()
	for i := 0; i < 3; i++ {
		if err := waitForAvmooRateLimit(context.Background()); err != nil {
			t.Fatalf("waitForAvmooRateLimit() request %d: %v", i+1, err)
		}
	}

	if elapsed := time.Since(start); elapsed < (2*avmooRequestInterval - 50*time.Millisecond) {
		t.Fatalf("rate limiter allowed 3 requests in %s", elapsed)
	}
}

func TestAvmooRateLimiterHonorsContext(t *testing.T) {
	resetAvmooRateLimiterForTest()
	t.Cleanup(resetAvmooRateLimiterForTest)

	avmooRateLimiter.Lock()
	avmooRateLimiter.next = time.Now().Add(time.Hour)
	avmooRateLimiter.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := waitForAvmooRateLimit(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waitForAvmooRateLimit() err = %v, want context deadline exceeded", err)
	}
}

const avmooDetailFixture = `
<!doctype html>
<html>
<head><title>IPX-228 中年オヤジと制服美少女の汗だく唾液みどろ特濃ベロキス性交 岬ななみ - AVMOO</title></head>
<body>
  <nav><div class="container"></div></nav>
  <div class="container">
    <h3>IPX-228 中年オヤジと制服美少女の汗だく唾液みどろ特濃ベロキス性交 岬ななみ</h3>
    <div class="row movie">
      <div class="col-md-9 screencap">
        <a class="bigImage" href="https://jp.netcdn.space/digital/video/ipx00228/ipx00228pl.jpg">
          <img src="https://jp.netcdn.space/digital/video/ipx00228/ipx00228pl.jpg">
        </a>
      </div>
      <div class="col-md-3 info">
        <p><span class="header">識別碼:</span> <span style="color:#CC0000;">IPX-228</span></p>
        <p><span class="header">發行日期:</span> 2018-11-10</p>
        <p><span class="header">長度:</span> 171分鐘</p>
        <p><span class="header">導演:</span> <a href="//avmoo.shop/tw/director/cec15db527d742bc">五右衛門</a></p>
        <p class="header">製作商: </p>
        <p><a href="//avmoo.shop/tw/studio/e4db8b2a7043a74a">アイデアポケット</a></p>
        <p class="header">發行商: </p>
        <p><a href="//avmoo.shop/tw/label/8e6c8cf10c52df0a">ティッシュ</a></p>
        <p class="header">系列:</p>
        <p><a href="//avmoo.shop/tw/series/4a59af0fa75259a6">中年オヤジと制服美少女の汗だく唾液みどろ特濃ベロキス性交</a></p>
        <p class="header">類別:</p>
        <p>
          <span class="genre"><a href="//avmoo.shop/tw/genre/5a07be553e5ab0fd">校服</a></span>
          <span class="genre"><a href="//avmoo.shop/tw/genre/c4145926405d550f">單體作品</a></span>
          <span class="genre"><a href="//avmoo.shop/tw/genre/bfcaa1b424700e19">DMM獨家</a></span>
          <span class="genre"><a href="//avmoo.shop/tw/genre/1d845bf3af10f908">花癡</a></span>
          <span class="genre"><a href="//avmoo.shop/tw/genre/b0eaad139052cec8">美少女</a></span>
          <span class="genre"><a href="//avmoo.shop/tw/genre/d65b5063f5aaeaed">數位馬賽克</a></span>
          <span class="genre"><a href="//avmoo.shop/tw/genre/5f9f62d40baa77cf">高畫質</a></span>
          <span class="genre"><a href="//avmoo.shop/tw/genre/f19775d3dc23f16b">接吻</a></span>
          <span class="genre"><a href="//avmoo.shop/tw/genre/a33f4c2859e6936f">流汗</a></span>
        </p>
      </div>
    </div>
    <h4>演員</h4>
    <div id="avatar-waterfall">
      <a class="avatar-box" href="//avmoo.shop/tw/star/e0ff5947c4ceebca"><span>岬ななみ</span></a>
    </div>
  </div>
</body>
</html>`
