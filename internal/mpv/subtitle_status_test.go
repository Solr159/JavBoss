package mpv

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestSubtitleStatusTransitions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		result   *SubtitleFile
		err      error
		disabled bool
		manual   bool
		existing bool
		want     []string
	}{
		{name: "download", result: &SubtitleFile{Path: "a.srt", Status: "ready"}, want: []string{"searching", "add:ready"}},
		{name: "sidecar", result: &SubtitleFile{Path: "a.srt", Status: "existing"}, want: []string{"searching", "add:existing"}},
		{name: "missing", result: &SubtitleFile{Status: "not_found"}, want: []string{"searching", "not_found"}},
		{name: "ambiguous", result: &SubtitleFile{Status: "ambiguous"}, want: []string{"searching", "ambiguous"}},
		{name: "disabled without local subtitles", disabled: true, result: &SubtitleFile{Status: "disabled"}, want: []string{"disabled"}},
		{name: "disabled with cached subtitles", disabled: true, result: &SubtitleFile{Path: "a.srt", Status: "ready"}, want: []string{"add:ready"}},
		{name: "disabled with sidecar", disabled: true, result: &SubtitleFile{Path: "a.srt", Status: "existing"}, want: []string{"add:existing"}},
		{name: "disabled with existing track", disabled: true, existing: true, want: []string{"existing"}},
		{name: "manual while disabled", disabled: true, manual: true, result: &SubtitleFile{Path: "a.srt", Status: "ready"}, want: []string{"searching", "add:ready"}},
		{name: "disabled during search", result: &SubtitleFile{Status: "disabled"}, want: []string{"searching", "disabled"}},
		{name: "existing track", existing: true, want: []string{"existing"}},
		{name: "manual with existing track", existing: true, manual: true, result: &SubtitleFile{Path: "b.srt", Status: "ready"}, want: []string{"searching", "add:ready"}},
		{name: "network failure", err: errors.New("download failed"), want: []string{"searching", "error"}},
		{name: "timeout", err: context.DeadlineExceeded, want: []string{"searching", "error"}},
		{name: "cancelled", err: context.Canceled, want: []string{"searching"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			commands := make(chan []any, 16)
			previous := dialMPVIPCOverride
			t.Cleanup(func() { dialMPVIPCOverride = previous })
			dialMPVIPCOverride = func(string, time.Duration) (io.ReadWriteCloser, error) {
				client, server := net.Pipe()
				go func() {
					defer server.Close()
					var request struct {
						Command   []any `json:"command"`
						RequestID int64 `json:"request_id"`
					}
					if json.NewDecoder(server).Decode(&request) != nil {
						return
					}
					commands <- request.Command
					var data any
					if request.Command[0] == "get_property" {
						switch request.Command[1] {
						case "user-data/javboss/subtitle-session":
							data = subtitleSession{Generation: 3, Entry: 7, Loaded: true}
						case "track-list":
							data = []map[string]string{}
							if tc.existing {
								data = []map[string]string{{"type": "sub"}}
							}
						}
					}
					_ = json.NewEncoder(server).Encode(map[string]any{"request_id": request.RequestID, "error": "success", "data": data})
				}()
				return client, nil
			}
			searches := 0
			e := &playlistEvents{entryID: 7, loaded: true, subtitles: map[int64]SubtitleResolver{7: func(_ context.Context, request SubtitleRequest) (*SubtitleFile, error) {
				if request.Manual != tc.manual {
					t.Fatal("manual request flag was lost")
				}
				if request.SettingsOnly {
					file := &SubtitleFile{}
					if tc.disabled {
						file.Status = "disabled"
					}
					return file, nil
				}
				if request.CacheOnly {
					if !request.DownloadOnly {
						t.Fatal("cache lookup must skip sidecars")
					}
					return nil, nil
				}
				searches++
				return tc.result, tc.err
			}}}
			if tc.manual {
				event := playlistEvent{Event: "client-message", Args: []string{"javboss-subtitle-load", "3", "7"}}
				callback := e.handle(event)
				if callback == nil {
					t.Fatal("manual request ignored")
				}
				if e.handle(event) != nil {
					t.Fatal("duplicate lookup was started")
				}
				callback()
			} else {
				e.subtitleCallback()()
			}
			var got []string
			for len(commands) > 0 {
				args := <-commands
				if args[0] != "script-message" {
					continue
				}
				if args[2] != "3" || args[3] != "7" {
					t.Fatalf("missing playback guard: %v", args)
				}
				if args[1] == "javboss-subtitle-status" {
					got = append(got, args[4].(string))
				} else if args[1] == "javboss-subtitle-add" && args[4] != "" {
					got = append(got, "add:"+args[8].(string))
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("statuses = %v, want %v", got, tc.want)
			}
			if tc.existing && !tc.manual && searches != 0 {
				t.Fatal("unnecessary subtitle search")
			}
		})
	}
}
