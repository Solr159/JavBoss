package western

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestThePornDBSceneMetadata(t *testing.T) {
	var payload thePornDBResponse
	err := json.Unmarshal([]byte(`{"data":[{
    "id":"scene-123","title":"Office Scene","type":"Scene","date":"2026-08-01",
    "url":"https://example.test/scene","background":{"full":"https://example.test/cover.jpg"},
    "site":{"name":"Example Studio"},
    "performers":[{"name":"Alias","parent":{"full_name":"Jane Doe"}}],
    "tags":[{"name":"Office"},{"name":"Roleplay"}]
  }]}`), &payload)
	if err != nil {
		t.Fatal(err)
	}
	got := payload.Data[0].metadata()
	if got == nil || got.Source != "theporndb" || got.SourceID != "scene-123" || got.Studio != "Example Studio" {
		t.Fatalf("unexpected metadata: %#v", got)
	}
	if !reflect.DeepEqual(got.Performers, []string{"Jane Doe"}) || !reflect.DeepEqual(got.Labels, []string{"Office", "Roleplay"}) {
		t.Fatalf("unexpected lists: %#v", got)
	}
}
