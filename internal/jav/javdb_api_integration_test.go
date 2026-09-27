package jav

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"javboss/internal/jav/internal/parseutil"
	"javboss/internal/jav/javdb"
)

const javDBAPITestMovie = `{"id":"m1","number":"ABC-001","title":"中文译名","origin_title":"日本語の原題","maker_name":"Maker","maker_id":42,"series_name":"Series","series_id":"s1","release_date":"2025-01-02","duration":"120","cover_url":"https://images.test/cover.jpg","type":0,"tags":[{"name":"Tag"},{"name":"Tag"}],"actors":[{"id":"a1","name":"Actress","gender":0},{"id":"a2","name":"Male","gender":1},{"id":"a3","name":"Unknown"}],"preview_images":[{"thumb_url":"https://images.test/thumb.jpg","large_url":"https://images.test/full.jpg"},{"large_url":"https://images.test/full.jpg"},{"large_url":"https://images.test/only.jpg"}]}`

func newJavDBAPITestProvider(t *testing.T, handler http.HandlerFunc) *javdb.API {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return javdb.NewAPI(server.Client(), server.URL)
}

func TestJavDBAPILookup(t *testing.T) {
	var deviceID string
	calls := 0
	p := newJavDBAPITestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("jdsignature") == "" {
			t.Error("missing signature")
		}
		if r.Header.Get("User-Agent") != "Dart/3.4 (dart:io)" || r.Header.Get("Accept-Language") != "zh-TW" {
			t.Error("missing app headers")
		}
		q := r.URL.Query()
		for key, want := range map[string]string{"app_channel": "official", "app_version": "1.9.28", "app_version_number": "10928", "platform": "android", "system_version": "13", "device_model": "Pixel 6", "device_name": "Pixel"} {
			if q.Get(key) != want {
				t.Errorf("%s = %q", key, q.Get(key))
			}
		}
		if deviceID == "" {
			deviceID = q.Get("device_uuid")
		}
		if len(deviceID) != 36 || q.Get("device_uuid") != deviceID {
			t.Error("device identity must be stable")
		}
		switch r.URL.Path {
		case "/api/v2/search":
			if q.Get("q") != "abc-001" || q.Get("page") != "1" || q.Get("limit") != "100" || q.Has("movie_type") {
				t.Errorf("search query: %v", q)
			}
			fmt.Fprint(w, `{"success":1,"data":{"movies":[{"id":"wrong","number":"ABC-002"},{"id":"m1","number":"ABC-001"}]}}`)
		case "/api/v4/movies/m1":
			fmt.Fprintf(w, `{"success":"true","data":{"movie":%s}}`, javDBAPITestMovie)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})

	client := NewClient(map[Provider]any{ProviderJavDBAPI: p}, newMemoryLookupCache())

	lookupCacheSetHit(client, "v1:jav:javdb-api:lookup_jav:ABC-001", &JavInfo{Code: "ABC-001", Title: "旧中文标题"})
	lookupCacheSetHit(client, "v2:jav:javdb-api:lookup_jav:ABC-001", &JavInfo{Code: "ABC-001", Title: "日本語の原題"})
	lookupCacheSetHit(client, "v3:jav:javdb-api:lookup_jav:ABC-001", &JavInfo{Code: "ABC-001", Title: "日本語の原題", ZhTitle: "中文译名"})
	info, err := client.LookupJavByCode(context.Background(), " abc-001 ", ProviderJavDBAPI)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || info.Provider != ProviderJavDBAPI || info.Code != "ABC-001" || info.Title != "日本語の原題" || info.ZhTitle != "中文译名" || info.Studio != "Maker" || info.Series != "Series" || info.DurationMin != 120 || info.ReleaseUnix != parseutil.ParseDateUnix("2025-01-02") || info.CoverURL != "https://images.test/cover.jpg" {
		t.Fatalf("bad metadata: %+v, calls %d", info, calls)
	}
	cached, err := client.LookupJavByCode(context.Background(), "ABC-001", ProviderJavDBAPI)
	if err != nil || cached.Title != info.Title || cached.ZhTitle != info.ZhTitle || calls != 2 {
		t.Fatalf("original title cache: info=%+v err=%v calls=%d", cached, err, calls)
	}
	if !reflect.DeepEqual(info.Actors, []string{"Actress"}) || !reflect.DeepEqual(info.Tags, []string{"Tag"}) {
		t.Fatalf("bad names: %+v", info)
	}
	if info.IsUncensored == nil || *info.IsUncensored {
		t.Fatal("explicit censored state lost")
	}
	if len(info.SampleImages) != 2 || info.SampleImages[0].DetailURL != "https://images.test/full.jpg" || info.SampleImages[1].ThumbnailURL != "https://images.test/only.jpg" {
		t.Fatalf("bad sample images: %+v", info.SampleImages)
	}
}

