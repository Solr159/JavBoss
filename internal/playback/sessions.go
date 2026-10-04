// Package playback tracks cumulative playback checkpoints without persisting sessions.
package playback

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var (
	ErrExpired  = errors.New("playback session expired")
	ErrInvalid  = errors.New("invalid playback checkpoint")
	ErrCapacity = errors.New("too many playback sessions")
)

const SessionTTL = 24 * time.Hour
const maxSessions = 4096

type session struct {
	videoID, javID, credited int64
	created, touched         time.Time
}

type Sessions struct {
	mu    sync.Mutex
	items map[string]*session
	now   func() time.Time
	add   func(context.Context, int64, int64, int64) error
}

func NewSessions(add func(context.Context, int64, int64, int64) error) *Sessions {
	return &Sessions{items: make(map[string]*session), now: time.Now, add: add}
}

func (s *Sessions) Create(videoID, javID int64) (string, error) {
	if videoID <= 0 || javID < 0 {
		return "", ErrInvalid
	}
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(raw[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for key, item := range s.items {
		if now.Sub(item.touched) >= SessionTTL {
			delete(s.items, key)
		}
	}
	if len(s.items) >= maxSessions {
		return "", ErrCapacity
	}
	s.items[id] = &session{videoID: videoID, javID: javID, created: now, touched: now}
	return id, nil
}

// Report serializes checkpoint updates, including the database write. Memory is
// advanced only after commit. Unknown IDs are never implicitly recreated.
func (s *Sessions) Report(ctx context.Context, id string, videoID, cumulativeMS int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	item := s.items[id]
	if item == nil {
		return ErrExpired
	}
	if now.Sub(item.touched) >= SessionTTL {
		delete(s.items, id)
		return ErrExpired
	}
	if item.videoID != videoID || cumulativeMS < 0 || cumulativeMS > now.Sub(item.created).Milliseconds()+5000 {
		return ErrInvalid
	}
	if delta := cumulativeMS - item.credited; delta > 0 {
		if err := s.add(ctx, item.videoID, item.javID, delta); err != nil {
			return err
		}
		item.credited = cumulativeMS
	}
	item.touched = now
	return nil
}
