package translation

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"javboss/internal/db"
	"javboss/internal/models"
)

type batchTestTransport func(*http.Request) (*http.Response, error)

func (f batchTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestApplicationCancellationStopsBothTranslationBatches(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "translation.db"))
	if err != nil {
		t.Fatal(err)
	}
	connection, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	previousStore := db.SetDefault(db.NewStore(database))
	t.Cleanup(func() { db.SetDefault(previousStore) })
	if err := db.Default().UpsertConfig(t.Context(), map[string]string{
		"jav_title_translation_enabled":  "true",
		"jav_series_translation_enabled": "true",
		ProfilesKey:                      `{"deepseek":{"api_key":"key","model":"model"}}`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&models.Jav{Code: "CANCEL-001", Title: "Original title"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&models.JavSeries{Name: "Original series"}).Error; err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{}, 2)
	previousHTTP := SetHTTPClient(&http.Client{Transport: batchTestTransport(func(r *http.Request) (*http.Response, error) {
		entered <- struct{}{}
		<-r.Context().Done()
		return nil, r.Context().Err()
	})})
	ctx, cancel := context.WithCancel(t.Context())
	Init(ctx)
	t.Cleanup(func() {
		cancel()
		WaitBatches()
		SetHTTPClient(previousHTTP)
		Init(context.Background())
	})
	for _, series := range []bool{false, true} {
		status, err := StartBatch(t.Context(), series, "missing", false)
		if err != nil || !status.Running {
			t.Fatalf("start series=%v: %+v, %v", series, status, err)
		}
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("batch did not reach the provider")
		}
	}
	cancel()
	done := make(chan struct{})
	go func() { WaitBatches(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not wait for both batches")
	}
	for _, series := range []bool{false, true} {
		status := GetBatchStatus(series)
		if status.Running || status.Translated != 0 {
			t.Fatalf("cancelled batch series=%v: %+v", series, status)
		}
		items, err := (Domain{Series: series}).List(t.Context())
		if err != nil || len(items) != 1 || items[0].Translation != "" {
			t.Fatalf("cancellation changed saved text: %+v, %v", items, err)
		}
		_, err = StartBatch(t.Context(), series, "missing", false)
		if code, _, _ := ErrorDetails(err); err == nil || code != http.StatusServiceUnavailable {
			t.Fatalf("started a batch after shutdown: %v", err)
		}
	}
}
