package service

import (
	"bytes"
	"context"
	"log"
	"reflect"
	"strings"
	"testing"

	"javboss/internal/common/logging"
	"javboss/internal/models"
)

func TestJavScrapeCodesForVideoUsesForcedCodeOnly(t *testing.T) {
	got := javScrapeCodesForVideo("ABC-001 DEF-002.mp4", "XYZ-999")
	want := []string{"XYZ-999"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("javScrapeCodesForVideo() = %#v, want %#v", got, want)
	}
}

func TestForcedJavScrapeCodeSupportsManualOverride(t *testing.T) {
	got := forcedJavScrapeCode(":manual:abc-001")
	if got != "ABC-001" {
		t.Fatalf("forcedJavScrapeCode() = %q, want ABC-001", got)
	}
}

func TestJavLinkOutcomeLabel(t *testing.T) {
	cases := []struct {
		outcome javLinkOutcome
		want    string
	}{
		{javLinkOutcomeFailed, "failed"},
		{javLinkOutcomeAlreadyLinked, "already_linked"},
		{javLinkOutcomeSkippedShort, "skipped_short"},
		{javLinkOutcomeSkippedOverride, "skipped_override"},
		{javLinkOutcomeNoCode, "no_code"},
		{javLinkOutcomeNotFound, "not_found"},
		{javLinkOutcomeLinkedExisting, "linked_existing"},
		{javLinkOutcomeResolved, "resolved"},
	}
	for _, tc := range cases {
		if got := tc.outcome.label(); got != tc.want {
			t.Errorf("outcome %d label() = %q, want %q", tc.outcome, got, tc.want)
		}
	}
}

func TestJavLinkStatsRecordOnlySamplesFreshSkippedFiles(t *testing.T) {
	var stats javLinkStats
	stats.record(javLinkOutcomeNoCode, "file=A.mp4", true)
	stats.record(javLinkOutcomeNoCode, "file=B.mp4", false) // Re-scan: counted but not sampled.
	stats.record(javLinkOutcomeNotFound, "file=C.mp4 codes=ABC-001", true)
	stats.record(javLinkOutcomeResolved, "file=D.mp4", true)
	stats.record(javLinkOutcomeFailed, "file=E.mp4", true)

	s := stats.snapshot()
	if s.noCode != 2 || s.notFound != 1 || s.resolved != 1 || s.failed != 1 {
		t.Fatalf("counters = no_code:%d not_found:%d resolved:%d failed:%d", s.noCode, s.notFound, s.resolved, s.failed)
	}
	if want := []string{"file=A.mp4"}; !reflect.DeepEqual(s.noCodeSamples, want) {
		t.Fatalf("noCodeSamples = %#v, want %#v", s.noCodeSamples, want)
	}
	if want := []string{"file=C.mp4 codes=ABC-001"}; !reflect.DeepEqual(s.notFoundSamples, want) {
		t.Fatalf("notFoundSamples = %#v, want %#v", s.notFoundSamples, want)
	}
}

func TestJavLinkStatsSampleListIsCapped(t *testing.T) {
	var stats javLinkStats
	for i := 0; i < javLinkSampleLimit+5; i++ {
		stats.record(javLinkOutcomeNoCode, "file", true)
	}
	s := stats.snapshot()
	if s.noCode != javLinkSampleLimit+5 {
		t.Fatalf("noCode = %d, want %d", s.noCode, javLinkSampleLimit+5)
	}
	if len(s.noCodeSamples) != javLinkSampleLimit {
		t.Fatalf("len(noCodeSamples) = %d, want %d", len(s.noCodeSamples), javLinkSampleLimit)
	}
}

func TestJavLinkBatchTracksFreshLocationsWithoutDuplicateEnqueue(t *testing.T) {
	ctx := context.Background()
	progress := &directoryScanProgress{}
	ctx = context.WithValue(ctx, directoryScanProgressKey{}, progress)
	batch := &javLinkBatch{
		ctx:            ctx,
		tasks:          make(chan int64, 4),
		seen:           make(map[int64]struct{}),
		freshLocations: make(map[int64]struct{}),
	}

	batch.EnqueueFresh(7)
	batch.Enqueue(7) // Duplicate must not downgrade or double-count freshness.
	batch.Enqueue(8)

	if !batch.isFresh(7) {
		t.Fatal("location 7 should be fresh")
	}
	if batch.isFresh(8) {
		t.Fatal("location 8 should not be fresh")
	}
	s := batch.stats.snapshot()
	if s.enqueued != 2 || s.fresh != 1 {
		t.Fatalf("enqueued=%d fresh=%d, want enqueued=2 fresh=1", s.enqueued, s.fresh)
	}
}

func TestJavLinkBatchLogSummaryReportsSkipReasons(t *testing.T) {
	var buf bytes.Buffer
	previous := log.Default()
	logging.SetLogger(log.New(&buf, "", 0))
	logging.SetColorEnabled(false)
	t.Cleanup(func() {
		logging.SetLogger(previous)
		logging.SetColorEnabled(true)
	})

	batch := &javLinkBatch{}
	batch.stats.recordEnqueued(true)
	batch.stats.record(javLinkOutcomeNoCode, "file=Funny Clip.mp4", true)
	batch.stats.record(javLinkOutcomeNotFound, "file=ABC-001.mp4 codes=ABC-001", true)
	batch.stats.record(javLinkOutcomeResolved, "file=DEF-002.mp4", true)

	batch.logSummary(models.Directory{ID: 7, Path: "/tmp/videos"})

	out := buf.String()
	for _, want := range []string{
		"sync directory jav link summary: id=7 path=/tmp/videos",
		"no_code=1",
		"not_found=1",
		"resolved=1",
		"sync directory jav link no-code files: id=7",
		"Funny Clip.mp4",
		"sync directory jav link not-found files: id=7",
		"ABC-001.mp4 codes=ABC-001",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("log output missing %q:\n%s", want, out)
		}
	}
}

func TestJavLinkBatchLogSummarySkipsEmptyBatch(t *testing.T) {
	var buf bytes.Buffer
	previous := log.Default()
	logging.SetLogger(log.New(&buf, "", 0))
	t.Cleanup(func() { logging.SetLogger(previous) })

	(&javLinkBatch{}).logSummary(models.Directory{ID: 1, Path: "/tmp/videos"})
	if buf.Len() != 0 {
		t.Fatalf("empty batch should not log, got: %s", buf.String())
	}
}
