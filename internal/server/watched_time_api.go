package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	dbpkg "javboss/internal/db"
)

type watchedTimeHub struct {
	mu          sync.Mutex
	subscribers map[chan dbpkg.WatchedTimeSnapshot]struct{}
}

var watchedTimeEvents = &watchedTimeHub{subscribers: make(map[chan dbpkg.WatchedTimeSnapshot]struct{})}

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

func saveWatchedTime(ctx context.Context, videoID, javID, deltaMS int64) error {
	update, err := dbpkg.AddWatchedTimeWithTotals(ctx, videoID, javID, deltaMS)
	if err == nil && (len(update.Videos) > 0 || len(update.Javs) > 0) {
		watchedTimeEvents.publish(update)
	}
	return err
}

// GET /videos/watched-time?video_ids=1,2&jav_ids=3 returns current totals.
// Each comma-separated ID list is limited to 200 positive IDs; missing rows are omitted.
func getWatchedTime(c *gin.Context) {
	parse := func(raw string) ([]int64, error) {
		if strings.TrimSpace(raw) == "" {
			return nil, nil
		}
		parts := strings.Split(raw, ",")
		if len(parts) > 200 {
			return nil, fmt.Errorf("too many ids")
		}
		ids := make([]int64, 0, len(parts))
		for _, part := range parts {
			id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if err != nil || id <= 0 {
				return nil, fmt.Errorf("invalid id")
			}
			ids = append(ids, id)
		}
		return ids, nil
	}
	videoIDs, videoErr := parse(c.Query("video_ids"))
	javIDs, javErr := parse(c.Query("jav_ids"))
	if videoErr != nil || javErr != nil {
		respondLocalizedError(c, http.StatusBadRequest, "观看时长查询参数无效", "Invalid watched time query")
		return
	}
	snapshot, err := dbpkg.GetWatchedTimes(c.Request.Context(), videoIDs, javIDs)
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "加载观看时长失败", "Failed to load watched time")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, snapshot)
}

// GET /videos/watched-time/events streams committed totals as watched-time SSE
// events. On every connection (including reconnects), clients query a snapshot.
func streamWatchedTime(c *gin.Context) {
	updates, unsubscribe := watchedTimeEvents.subscribe()
	defer unsubscribe()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("X-Accel-Buffering", "no")
	controller := http.NewResponseController(c.Writer)
	write := func(data string) bool {
		// Override the server's ordinary 30s response timeout for this stream,
		// while keeping each individual write bounded for disconnected clients.
		_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_, err := fmt.Fprint(c.Writer, data)
		if err == nil {
			err = controller.Flush()
		}
		_ = controller.SetWriteDeadline(time.Time{})
		return err == nil
	}
	if !write("retry: 1000\n: connected\n\n") {
		return
	}
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	// Reconnect periodically so the authentication middleware rechecks the session.
	lifetime := time.NewTimer(5 * time.Minute)
	defer lifetime.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-lifetime.C:
			return
		case <-heartbeat.C:
			if !write(": heartbeat\n\n") {
				return
			}
		case update, ok := <-updates:
			if !ok {
				return
			}
			data, err := json.Marshal(update)
			if err != nil || !write("event: watched-time\ndata: "+string(data)+"\n\n") {
				return
			}
		}
	}
}
