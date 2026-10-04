package mpv

import (
	"encoding/json"
	"testing"
	"time"
)

func TestWatchEventsExcludePauseBufferSeekAndFileLoading(t *testing.T) {
	e := &playlistEvents{watch: watchClock{idle: true}}
	start := time.Now()
	property := func(name string, value bool) playlistEvent {
		data, _ := json.Marshal(value)
		return playlistEvent{Event: "property-change", Name: name, Data: data}
	}
	for _, step := range []struct {
		at    int
		event playlistEvent
		want  int
	}{
		{0, playlistEvent{Event: "start-file", EntryID: 1}, 0},
		{5, playlistEvent{Event: "file-loaded"}, 0},
		{5, property("core-idle", false), 0},
		{5, playlistEvent{Event: "playback-restart"}, 0},
		{15, property("pause", true), 10},
		{25, property("pause", false), 10},
		{30, property("core-idle", true), 15},
		{40, property("core-idle", false), 15},
		{45, playlistEvent{Event: "seek"}, 20},
		{50, property("seeking", false), 20},
		{51, playlistEvent{Event: "playback-restart"}, 20},
		{56, playlistEvent{Event: "end-file"}, 25},
		{70, playlistEvent{Event: "start-file", EntryID: 2}, 0},
		{72, playlistEvent{Event: "end-file"}, 0},
	} {
		e.handleAt(step.event, start.Add(time.Duration(step.at)*time.Second))
		if got := int(e.watch.total / time.Second); got != step.want {
			t.Fatalf("at %d %s: got %d want %d", step.at, step.event.Event, got, step.want)
		}
	}
}
