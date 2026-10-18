package monitor

import "context"

var applicationSampler = NewSampler()

// Sample returns resource usage using shared sampling history.
func Sample(ctx context.Context, dataPath string, container bool) (Snapshot, error) {
	return applicationSampler.Sample(ctx, dataPath, container)
}
