package mpv

import "time"

// After playback starts, count elapsed time until pause or EOF. Seeking and
// buffering do not affect the clock; media position and speed are irrelevant.
type watchClock struct {
	ready, paused, ended bool
	last                 time.Time
	total                time.Duration
}

func (c *watchClock) active() bool { return c.ready && !c.paused && !c.ended }

func (c *watchClock) advance(now time.Time) {
	if !c.last.IsZero() && c.active() && now.After(c.last) {
		c.total += now.Sub(c.last)
	}
	c.last = now
}
