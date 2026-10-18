package playback

import (
	dbpkg "javboss/internal/db"
	"testing"
)

func TestWatchedTimeSlowSubscriberReconnects(t *testing.T) {
	hub := &watchedTimeHub{subscribers: make(map[chan dbpkg.WatchedTimeSnapshot]struct{})}
	slow, unsubscribe := hub.subscribe()
	defer unsubscribe()
	for i := 0; i < 33; i++ {
		hub.publish(dbpkg.WatchedTimeSnapshot{})
	}
	if len(hub.subscribers) != 0 {
		t.Fatal("slow subscriber was retained")
	}
	count := 0
	for range slow {
		count++
	}
	if count != 32 {
		t.Fatalf("buffered events: %d", count)
	}
}
