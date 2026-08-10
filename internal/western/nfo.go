package western

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type westernNFO struct {
	XMLName       xml.Name    `xml:"movie"`
	Generator     string      `xml:"generator"`
	Title         string      `xml:"title,omitempty"`
	OriginalTitle string      `xml:"originaltitle,omitempty"`
	Plot          string      `xml:"plot,omitempty"`
	Outline       string      `xml:"outline,omitempty"`
	Premiered     string      `xml:"premiered,omitempty"`
	Released      string      `xml:"releasedate,omitempty"`
	Studio        string      `xml:"studio,omitempty"`
	Genres        []string    `xml:"genre,omitempty"`
	Tags          []string    `xml:"tag,omitempty"`
	Actors        []nfoActor  `xml:"actor,omitempty"`
	UniqueIDs     []nfoUnique `xml:"uniqueid,omitempty"`
}

// WriteNFO writes an Emby-compatible sidecar without overwriting a user's
// unrelated NFO. JavBoss-owned Western sidecars may be refreshed.
func WriteNFO(videoPath string, metadata Metadata) error {
	nfoPath := strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + ".nfo"
	if data, err := os.ReadFile(nfoPath); err == nil {
		var existing westernNFO
		if xml.Unmarshal(data, &existing) != nil || !strings.EqualFold(strings.TrimSpace(existing.Generator), "JavBoss") {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read existing western NFO: %w", err)
	}

	value := westernNFO{
		Generator:     "JavBoss",
		Title:         strings.TrimSpace(metadata.Title),
		OriginalTitle: strings.TrimSpace(metadata.OriginalTitle),
		Plot:          strings.TrimSpace(metadata.Description),
		Outline:       strings.TrimSpace(metadata.Description),
		Premiered:     strings.TrimSpace(metadata.ReleaseDate),
		Released:      strings.TrimSpace(metadata.ReleaseDate),
		Studio:        strings.TrimSpace(metadata.Studio),
		Genres:        cleanList(metadata.Genres),
		Tags:          cleanList(metadata.Labels),
	}
	for _, performer := range cleanList(metadata.Performers) {
		value.Actors = append(value.Actors, nfoActor{Name: performer})
	}
	if sourceID := strings.TrimSpace(metadata.SourceID); sourceID != "" {
		value.UniqueIDs = append(value.UniqueIDs, nfoUnique{Type: strings.TrimSpace(metadata.Source), Value: sourceID})
	}

	var builder strings.Builder
	builder.WriteString(xml.Header)
	encoder := xml.NewEncoder(&builder)
	encoder.Indent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode western NFO: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("finish western NFO: %w", err)
	}
	return writeWesternFileAtomically(nfoPath, []byte(builder.String()))
}

// RemoveNFO removes only a JavBoss-owned Western sidecar.
func RemoveNFO(videoPath string) error {
	nfoPath := strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + ".nfo"
	data, err := os.ReadFile(nfoPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read existing western NFO: %w", err)
	}
	var existing westernNFO
	if xml.Unmarshal(data, &existing) != nil || !strings.EqualFold(strings.TrimSpace(existing.Generator), "JavBoss") {
		return nil
	}
	if err := os.Remove(nfoPath); err != nil {
		return fmt.Errorf("remove western NFO: %w", err)
	}
	return nil
}

func writeWesternFileAtomically(path string, data []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".javboss-western-*.tmp")
	if err != nil {
		return fmt.Errorf("create western NFO temp file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write western NFO temp file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace western NFO: %w", err)
	}
	return nil
}
