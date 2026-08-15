package service

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"sync"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/util"
)

// StartIdolProfileScanner periodically scans JAV idols with incomplete profile data.
// It runs ScanIdolProfiles immediately and then on every interval until ctx is done, filling
// missing profile fields such as names, measurements, birth date, and profile URL from external
// actress metadata providers.
func StartIdolProfileScanner(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if err := ScanIdolProfiles(ctx); err != nil {
				logging.Error("idol profile scan failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// ScanIdolProfiles scans jav_idol rows that are missing profile fields.
// For each idol, it tries to find a solo work code, queries MinnanoAV, JavDatabase, and JavModel
// concurrently, merges details in that priority order, normalizes Chinese names, and writes the
// completed profile fields back to the database.
func ScanIdolProfiles(ctx context.Context) error {
	idols, err := db.ListIdolsMissingProfile(ctx)
	if err != nil {
		return err
	}
	rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(idols), func(i, j int) {
		idols[i], idols[j] = idols[j], idols[i]
	})
	logging.Info("found %d idols missing profile info", len(idols))
	for _, idol := range idols {
		if err := ctx.Err(); err != nil {
			return err
		}
		lookupName := strings.TrimSpace(idol.JapaneseName)
		if lookupName == "" {
			lookupName = strings.TrimSpace(idol.Name)
		}
		var (
			javDatabaseInfo *jav.ActressInfo
			minnanoAVInfo   *jav.ActressInfo
			javModelInfo    *jav.ActressInfo
			code            string
		)
		code, err = db.FindIdolSoloCode(ctx, idol.ID)
		if err != nil {
			logging.Error("find solo code failed idol=%s err=%v", idol.Name, err)
		}

		var javDatabaseLookup idolActressLookup
		if code != "" {
			javDatabaseLookup = func() (*jav.ActressInfo, error) {
				return jav.LookupActressByCode(code, jav.ProviderJavDatabase)
			}
		}

		var minnanoAVLookup, javModelLookup idolActressLookup
		if lookupName != "" {
			minnanoAVLookup = func() (*jav.ActressInfo, error) {
				return jav.LookupActressByJapaneseName(lookupName, jav.ProviderMinnanoAV)
			}
			javModelLookup = func() (*jav.ActressInfo, error) {
				return jav.LookupActressByJapaneseName(lookupName, jav.ProviderJavModel)
			}
		}

		lookupResults := lookupActressProfilesConcurrently(minnanoAVLookup, javDatabaseLookup, javModelLookup)
		minnanoAVInfo = lookupResults[0].info
		javDatabaseInfo = lookupResults[1].info
		javModelInfo = lookupResults[2].info
		if lookupErr := lookupResults[0].err; lookupErr != nil && !errors.Is(lookupErr, jav.ResourceNotFonud) {
			logging.Error("lookup actress (minnanoav) failed idol=%d name=%s err=%v", idol.ID, lookupName, lookupErr)
		}
		if lookupErr := lookupResults[1].err; lookupErr != nil && !errors.Is(lookupErr, jav.ResourceNotFonud) {
			logging.Error("lookup actress (javdatabase) failed idol=%s code=%s err=%v", idol.Name, code, lookupErr)
		}
		if lookupErr := lookupResults[2].err; lookupErr != nil && !errors.Is(lookupErr, jav.ResourceNotFonud) {
			logging.Error("lookup actress (javmodel) failed idol=%d name=%s err=%v", idol.ID, lookupName, lookupErr)
		}

		info := mergeActressInfosByPriority(minnanoAVInfo, javDatabaseInfo, javModelInfo)
		if info == nil {
			continue
		}
		if info.ChineseName != "" {
			info.ChineseName = util.SimplifyChineseName(info.ChineseName)
		}
		updated, err := db.UpdateIdolProfile(ctx, idol.ID, info)
		if err != nil {
			logging.Error("update idol profile failed idol=%d name=%s err=%v", idol.ID, idol.Name, err)
			continue
		}
		if updated {
			logging.Info("idol profile updated idol=%d name=%s code=%s", idol.ID, idol.Name, code)
		}
	}
	return nil
}

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
