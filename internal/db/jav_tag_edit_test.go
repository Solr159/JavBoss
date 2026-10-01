package db

import (
	"context"
	"reflect"
	"testing"

	"javboss/internal/jav/metadata"
	"javboss/internal/models"
)

func TestRenameAndDeleteJavTags(t *testing.T) {
	for _, tc := range []struct {
		name      string
		isUser    bool
		providers []metadata.Provider
	}{
		{name: "custom", isUser: true, providers: []metadata.Provider{metadata.ProviderUser}},
		{name: "scraped", providers: []metadata.Provider{metadata.ProviderJavBus, metadata.ProviderManualScrape}},
		{name: "unattached scraped"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gdb := openTestDB(t)
			ctx := context.Background()
			tag := models.JavTag{Name: "Original", IsUser: tc.isUser}
			other := models.JavTag{Name: "Original", IsUser: !tc.isUser}
			for _, value := range []*models.JavTag{&tag, &other} {
				if err := gdb.Create(value).Error; err != nil {
					t.Fatalf("create tag: %v", err)
				}
			}
			items := []models.Jav{{Code: "EDIT-001"}, {Code: "EDIT-002"}}
			if err := gdb.Create(&items).Error; err != nil {
				t.Fatalf("create JAVs: %v", err)
			}
			relations := []models.JavTagMap{{
				JavID: items[0].ID, JavTagID: other.ID, Provider: int(metadata.ProviderUser),
			}}
			for _, item := range items {
				for _, provider := range tc.providers {
					relations = append(relations, models.JavTagMap{
						JavID: item.ID, JavTagID: tag.ID, Provider: int(provider),
					})
				}
			}
			if err := gdb.Create(&relations).Error; err != nil {
				t.Fatalf("create tag relations: %v", err)
			}
			var before []models.JavTagMap
			if err := gdb.Order("jav_id, jav_tag_id, provider").Find(&before).Error; err != nil {
				t.Fatalf("load original relations: %v", err)
			}

			if err := RenameJavTag(ctx, tag.ID, "  Renamed  "); err != nil {
				t.Fatalf("rename tag: %v", err)
			}
			var renamed models.JavTag
			if err := gdb.First(&renamed, tag.ID).Error; err != nil {
				t.Fatalf("load renamed tag: %v", err)
			}
			if renamed.Name != "Renamed" || renamed.IsUser != tc.isUser {
				t.Fatalf("unexpected renamed tag: %+v", renamed)
			}
			var after []models.JavTagMap
			if err := gdb.Order("jav_id, jav_tag_id, provider").Find(&after).Error; err != nil {
				t.Fatalf("load renamed relations: %v", err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("rename changed relations: got %+v, want %+v", after, before)
			}

			if err := DeleteJavTag(ctx, tag.ID); err != nil {
				t.Fatalf("delete tag: %v", err)
			}
			var remaining []models.JavTag
			if err := gdb.Find(&remaining).Error; err != nil {
				t.Fatalf("load remaining tags: %v", err)
			}
			if len(remaining) != 1 || remaining[0].ID != other.ID || remaining[0].Name != other.Name {
				t.Fatalf("unexpected remaining tags: %+v", remaining)
			}
			var remainingRelations []models.JavTagMap
			if err := gdb.Find(&remainingRelations).Error; err != nil {
				t.Fatalf("load remaining relations: %v", err)
			}
			if len(remainingRelations) != 1 || remainingRelations[0].JavTagID != other.ID {
				t.Fatalf("unexpected remaining relations: %+v", remainingRelations)
			}
			var itemCount int64
			if err := gdb.Model(&models.Jav{}).Count(&itemCount).Error; err != nil {
				t.Fatalf("count JAVs: %v", err)
			}
			if itemCount != int64(len(items)) {
				t.Fatalf("JAV count = %d, want %d", itemCount, len(items))
			}
		})
	}
}
