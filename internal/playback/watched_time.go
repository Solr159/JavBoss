package playback

import (
	"context"
	dbpkg "javboss/internal/db"
	"sync"
)

type watchedTimeHub struct {
	mu          sync.Mutex
	subscribers map[chan dbpkg.WatchedTimeSnapshot]struct{}
}

func (h *watchedTimeHub) subscribe() (<-chan dbpkg.WatchedTimeSnapshot, func()) {
	ch := make(chan dbpkg.WatchedTimeSnapshot, 32)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subscribers[ch]; ok {
			delete(h.subscribers, ch)
			close(ch)
		}
	}
}

func (h *watchedTimeHub) publish(update dbpkg.WatchedTimeSnapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subscribers {
		select {
		case ch <- update:
		default:
			// A slow client reconnects and queries a snapshot; never block playback
			// or silently leave a subscriber with a missing final checkpoint.
			delete(h.subscribers, ch)
			close(ch)
		}
	}
}

func SaveWatchedTime(ctx context.Context, videoID, javID, deltaMS int64) error {
	update, err := dbpkg.Default().AddWatchedTimeWithTotals(ctx, videoID, javID, deltaMS)
	if err == nil && (len(update.Videos) > 0 || len(update.Javs) > 0) {
		watchedTimeEvents.publish(update)
	}
	return err
}

var watchedTimeEvents = &watchedTimeHub{subscribers: make(map[chan dbpkg.WatchedTimeSnapshot]struct{})}

func SubscribeWatchedTime() (<-chan dbpkg.WatchedTimeSnapshot, func()) {
	return watchedTimeEvents.subscribe()
}
