package javdbapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"javboss/internal/jav/metadata"
)

const javDBAPITestMovie = `{"id":"m1","number":"ABC-001","title":"中文译名","origin_title":"日本語の原題","maker_name":"Maker","maker_id":42,"series_name":"Series","series_id":"s1","release_date":"2025-01-02","duration":"120","cover_url":"https://images.test/cover.jpg","type":0,"tags":[{"name":"Tag"},{"name":"Tag"}],"actors":[{"id":"a1","name":"Actress","gender":0},{"id":"a2","name":"Male","gender":1},{"id":"a3","name":"Unknown"}],"preview_images":[{"thumb_url":"https://images.test/thumb.jpg","large_url":"https://images.test/full.jpg"},{"large_url":"https://images.test/full.jpg"},{"large_url":"https://images.test/only.jpg"}]}`

func newJavDBAPITestProvider(t *testing.T, handler http.HandlerFunc) *JavDBAPIClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &JavDBAPIClient{httpClient: server.Client(), baseURL: server.URL}
}

func TestJavDBAPISignature(t *testing.T) {
	if got := javDBAPISignature(1784134914); got != "1784134914.lpw6vgqzsp.85b53cc0034eff62f361723615a3b8e3" {
		t.Fatalf("signature: %s", got)
	}
}

func TestJavDBAPIStudioUsesMakerIndependentlyOfPublisher(t *testing.T) {
	// SSNI-987 returns S1 as maker while both publisher fields are null.
	for _, publisher := range []string{
		`"publisher_name":null,"publisher_id":null`,
		`"publisher_name":"Other Label","publisher_id":"other"`,
	} {
		t.Run(publisher, func(t *testing.T) {
			p := newJavDBAPITestProvider(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v2/search" {
					fmt.Fprint(w, `{"success":1,"data":{"movies":[{"id":"MZB0P","number":"SSNI-987"}]}}`)
					return
				}
				fmt.Fprintf(w, `{"success":1,"data":{"movie":{"number":"SSNI-987","origin_title":"Fixture title","maker_name":"S1 NO.1 STYLE","maker_id":"7R",%s}}}`, publisher)
			})
			info, err := p.LookupJavByCode(context.Background(), "SSNI-987")
			if err != nil || info == nil || info.Studio != "S1 NO.1 STYLE" {
				t.Fatalf("studio metadata: info=%+v err=%v", info, err)
			}
			link, err := p.LookupStudioURLByCode(context.Background(), "SSNI-987")
			if err != nil || link != "https://javdb.com/makers/7R" {
				t.Fatalf("studio link: %q, %v", link, err)
			}
		})
	}
}

func TestJavDBAPIResolveNumber(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		movies     []javDBAPIMovie
		want       string
		notFound   bool
	}{
		{"fc2 ppv alias", "FC2-PPV-1234567", []javDBAPIMovie{{ID: "one", Number: "FC2-1234567"}}, "one", false},
		{"fc2 returned ppv alias", "FC2-1234567", []javDBAPIMovie{{ID: "one", Number: "fc2-ppv-1234567"}}, "one", false},
		{"fc2 different digits", "FC2-PPV-1234567", []javDBAPIMovie{{ID: "one", Number: "FC2-1234568"}}, "", true},
		{"fc2 no prefix match", "FC2-PPV-1234567", []javDBAPIMovie{{ID: "one", Number: "FC2-12345678"}}, "", true},
		{"fc2 requires digits", "FC2-PPV-123ABC", []javDBAPIMovie{{ID: "one", Number: "FC2-123ABC"}}, "", true},
		{"fc2 ambiguous aliases", "FC2-PPV-1234567", []javDBAPIMovie{{ID: "one", Number: "FC2-1234567"}, {ID: "two", Number: "FC2-PPV-1234567"}}, "", false},
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
			if got != tc.want || (err != nil) != (tc.want == "") || errors.Is(err, metadata.ErrNotFound) != tc.notFound {
				t.Fatalf("got %q, %v", got, err)
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
			info, err := p.LookupJavByCode(context.Background(), "ABC-001")
			if (err == nil) != tc.valid || errors.Is(err, metadata.ErrNotFound) {
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
				{func() (string, error) {
					return p.LookupActressURLByCodeAndName(context.Background(), "ABC-001", "Actress")
				}, "https://javdb.com/actors/a1"},
				{func() (string, error) { return p.LookupSeriesURLByCode(context.Background(), "ABC-001") }, "https://javdb.com/series/s1"},
				{func() (string, error) { return p.LookupStudioURLByCode(context.Background(), "ABC-001") }, "https://javdb.com/makers/42"},
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
	if _, err := p.LookupJavByCode(context.Background(), "  "); !errors.Is(err, metadata.ErrNotFound) {
		t.Fatalf("empty code: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var dest any
	if err := p.get(ctx, "/api/v2/search", nil, &dest); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request: %v", err)
	}
}

func TestJavDBAPIFC2Detail(t *testing.T) {
	for _, tc := range []struct {
		name, input, number string
		valid               bool
	}{
		{"short number", "FC2-PPV-1234567", "FC2-1234567", true},
		{"full number", " fc2-ppv-1234567 ", "FC2-PPV-1234567", true},
		{"short input", "FC2-1234567", "fc2-ppv-1234567", true},
		{"wrong detail", "FC2-PPV-1234567", "FC2-1234568", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newJavDBAPITestProvider(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v2/search" {
					if got := r.URL.Query().Get("q"); got != "FC2-1234567" {
						t.Errorf("query = %q, want FC2-1234567", got)
					}
					fmt.Fprint(w, `{"success":1,"data":{"movies":[{"id":"m1","number":"FC2-PPV-1234567"}]}}`)
					return
				}
				fmt.Fprintf(w, `{"success":1,"data":{"movie":{"number":%q,"origin_title":"Title"}}}`, tc.number)
			})
			info, err := p.LookupJavByCode(context.Background(), tc.input)
			if !tc.valid {
				if err == nil || info != nil {
					t.Fatalf("accepted mismatched detail: %+v, %v", info, err)
				}
				return
			}
			if err != nil || info.Code != "FC2-PPV-1234567" {
				t.Fatalf("result = %+v, %v; want canonical FC2 code", info, err)
			}
		})
	}
}

func TestJavDBAPILookupCancelsInFlightRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	p := newJavDBAPITestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := p.LookupJavByCode(ctx, "ABC-001")
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("lookup ignored request cancellation")
	}
}

func TestAPIClientsAreIndependent(t *testing.T) {
	first, second := New(&http.Client{}, ""), New(&http.Client{}, "")
	first.init()
	second.init()
	if first.httpClient == second.httpClient || first.limiter == second.limiter || first.deviceID == second.deviceID {
		t.Fatal("API clients share transport, limiter or device identity")
	}
}

func TestJavDBAPIProviderIdentity(t *testing.T) {
	if metadata.ProviderJavDBAPI != 12 || metadata.ParseProvider(12) != metadata.ProviderJavDBAPI || metadata.ProviderJavDBAPI.String() != "javdb-api" {
		t.Fatal("provider identity changed")
	}

	// Ensure the device ID is not shared by different installations/instances.
	first, second := &JavDBAPIClient{}, &JavDBAPIClient{}
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
			info, err := p.LookupJavByCode(context.Background(), "ABC-001")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(info.IsUncensored, tc.want) {
				t.Fatalf("IsUncensored = %v, want %v", info.IsUncensored, tc.want)
			}
		})
	}
}
