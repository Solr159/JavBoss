package mpv

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"javboss/internal/common/logging"
)

type playlistEvent struct {
	Event   string `json:"event"`
	EntryID int64  `json:"playlist_entry_id"`
}

type playlistEvents struct {
	conn      io.ReadWriteCloser
	mu        sync.Mutex
	callbacks map[int64]func()
	current   func()
	loaded    bool
	counted   bool
}

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
	events := &playlistEvents{conn: conn}
	go func() {
		defer conn.Close()
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

func (e *playlistEvents) close() { _ = e.conn.Close() }

func (e *playlistEvents) setCallbacks(callbacks map[int64]func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.callbacks = callbacks
}

func (e *playlistEvents) handle(event playlistEvent) func() {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch event.Event {
	case "start-file":
		e.current = e.callbacks[event.EntryID]
		e.loaded, e.counted = false, false
	case "file-loaded":
		e.loaded = true
	case "playback-restart":
		// This event also occurs on seeks and buffering recovery. Only count
		// the first one after a file has successfully loaded.
		if e.loaded && !e.counted {
			e.counted = true
			return e.current
		}
	case "end-file":
		e.current = nil
		e.loaded = false
	}
	return nil
}
