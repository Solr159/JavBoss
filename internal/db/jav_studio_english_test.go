package db

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"javboss/internal/jav"
	"javboss/internal/models"
)

func TestPromoteJavStudioEnglishName(t *testing.T) {
	for _, tc := range []struct {
		name, current, incoming, want string
		updated                       bool
	}{
		{"rename Japanese", "ソフト・オン・デマンド", " SOD Create ", "SOD Create", true},
		{"missing English", "日文片商", "", "日文片商", false},
		{"Japanese result", "日文片商", "別の片商", "日文片商", false},
		{"mixed result", "日文片商", "SODクリエイト", "日文片商", false},
		{"punctuation result", "日文片商", "---", "日文片商", false},
		{"keep existing English", "SOD Create", "Another English Name", "SOD Create", false},
		{"accented Latin", "日文片商", "Café Studio", "Café Studio", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gdb := openTestDB(t)
			ctx := context.Background()
			rec, err := SaveJavInfo(ctx, &jav.JavInfo{Code: "TEST-001", Title: "Original", Studio: tc.current, Provider: jav.ProviderJavDBAPI})
			if err != nil {
				t.Fatal(err)
			}
			studioID := *rec.StudioID
			if err := gdb.Create(&models.JavStudioAlias{JavStudioID: studioID, Alias: "既存の別名"}).Error; err != nil {
				t.Fatal(err)
			}
			updated, err := PromoteJavStudioEnglishName(ctx, rec.ID, tc.incoming)
			if err != nil || updated != tc.updated {
				t.Fatalf("updated=%v err=%v", updated, err)
			}
			var studio models.JavStudio
			if err := gdb.First(&studio, studioID).Error; err != nil {
				t.Fatal(err)
			}
			if studio.Name != tc.want {
				t.Fatalf("name=%q want=%q", studio.Name, tc.want)
			}
			var aliases []string
			if err := gdb.Model(&models.JavStudioAlias{}).Where("jav_studio_id = ?", studioID).Order("alias").Pluck("alias", &aliases).Error; err != nil {
				t.Fatal(err)
			}
			wantAliases := []string{"既存の別名"}
			if tc.updated {
				wantAliases = append([]string{tc.current}, wantAliases...)
			}
			sort.Strings(wantAliases)
			if !reflect.DeepEqual(aliases, wantAliases) {
				t.Fatalf("aliases=%v want=%v", aliases, wantAliases)
			}
			if updated, err := PromoteJavStudioEnglishName(ctx, rec.ID, tc.incoming); err != nil || updated {
				t.Fatalf("repeat updated=%v err=%v", updated, err)
			}
			again, err := SaveJavInfo(ctx, &jav.JavInfo{Code: "TEST-001", Title: "Original", Studio: tc.current, Provider: jav.ProviderJavDBAPI})
			if err != nil {
				t.Fatal(err)
			}
			if *again.StudioID != studioID {
				t.Fatal("subsequent original-name scrape created another studio")
			}
		})
	}
}

func TestPromoteJavStudioEnglishNameUsesExistingAlias(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	rec, err := SaveJavInfo(ctx, &jav.JavInfo{Code: "TEST-001", Title: "Title", Studio: "元の片商", Provider: jav.ProviderJavDBAPI})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&models.JavStudioAlias{JavStudioID: *rec.StudioID, Alias: "English Studio"}).Error; err != nil {
		t.Fatal(err)
	}
	if updated, err := PromoteJavStudioEnglishName(ctx, rec.ID, "English Studio"); err != nil || !updated {
		t.Fatalf("updated=%v err=%v", updated, err)
	}
	var studio models.JavStudio
	if err := gdb.First(&studio, *rec.StudioID).Error; err != nil {
		t.Fatal(err)
	}
	if studio.Name != "English Studio" {
		t.Fatal(studio.Name)
	}
	var aliases []string
	if err := gdb.Model(&models.JavStudioAlias{}).Where("jav_studio_id = ?", studio.ID).Pluck("alias", &aliases).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(aliases, []string{"元の片商"}) {
		t.Fatalf("aliases=%v", aliases)
	}
}

