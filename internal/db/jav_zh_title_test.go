package db

import (
	"context"
	"encoding/json"
	"testing"

	"javboss/internal/jav"
	"javboss/internal/models"
)

func TestSaveJavInfoPersistsBothTitles(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	info := &jav.JavInfo{Code: "MIDE-557", Title: "日本語の原題", ZhTitle: " 中文标题 ", Provider: jav.ProviderJavDBAPI}
	rec, err := SaveJavInfo(ctx, info)
	if err != nil {
		t.Fatal(err)
	}
	var stored models.Jav
	assertTitles := func(wantTitle, wantZhTitle string) {
		t.Helper()
		if err := gdb.First(&stored, rec.ID).Error; err != nil {
			t.Fatal(err)
		}
		if stored.Title != wantTitle || stored.ZhTitle != wantZhTitle {
			t.Fatalf("stored titles = %q, %q", stored.Title, stored.ZhTitle)
		}
	}
	assertTitles("日本語の原題", "中文标题")
	info.Title, info.ZhTitle = "更新した原題", "更新后的中文标题"
	if _, err := SaveJavInfo(ctx, info); err != nil {
		t.Fatal(err)
	}
	assertTitles("更新した原題", "更新后的中文标题")
	// Providers without a translated title must not erase one already stored.
	info.Provider, info.ZhTitle = jav.ProviderJavBus, ""
	if _, err := SaveJavInfo(ctx, info); err != nil {
		t.Fatal(err)
	}
	assertTitles("更新した原題", "更新后的中文标题")
	payload, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["title"] != stored.Title || fields["zh_title"] != stored.ZhTitle {
		t.Fatalf("API title fields: %s", payload)
	}
}
