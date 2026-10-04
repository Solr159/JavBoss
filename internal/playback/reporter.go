package playback

import (
	"context"
	"errors"
	"sync"
	"time"

	"javboss/internal/common/logging"
)

// Reporter keeps network/database work off the player's event reader. Updates
// coalesce to the newest cumulative value, so retries cannot double count.
type Reporter struct {
	mu     sync.Mutex
	latest int64
	final  bool
	wake   chan struct{}
	done   chan struct{}
	ctx    context.Context
	cancel context.CancelFunc
	create func(context.Context) (string, error)
	report func(context.Context, string, int64) error
}

func NewReporter(create func(context.Context) (string, error), report func(context.Context, string, int64) error) *Reporter {
	ctx, cancel := context.WithCancel(context.Background())
	r := &Reporter{wake: make(chan struct{}, 1), done: make(chan struct{}), ctx: ctx, cancel: cancel, create: create, report: report}
	go r.run()
	return r
}

func (r *Reporter) Update(totalMS int64, final bool) {
	r.mu.Lock()
	if totalMS > r.latest {
		r.latest = totalMS
	}
	if final && !r.final {
		r.final = true
		// Normal close gets a bounded opportunity to save/retry the tail.
		time.AfterFunc(3*time.Second, r.cancel)
	}
	r.mu.Unlock()
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// Observe refreshes the local clock without scheduling an immediate write.
func (r *Reporter) Observe(totalMS int64) {
	r.mu.Lock()
	if totalMS > r.latest {
		r.latest = totalMS
	}
	r.mu.Unlock()
}

func (r *Reporter) Wait() { <-r.done }

func (r *Reporter) Finished() bool {
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

func (r *Reporter) snapshot() (int64, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.latest, r.final
}

func (r *Reporter) run() {
	defer close(r.done)
	defer r.cancel()
	var id string
	var baseline, acknowledged int64
	for {
		total, final := r.snapshot()
		ctx, cancel := context.WithTimeout(r.ctx, 3*time.Second)
		var err error
		if id == "" {
			// Time before a session can be created is intentionally discarded.
			baseline = total
			id, err = r.create(ctx)
			acknowledged = 0
		}
		if err == nil && id != "" && total-baseline > acknowledged {
			err = r.report(ctx, id, total-baseline)
			if err == nil {
				acknowledged = total - baseline
			}
		}
		cancel()
		if errors.Is(err, ErrExpired) {
			id = ""
		} else if err != nil && r.ctx.Err() == nil {
			logging.Error("save playback checkpoint: %v", err)
		}
		if final && err == nil {
			return
		}
		delay := 10 * time.Second
		if err != nil || final {
			delay = time.Second
		}
		timer := time.NewTimer(delay)
		select {
		case <-r.ctx.Done():
			timer.Stop()
			return
		case <-r.wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}
