package api

import "sync/atomic"

type lanAccessUpdate func(bool, func() error) error

var lanAccessUpdater atomic.Pointer[lanAccessUpdate]

// Only the HTTP listener owns this callback. Its closure serializes rebinding and shutdown.
func setLANAccessUpdater(update lanAccessUpdate) *lanAccessUpdate {
	if update == nil {
		return lanAccessUpdater.Swap(nil)
	}
	return lanAccessUpdater.Swap(&update)
}
func updateLANAccess(enabled bool, save func() error) error {
	if update := lanAccessUpdater.Load(); update != nil {
		return (*update)(enabled, save)
	}
	return save()
}
