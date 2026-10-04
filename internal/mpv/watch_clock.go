package mpv

import "time"

// The clock includes time browsing with seeks, independent of media position or
// speed. Initial loading, explicit pauses, cache waits and EOF are excluded.
type watchClock struct {
	ready, paused, buffering, ended bool
	last                            time.Time
	total                           time.Duration
}

func (c *watchClock) active() bool { return c.ready && !c.paused && !c.buffering && !c.ended }

func (c *watchClock) advance(now time.Time) {
	if !c.last.IsZero() && c.active() && now.After(c.last) {
		c.total += now.Sub(c.last)
	}
	c.last = now
}
