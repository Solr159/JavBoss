package jav

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const javDBAPITestMovie = `{"id":"m1","number":"ABC-001","title":"中文译名","origin_title":"日本語の原題","maker_name":"Maker","maker_id":42,"series_name":"Series","series_id":"s1","release_date":"2025-01-02","duration":"120","cover_url":"https://images.test/cover.jpg","type":0,"tags":[{"name":"Tag"},{"name":"Tag"}],"actors":[{"id":"a1","name":"Actress","gender":0},{"id":"a2","name":"Male","gender":1},{"id":"a3","name":"Unknown"}],"preview_images":[{"thumb_url":"https://images.test/thumb.jpg","large_url":"https://images.test/full.jpg"},{"large_url":"https://images.test/full.jpg"},{"large_url":"https://images.test/only.jpg"}]}`

func newJavDBAPITestProvider(t *testing.T, handler http.HandlerFunc) *javDBAPI {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &javDBAPI{client: server.Client(), baseURL: server.URL}
}

func TestJavDBAPISignature(t *testing.T) {
	if got := javDBAPISignature(1784134914); got != "1784134914.lpw6vgqzsp.85b53cc0034eff62f361723615a3b8e3" {
		t.Fatalf("signature: %s", got)
	}
}

func TestJavDBAPILookup(t *testing.T) {
	var deviceID string
	calls := 0
	p := newJavDBAPITestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		ts, err := strconv.ParseInt(strings.Split(r.Header.Get("jdsignature"), ".")[0], 10, 64)
		if err != nil || r.Header.Get("jdsignature") != javDBAPISignature(ts) {
			t.Error("invalid signature")
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
	original := lookupProvidersByProvider[ProviderJavDBAPI]
	lookupProvidersByProvider[ProviderJavDBAPI] = p
	SetCache(newMemoryLookupCache())
	t.Cleanup(func() {
		lookupProvidersByProvider[ProviderJavDBAPI] = original
		SetCache(nil)
	})
	lookupCacheSetHit("v1:jav:javdb-api:lookup_jav:ABC-001", &JavInfo{Code: "ABC-001", Title: "旧中文标题"})
	lookupCacheSetHit("v2:jav:javdb-api:lookup_jav:ABC-001", &JavInfo{Code: "ABC-001", Title: "日本語の原題"})
	lookupCacheSetHit("v3:jav:javdb-api:lookup_jav:ABC-001", &JavInfo{Code: "ABC-001", Title: "日本語の原題", ZhTitle: "中文译名"})
	info, err := LookupJavByCode(" abc-001 ", ProviderJavDBAPI)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || info.Provider != ProviderJavDBAPI || info.Code != "ABC-001" || info.Title != "日本語の原題" || info.ZhTitle != "中文译名" || info.Studio != "Maker" || info.Series != "Series" || info.DurationMin != 120 || info.ReleaseUnix != parseDateUnix("2025-01-02") || info.CoverURL != "https://images.test/cover.jpg" {
		t.Fatalf("bad metadata: %+v, calls %d", info, calls)
	}
	cached, err := LookupJavByCode("ABC-001", ProviderJavDBAPI)
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

func TestJavDBAPIResolveNumber(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		movies     []javDBAPIMovie
		want       string
		notFound   bool
	}{
		{"exact wins", "abc-001", []javDBAPIMovie{{ID: "other", Number: "ABC001"}, {ID: "exact", Number: "ABC-001"}}, "exact", false},
		{"different separator", "ABC_001", []javDBAPIMovie{{ID: "one", Number: "ABC-001"}}, "", true},
		{"numeric separator mismatch", "053026_001", []javDBAPIMovie{{ID: "one", Number: "053026-001"}}, "", true},
		{"numeric exact wins", "053026_001", []javDBAPIMovie{{ID: "other", Number: "053026-001"}, {ID: "exact", Number: "053026_001"}}, "exact", false},
		{"missing separator", "ABC-001", []javDBAPIMovie{{ID: "one", Number: "ABC001"}}, "", true},
		{"case and whitespace", " abc-001 ", []javDBAPIMovie{{ID: "one", Number: " ABC-001 "}}, "one", false},
		{"duplicates", "ABC-001", []javDBAPIMovie{{ID: "one", Number: "ABC-001"}, {ID: "one", Number: "ABC-001"}}, "one", false},
		{"ambiguous exact", "ABC-001", []javDBAPIMovie{{ID: "one", Number: "ABC-001"}, {ID: "two", Number: "ABC-001"}}, "", false},
		{"no normalized fallback", "ABC_001", []javDBAPIMovie{{ID: "one", Number: "ABC-001"}, {ID: "two", Number: "ABC001"}}, "", true},
		{"no first hit fallback", "ABC-001", []javDBAPIMovie{{ID: "wrong", Number: "ABC-002"}}, "", true},
		{"missing id", "ABC-001", []javDBAPIMovie{{Number: "ABC-001"}}, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveJavDBAPIMovieID(tc.movies, tc.code)
			if got != tc.want || (err != nil) != (tc.want == "") || errors.Is(err, ResourceNotFonud) != tc.notFound {
				t.Fatalf("got %q, %v", got, err)
			}
		})
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
			original := lookupProvidersByProvider[ProviderJavDBAPI]
			lookupProvidersByProvider[ProviderJavDBAPI] = p
			SetCache(newMemoryLookupCache())
			t.Cleanup(func() {
				lookupProvidersByProvider[ProviderJavDBAPI] = original
				SetCache(nil)
			})
			// A previous lookup may have cached the wrong number under this query.
			lookupCacheSetHit("v4:jav:javdb-api:lookup_jav:"+code, &JavInfo{Code: "053026-001", Title: "Wrong"})
			info, err := LookupJavByCode(code, ProviderJavDBAPI)
			if (err == nil) != tc.valid || errors.Is(err, ResourceNotFonud) != tc.notFound {
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
			original := lookupProvidersByProvider[ProviderJavDBAPI]
			lookupProvidersByProvider[ProviderJavDBAPI] = p
			SetCache(newMemoryLookupCache())
			t.Cleanup(func() { lookupProvidersByProvider[ProviderJavDBAPI] = original; SetCache(nil) })
			for i := 0; i < 2; i++ {
				_, err := LookupJavByCode("ABC-001", ProviderJavDBAPI)
				if err == nil || errors.Is(err, ResourceNotFonud) != tc.notFound {
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

func TestJavDBAPIDetailValidationAndLinks(t *testing.T) {
	for _, tc := range []struct {
		name, detail string
		valid        bool
		wantTitle    string
	}{
		{"nested", `{"movie":` + javDBAPITestMovie + `}`, true, "日本語の原題"},
		{"flat", javDBAPITestMovie, true, "日本語の原題"},
		{"original without localized title", `{"movie":{"number":"ABC-001","origin_title":" 日本語の原題 "}}`, true, "日本語の原題"},
		{"missing original title", `{"movie":{"number":"ABC-001","title":" 元のタイトル "}}`, true, "元のタイトル"},
		{"blank original title", `{"movie":{"number":"ABC-001","origin_title":"  ","title":"元のタイトル"}}`, true, "元のタイトル"},
		{"null original title", `{"movie":{"number":"ABC-001","origin_title":null,"title":"元のタイトル"}}`, true, "元のタイトル"},
		{"null", `{"movie":null}`, false, ""},
		{"empty", `{}`, false, ""},
		{"wrong number", `{"movie":{"number":"ABC-002","title":"Wrong"}}`, false, ""},
		{"blank titles", `{"movie":{"number":"ABC-001","title":" ","origin_title":" "}}`, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newJavDBAPITestProvider(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v2/search" {
					fmt.Fprint(w, `{"success":1,"data":{"movies":[{"id":"m1","number":"ABC-001"}]}}`)
					return
				}
				fmt.Fprintf(w, `{"success":1,"data":%s}`, tc.detail)
			})
			info, err := p.LookupJavByCode("ABC-001")
			if (err == nil) != tc.valid || errors.Is(err, ResourceNotFonud) {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.valid {
				return
			}
			if info.Title != tc.wantTitle {
				t.Fatalf("title = %q, want %q", info.Title, tc.wantTitle)
			}
			if tc.name != "nested" && tc.name != "flat" {
				return
			}
			for _, link := range []struct {
				fetch func() (string, error)
				want  string
			}{
				{func() (string, error) { return p.LookupActressURLByCodeAndName("ABC-001", "Actress") }, "https://javdb.com/actors/a1"},
				{func() (string, error) { return p.LookupSeriesURLByCode("ABC-001") }, "https://javdb.com/series/s1"},
				{func() (string, error) { return p.LookupStudioURLByCode("ABC-001") }, "https://javdb.com/makers/42"},
			} {
				got, err := link.fetch()
				if err != nil || got != link.want {
					t.Fatalf("link %q, %v", got, err)
				}
			}
		})
	}
}

func TestJavDBAPIEmptyCodeAndCancellation(t *testing.T) {
	p := newJavDBAPITestProvider(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected request") })
	if _, err := p.LookupJavByCode("  "); !errors.Is(err, ResourceNotFonud) {
		t.Fatalf("empty code: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var dest any
	if err := p.get(ctx, "/api/v2/search", nil, &dest); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request: %v", err)
	}
}

func TestJavDBAPIProviderIdentity(t *testing.T) {
	if ProviderJavDBAPI != 12 || ParseProvider(12) != ProviderJavDBAPI || ProviderJavDBAPI.String() != "javdb-api" {
		t.Fatal("provider identity changed")
	}
	if lookupCacheKey(ProviderJavDBAPI, "lookup_jav", "abc-001") == lookupCacheKey(ProviderJavDB, "lookup_jav", "abc-001") {
		t.Fatal("API and HTML cache keys overlap")
	}
	// Ensure the device ID is not shared by different installations/instances.
	first, second := &javDBAPI{}, &javDBAPI{}
	first.init()
	second.init()
	if first.deviceID == second.deviceID {
		t.Fatal("device IDs are not unique")
	}
}

func TestJavDBAPIUncensoredFromDetailType(t *testing.T) {
	censored, uncensored := false, true
	for _, tc := range []struct {
		name      string
		typeField string
		want      *bool
	}{
		{"censored", `,"type":0`, &censored},
		{"uncensored", `,"type":1`, &uncensored},
		{"numeric string", `,"type":"1"`, &uncensored},
		{"western remains unknown", `,"type":2`, nil},
		{"fc2 remains unknown", `,"type":3`, nil},
		{"unrecognized remains unknown", `,"type":99`, nil},
		{"missing remains unknown", ``, nil},
		{"null remains unknown", `,"type":null`, nil},
		{"search parameter is not detail type", `,"movie_type":0`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newJavDBAPITestProvider(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v2/search" {
					fmt.Fprint(w, `{"success":1,"data":{"movies":[{"id":"m1","number":"ABC-001"}]}}`)
					return
				}
				fmt.Fprintf(w, `{"success":1,"data":{"movie":{"number":"ABC-001","origin_title":"原題"%s}}}`, tc.typeField)
			})
			info, err := p.LookupJavByCode("ABC-001")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(info.IsUncensored, tc.want) {
				t.Fatalf("IsUncensored = %v, want %v", info.IsUncensored, tc.want)
			}
		})
	}
}
