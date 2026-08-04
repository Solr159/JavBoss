package client

import (
	"testing"
	"time"
)

func TestRetireScreenshotJobRemovesOnlyInactiveCurrentJob(t *testing.T) {
	now := time.Now()
	client := &Client{screenshotJobs: make(map[int64]*screenshotSyncJob)}
	inactive := &screenshotSyncJob{
		client:        client,
		videoID:       42,
		lastActivated: now.Add(-screenshotSyncIdleTime),
	}
	client.screenshotJobs[inactive.videoID] = inactive

	if !client.retireScreenshotJob(inactive, now) {
		t.Fatal("inactive screenshot job was not retired")
	}
	if client.screenshotJobs[inactive.videoID] != nil {
		t.Fatal("inactive screenshot job remains registered")
	}

	active := &screenshotSyncJob{
		client:        client,
		videoID:       43,
		lastActivated: now,
	}
	client.screenshotJobs[active.videoID] = active
	if client.retireScreenshotJob(active, now) {
		t.Fatal("active screenshot job was retired")
	}
	if client.screenshotJobs[active.videoID] != active {
		t.Fatal("active screenshot job registration changed")
	}

	replacement := &screenshotSyncJob{client: client, videoID: 44, lastActivated: now}
	stale := &screenshotSyncJob{
		client:        client,
		videoID:       replacement.videoID,
		lastActivated: now.Add(-screenshotSyncIdleTime),
	}
	client.screenshotJobs[replacement.videoID] = replacement
	if client.retireScreenshotJob(stale, now) {
		t.Fatal("stale screenshot job removed its replacement")
	}
	if client.screenshotJobs[replacement.videoID] != replacement {
		t.Fatal("replacement screenshot job registration changed")
	}
}
