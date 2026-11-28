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
