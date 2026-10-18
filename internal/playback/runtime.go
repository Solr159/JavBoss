package playback

import "context"

var applicationSessions = NewSessions(SaveWatchedTime)

func CreateSession(videoID, javID int64) (string, error) {
	return applicationSessions.Create(videoID, javID)
}

func ReportSession(ctx context.Context, id string, videoID, cumulativeMS int64) error {
	return applicationSessions.Report(ctx, id, videoID, cumulativeMS)
}
