package enrichment

import (
	"sync"

	"javboss/internal/jav"
)

type idolActressLookup func() (*jav.ActressInfo, error)

type idolActressLookupResult struct {
	info *jav.ActressInfo
	err  error
}

func lookupActressProfilesConcurrently(lookups ...idolActressLookup) []idolActressLookupResult {
	results := make([]idolActressLookupResult, len(lookups))
	var workers sync.WaitGroup
	for index, lookup := range lookups {
		if lookup == nil {
			continue
		}
		workers.Add(1)
		go func(index int, lookup idolActressLookup) {
			defer workers.Done()
			results[index].info, results[index].err = lookup()
		}(index, lookup)
	}
	workers.Wait()
	return results
}

func mergeActressInfosByPriority(infos ...*jav.ActressInfo) *jav.ActressInfo {
	var merged *jav.ActressInfo
	for _, info := range infos {
		merged = mergeActressInfo(merged, info)
	}
	return merged
}

func mergeActressInfo(primary, secondary *jav.ActressInfo) *jav.ActressInfo {
	if primary == nil && secondary == nil {
		return nil
	}
	if primary == nil {
		copied := *secondary
		return &copied
	}
	merged := *primary
	if secondary == nil {
		return &merged
	}
	if merged.RomanName == "" {
		merged.RomanName = secondary.RomanName
	}
	if merged.JapaneseName == "" {
		merged.JapaneseName = secondary.JapaneseName
	}
	if merged.ChineseName == "" {
		merged.ChineseName = secondary.ChineseName
	}
	if merged.HeightCM == 0 {
		merged.HeightCM = secondary.HeightCM
	}
	if merged.Bust == 0 {
		merged.Bust = secondary.Bust
	}
	if merged.Waist == 0 {
		merged.Waist = secondary.Waist
	}
	if merged.Hips == 0 {
		merged.Hips = secondary.Hips
	}
	if merged.BirthDate == 0 {
		merged.BirthDate = secondary.BirthDate
	}
	if merged.Cup == 0 {
		merged.Cup = secondary.Cup
	}
	if merged.ProfileURL == "" {
		merged.ProfileURL = secondary.ProfileURL
	}
	return &merged
}
