package translation

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTitleTranslationLimitCancelsQueuedRequestsWithoutLeakingSlotsOrKeys(t *testing.T) {
	releases := []func(){}
	for id := int64(1); id <= Concurrency; id++ {
		release, err := Acquire(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	for _, id := range []int64{1, 100, 0} {
		// Respect cancellation both while waiting for a duplicate title and for capacity.
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		release, err := Acquire(ctx, id)
		cancel()
		if release != nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("id=%d err=%v", id, err)
		}
	}
	for _, release := range releases {
		release()
	}
	if len(titleTranslationSlots) != 0 {
		t.Fatal("provider slots leaked")
	}
	titleTranslationKeys.Lock()
	remaining := len(titleTranslationKeys.items)
	titleTranslationKeys.Unlock()
	if remaining != 0 {
		t.Fatalf("title locks leaked: %d", remaining)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	release, err := Acquire(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestTranslationLocksSeparateTitlesAndSeriesWithSameID(t *testing.T) {
	titleRelease, err := (Domain{}).Acquire(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer titleRelease()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	seriesRelease, err := (Domain{Series: true}).Acquire(ctx, 1)
	if err != nil {
		t.Fatalf("series blocked by unrelated title: %v", err)
	}
	defer seriesRelease()
	ctx, cancelDuplicate := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelDuplicate()
	if release, err := (Domain{Series: true}).Acquire(ctx, 1); release != nil || !errors.Is(err, context.DeadlineExceeded) {
		if release != nil {
			release()
		}
		t.Fatalf("duplicate series did not serialize: %v", err)
	}
}
