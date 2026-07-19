package server

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestServerHandlersDoNotReturnLegacyErrorField(t *testing.T) {
	legacyErrorField := regexp.MustCompile(`"error"\s*:`)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read server package: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if legacyErrorField.Match(data) {
			t.Errorf("%s contains a legacy error response; use respondLocalizedError", name)
		}
	}
}
