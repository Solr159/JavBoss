package avwiki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"javboss/internal/jav/internal/ratelimit"
	"javboss/internal/jav/metadata"
)

func testClient(t *testing.T, handler http.HandlerFunc) *AVWikiClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &AVWikiClient{baseURL: server.URL, httpClient: server.Client(), limiter: ratelimit.New(0)}
}

func TestLookupExactNameAcrossPages(t *testing.T) {
	var calls int
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/wp-json/wp/v2/tags" || r.URL.Query().Get("search") != "女優名" || r.Header.Get("Accept") != "application/json" {
			t.Errorf("unexpected request: %s headers=%v", r.URL, r.Header)
		}
		name := "女優名別人"
		if r.URL.Query().Get("page") == "2" {
			name = "女優名"
		}
		w.Header().Set("X-WP-TotalPages", "2")
		json.NewEncoder(w).Encode([]actressTag{{Name: name, Link: "http://" + r.Host + "/av-actress/test/", Description: profileFixture(name, "test name", "1992年5月9日", "T160-B85-W60-H88")}})
	})
	info, err := client.LookupActressByName(context.Background(), " 女優 名 ")
	if err != nil || info == nil || info.JapaneseName != "女優名" || info.Bust != 85 || calls != 2 {
		t.Fatalf("info=%+v error=%v calls=%d", info, err, calls)
	}
}

func TestLookupErrorsAndMisses(t *testing.T) {
	for _, tc := range []struct {
		name, body, challenge string
		status                int
		wantNotFound          bool
	}{
		{"empty result", `[]`, "", 200, true},
		{"partial match", `[{"name":"女優名別人"}]`, "", 200, true},
		{"alias only", `[{"name":"別人","description":"女優名"}]`, "", 200, true},
		{"blocked", `<html>Just a moment</html>`, "challenge", 403, false},
		{"rate limit", `{}`, "", 429, false},
		{"api missing", `{}`, "", 404, false},
		{"server error", `{}`, "", 500, false},
		{"html instead of json", `<html>verification</html>`, "", 200, false},
		{"object instead of tags", `{"code":"rest_forbidden"}`, "", 200, false},
		{"null instead of tags", `null`, "", 200, false},
		{"oversized response", strings.Repeat(" ", maxResponseBytes+1), "", 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("cf-mitigated", tc.challenge)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			})
			info, err := client.LookupActressByName(context.Background(), "女優名")
			if info != nil || err == nil || errors.Is(err, metadata.ErrNotFound) != tc.wantNotFound {
				t.Fatalf("info=%+v error=%v wantNotFound=%v", info, err, tc.wantNotFound)
			}
		})
	}
}

func TestLookupRejectsAmbiguousNames(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		tags := []actressTag{}
		for _, slug := range []string{"one", "two"} {
			tags = append(tags, actressTag{Name: "女優名", Link: "http://" + r.Host + "/av-actress/" + slug + "/", Description: profileFixture("女優名", "test name", "", "")})
		}
		json.NewEncoder(w).Encode(tags)
	})
	if info, err := client.LookupActressByName(context.Background(), "女優名"); info != nil || err == nil || errors.Is(err, metadata.ErrNotFound) {
		t.Fatalf("info=%+v error=%v", info, err)
	}
}

func TestCancellationWhileWaitingForLimiter(t *testing.T) {
	var calls atomic.Int32
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `[]`)
	})
	client.limiter = ratelimit.New(time.Hour)
	if err := client.limiter.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := client.LookupActressByName(ctx, "女優名"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("request made after cancellation")
	}
}
