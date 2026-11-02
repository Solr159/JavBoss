package update

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type replacedFile struct {
	Target    string `json:"target"`
	Backup    string `json:"backup"`
	Old       bool   `json:"old"`
	Installed bool   `json:"installed"`
}

type replacement struct {
	files          []*replacedFile
	journal        string
	rename         func(string, string) error
	rollbackFailed bool
}

func replacePayload(base, payload, backup string) (*replacement, error) {
	t := &replacement{journal: filepath.Join(backup, "files.json"), rename: os.Rename}
	return t, t.apply(base, payload, backup)
}

func (t *replacement) apply(base, payload, backup string) (err error) {
	defer func() {
		if err != nil {
			rollbackErr := t.rollback()
			t.rollbackFailed = rollbackErr != nil
			err = errors.Join(err, rollbackErr)
		}
	}()
	oldEntries, err := os.ReadDir(base)
	if err != nil {
		return fmt.Errorf("read installation directory: %w", err)
	}
	newEntries, err := os.ReadDir(payload)
	if err != nil {
		return fmt.Errorf("read update payload: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(backup, "files"), 0700); err != nil {
		return err
	}
	// Move all replaceable top-level entries out first, including obsolete files.
	// Renaming directories and symlinks avoids following links outside the install.
	for _, item := range oldEntries {
		if preserveUpdateEntry(item.Name()) {
			continue
		}
		entry := &replacedFile{
			Target: filepath.Join(base, item.Name()),
			Backup: filepath.Join(backup, "files", item.Name()),
		}
		t.files = append(t.files, entry)
		if err := t.rename(entry.Target, entry.Backup); err != nil {
			return err
		}
		entry.Old = true
		if err := writeJSON(t.journal, t.files); err != nil {
			return err
		}
	}
	// Install the complete package except persistent user state and updater state.
	// Separate journal entries let rollback remove every new entry before restoring
	// the old tree, even if a path changed type or capitalization between versions.
	for _, item := range newEntries {
		if preserveUpdateEntry(item.Name()) {
			continue
		}
		entry := &replacedFile{Target: filepath.Join(base, item.Name())}
		t.files = append(t.files, entry)
		if err := t.rename(filepath.Join(payload, item.Name()), entry.Target); err != nil {
			return err
		}
		entry.Installed = true
		if err := writeJSON(t.journal, t.files); err != nil {
			return err
		}
	}
	return nil
}

func preserveUpdateEntry(name string) bool {
	// Protect aliases on case-insensitive macOS and Windows filesystems as well.
	// .updates contains the running helper, payload and rollback backups. Successful
	// workspaces are removed by cleanupUpdate only after the helper exits.
	switch strings.ToLower(name) {
	case "data", "config.toml", "logs", ".updates":
		return true
	default:
		return false
	}
}

func (t *replacement) rollback() error {
	var failures []error
	for i := len(t.files) - 1; i >= 0; i-- {
		entry := t.files[i]
		if entry.Installed {
			if err := os.RemoveAll(entry.Target); err != nil {
				failures = append(failures, err)
				continue
			}
			entry.Installed = false
		}
		if entry.Old {
			if err := os.Rename(entry.Backup, entry.Target); err != nil {
				failures = append(failures, err)
				continue
			}
			entry.Old = false
		}
	}
	return errors.Join(failures...)
}
