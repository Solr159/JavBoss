package db

import (
	"context"
	"testing"

	"javboss/internal/models"
)

func TestManageAndAssignTagCategories(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()

	categoryA, err := CreateTagCategory(ctx, " A ")
	if err != nil {
		t.Fatalf("CreateTagCategory A: %v", err)
	}
	categoryB, err := CreateTagCategory(ctx, "B")
	if err != nil {
		t.Fatalf("CreateTagCategory B: %v", err)
	}
	if categoryA.Name != "A" || categoryA.SortOrder != 0 || categoryB.SortOrder != 1 {
		t.Fatalf("unexpected categories: A=%+v B=%+v", categoryA, categoryB)
	}

	tag, err := CreateTag(ctx, "grouped")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	if err := AssignTagsCategory(ctx, []int64{tag.ID}, &categoryB.ID); err != nil {
		t.Fatalf("AssignTagsCategory: %v", err)
	}
	tags, err := ListTags(ctx, nil)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 1 || tags[0].CategoryID == nil || *tags[0].CategoryID != categoryB.ID || tags[0].Category != "B" {
		t.Fatalf("listed tag category = %+v", tags)
	}

	if err := ReorderTagCategories(ctx, []int64{categoryB.ID, 0, categoryA.ID}); err != nil {
		t.Fatalf("ReorderTagCategories: %v", err)
	}
	if err := RenameTagCategory(ctx, categoryB.ID, "Renamed"); err != nil {
		t.Fatalf("RenameTagCategory: %v", err)
	}
	categories, err := ListTagCategories(ctx)
	if err != nil {
		t.Fatalf("ListTagCategories: %v", err)
	}
	if len(categories) != 2 || categories[0].ID != categoryB.ID || categories[0].SortOrder != 0 || categories[0].Name != "Renamed" || categories[1].SortOrder != 2 {
		t.Fatalf("reordered categories = %+v", categories)
	}

	if err := DeleteTagCategory(ctx, categoryB.ID); err != nil {
		t.Fatalf("DeleteTagCategory: %v", err)
	}
	var stored models.Tag
	if err := gdb.First(&stored, tag.ID).Error; err != nil {
		t.Fatalf("load tag: %v", err)
	}
	if stored.CategoryID != nil {
		t.Fatalf("deleted category still assigned: %+v", stored)
	}
	categories, err = ListTagCategories(ctx)
	if err != nil {
		t.Fatalf("ListTagCategories after delete: %v", err)
	}
	if len(categories) != 1 || categories[0].ID != categoryA.ID || categories[0].SortOrder != 1 {
		t.Fatalf("normalized categories = %+v", categories)
	}
}

func TestAssignTagsCategoryRejectsMissingCategory(t *testing.T) {
	openTestDB(t)
	tag, err := CreateTag(context.Background(), "tag")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	missingID := int64(999)
	if err := AssignTagsCategory(context.Background(), []int64{tag.ID}, &missingID); err == nil {
		t.Fatal("AssignTagsCategory accepted a missing category")
	}
}
