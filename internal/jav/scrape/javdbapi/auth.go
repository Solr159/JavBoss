package javdbapi

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"javboss/internal/common/logging"
)

// Authentication failures are not ErrNotFound: blocked lookups must not be
// cached as missing movies. All API clients share the flag until process restart.
// Existing successful metadata cache entries remain usable.
var errAuthenticationFailed = errors.New("javdb-api: authentication failed; requests paused until restart")

var authenticationFailed atomic.Bool

func checkAuthentication(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if authenticationFailed.Load() {
		return errAuthenticationFailed
	}
	return nil
}

func pauseAuthentication(reason string) error {
	if authenticationFailed.CompareAndSwap(false, true) {
		logging.Error("javdb-api authentication failed (%s); requests paused until restart", reason)
	}
	return fmt.Errorf("%w (%s)", errAuthenticationFailed, reason)
}
