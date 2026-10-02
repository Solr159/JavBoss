package enrichment

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

func TestMergeActressInfosUsesProviderPriority(t *testing.T) {
	avWikiInfo := &jav.ActressInfo{RomanName: "AV Wiki Roman", HeightCM: 165, Waist: 60, Bust: 85}
	javDatabaseInfo := &jav.ActressInfo{
		RomanName:   "JavDatabase Roman",
		ChineseName: "数据库中文名",
		HeightCM:    170,
		Bust:        88,
	}
	javModelInfo := &jav.ActressInfo{
		RomanName:   "JavModel Roman",
		ChineseName: "模型中文名",
		Bust:        90,
		Cup:         5,
	}

	info := mergeActressInfosByPriority(avWikiInfo, javDatabaseInfo, javModelInfo)
	if info == nil {
		t.Fatal("mergeActressInfosByPriority returned nil")
	}
	if info.RomanName != "AV Wiki Roman" || info.HeightCM != 165 {
		t.Fatalf("avwiki priority fields were replaced: %#v", info)
	}
	if info.Waist != 60 || info.Bust != 85 {
		t.Fatalf("avwiki fallback fields were not used: %#v", info)
	}
	if info.ChineseName != "数据库中文名" {
		t.Fatalf("javdatabase fallback fields were not used: %#v", info)
	}
	if info.Cup != 5 {
		t.Fatalf("javmodel fallback field was not used: %#v", info)
	}
	info = mergeActressInfosByPriority(nil, javDatabaseInfo, javModelInfo)
	if info.RomanName != "JavDatabase Roman" || info.HeightCM != 170 || info.Bust != 88 || info.Cup != 5 {
		t.Fatalf("remaining providers did not fill profile when avwiki was unavailable: %#v", info)
	}
}
