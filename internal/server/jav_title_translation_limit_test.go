package server

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTitleTranslationLimitCancelsQueuedRequestsWithoutLeakingSlotsOrKeys(t *testing.T) {
	releases := []func(){}
	for id := int64(1); id <= titleTranslationConcurrency; id++ {
		release, err := acquireTitleTranslation(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	for _, id := range []int64{1, 100, 0} {
		// Respect cancellation both while waiting for a duplicate title and for capacity.
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		release, err := acquireTitleTranslation(ctx, id)
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
	release, err := acquireTitleTranslation(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	release()
}
