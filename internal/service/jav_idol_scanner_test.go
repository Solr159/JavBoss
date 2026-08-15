package service

import (
	"fmt"
	"testing"
	"time"

	"javboss/internal/jav"
)

func TestLookupActressProfilesConcurrently(t *testing.T) {
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	lookups := make([]idolActressLookup, 3)
	for index := range lookups {
		index := index
		lookups[index] = func() (*jav.ActressInfo, error) {
			started <- struct{}{}
			<-release
			return &jav.ActressInfo{JapaneseName: fmt.Sprintf("actress-%d", index)}, nil
		}
	}

	resultChannel := make(chan []idolActressLookupResult, 1)
	go func() {
		resultChannel <- lookupActressProfilesConcurrently(lookups...)
	}()

	for index := 0; index < len(lookups); index++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatalf("lookup %d did not start concurrently", index)
		}
	}
	close(release)

	results := <-resultChannel
	for index, result := range results {
		if result.err != nil {
			t.Fatalf("lookup %d: %v", index, result.err)
		}
		wantName := fmt.Sprintf("actress-%d", index)
		if result.info == nil || result.info.JapaneseName != wantName {
			t.Fatalf("lookup %d result = %#v, want %s", index, result.info, wantName)
		}
	}
}
