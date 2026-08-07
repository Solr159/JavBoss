package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseListenAddr(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		port       int
		want       string
		wantErr    bool
		withConfig bool
		allowLAN   bool
	}{
		{name: "missing config uses default", want: "127.0.0.1:8655"},
		{name: "legacy zero uses default", config: "port = 0\n", want: "127.0.0.1:8655", withConfig: true},
		{name: "custom port is preserved", config: "port = 9123\n", want: "127.0.0.1:9123", withConfig: true},
		{name: "command line port overrides config", config: "port = 9123\n", port: 9456, want: "127.0.0.1:9456", withConfig: true},
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

			got, err := releaseListenAddr(baseDir, tt.allowLAN, tt.port)
			if (err != nil) != tt.wantErr {
				t.Fatalf("releaseListenAddr() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("releaseListenAddr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizePortOverride(t *testing.T) {
	for _, tt := range []struct {
		value   int
		want    int
		wantErr bool
	}{
		{value: 0, want: 0},
		{value: 1, want: 1},
		{value: 65535, want: 65535},
		{value: -1, wantErr: true},
		{value: 65536, wantErr: true},
	} {
		got, err := normalizePortOverride(tt.value)
		if (err != nil) != tt.wantErr {
			t.Errorf("normalizePortOverride(%d) error = %v, wantErr %v", tt.value, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("normalizePortOverride(%d) = %d, want %d", tt.value, got, tt.want)
		}
	}
}

func TestConfiguredPortWithOverride(t *testing.T) {
	if got := configuredPortWithOverride(8655, 9123); got != 9123 {
		t.Fatalf("configuredPortWithOverride() = %d, want command-line port 9123", got)
	}
	if got := configuredPortWithOverride(8655, 0); got != 8655 {
		t.Fatalf("configuredPortWithOverride() = %d, want configured port 8655", got)
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

func TestShouldRunClientModeDependsOnlyOnServerURL(t *testing.T) {
	for _, test := range []struct {
		name      string
		serverURL string
		want      bool
	}{
		{name: "missing server URL"},
		{name: "blank server URL", serverURL: "  "},
		{name: "configured server URL", serverURL: "http://localhost:17654", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldRunClientMode(test.serverURL); got != test.want {
				t.Fatalf("shouldRunClientMode(%q) = %t, want %t", test.serverURL, got, test.want)
			}
		})
	}
}

func TestResolveClientServerURLPrefersCommandLineFlag(t *testing.T) {
	tests := []struct {
		name       string
		flagValue  string
		configured string
		want       string
	}{
		{name: "command line overrides config", flagValue: " https://client.example.com ", configured: "https://config.example.com", want: "https://client.example.com"},
		{name: "config is used without command line flag", configured: " https://config.example.com ", want: "https://config.example.com"},
		{name: "blank command line flag falls back to config", flagValue: "  ", configured: "https://config.example.com", want: "https://config.example.com"},
		{name: "both values are blank", flagValue: " ", configured: "  ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveClientServerURL(tt.flagValue, tt.configured); got != tt.want {
				t.Fatalf("resolveClientServerURL(%q, %q) = %q, want %q", tt.flagValue, tt.configured, got, tt.want)
			}
		})
	}
}

func TestReleaseClientStartupHintIncludesModeAndRemoteServer(t *testing.T) {
	localURL := "http://localhost:8655"
	remoteURL := "https://server.example.com"

	for _, test := range []struct {
		name     string
		chinese  bool
		contains []string
	}{
		{
			name:     "Chinese",
			chinese:  true,
			contains: []string{"JavBoss 已通过 Client 模式启动，访问地址：" + localURL, "远程 Server 地址：" + remoteURL},
		},
		{
			name:     "English",
			chinese:  false,
			contains: []string{"JavBoss started in Client mode. URL: " + localURL, "Remote Server: " + remoteURL},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			hint := releaseClientStartupHint(localURL, remoteURL, test.chinese)
			for _, expected := range test.contains {
				if !strings.Contains(hint, expected) {
					t.Fatalf("startup hint %q does not contain %q", hint, expected)
				}
			}
		})
	}
}
