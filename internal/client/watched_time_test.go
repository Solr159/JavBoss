package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWatchReporterPinsRemoteAndActualLocation(t *testing.T) {
	created := make(chan struct{}, 1)
	reported := make(chan int64, 1)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "session=original" {
			t.Errorf("wrong cookie: %q", r.Header.Get("Cookie"))
		}
		if r.Method == http.MethodPost {
			var body struct {
				LocationID int64 `json:"location_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.LocationID != 8 {
				t.Errorf("body=%+v err=%v", body, err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"session_id":"server-issued-session"}`))
			created <- struct{}{}
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/server-issued-session") {
			t.Errorf("unexpected URL %s", r.URL.Path)
		}
		var body struct {
			Total int64 `json:"watched_ms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusNoContent)
		reported <- body.Total
	}))
	defer remote.Close()
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("checkpoint reached new server: %s", r.URL.Path)
		w.WriteHeader(500)
	}))
	defer other.Close()
	c := newTestClient(t, remote.URL, nil)
	factory := c.playbackReporter(42, 8, "session=original")
	if err := c.setRemoteURL(other.URL); err != nil {
		t.Fatal(err)
	}
	r := factory()
	select {
	case <-created:
	case <-time.After(time.Second):
		t.Fatal("no session created")
	}
	r.Update(1500, true)
	r.Wait()
	select {
	case total := <-reported:
		if total != 1500 {
			t.Fatalf("total=%d", total)
		}
	default:
		t.Fatal("missing final checkpoint")
	}
}
