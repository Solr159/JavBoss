package download

import "context"

var downloads = newQueue()

// Start recovers interrupted jobs and starts the application scheduler once.
// Canceling ctx stops the scheduler and its active downloads.
func Start(ctx context.Context) { downloads.Start(ctx) }

// Wake requests a queue check. Calls before Start are coalesced until startup.
func Wake() { downloads.Wake() }

// Cancel stops an active job after the caller persists its canceled status.
func Cancel(id int64) { downloads.Cancel(id) }
