package jav

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func TestParseJavMenuMovieInfoFromFixture(t *testing.T) {
	doc := mustParseJavMenuFixture(t)

	info := parseJavMenuMovieInfo(doc)
	if info == nil {
		t.Fatal("expected info, got nil")
	}
	if info.Provider != ProviderJavMenu {
		t.Fatalf("unexpected provider: %s", info.Provider.String())
	}
	if info.Code != "IPX-228" {
		t.Fatalf("unexpected code: %q", info.Code)
	}
	if info.Title != "中年オヤジと制服美少女の汗だく唾液みどろ特濃ベロキス性交 岬ななみ" {
		t.Fatalf("unexpected title: %q", info.Title)
	}
	if info.Studio != "ティッシュ" {
		t.Fatalf("unexpected studio: %q", info.Studio)
	}
	if info.Series != "中年オヤジと制服美少女の汗だく唾液みどろ特濃ベロキス性交" {
		t.Fatalf("unexpected series: %q", info.Series)
	}

	wantRelease := time.Date(2018, 11, 13, 0, 0, 0, 0, time.UTC).Unix()
	if info.ReleaseUnix != wantRelease {
		t.Fatalf("unexpected release unix: got %d want %d", info.ReleaseUnix, wantRelease)
	}
	if info.DurationMin != 170 {
		t.Fatalf("unexpected duration: %d", info.DurationMin)
	}

	wantTags := []string{"美少女", "淫亂，真實", "數位馬賽克", "接吻", "校服", "流汗"}
	if len(info.Tags) != len(wantTags) {
		t.Fatalf("unexpected tags length: got %d want %d %#v", len(info.Tags), len(wantTags), info.Tags)
	}
	for i, tag := range wantTags {
		if info.Tags[i] != tag {
			t.Fatalf("unexpected tag at %d: got %q want %q", i, info.Tags[i], tag)
		}
	}

	wantActors := []string{"岬奈奈美"}
	if len(info.Actors) != len(wantActors) {
		t.Fatalf("unexpected actors length: got %d want %d %#v", len(info.Actors), len(wantActors), info.Actors)
	}
	for i, actor := range wantActors {
		if info.Actors[i] != actor {
			t.Fatalf("unexpected actor at %d: got %q want %q", i, info.Actors[i], actor)
		}
	}
}

func mustParseJavMenuFixture(t *testing.T) *html.Node {
	t.Helper()

	doc, err := html.Parse(strings.NewReader(`
		<html>
			<head>
				<title>IPX-228 中年オヤジと制服美少女の汗だく唾液みどろ特濃ベロキス性交 岬ななみ 免費AV在線看</title>
			</head>
			<body>
				<h1>IPX-228 中年オヤジと制服美少女の汗だく唾液みどろ特濃ベロキス性交 岬ななみ</h1>
				<div class="card rounded">
					<div class="card-body">
						<h2>影片資料</h2>
						<div><span>番號:</span> IPX-228</div>
						<div><span>發佈於:</span> 2018-11-13</div>
						<div><span>時長:</span> 170分鐘</div>
						<div><span>出版:</span> <a href="/studio/tissue">ティッシュ</a></div>
						<div><span>系列:</span> <a href="/series/ipx">中年オヤジと制服美少女の汗だく唾液みどろ特濃ベロキス性交</a></div>
						<div>
							<span>類別:</span>
							<a class="genre" href="/genre/1">美少女</a>
							<a class="genre" href="/genre/2">淫亂，真實</a>
							<a class="genre" href="/genre/3">數位馬賽克</a>
							<a class="genre" href="/genre/4">接吻</a>
							<a class="genre" href="/genre/5">校服</a>
							<a class="genre" href="/genre/6">流汗</a>
						</div>
						<div>
							<span>女優:</span>
							<a class="actress" href="/actress/nanami-misaki">岬奈奈美</a>
						</div>
					</div>
				</div>
			</body>
		</html>`))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return doc
}
