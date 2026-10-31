package directory

import (
	"os"
	"path/filepath"
	"testing"
)

// A directory can scan only the subdirectories ticked in the picker. The walk roots
// must be exactly those selections, while file locations stay relative to the root.
func TestDirectoryWalkRootsHonoursScanScope(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{"a", "b", filepath.Join("a", "inner")} {
		if err := os.MkdirAll(filepath.Join(root, relative), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name    string
		include []string
		want    []string
	}{
		{name: "no selection scans the whole directory", include: nil, want: []string{root}},
		{name: "empty selection scans the whole directory", include: []string{}, want: []string{root}},
		{name: "one subdirectory", include: []string{"a"}, want: []string{filepath.Join(root, "a")}},
		{
			name:    "two subdirectories",
			include: []string{"a", "b"},
			want:    []string{filepath.Join(root, "a"), filepath.Join(root, "b")},
		},
		{
			name:    "nested selection is covered by its parent",
			include: []string{"a", filepath.Join("a", "inner")},
			want:    []string{filepath.Join(root, "a")},
		},
		{
			name:    "backslash separators are accepted",
			include: []string{"a\\inner"},
			want:    []string{filepath.Join(root, "a", "inner")},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			roots, err := directoryWalkRoots(root, tc.include)
			if err != nil {
				t.Fatalf("directoryWalkRoots: %v", err)
			}
			if len(roots) != len(tc.want) {
				t.Fatalf("roots = %v, want %v", roots, tc.want)
			}
			for i := range roots {
				if roots[i] != tc.want[i] {
					t.Fatalf("roots = %v, want %v", roots, tc.want)
				}
			}
		})
	}
}

func TestDirectoryWalkRootsRejectsEscapingScopes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file.mp4"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		include []string
	}{
		{name: "parent traversal", include: []string{".."}},
		{name: "nested parent traversal", include: []string{"a/../.."}},
		{name: "absolute path", include: []string{t.TempDir()}},
		{name: "file instead of directory", include: []string{"file.mp4"}},
		{name: "missing directory", include: []string{"missing"}},
		{name: "blank selection", include: []string{"  "}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if roots, err := directoryWalkRoots(root, tc.include); err == nil {
				t.Fatalf("expected rejection, got roots %v", roots)
			}
		})
	}
}
