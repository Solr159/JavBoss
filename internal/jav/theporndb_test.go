package jav

import (
	"strings"
	"testing"
	"time"
)

func TestParseThePornDBJavInfo(t *testing.T) {
	payload, err := decodeThePornDBResponse(strings.NewReader(thePornDBFixture))
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}

	record := findThePornDBRecordByCode(payload, "ATID318")
	if record == nil {
		t.Fatal("expected matching record")
	}
	info := parseThePornDBJavInfo(*record)
	if info == nil {
		t.Fatal("expected info, got nil")
	}
	if info.Provider != ProviderThePornDB {
		t.Fatalf("unexpected provider: %s", info.Provider.String())
	}
	if info.Code != "ATID-318" {
		t.Fatalf("unexpected code: %q", info.Code)
	}
	if info.Title != "a Female Teacher Sex Toys Conversion Project Tsumugi Akari" {
		t.Fatalf("unexpected title: %q", info.Title)
	}
	if info.CoverURL != "https://cdn.theporndb.net/background.webp" {
		t.Fatalf("unexpected cover URL: %q", info.CoverURL)
	}

	wantRelease := time.Date(2018, 10, 7, 0, 0, 0, 0, time.UTC).Unix()
	if info.ReleaseUnix != wantRelease {
		t.Fatalf("unexpected release unix: got %d want %d", info.ReleaseUnix, wantRelease)
	}
	if info.DurationMin != 108 {
		t.Fatalf("unexpected duration: %d", info.DurationMin)
	}

	wantTags := []string{"Creampie", "Teacher"}
	if len(info.Tags) != len(wantTags) {
		t.Fatalf("unexpected tags length: got %d want %d %#v", len(info.Tags), len(wantTags), info.Tags)
	}
	for i, tag := range wantTags {
		if info.Tags[i] != tag {
			t.Fatalf("unexpected tag at %d: got %q want %q", i, info.Tags[i], tag)
		}
	}

	wantActors := []string{"Tsumugi Akari"}
	if len(info.Actors) != len(wantActors) {
		t.Fatalf("unexpected actors length: got %d want %d", len(info.Actors), len(wantActors))
	}
	for i, actor := range wantActors {
		if info.Actors[i] != actor {
			t.Fatalf("unexpected actor at %d: got %q want %q", i, info.Actors[i], actor)
		}
	}
}

func TestThePornDBCoverMatchesNormalizedCode(t *testing.T) {
	payload, err := decodeThePornDBResponse(strings.NewReader(thePornDBFixture))
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}

	record := findThePornDBRecordByCode(payload, "atid-318")
	if record == nil {
		t.Fatal("expected matching record")
	}
	if record.Background.Full != "https://cdn.theporndb.net/background.webp" {
		t.Fatalf("unexpected cover: %q", record.Background.Full)
	}
}

const thePornDBFixture = `
{
  "data": [
    {
      "title": "ATID-318: a Female Teacher Sex Toys Conversion Project Tsumugi Akari",
      "external_id": "atid-318",
      "date": "2018-10-07",
      "duration": 6480,
      "background": {
        "full": "https://cdn.theporndb.net/background.webp"
      },
      "performers": [
        {
          "name": "Tsumugi Akari",
          "parent": {
            "name": "Tsumugi Akari",
            "full_name": "Tsumugi Akari"
          }
        }
      ],
      "tags": [
        {"name": "Creampie"},
        {"name": "Teacher"}
      ]
    }
  ]
}`
