package mpv

import "testing"

func TestPlaylistCountsOnlySuccessfulPlaybackAndReplays(t *testing.T) {
	counts := map[int64]int{}
	events := &playlistEvents{}
	events.setCallbacks(map[int64]func(){
		1: func() { counts[1]++ },
		2: func() { counts[2]++ },
		3: func() { counts[3]++ },
	})
	for _, step := range []struct {
		name                 string
		event                playlistEvent
		first, second, third int
	}{
		{"opening is not playback", playlistEvent{Event: "start-file", EntryID: 1}, 0, 0, 0},
		{"loading is not playback", playlistEvent{Event: "file-loaded"}, 0, 0, 0},
		{"first frame", playlistEvent{Event: "playback-restart"}, 1, 0, 0},
		{"seek does not count", playlistEvent{Event: "playback-restart"}, 1, 0, 0},
		{"buffer recovery does not count", playlistEvent{Event: "playback-restart"}, 1, 0, 0},
		{"end first", playlistEvent{Event: "end-file"}, 1, 0, 0},
		{"skip over second", playlistEvent{Event: "start-file", EntryID: 3}, 1, 0, 0},
		{"third fails", playlistEvent{Event: "end-file"}, 1, 0, 0},
		{"reopen first", playlistEvent{Event: "start-file", EntryID: 1}, 1, 0, 0},
		{"reload first", playlistEvent{Event: "file-loaded"}, 1, 0, 0},
		{"replay counts again", playlistEvent{Event: "playback-restart"}, 2, 0, 0},
		{"end replay", playlistEvent{Event: "end-file"}, 2, 0, 0},
		{"untracked single video", playlistEvent{Event: "start-file", EntryID: 4}, 2, 0, 0},
		{"load single", playlistEvent{Event: "file-loaded"}, 2, 0, 0},
		{"single does not reuse playlist callback", playlistEvent{Event: "playback-restart"}, 2, 0, 0},
	} {
		t.Run(step.name, func(t *testing.T) {
			if callback := events.handle(step.event); callback != nil {
				callback()
			}
			if counts[1] != step.first || counts[2] != step.second || counts[3] != step.third {
				t.Fatalf("counts=%v, want %d,%d,%d", counts, step.first, step.second, step.third)
			}
		})
	}
}
