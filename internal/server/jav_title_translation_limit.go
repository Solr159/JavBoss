package server

import (
	"context"
	"sync"
)

const titleTranslationConcurrency = 3

var titleTranslationSlots = make(chan struct{}, titleTranslationConcurrency)
var titleTranslationKeys = struct {
	sync.Mutex
	items map[int64]*titleTranslationKey
}{items: make(map[int64]*titleTranslationKey)}

type titleTranslationKey struct {
	busy chan struct{}
	refs int
}

// Different titles share a global concurrency budget; the same title is serialized
// across pages, tabs, manual requests and the library job before rechecking the DB.
func acquireTitleTranslation(ctx context.Context, id int64) (func(), error) {
	releaseKey := func() {}
	if id != 0 {
		titleTranslationKeys.Lock()
		key := titleTranslationKeys.items[id]
		if key == nil {
			key = &titleTranslationKey{busy: make(chan struct{}, 1)}
			titleTranslationKeys.items[id] = key
		}
		key.refs++
		titleTranslationKeys.Unlock()
		unref := func() {
			titleTranslationKeys.Lock()
			defer titleTranslationKeys.Unlock()
			key.refs--
			if key.refs == 0 {
				delete(titleTranslationKeys.items, id)
			}
		}
		select {
		case key.busy <- struct{}{}:
			releaseKey = func() { <-key.busy; unref() }
		case <-ctx.Done():
			unref()
			return nil, ctx.Err()
		}
	}
	select {
	case titleTranslationSlots <- struct{}{}:
		release := func() { <-titleTranslationSlots; releaseKey() }
		if err := ctx.Err(); err != nil {
			release()
			return nil, err
		}
		return release, nil
	case <-ctx.Done():
		releaseKey()
		return nil, ctx.Err()
	}
}
