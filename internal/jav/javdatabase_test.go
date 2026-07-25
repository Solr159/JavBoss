package jav

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func resetJavDatabaseRateLimiterForTest() {
	javDatabaseRateLimiter.Lock()
	javDatabaseRateLimiter.next = time.Time{}
	javDatabaseRateLimiter.Unlock()
}

func TestParseJavDatabaseMovieInfo(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<head>
  <title>IPX-004 - Tsumugi Akari - JAV Database</title>
  <meta property="og:image" content="https://www.javdatabase.com/covers/ipx-004.jpg">
</head>
<body>
  <div class="movietable" style="padding-top: 5px;">
    <div class="row">
      <div class="col-md-2 col-lg-2 col-xxl-2 col-4"></div>
      <div class="col-md-10 col-lg-10 col-xxl-10 col-8">
        <p class="mb-1"><b>Title: </b>Together With A Miraculous Beautiful Girl</p>
        <p class="mb-1"><b>DVD ID: </b>IPX-004</p>
        <p class="mb-1"><b>Release Date: </b>2017-09-09</p>
        <p class="mb-1"><b>Runtime: </b>159  (HD: 159) min.</p>
        <p class="mb-1"><b>Studio: </b><span><a href="/studios/idea-pocket/">Idea Pocket</a></span></p>
        <p class="mb-1"><b>Series: </b><span><a href="/series/beautiful-girl/">Beautiful Girl Series</a></span></p>
        <p class="mb-1"><b>Genre(s): </b><span><a href="/genres/a">Beautiful Girl</a></span> <span><a href="/genres/b">Hi-Def</a></span></p>
        <p class="mb-1"><b>Idol(s)/Actress(es): </b><span><a href="/idols/tsumugi-akari/">Tsumugi Akari</a></span></p>
      </div>
    </div>
  </div>
  <div class="image-gallery-section">
    <a href="#" data-image-src="https://pics.dmm.co.jp/ipx004jp-1.jpg">
      <img src="https://pics.dmm.co.jp/ipx004-1.jpg">
    </a>
  </div>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	info := parseJavDatabaseMovieInfo(doc)
	if info == nil {
		t.Fatal("expected info, got nil")
	}

	if info.Title != "Together With A Miraculous Beautiful Girl" {
		t.Fatalf("unexpected title: %q", info.Title)
	}
	if info.Code != "IPX-004" {
		t.Fatalf("unexpected code: %q", info.Code)
	}
	if info.Studio != "Idea Pocket" {
		t.Fatalf("unexpected studio: %q", info.Studio)
	}
	if info.Series != "Beautiful Girl Series" {
		t.Fatalf("unexpected series: %q", info.Series)
	}
	if info.CoverURL != "https://www.javdatabase.com/covers/ipx-004.jpg" {
		t.Fatalf("unexpected cover url: %q", info.CoverURL)
	}
	if len(info.SampleImages) != 1 ||
		info.SampleImages[0].ThumbnailURL != "https://pics.dmm.co.jp/ipx004-1.jpg" ||
		info.SampleImages[0].DetailURL != "https://pics.dmm.co.jp/ipx004jp-1.jpg" {
		t.Fatalf("unexpected sample images: %#v", info.SampleImages)
	}

	wantRelease := time.Date(2017, 9, 9, 0, 0, 0, 0, time.UTC).Unix()
	if info.ReleaseUnix != wantRelease {
		t.Fatalf("unexpected release unix: got %d want %d", info.ReleaseUnix, wantRelease)
	}
	if info.DurationMin != 159 {
		t.Fatalf("unexpected duration: %d", info.DurationMin)
	}

	wantTags := []string{"Beautiful Girl", "Hi-Def"}
	if len(info.Tags) != len(wantTags) {
		t.Fatalf("unexpected tags length: got %d want %d", len(info.Tags), len(wantTags))
	}
	for i, tag := range wantTags {
		if info.Tags[i] != tag {
			t.Fatalf("unexpected tag at %d: got %q want %q", i, info.Tags[i], tag)
		}
	}

	wantActors := []string{"Tsumugi Akari"}
	if len(info.Actors) != len(wantActors) {
		t.Fatalf("unexpected actors length: got %d want %d", len(info.Actors), len(wantActors))
	}
	for i, actor := range wantActors {
		if info.Actors[i] != actor {
			t.Fatalf("unexpected actor at %d: got %q want %q", i, info.Actors[i], actor)
		}
	}
}

func TestJavDatabaseRateLimiterInterval(t *testing.T) {
	if javDatabaseRequestInterval != 500*time.Millisecond {
		t.Fatalf("javdatabase interval = %s, want 500ms", javDatabaseRequestInterval)
	}
}

func TestJavDatabaseRateLimiterSpacesRequests(t *testing.T) {
	resetJavDatabaseRateLimiterForTest()
	t.Cleanup(resetJavDatabaseRateLimiterForTest)

	start := time.Now()
	for i := 0; i < 3; i++ {
		if err := waitForJavDatabaseRateLimit(context.Background()); err != nil {
			t.Fatalf("waitForJavDatabaseRateLimit() request %d: %v", i+1, err)
		}
	}

	if elapsed := time.Since(start); elapsed < (2*javDatabaseRequestInterval - 50*time.Millisecond) {
		t.Fatalf("rate limiter allowed 3 requests in %s", elapsed)
	}
}

func TestJavDatabaseRateLimiterHonorsContext(t *testing.T) {
	resetJavDatabaseRateLimiterForTest()
	t.Cleanup(resetJavDatabaseRateLimiterForTest)

	javDatabaseRateLimiter.Lock()
	javDatabaseRateLimiter.next = time.Now().Add(time.Hour)
	javDatabaseRateLimiter.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := waitForJavDatabaseRateLimit(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waitForJavDatabaseRateLimit() err = %v, want context deadline exceeded", err)
	}
}

