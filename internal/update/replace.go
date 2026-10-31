package update

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	// Only release-owned paths are replaced. Never copy default configuration or
	// touch data/tools (user-installed FFmpeg), covers, videos, or client settings.
	paths := []string{executableName(), "web/dist", "internal/bin/ffprobe", "internal/bin/ffprobe.exe",
		"internal/bin/ffmpeg", "internal/bin/ffmpeg.exe", "internal/bin/mpv", "modernz", "javboss.command", ".version"}
	for _, rel := range paths {
		rel = filepath.FromSlash(rel)
		src := filepath.Join(payload, rel)
		if _, err := os.Lstat(src); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		if err := validateParents(base, rel); err != nil {
			return err
		}
		target, saved := filepath.Join(base, rel), filepath.Join(backup, "files", rel)
		if err := os.MkdirAll(filepath.Dir(saved), 0700); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		entry := &replacedFile{Target: target, Backup: saved}
		t.files = append(t.files, entry)
		if _, err := os.Lstat(target); err == nil {
			if err := t.rename(target, saved); err != nil {
				return err
			}
			entry.Old = true
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := writeJSON(t.journal, t.files); err != nil {
			return err
		}
		if err := t.rename(src, target); err != nil {
			return err
		}
		entry.Installed = true
		if err := writeJSON(t.journal, t.files); err != nil {
			return err
		}
	}
	return nil
}

func validateParents(base, rel string) error {
	for parent := filepath.Dir(rel); parent != "."; parent = filepath.Dir(parent) {
		info, err := os.Lstat(filepath.Join(base, parent))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("update parent is not a regular directory: %s", parent)
		}
	}
	return nil
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
