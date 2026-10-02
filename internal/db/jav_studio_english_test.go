package db

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"javboss/internal/models"
)

func TestReconcileJavStudioNames(t *testing.T) {
	for _, tc := range []struct {
		name, current, want string
		names, aliases      []string
	}{
		{"fill English", "", "English Studio", []string{"日本語", " English Studio ", "Other English"}, []string{"Other English", "日本語"}},
		{"promote current", "元の片商", "English Studio", []string{"別の片商", "English Studio"}, []string{"元の片商", "別の片商"}},
		{"first English wins", "", "First Studio", []string{"First Studio", "Second Studio"}, []string{"Second Studio"}},
		{"non-English fallback", "", "第一片商", []string{"第一片商", "第二片商"}, []string{"第二片商"}},
		{"keep current without English", "元の片商", "元の片商", []string{"別の片商"}, []string{"別の片商"}},
		{"deduplicate", "", "English Studio", []string{"", " ", " English Studio ", "English Studio", "日本語", "日本語"}, []string{"日本語"}},
		{"keep existing English", "Stable Studio", "Stable Studio", []string{"Different Studio"}, nil},
		{"empty results", "元の片商", "元の片商", []string{"", " "}, nil},
		{"accented Latin", "元の片商", "Café Studio", []string{"Café Studio"}, []string{"元の片商"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gdb := openTestDB(t)
			row := models.Jav{Code: "STUDIO-001"}
			if tc.current != "" {
				studio := models.JavStudio{Name: tc.current}
				if err := gdb.Create(&studio).Error; err != nil {
					t.Fatal(err)
				}
				row.StudioID = &studio.ID
			}
			if err := gdb.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := ReconcileJavStudioNames(context.Background(), row.ID, row.StudioID, tc.names); err != nil {
				t.Fatal(err)
			}
			assertJavStudio(t, gdb, row.Code, tc.want)
			var aliases []string
			if err := gdb.Model(&models.JavStudioAlias{}).Pluck("alias", &aliases).Error; err != nil {
				t.Fatal(err)
			}
			sort.Strings(aliases)
			sort.Strings(tc.aliases)
			if len(aliases) != len(tc.aliases) || (len(aliases) > 0 && !reflect.DeepEqual(aliases, tc.aliases)) {
				t.Fatalf("aliases=%v want=%v", aliases, tc.aliases)
			}
			if err := gdb.First(&row, row.ID).Error; err != nil {
				t.Fatal(err)
			}
			if updated, err := ReconcileJavStudioNames(context.Background(), row.ID, row.StudioID, tc.names); err != nil || updated {
				t.Fatalf("repeat updated=%v err=%v", updated, err)
			}
		})
	}
}

func TestReconcileJavStudioNamesMergesRelationships(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	local := models.JavStudio{Name: "元の片商"}
	canonical := models.JavStudio{Name: "Stable English"}
	other := models.JavStudio{Name: "Other English"}
	for _, studio := range []*models.JavStudio{&local, &canonical, &other} {
		if err := gdb.Create(studio).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, alias := range []models.JavStudioAlias{
		{JavStudioID: canonical.ID, Alias: "Provider English"},
		{JavStudioID: local.ID, Alias: "旧別名"},
		{JavStudioID: other.ID, Alias: "Other Alias"},
	} {
		if err := gdb.Create(&alias).Error; err != nil {
			t.Fatal(err)
		}
	}
	rows := []models.Jav{{Code: "ONE-001", StudioID: &local.ID}, {Code: "TWO-001", StudioID: &other.ID}}
	if err := gdb.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	series := models.JavSeries{Name: "Series", StudioID: &local.ID}
	group := models.JavFavoriteGroup{Name: "Favorites", EntityType: JavFavoriteEntityStudio}
	for _, record := range []any{&series, &group} {
		if err := gdb.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	favorite := models.JavFavoriteMap{JavFavoriteGroupID: group.ID, EntityType: JavFavoriteEntityStudio, EntityID: other.ID, SortOrder: 7}
	if err := gdb.Create(&favorite).Error; err != nil {
		t.Fatal(err)
	}
	if updated, err := ReconcileJavStudioNames(ctx, rows[0].ID, &local.ID, []string{"Provider English", "Other English", "元の片商", "New Alias"}); err != nil || !updated {
		t.Fatalf("merge updated=%v err=%v", updated, err)
	}
	for _, tc := range []struct {
		model any
		where string
		value int64
		count int64
	}{
		{&models.JavStudio{}, "id <> ?", canonical.ID, 0},
		{&models.Jav{}, "studio_id = ?", canonical.ID, 2},
		{&models.JavSeries{}, "studio_id = ?", canonical.ID, 1},
		{&models.JavFavoriteMap{}, "entity_id = ?", canonical.ID, 1},
	} {
		var count int64
		if err := gdb.Model(tc.model).Where(tc.where, tc.value).Count(&count).Error; err != nil || count != tc.count {
			t.Fatalf("%T count=%d want=%d err=%v", tc.model, count, tc.count, err)
		}
	}
	assertJavStudio(t, gdb, rows[0].Code, "Stable English")
	for _, name := range []string{"Provider English", "Other English", "元の片商", "New Alias", "旧別名", "Other Alias"} {
		var alias models.JavStudioAlias
		if err := gdb.Where("alias = ?", name).First(&alias).Error; err != nil || alias.JavStudioID != canonical.ID {
			t.Fatalf("alias %q=%+v err=%v", name, alias, err)
		}
	}
}

func TestReconcileJavStudioNamesPromotesExistingAlias(t *testing.T) {
	gdb := openTestDB(t)
	studio := models.JavStudio{Name: "日本語"}
	if err := gdb.Create(&studio).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&models.JavStudioAlias{JavStudioID: studio.ID, Alias: "English Studio"}).Error; err != nil {
		t.Fatal(err)
	}
	row := models.Jav{Code: "STUDIO-001"}
	if err := gdb.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileJavStudioNames(context.Background(), row.ID, nil, []string{"English Studio"}); err != nil {
		t.Fatal(err)
	}
	assertJavStudio(t, gdb, row.Code, "English Studio")
	var aliases []models.JavStudioAlias
	if err := gdb.Find(&aliases).Error; err != nil || len(aliases) != 1 || aliases[0].Alias != "日本語" {
		t.Fatalf("aliases=%+v err=%v", aliases, err)
	}
}
