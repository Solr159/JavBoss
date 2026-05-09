package service

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"time"

	"pornboss/internal/common/logging"
	"pornboss/internal/db"
	"pornboss/internal/jav"
	"pornboss/internal/util"
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
// For each idol, it tries to find a solo work code to query JavDatabase, also looks up the idol by
// Japanese/name data in JavModel, merges the returned actress details, normalizes Chinese names,
// and writes the completed profile fields back to the database.
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
			javModelInfo    *jav.ActressInfo
			code            string
		)
		code, err = db.FindIdolSoloCode(ctx, idol.ID)
		if err != nil {
			logging.Error("find solo code failed idol=%s err=%v", idol.Name, err)
		}
		if code != "" {
			javDatabaseInfo, err = jav.LookupActressByCode(code, jav.ProviderJavDatabase)
			if err != nil && !errors.Is(err, jav.ResourceNotFonud) {
				logging.Error("lookup actress failed idol=%s code=%s err=%v", idol.Name, code, err)
			}
		}

		javModelInfo, err = jav.LookupActressByJapaneseName(lookupName, jav.ProviderJavModel)
		if err != nil && !errors.Is(err, jav.ResourceNotFonud) {
			logging.Error("lookup actress (javmodel) failed idol=%d name=%s err=%v", idol.ID, lookupName, err)
		}

		info := mergeActressInfo(javDatabaseInfo, javModelInfo)
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
