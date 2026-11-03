package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeDirectoryScanScope(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{"a", filepath.Join("a", "inner"), "b"} {
		if err := os.MkdirAll(filepath.Join(root, relative), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "file.mp4"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name    string
		include []string
		want    []string
	}{
		{name: "no selection", include: nil, want: nil},
		{name: "empty selection", include: []string{}, want: nil},
		{name: "single", include: []string{"a"}, want: []string{"a"}},
		{name: "sorted", include: []string{"b", "a"}, want: []string{"a", "b"}},
		{
			name:    "nested selections collapse to the parent",
			include: []string{"a/inner", "a"},
			want:    []string{"a"},
		},
		{name: "duplicates collapse", include: []string{"a", "a/", "./a"}, want: []string{"a"}},
		{name: "blank entries are ignored", include: []string{" ", "b"}, want: []string{"b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scope, err := normalizeDirectoryScanScope(root, tc.include)
			if err != nil {
				t.Fatalf("normalizeDirectoryScanScope: %v", err)
			}
			got := scope.Includes()
			if len(got) != len(tc.want) {
				t.Fatalf("scope = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("scope = %v, want %v", got, tc.want)
				}
			}
		})
	}

	for _, tc := range []struct {
		name    string
		include []string
	}{
		{name: "parent traversal", include: []string{".."}},
		{name: "absolute path", include: []string{t.TempDir()}},
		{name: "missing directory", include: []string{"missing"}},
		{name: "regular file", include: []string{"file.mp4"}},
		{name: "nul byte", include: []string{"a\x00b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := normalizeDirectoryScanScope(root, tc.include); err == nil {
				t.Fatalf("expected rejection for %v", tc.include)
			}
		})
	}
}

func TestCreateDirectoryWithScopePersistsInclude(t *testing.T) {
	deps := newTestDependencies()
	openTestDB(deps, t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "keep"), 0700); err != nil {
		t.Fatal(err)
	}

	dir, err := deps.store.CreateDirectoryWithScope(t.Context(), root, []string{"keep"})
	if err != nil {
		t.Fatalf("create directory with scope: %v", err)
	}
	if got := dir.ScanScope.Includes(); len(got) != 1 || got[0] != "keep" {
		t.Fatalf("scan scope = %v, want [keep]", got)
	}

	stored, err := deps.store.GetDirectory(t.Context(), dir.ID)
	if err != nil || stored == nil {
		t.Fatalf("reload directory: %v", err)
	}
	if got := stored.ScanScope.Includes(); len(got) != 1 || got[0] != "keep" {
		t.Fatalf("persisted scan scope = %v, want [keep]", got)
	}

	// Clearing the selection restores the default of scanning every subdirectory.
	empty := []string{}
	updated, err := deps.store.UpdateDirectoryWithScope(t.Context(), dir.ID, nil, nil, nil, &empty)
	if err != nil {
		t.Fatalf("clear scan scope: %v", err)
	}
	if got := updated.ScanScope.Includes(); len(got) != 0 {
		t.Fatalf("scan scope after clear = %v, want none", got)
	}
}
