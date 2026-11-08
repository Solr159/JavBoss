package mpv

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/playback"
)

type playlistEvent struct {
	Event   string          `json:"event"`
	EntryID int64           `json:"playlist_entry_id"`
	Name    string          `json:"name"`
	Data    json.RawMessage `json:"data"`
	Args    []string        `json:"args"`
}

type playlistEvents struct {
	endpoint                string
	entryID                 int64
	subtitles               map[int64]SubtitleResolver
	subtitleSavers          map[int64]SubtitleOffsetSaver
	subtitleSelectionSavers map[int64]SubtitleSelectionSaver
	subtitleSaveDone        chan struct{}
	subtitleCancel          context.CancelFunc
	subtitleContext         context.Context
	conn                    io.ReadWriteCloser
	mu                      sync.Mutex
	callbacks               map[int64]func()
	current                 func()
	loaded                  bool
	counted                 bool
	watch                   watchClock
	reporters               map[int64]func() *playback.Reporter
	reporter                *playback.Reporter
	finished                []*playback.Reporter
	done                    chan struct{}
}

var playlistWatchers sync.WaitGroup

func newPlaylistEvents(endpoint string) (*playlistEvents, error) {
	conn, err := dialMPVIPC(endpoint, ipcCommandTimeout)
	if err != nil {
		return nil, err
	}
	// Complete a request on this connection before loading any media, ensuring
	// the event client exists before the first start-file event is generated.
	if c, ok := conn.(interface{ SetDeadline(time.Time) error }); ok {
		_ = c.SetDeadline(time.Now().Add(ipcCommandTimeout))
	}
	if err := json.NewEncoder(conn).Encode(ipcRequest{Command: []any{"get_property", "idle"}, RequestID: 1}); err != nil {
		conn.Close()
		return nil, err
	}
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("connect playlist event listener: %w", err)
		}
		var response ipcResponse
		if json.Unmarshal(line, &response) == nil && response.RequestID == 1 {
			break
		}
	}
	if c, ok := conn.(interface{ SetDeadline(time.Time) error }); ok {
		_ = c.SetDeadline(time.Time{})
	}
	// Only explicit pauses and EOF stop an already started video's clock.
	for i, name := range []string{"pause", "eof-reached"} {
		if err := json.NewEncoder(conn).Encode(ipcRequest{Command: []any{"observe_property", i + 1, name}, RequestID: int64(i + 2)}); err != nil {
			conn.Close()
			return nil, err
		}
	}
	events := &playlistEvents{conn: conn, done: make(chan struct{}), endpoint: endpoint}
	playlistWatchers.Add(1)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-events.done:
				return
			case <-ticker.C:
				events.mu.Lock()
				events.watch.advance(time.Now())
				if events.reporter != nil {
					events.reporter.Observe(events.watch.total.Milliseconds())
				}
				events.mu.Unlock()
			}
		}
	}()
	go func() {
		defer playlistWatchers.Done()
		defer close(events.done)
		defer conn.Close()
		defer func() {
			events.mu.Lock()
			if events.subtitleCancel != nil {
				events.subtitleCancel()
			}
			events.watch.advance(time.Now())
			events.checkpoint(true)
			finished := events.finished
			savesDone := events.subtitleSaveDone
			events.mu.Unlock()
			for _, reporter := range finished {
				reporter.Wait()
			}
			if savesDone != nil {
				<-savesDone
			}
		}()
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}
			var event playlistEvent
			if json.Unmarshal(line, &event) == nil {
				if callback := events.handle(event); callback != nil {
					// Database/network updates must not block MPV's event stream.
					go func() {
						defer func() {
							if value := recover(); value != nil {
								logging.Error("playlist playback callback panicked: %v", value)
							}
						}()
						callback()
					}()
				}
			}
		}
	}()
	return events, nil
}

func (e *playlistEvents) close() {
	_ = e.conn.Close()
	if e.done != nil {
		<-e.done
	}
}

func (e *playlistEvents) setWatchReporters(reporters map[int64]func() *playback.Reporter) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.reporters = reporters
}

// checkpoint is called under mu; Reporter.Update only coalesces in memory.
func (e *playlistEvents) checkpoint(final bool) {
	if e.reporter == nil {
		return
	}
	e.reporter.Update(e.watch.total.Milliseconds(), final)
	if final {
		// A reusable player can live indefinitely; retain only pending tail writes.
		pending := make([]*playback.Reporter, 0, len(e.finished)+1)
		for _, reporter := range e.finished {
			if !reporter.Finished() {
				pending = append(pending, reporter)
			}
		}
		e.finished = pending
		e.finished = append(e.finished, e.reporter)
		e.reporter = nil
	}
}

func (e *playlistEvents) setCallbacks(callbacks map[int64]func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.callbacks = callbacks
}

func (e *playlistEvents) handle(event playlistEvent) func() {
	return e.handleAt(event, time.Now())
}

func (e *playlistEvents) handleAt(event playlistEvent, now time.Time) func() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.watch.advance(now)
	wasActive := e.watch.active()
	switch event.Event {
	case "client-message":
		if len(event.Args) > 0 && event.Args[0] == "javboss-subtitle-selection" {
			return e.subtitleSelectionCallback(event.Args)
		}
		if len(event.Args) > 0 && event.Args[0] == "javboss-subtitle-load" {
			return e.subtitleLoadRequest(event.Args)
		}
		return e.subtitleSaveCallback(event.Args)
	case "start-file":
		if e.subtitleCancel != nil {
			e.subtitleCancel()
			e.subtitleCancel = nil
		}
		e.entryID = event.EntryID
		e.checkpoint(true)
		e.watch.total = 0
		e.watch.ready = false
		if factory := e.reporters[event.EntryID]; factory != nil {
			e.reporter = factory()
		}
		e.current = e.callbacks[event.EntryID]
		e.loaded, e.counted = false, false
	case "file-loaded":
		e.loaded = true
		return e.subtitleCallback()
	case "playback-restart":
		e.watch.ready = e.loaded
		// This event also occurs on seeks and buffering recovery. Only count
		// the first one after a file has successfully loaded.
		if e.loaded && !e.counted {
			e.counted = true
			return e.current
		}
	case "end-file":
		if e.subtitleCancel != nil {
			e.subtitleCancel()
			e.subtitleCancel = nil
		}
		e.checkpoint(true)
		e.watch.ready = false
		e.current = nil
		e.loaded = false
	case "property-change":
		var value bool
		if json.Unmarshal(event.Data, &value) == nil && string(event.Data) != "null" {
			switch event.Name {
			case "pause":
				e.watch.paused = value
			case "eof-reached":
				e.watch.ended = value
			}
		}
	}
	if wasActive && !e.watch.active() {
		e.checkpoint(false)
	}
	return nil
}

func (e *playlistEvents) setSubtitles(resolvers map[int64]SubtitleResolver, savers map[int64]SubtitleOffsetSaver, enabledSavers map[int64]SubtitleSelectionSaver) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.subtitles = resolvers
	e.subtitleSavers = savers
	e.subtitleSelectionSavers = enabledSavers
}
