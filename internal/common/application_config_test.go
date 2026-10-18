package common

import "testing"

func TestApplicationConfigSnapshots(t *testing.T) {
	original := SetConfig(nil)
	t.Cleanup(func() { SetConfig(original) })
	if CurrentConfig() != nil {
		t.Fatal("unconfigured paths should be nil")
	}
	cfg := &Config{DatabasePath: "data/test.db", JavCoverDir: "data/covers"}
	SetConfig(cfg)
	cfg.DatabasePath = "changed by caller"
	read := CurrentConfig()
	read.JavCoverDir = "changed by reader"
	if got := CurrentConfig(); got.DatabasePath != "data/test.db" || got.JavCoverDir != "data/covers" {
		t.Fatalf("configuration was mutated through a shared pointer: %+v", got)
	}
	previous := SetConfig(&Config{DatabasePath: "replacement.db"})
	SetConfig(previous)
	if got := CurrentConfig(); got.DatabasePath != "data/test.db" {
		t.Fatalf("restore: %+v", got)
	}
}
