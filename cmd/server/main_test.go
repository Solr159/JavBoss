package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseListenAddr(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		want       string
		wantErr    bool
		withConfig bool
		allowLAN   bool
	}{
		{name: "missing config uses default", want: "127.0.0.1:8655"},
		{name: "legacy zero uses default", config: "port = 0\n", want: "127.0.0.1:8655", withConfig: true},
		{name: "custom port is preserved", config: "port = 9123\n", want: "127.0.0.1:9123", withConfig: true},
		{name: "LAN access listens on all interfaces", want: "0.0.0.0:8655", allowLAN: true},
		{name: "invalid port is rejected", config: "port = 65536\n", wantErr: true, withConfig: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir := t.TempDir()
			if tt.withConfig {
				if err := os.WriteFile(filepath.Join(baseDir, "config.toml"), []byte(tt.config), 0o600); err != nil {
					t.Fatalf("write config: %v", err)
				}
			}

			got, err := releaseListenAddr(baseDir, tt.allowLAN)
			if (err != nil) != tt.wantErr {
				t.Fatalf("releaseListenAddr() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("releaseListenAddr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfiguredListenAddr(t *testing.T) {
	tests := []struct {
		name          string
		allowLAN      bool
		containerMode bool
		want          string
	}{
		{name: "non-container defaults to loopback", want: "127.0.0.1:17654"},
		{name: "LAN access uses all interfaces", allowLAN: true, want: "0.0.0.0:17654"},
		{name: "container uses all interfaces", containerMode: true, want: "0.0.0.0:17654"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := configuredListenAddr(defaultDevelopmentPort, tt.allowLAN, tt.containerMode)
			if got != tt.want {
				t.Fatalf("configuredListenAddr() = %q, want %q", got, tt.want)
			}
		})
	}
}
