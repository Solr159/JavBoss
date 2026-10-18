package enrichment

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/db"
	"javboss/internal/jav/scrape"
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
	idols, err := db.Default().ListIdolsMissingProfile(ctx)
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
			javDatabaseInfo *scrape.ActressInfo
			avWikiInfo      *scrape.ActressInfo
			javModelInfo    *scrape.ActressInfo
			code            string
		)
		code, err = db.Default().FindIdolSoloCode(ctx, idol.ID)
		if err != nil {
			logging.Error("find solo code failed idol=%s err=%v", idol.Name, err)
		}

		var javDatabaseLookup idolActressLookup
		if code != "" {
			javDatabaseLookup = func() (*scrape.ActressInfo, error) {
				return scrape.LookupActressByCode(ctx, code, scrape.ProviderJavDatabase)
			}
		}

		var avWikiLookup, javModelLookup idolActressLookup
		if lookupName != "" {
			avWikiLookup = func() (*scrape.ActressInfo, error) {
				return scrape.LookupActressByJapaneseName(ctx, lookupName, scrape.ProviderAVWiki)
			}
			javModelLookup = func() (*scrape.ActressInfo, error) {
				return scrape.LookupActressByJapaneseName(ctx, lookupName, scrape.ProviderJavModel)
			}
		}

		lookupResults := lookupActressProfilesConcurrently(avWikiLookup, javDatabaseLookup, javModelLookup)
		avWikiInfo = lookupResults[0].info
		javDatabaseInfo = lookupResults[1].info
		javModelInfo = lookupResults[2].info
		if lookupErr := lookupResults[0].err; lookupErr != nil && !errors.Is(lookupErr, scrape.ErrNotFound) {
			logging.Error("lookup actress (avwiki) failed idol=%d name=%s err=%v", idol.ID, lookupName, lookupErr)
		}
		if lookupErr := lookupResults[1].err; lookupErr != nil && !errors.Is(lookupErr, scrape.ErrNotFound) {
			logging.Error("lookup actress (javdatabase) failed idol=%s code=%s err=%v", idol.Name, code, lookupErr)
		}
		if lookupErr := lookupResults[2].err; lookupErr != nil && !errors.Is(lookupErr, scrape.ErrNotFound) {
			logging.Error("lookup actress (javmodel) failed idol=%d name=%s err=%v", idol.ID, lookupName, lookupErr)
		}

		info := mergeActressInfosByPriority(avWikiInfo, javDatabaseInfo, javModelInfo)
		if info == nil {
			continue
		}
		if info.ChineseName != "" {
			info.ChineseName = util.SimplifyChineseName(info.ChineseName)
		}
		updated, err := db.Default().UpdateIdolProfile(ctx, idol.ID, info)
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