func TestJavDBAPILookupPreservesNumberSeparators(t *testing.T) {
	const code = "053026_001"
	for _, tc := range []struct {
		name, searchNumber, detailNumber string
		valid, notFound                  bool
		wantCalls                        int
	}{
		{"search mismatch", "053026-001", "053026-001", false, true, 1},
		{"detail mismatch", code, "053026-001", false, false, 2},
		{"exact match", code, code, true, false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			p := newJavDBAPITestProvider(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				switch r.URL.Path {
				case "/api/v2/search":
					fmt.Fprintf(w, `{"success":1,"data":{"movies":[{"id":"m1","number":%q}]}}`, tc.searchNumber)
				case "/api/v4/movies/m1":
					fmt.Fprintf(w, `{"success":1,"data":{"movie":{"number":%q,"origin_title":"Title"}}}`, tc.detailNumber)
				default:
					t.Errorf("unexpected path: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			})

			client := NewClient(map[Provider]any{ProviderJavDBAPI: p}, newMemoryLookupCache())

			// A previous lookup may have cached the wrong number under this query.
			lookupCacheSetHit(client, "v4:jav:javdb-api:lookup_jav:"+code, &JavInfo{Code: "053026-001", Title: "Wrong"})
			info, err := client.LookupJavByCode(context.Background(), code, ProviderJavDBAPI)
			if (err == nil) != tc.valid || errors.Is(err, ErrNotFound) != tc.notFound {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.valid && (info == nil || info.Code != code) {
				t.Fatalf("wrong movie returned: %+v", info)
			}
			if !tc.valid && info != nil {
				t.Fatalf("mismatched movie returned: %+v", info)
			}
			if calls != tc.wantCalls {
				t.Fatalf("requests=%d want=%d", calls, tc.wantCalls)
			}
		})
	}
}

func TestJavDBAPIErrorsAndCache(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		notFound   bool
	}{
		{"not found", ``, 404, true},
		{"empty search", `{"success":true,"data":{"movies":[]}}`, 200, true},
		{"null search", `{"success":1,"data":{"movies":null}}`, 200, true},
		{"rate limited", ``, 429, false},
		{"blocked", `<html>blocked</html>`, 403, false},
		{"invalid json", `<html>challenge</html>`, 200, false},
		{"unauthorized", `{"success":0,"action":"LoginRequired"}`, 200, false},
		{"missing data", `{"success":1}`, 200, false},
		{"missing movies", `{"success":1,"data":{}}`, 200, false},
		{"malformed movies", `{"success":1,"data":{"movies":{}}}`, 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			p := newJavDBAPITestProvider(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			})

			client := NewClient(map[Provider]any{ProviderJavDBAPI: p}, newMemoryLookupCache())

			for i := 0; i < 2; i++ {
				_, err := client.LookupJavByCode(context.Background(), "ABC-001", ProviderJavDBAPI)
				if err == nil || errors.Is(err, ErrNotFound) != tc.notFound {
					t.Fatalf("error = %v, notFound = %v", err, tc.notFound)
				}
			}
			wantCalls := 2
			if tc.notFound {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("requests = %d, want %d", calls, wantCalls)
			}
		})
	}
}