func TestNormalizeJavDatabaseCodeDoesNotPrefixMatch(t *testing.T) {
	if normalizeJavDatabaseCode("ipx-1") == normalizeJavDatabaseCode("IPX-100") {
		t.Fatal("ipx-1 must not match IPX-100")
	}
	if normalizeJavDatabaseCode("ipx 004") != normalizeJavDatabaseCode("IPX-004") {
		t.Fatal("expected separator-insensitive match")
	}
}

func TestParseJavDatabaseCoverURL(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<head>
  <meta property="og:image" content="/covers/ipx-004.jpg">
</head>
<body>
  <img class="poster" src="/covers/fallback.jpg">
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	coverURL := parseJavDatabaseCoverURL(doc, "https://www.javdatabase.com/movies/IPX-004")
	if coverURL != "https://www.javdatabase.com/covers/ipx-004.jpg" {
		t.Fatalf("unexpected cover url: %q", coverURL)
	}
}

func TestParseJavDatabaseActressInfoTrimsTrailingDashFromJapaneseName(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<body>
  <div class="entry-content">
    <h1 class="idol-name">Lara Kudo</h1>
    <p><b>Japanese Name:</b> 工藤ララ  - </p>
    <p><b>Height:</b> 160 cm</p>
  </div>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	info := parseJavDatabaseActressInfo(doc)
	if info == nil {
		t.Fatal("expected info, got nil")
	}
	if info.JapaneseName != "工藤ララ" {
		t.Fatalf("unexpected japanese name: %q", info.JapaneseName)
	}
}

func TestParseJavDatabaseActressInfoFromIPX228Profile(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<body>
  <div class="entry-content">
    <div class="row">
      <div class="col-12 col-xxl-7">
        <h1 class="idol-name">Nanami Misaki - JAV Profile</h1>
        <b>Age:</b> <a href="/idols/?_birth_year=1996">30</a>
        - <b>DOB:</b> <a href="/idols/?_birth_year=1996">1996-06-09</a>
        - <b>Debut:</b> <a href="/idols/?_debut_year=2017">2017-10-28</a>
        - <b>Measurements:</b> 83-59-83
        - <b>Cup:</b> <a href="/idols/?_cup_size=d">D</a>
        - <b>Height:</b> <a href="/idols/?_height=150">150 cm</a>
        - <b>Shoe Size:</b> ?<br>
        <p>
          <b>Tags:</b>
          <a href="/idols/?_age_group=twenties">Twenties</a> -
          <a href="/suggest-idol-tags/">Suggest Tags</a><br>
          <b>JP:</b> 岬ななみ <br>
        </p>
      </div>
    </div>
  </div>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	info := parseJavDatabaseActressInfo(doc)
	if info == nil {
		t.Fatal("expected info, got nil")
	}
	if info.RomanName != "Nanami Misaki" {
		t.Fatalf("unexpected roman name: %q", info.RomanName)
	}
	if info.JapaneseName != "岬ななみ" {
		t.Fatalf("unexpected japanese name: %q", info.JapaneseName)
	}
	if info.HeightCM != 150 {
		t.Fatalf("unexpected height: %d", info.HeightCM)
	}
	if info.Bust != 83 || info.Waist != 59 || info.Hips != 83 {
		t.Fatalf("unexpected measurements: %d-%d-%d", info.Bust, info.Waist, info.Hips)
	}
	wantBirthDate := int(time.Date(1996, 6, 9, 0, 0, 0, 0, time.UTC).Unix())
	if info.BirthDate != wantBirthDate {
		t.Fatalf("unexpected birth date: got %d want %d", info.BirthDate, wantBirthDate)
	}
	if info.Cup != 4 {
		t.Fatalf("unexpected cup: %d", info.Cup)
	}
}

func TestFindJavDatabaseActressLinkIgnoresGenreNames(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
<body>
  <div class="movietable">
    <p class="mb-1"><b>Genre(s): </b>
      <span><a href="https://www.javdatabase.com/genres/beautiful-girl/" rel="tag">Beautiful Girl</a></span>
      <span><a href="https://www.javdatabase.com/genres/creampie/" rel="tag">Creampie</a></span>
      <span><a href="https://www.javdatabase.com/genres/debut/" rel="tag">Debut</a></span>
      <span><a href="https://www.javdatabase.com/genres/digital-mosaic/" rel="tag">Digital Mosaic</a></span>
      <span><a href="https://www.javdatabase.com/genres/featured-actress/" rel="tag">Featured Actress</a></span>
      <span><a href="https://www.javdatabase.com/genres/hi-def/" rel="tag">Hi-Def</a></span>
      <span><a href="https://www.javdatabase.com/genres/idol-celebrity/" rel="tag">Idol &amp; Celebrity</a></span>
      <span><a href="https://www.javdatabase.com/genres/shotacon/" rel="tag">Shotacon</a></span>
    </p>
    <p class="mb-1"><b>Idol(s)/Actress(es): </b>
      <span><a href="https://www.javdatabase.com/idols/sora-inoue/">Sora Inoue</a></span>
    </p>
  </div>
</body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	link, err := findJavDatabaseActressLink(doc)
	if err != nil {
		t.Fatalf("findJavDatabaseActressLink() error = %v", err)
	}
	if link != "https://www.javdatabase.com/idols/sora-inoue/" {
		t.Fatalf("unexpected actress link: %q", link)
	}
}
