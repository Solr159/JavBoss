package mpv

import (
	"encoding/json"
	"testing"
	"time"
)

func TestWatchEventsCountUnpausedTimeAfterPlaybackStarts(t *testing.T) {
	e := &playlistEvents{}
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
		{1, property("pause", false), 0},
		{2, property("paused-for-cache", true), 0},
		{5, playlistEvent{Event: "file-loaded"}, 0},
		{5, property("core-idle", false), 0},
		{5, playlistEvent{Event: "playback-restart"}, 0},
		{15, property("pause", true), 10},
		{20, playlistEvent{Event: "seek"}, 10},
		{25, property("pause", false), 10},
		{30, property("paused-for-cache", true), 15},
		{35, playlistEvent{Event: "seek"}, 20},
		{40, property("paused-for-cache", false), 25},
		{45, playlistEvent{Event: "seek"}, 30},
		{46, property("core-idle", true), 31},
		{47, property("seeking", true), 32},
		{48, playlistEvent{Event: "seek"}, 33},
		{49, playlistEvent{Event: "seek"}, 34},
		{50, property("seeking", false), 35},
		{51, playlistEvent{Event: "playback-restart"}, 36},
		{56, property("eof-reached", true), 41},
		{60, playlistEvent{Event: "seek"}, 41},
		{65, property("eof-reached", false), 41},
		{66, playlistEvent{Event: "end-file"}, 42},
		{70, playlistEvent{Event: "start-file", EntryID: 2}, 0},
		{71, playlistEvent{Event: "seek"}, 0},
		{72, playlistEvent{Event: "end-file"}, 0},
	} {
		e.handleAt(step.event, start.Add(time.Duration(step.at)*time.Second))
		if got := int(e.watch.total / time.Second); got != step.want {
			t.Fatalf("at %d %s: got %d want %d", step.at, step.event.Event, got, step.want)
		}
	}
}
