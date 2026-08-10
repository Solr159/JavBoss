package western

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadNFO(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "scene.mp4")
	nfo := `<?xml version="1.0" encoding="UTF-8"?>
<movie>
  <title>Office Scene</title>
  <originaltitle>Original Office Scene</originaltitle>
  <plot>A description.</plot>
  <premiered>2026-08-01</premiered>
  <studio>Example Studio</studio>
  <uniqueid type="theporndb" default="true">scene-123</uniqueid>
  <genre>Roleplay</genre>
  <genre>Roleplay</genre>
  <tag>Office</tag>
  <actor><name>Jane Doe</name></actor>
</movie>`
	if err := os.WriteFile(filepath.Join(dir, "scene.nfo"), []byte(nfo), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ReadNFO(videoPath)
	if err != nil {
		t.Fatalf("ReadNFO: %v", err)
	}
	if got.Title != "Office Scene" || got.Studio != "Example Studio" || got.Source != "nfo:theporndb" || got.SourceID != "scene-123" {
		t.Fatalf("unexpected metadata: %#v", got)
	}
	if !reflect.DeepEqual(got.Performers, []string{"Jane Doe"}) || !reflect.DeepEqual(got.Genres, []string{"Roleplay"}) || !reflect.DeepEqual(got.Labels, []string{"Office"}) {
		t.Fatalf("unexpected lists: %#v", got)
	}
}

func TestReadNFOIgnoresJavBossSidecar(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "movie.nfo"), []byte(`<movie><generator>JavBoss</generator><title>ABC-001</title></movie>`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ReadNFO(filepath.Join(dir, "movie.mp4"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadNFO error = %v, want ErrNotFound", err)
	}
}

func TestWriteNFOAndReadNFO(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "scene.mp4")
	if err := os.WriteFile(videoPath, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := Metadata{
		Title:         "Office Scene",
		OriginalTitle: "Original Office Scene",
		Description:   "A description.",
		ReleaseDate:   "2026-08-01",
		Studio:        "Example Studio",
		Source:        "theporndb",
		SourceID:      "scene-123",
		Performers:    []string{"Jane Doe"},
		Genres:        []string{"Roleplay"},
		Labels:        []string{"Office"},
	}
	if err := WriteNFO(videoPath, want); err != nil {
		t.Fatalf("WriteNFO: %v", err)
	}
	got, err := ReadNFO(videoPath)
	if err != nil {
		t.Fatalf("ReadNFO: %v", err)
	}
	if got.Source != "nfo:theporndb" || got.SourceID != want.SourceID || got.Title != want.Title {
		t.Fatalf("unexpected round trip metadata: %#v", got)
	}
}
