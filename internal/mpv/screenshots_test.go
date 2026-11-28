package mpv

import (
	"context"
	"testing"
)

func TestScreenshotRequestsCancelOnSwitchAndRejectWrongEntry(t *testing.T) {
	started := make(chan context.Context, 2)
	done := make(chan struct{}, 2)
	e := &playlistEvents{}
	e.setScreenshots(map[int64]ScreenshotResolver{1: func(ctx context.Context) ([]float64, error) {
		started <- ctx
		<-ctx.Done()
		return nil, ctx.Err()
	}})
	e.handle(playlistEvent{Event: "start-file", EntryID: 1})
	e.handle(playlistEvent{Event: "file-loaded"})
	request := func(generation, entry string) func() {
		return e.handle(playlistEvent{Event: "client-message", Args: []string{"javboss-screenshot-request", generation, entry}})
	}
	if request("1", "2") != nil || request("bad", "1") != nil {
		t.Fatal("accepted invalid request")
	}
	run := request("1", "1")
	if run == nil {
		t.Fatal("missing request callback")
	}
	go func() { run(); done <- struct{}{} }()
	ctx := <-started
	if request("1", "1") != nil {
		t.Fatal("overlapping refresh accepted")
	}
	e.handle(playlistEvent{Event: "start-file", EntryID: 1})
	if ctx.Err() != context.Canceled {
		t.Fatal("replaying the same entry did not cancel the old refresh")
	}
	<-done
	e.handle(playlistEvent{Event: "file-loaded"})
	run = request("2", "1")
	if run == nil {
		t.Fatal("refresh did not resume on replay")
	}
	go func() { run(); done <- struct{}{} }()
	ctx = <-started
	e.handle(playlistEvent{Event: "end-file"})
	if ctx.Err() != context.Canceled {
		t.Fatal("ending playback did not cancel the refresh")
	}
	<-done
	if request("2", "1") != nil {
		t.Fatal("refresh accepted after end-file")
	}
}

func TestScreenshotDeletionRejectsStaleSnapshotAndCancelsOnSwitch(t *testing.T) {
	calls := 0
	e := &playlistEvents{loaded: true, entryID: 1, screenshotSnapshot: &screenshotSnapshot{generation: "2", seconds: []float64{2.5}}}
	e.setScreenshotDeleters(map[int64]ScreenshotDeleter{1: func(context.Context, float64) error { calls++; return nil }})
	request := func(generation, entry, second string) func() {
		return e.handle(playlistEvent{Event: "client-message", Args: []string{"javboss-screenshot-delete", generation, entry, second}})
	}
	for _, args := range [][3]string{{"1", "1", "2.5"}, {"2", "1", "99"}, {"2", "2", "2.5"}, {"2", "1", "NaN"}} {
		if callback := request(args[0], args[1], args[2]); callback != nil {
			callback()
		}
		if calls != 0 {
			t.Fatalf("invalid request deleted a screenshot: %v", args)
		}
	}
	callback := request("2", "1", "2.5")
	if callback == nil {
		t.Fatal("valid deletion request was rejected")
	}
	e.handle(playlistEvent{Event: "start-file", EntryID: 2})
	callback()
	if calls != 0 {
		t.Fatal("canceled request deleted a screenshot after file switch")
	}
}
