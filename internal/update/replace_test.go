package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplacementHandlesChangedEntryTypesAndNames(t *testing.T) {
	base, payload, backup := t.TempDir(), t.TempDir(), t.TempDir()
	putFile(t, base, "file-to-dir", "old file")
	putFile(t, payload, "file-to-dir/child", "new child")
	putFile(t, base, "dir-to-file/child", "old child")
	putFile(t, payload, "dir-to-file", "new file")
	putFile(t, base, "Assets/old", "old asset")
	putFile(t, payload, "assets/new", "new asset")
	if err := os.Mkdir(filepath.Join(base, "old-empty"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(payload, "new-empty"), 0755); err != nil {
		t.Fatal(err)
	}
	tx, err := replacePayload(base, payload, backup)
	if err != nil {
		t.Fatal(err)
	}
	requireFile(t, base, "file-to-dir/child", "new child")
	requireFile(t, base, "dir-to-file", "new file")
	requireFile(t, base, "assets/new", "new asset")
	if _, err := os.Stat(filepath.Join(base, "old-empty")); !os.IsNotExist(err) {
		t.Fatalf("obsolete empty directory survived: %v", err)
	}
	if info, err := os.Stat(filepath.Join(base, "new-empty")); err != nil || !info.IsDir() {
		t.Fatalf("new empty directory missing: %v", err)
	}
	if err := tx.rollback(); err != nil {
		t.Fatal(err)
	}
	requireFile(t, base, "file-to-dir", "old file")
	requireFile(t, base, "dir-to-file/child", "old child")
	requireFile(t, base, "Assets/old", "old asset")
	if _, err := os.Stat(filepath.Join(base, "assets/new")); !os.IsNotExist(err) {
		t.Fatalf("new asset survived rollback: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "new-empty")); !os.IsNotExist(err) {
		t.Fatalf("new empty directory survived rollback: %v", err)
	}
	if info, err := os.Stat(filepath.Join(base, "old-empty")); err != nil || !info.IsDir() {
		t.Fatalf("old empty directory not restored: %v", err)
	}
}

func TestReplacementPreservesReservedNamesAcrossCaseVariants(t *testing.T) {
	for _, name := range []string{"data", "config.toml", "logs", ".updates"} {
		for _, existingName := range []string{name, strings.ToUpper(name)} {
			t.Run(existingName, func(t *testing.T) {
				base, payload, backup := t.TempDir(), t.TempDir(), t.TempDir()
				oldPath, newPath := existingName, strings.ToUpper(name)
				if name != "config.toml" {
					oldPath += "/keep"
					newPath += "/packaged"
				}
				putFile(t, base, oldPath, "user state")
				putFile(t, payload, newPath, "packaged defaults")
				tx, err := replacePayload(base, payload, backup)
				if err != nil {
					t.Fatal(err)
				}
				requireFile(t, base, oldPath, "user state")
				if len(tx.files) != 0 {
					t.Fatal("reserved entry was included in replacement")
				}
				if name != "config.toml" {
					if _, err := os.Stat(filepath.Join(base, newPath)); !os.IsNotExist(err) {
						t.Fatalf("packaged state was installed: %v", err)
					}
				}
			})
		}
	}
}

func TestReplacementDoesNotFollowExistingSymlinks(t *testing.T) {
	base, payload, backup, outside := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	putFile(t, outside, "keep", "external file")
	for _, name := range []string{"assets", "obsolete-link"} {
		if err := os.Symlink(outside, filepath.Join(base, name)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	putFile(t, payload, "assets/new", "new asset")
	tx, err := replacePayload(base, payload, backup)
	if err != nil {
		t.Fatal(err)
	}
	requireFile(t, base, "assets/new", "new asset")
	requireFile(t, outside, "keep", "external file")
	if _, err := os.Stat(filepath.Join(outside, "new")); !os.IsNotExist(err) {
		t.Fatalf("installed asset escaped through symlink: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(base, "obsolete-link")); !os.IsNotExist(err) {
		t.Fatalf("obsolete symlink survived: %v", err)
	}
	if err := tx.rollback(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"assets", "obsolete-link"} {
		if target, err := os.Readlink(filepath.Join(base, name)); err != nil || target != outside {
			t.Fatalf("original symlink not restored: %q, %v", target, err)
		}
	}
	requireFile(t, outside, "keep", "external file")
}
