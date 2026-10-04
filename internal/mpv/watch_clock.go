package mpv

import "time"

// The clock measures wall time spent playing, independent of media position or speed.
type watchClock struct {
	ready, paused, idle, seeking bool
	last                         time.Time
	total                        time.Duration
}

func (c *watchClock) active() bool { return c.ready && !c.paused && !c.idle && !c.seeking }

func (c *watchClock) advance(now time.Time) {
	if !c.last.IsZero() && c.active() && now.After(c.last) {
		c.total += now.Sub(c.last)
	}
	c.last = now
}
