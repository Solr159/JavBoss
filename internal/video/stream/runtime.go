package stream

import (
	"context"
	"net/http"
	"sync/atomic"
)

var active atomic.Pointer[streamer]

// Start configures transcoding and starts process monitoring. Call once before consumers.
// Canceling ctx stops transcoding and removes its temporary files.
func Start(ctx context.Context, cacheDir string) {
	s := newStreamer(cacheDir)
	active.Store(s)
	s.Start(ctx)
}

// ServeSegment serves a segment through the shared transcoding processes.
func ServeSegment(w http.ResponseWriter, r *http.Request, options Options) {
	s := active.Load()
	if s == nil {
		http.Error(w, "web playback service is unavailable", http.StatusServiceUnavailable)
		return
	}
	s.ServeSegment(w, r, options)
}
