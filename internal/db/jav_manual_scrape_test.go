package db

import (
	"context"
	"reflect"
	"testing"
	"time"

	"javboss/internal/jav/metadata"
	"javboss/internal/models"
)

func TestManualScrapeReplacesMetadata(t *testing.T) {
	for _, tt := range []struct {
		name       string
		targetCode string
		newTarget  bool
		empty      bool
	}{
		{name: "same JAV", targetCode: "OLD-001"},
		{name: "different existing JAV", targetCode: "NEW-001"},
		{name: "new JAV", targetCode: "NEW-001", newTarget: true},
		{name: "clear same JAV", targetCode: "OLD-001", empty: true},
		{name: "clear different JAV", targetCode: "NEW-001", empty: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gdb := openTestDB(t)
			ctx := context.Background()
			now := time.Now()
			seed := func(code string) *models.Jav {
				t.Helper()
				var rec *models.Jav
				// Seed manual tags first so subsequent providers coexist with them.
				for _, provider := range []metadata.Provider{
					metadata.ProviderManualScrape, metadata.ProviderJavBus, metadata.ProviderJavDB,
					metadata.ProviderJavDBAPI, metadata.ProviderAvmoo, metadata.ProviderAvsox, metadata.ProviderJavMenu,
				} {
					var err error
					rec, err = SaveJavInfo(ctx, &metadata.JavInfo{
						Code: code, Title: "Old title", Tags: []string{"Old tag"},
						Actors: []string{"Old actor"}, Provider: provider,
					})
					if err != nil {
						t.Fatalf("seed JAV: %v", err)
					}
				}
				return rec
			}
			old := seed("OLD-001")
			target := old
			if tt.targetCode != old.Code && !tt.newTarget {
				target = seed(tt.targetCode)
			}
			userTag, err := CreateJavTag(ctx, "User tag")
			if err != nil {
				t.Fatal(err)
			}
			if err := AddJavTagToJavs(ctx, userTag.ID, []int64{target.ID}); err != nil {
				t.Fatal(err)
			}
			var oldTags []models.JavTagMap
			if err := gdb.Where("jav_id = ?", old.ID).Order("provider").Find(&oldTags).Error; err != nil {
				t.Fatal(err)
			}

			dir := models.Directory{Path: "/tmp/manual-replace"}
			video := models.Video{Fingerprint: "manual-replace"}
			if err := gdb.Create(&dir).Error; err != nil {
				t.Fatal(err)
			}
			if err := gdb.Create(&video).Error; err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"one.mp4", "two.mp4"} {
				loc, err := UpsertVideoLocation(ctx, video.ID, dir.ID, path, now)
				if err != nil {
					t.Fatal(err)
				}
				if err := gdb.Model(loc).Update("jav_id", old.ID).Error; err != nil {
					t.Fatal(err)
				}
			}
			info := &metadata.JavInfo{
				Code: tt.targetCode, Title: "Manual title", Provider: metadata.ProviderManualScrape,
				Studio: "Manual Studio", Series: "Manual Series",
			}
			if !tt.empty {
				info.Tags = []string{"New tag"}
				info.Actors = []string{"New actor"}
			}
			rec, err := SaveManualJavInfoAndLinkVideoLocations(ctx, info, video.ID)
			if err != nil || rec == nil {
				t.Fatalf("manual scrape: record=%+v err=%v", rec, err)
			}
			var locations []models.VideoLocation
			if err := gdb.Where("video_id = ?", video.ID).Find(&locations).Error; err != nil {
				t.Fatal(err)
			}
			for _, loc := range locations {
				updated, err := GetVideoForLocation(ctx, video.ID, loc.ID)
				if err != nil || updated == nil || updated.Jav == nil {
					t.Fatalf("load updated video: %+v, %v", updated, err)
				}
				if updated.Jav.ID != rec.ID || updated.Jav.Code != tt.targetCode ||
					updated.JavScrapeOverride != models.JavScrapeOverrideManualPrefix+tt.targetCode {
					t.Fatalf("incorrect video association: %+v", updated)
				}
			}
			updatedJav, err := GetJav(ctx, rec.ID, nil)
			if err != nil {
				t.Fatal(err)
			}
			wantActors := []string{}
			if updatedJav.Studio == nil || updatedJav.Studio.Name != "Manual Studio" {
				t.Fatalf("manual studio not saved: %+v", updatedJav.Studio)
			}
			if updatedJav.Series == nil || updatedJav.Series.Name != "Manual Series" {
				t.Fatalf("manual series not saved: %+v", updatedJav.Series)
			}
			wantTags := map[string]int{}
			if !tt.newTarget {
				wantTags[userTag.Name] = int(metadata.ProviderUser)
			}
			if !tt.empty {
				wantActors = []string{"New actor"}
				wantTags["New tag"] = int(metadata.ProviderManualScrape)
			}
			actors := []string{}
			for _, idol := range updatedJav.Idols {
				actors = append(actors, idol.Name)
			}
			tags := map[string]int{}
			for _, tag := range updatedJav.Tags {
				tags[tag.Name] = tag.Provider
			}
			if !reflect.DeepEqual(actors, wantActors) {
				t.Errorf("actors = %v, want %v", actors, wantActors)
			}
			if !reflect.DeepEqual(tags, wantTags) || len(updatedJav.Tags) != len(wantTags) {
				t.Errorf("tags = %+v, want %v", updatedJav.Tags, wantTags)
			}
			if tt.targetCode != old.Code {
				assertJavIdolMaps(t, gdb, old.Code, map[string]bool{"Old actor": false})
				var storedTags []models.JavTagMap
				if err := gdb.Where("jav_id = ?", old.ID).Order("provider").Find(&storedTags).Error; err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(storedTags, oldTags) {
					t.Fatalf("original JAV tags changed: got %+v, want %+v", storedTags, oldTags)
				}
			}
		})
	}
}