func TestPromoteJavStudioEnglishNameMergesRelationships(t *testing.T) {
	for _, viaAlias := range []bool{false, true} {
		t.Run(map[bool]string{false: "name", true: "alias"}[viaAlias], func(t *testing.T) {
			gdb := openTestDB(t)
			ctx := context.Background()
			source := models.JavStudio{Name: "元の片商"}
			canonical := models.JavStudio{Name: "English Studio"}
			for _, s := range []*models.JavStudio{&source, &canonical} {
				if err := gdb.Create(s).Error; err != nil {
					t.Fatal(err)
				}
			}
			name := "English Studio"
			if viaAlias {
				name = "English Alternative"
				if err := gdb.Create(&models.JavStudioAlias{JavStudioID: canonical.ID, Alias: name}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := gdb.Create(&models.JavStudioAlias{JavStudioID: source.ID, Alias: "元の別名"}).Error; err != nil {
				t.Fatal(err)
			}
			series := models.JavSeries{Name: "Series", StudioID: &source.ID}
			if err := gdb.Create(&series).Error; err != nil {
				t.Fatal(err)
			}
			rows := []models.Jav{{Code: "TEST-001", StudioID: &source.ID}, {Code: "TEST-002", StudioID: &source.ID}}
			if err := gdb.Create(&rows).Error; err != nil {
				t.Fatal(err)
			}
			group := models.JavFavoriteGroup{Name: "Favorites", EntityType: JavFavoriteEntityStudio}
			if err := gdb.Create(&group).Error; err != nil {
				t.Fatal(err)
			}
			favorite := models.JavFavoriteMap{JavFavoriteGroupID: group.ID, EntityType: JavFavoriteEntityStudio, EntityID: source.ID, SortOrder: 7}
			if err := gdb.Create(&favorite).Error; err != nil {
				t.Fatal(err)
			}
			if updated, err := PromoteJavStudioEnglishName(ctx, rows[0].ID, name); err != nil || !updated {
				t.Fatalf("updated=%v err=%v", updated, err)
			}
			var count int64
			for _, check := range []struct {
				model any
				where string
				arg   any
				want  int64
			}{
				{&models.JavStudio{}, "id = ?", source.ID, 0},
				{&models.Jav{}, "studio_id = ?", canonical.ID, 2},
				{&models.JavSeries{}, "studio_id = ?", canonical.ID, 1},
				{&models.JavFavoriteMap{}, "entity_id = ?", canonical.ID, 1},
			} {
				if err := gdb.Model(check.model).Where(check.where, check.arg).Count(&count).Error; err != nil {
					t.Fatal(err)
				}
				if count != check.want {
					t.Fatalf("%T count=%d want=%d", check.model, count, check.want)
				}
			}
			var alias models.JavStudioAlias
			for _, name := range []string{"元の片商", "元の別名"} {
				if err := gdb.Where("alias = ?", name).First(&alias).Error; err != nil {
					t.Fatal(err)
				}
				if alias.JavStudioID != canonical.ID {
					t.Fatal("alias did not move")
				}
				alias = models.JavStudioAlias{}
			}
			rec, err := SaveJavInfo(ctx, &jav.JavInfo{Code: "TEST-001", Title: "Title", Studio: source.Name, Provider: jav.ProviderJavDBAPI})
			if err != nil || *rec.StudioID != canonical.ID {
				t.Fatalf("original name lookup: %+v %v", rec, err)
			}
		})
	}
}

func TestPromoteJavStudioEnglishNameFillsMissingStudio(t *testing.T) {
	gdb := openTestDB(t)
	rec := models.Jav{Code: "TEST-001"}
	if err := gdb.Create(&rec).Error; err != nil {
		t.Fatal(err)
	}
	if updated, err := PromoteJavStudioEnglishName(context.Background(), rec.ID, "English Studio"); err != nil || !updated {
		t.Fatalf("updated=%v err=%v", updated, err)
	}
	if err := gdb.Preload("Studio").First(&rec, rec.ID).Error; err != nil {
		t.Fatal(err)
	}
	if rec.Studio == nil || rec.Studio.Name != "English Studio" {
		t.Fatalf("studio=%+v", rec.Studio)
	}
}
