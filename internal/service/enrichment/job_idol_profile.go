package enrichment

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/util"
)

// StartIdolProfileEnrichment periodically fills missing JAV idol profile fields.
func StartIdolProfileEnrichment(ctx context.Context, interval time.Duration) {
	startPeriodicJob(ctx, interval, "idol profile", EnrichIdolProfiles)
}

// EnrichIdolProfiles fills missing profile fields on jav_idol rows.
// For each idol, it tries to find a solo work code, queries AV Wiki, JavDatabase, and JavModel
// concurrently, merges details in that priority order, normalizes Chinese names, and writes the
// completed profile fields back to the database.
func EnrichIdolProfiles(ctx context.Context) error {
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
			avWikiInfo      *jav.ActressInfo
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
				return jav.LookupActressByCode(ctx, code, jav.ProviderJavDatabase)
			}
		}

		var avWikiLookup, javModelLookup idolActressLookup
		if lookupName != "" {
			avWikiLookup = func() (*jav.ActressInfo, error) {
				return jav.LookupActressByJapaneseName(ctx, lookupName, jav.ProviderAVWiki)
			}
			javModelLookup = func() (*jav.ActressInfo, error) {
				return jav.LookupActressByJapaneseName(ctx, lookupName, jav.ProviderJavModel)
			}
		}

		lookupResults := lookupActressProfilesConcurrently(avWikiLookup, javDatabaseLookup, javModelLookup)
		avWikiInfo = lookupResults[0].info
		javDatabaseInfo = lookupResults[1].info
		javModelInfo = lookupResults[2].info
		if lookupErr := lookupResults[0].err; lookupErr != nil && !errors.Is(lookupErr, jav.ErrNotFound) {
			logging.Error("lookup actress (avwiki) failed idol=%d name=%s err=%v", idol.ID, lookupName, lookupErr)
		}
		if lookupErr := lookupResults[1].err; lookupErr != nil && !errors.Is(lookupErr, jav.ErrNotFound) {
			logging.Error("lookup actress (javdatabase) failed idol=%s code=%s err=%v", idol.Name, code, lookupErr)
		}
		if lookupErr := lookupResults[2].err; lookupErr != nil && !errors.Is(lookupErr, jav.ErrNotFound) {
			logging.Error("lookup actress (javmodel) failed idol=%d name=%s err=%v", idol.ID, lookupName, lookupErr)
		}

		info := mergeActressInfosByPriority(avWikiInfo, javDatabaseInfo, javModelInfo)
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
