package util

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mattn/go-ieproxy"
	"javboss/internal/common/logging"
	"javboss/internal/runtimeconfig"
)

const dockerHostGateway = "host.docker.internal"

const (
	ProxyModeAuto   = "auto"
	ProxyModeDirect = "direct"
	ProxyModeManual = "manual"
)

// A nil config means auto-detection; a config with a nil URL forces direct access.
type proxyConfig struct{ url *url.URL }

var (
	proxyOnce     sync.Once
	proxyFunc     func(*http.Request) (*url.URL, error)
	proxyOverride atomic.Pointer[proxyConfig]
)

// DetectProxyFunc returns a cached function honoring direct/manual settings before
// env/system proxies provided by go-ieproxy (env has priority inside).
func DetectProxyFunc() func(*http.Request) (*url.URL, error) {
	proxyOnce.Do(func() {
		proxyFunc = resolveProxy()
	})
	return proxyFunc
}

// SetProxy configures the manual HTTP proxy. Use port <= 0 for auto-detection.
func SetProxy(host string, port int) {
	if port <= 0 {
		proxyOverride.Store(nil)
		logging.Info("proxy: cleared configured proxy")
		return
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	u := &url.URL{Scheme: "http", Host: net.JoinHostPort(host, strconv.Itoa(port))}
	u = mapProxyURLForContainer(u)
	proxyOverride.Store(&proxyConfig{url: u})
	logging.Info("proxy: using configured proxy %s", u.Redacted())
}

// ResolveProxyMode defaults to auto while preserving legacy port-only manual settings.
func ResolveProxyMode(mode, portRaw string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ProxyModeAuto:
		return ProxyModeAuto
	case ProxyModeDirect:
		return ProxyModeDirect
	case ProxyModeManual:
		return ProxyModeManual
	case "":
		if port, err := strconv.Atoi(strings.TrimSpace(portRaw)); err == nil && port > 0 && port <= 65535 {
			return ProxyModeManual
		}
	}
	return ProxyModeAuto
}

// SetProxySettings atomically switches the policy used by existing HTTP clients.
func SetProxySettings(mode, host, port string) {
	switch ResolveProxyMode(mode, port) {
	case ProxyModeDirect:
		proxyOverride.Store(&proxyConfig{})
		logging.Info("proxy: using direct connections")
	case ProxyModeManual:
		SetProxyFromStrings(host, port)
	default:
		SetProxyPort(0)
	}
}

// SetProxyPort configures the local proxy port. Use <=0 for auto-detection.
func SetProxyPort(port int) {
	SetProxy("127.0.0.1", port)
}

// SetProxyFromStrings parses host and port strings and configures the manual proxy.
func SetProxyFromStrings(hostRaw, portRaw string) {
	portRaw = strings.TrimSpace(portRaw)
	if portRaw == "" {
		SetProxyPort(0)
		return
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil || port <= 0 || port > 65535 {
		SetProxyPort(0)
		return
	}
	SetProxy(hostRaw, port)
}

// SetProxyPortFromString parses a port string and configures the local proxy port.
func SetProxyPortFromString(raw string) {
	SetProxyFromStrings("127.0.0.1", raw)
}

func resolveProxy() func(*http.Request) (*url.URL, error) {
	return configuredProxy(ieproxy.GetProxyFunc())
}

func configuredProxy(systemProxy func(*http.Request) (*url.URL, error)) func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		if config := proxyOverride.Load(); config != nil {
			return config.url, nil
		}
		if systemProxy != nil {
			u, err := systemProxy(req)
			if err != nil {
				return nil, err
			}
			return mapProxyURLForContainer(u), nil
		}
		return nil, nil
	}
}

func mapProxyURLForContainer(u *url.URL) *url.URL {
	if u == nil || !runtimeconfig.ProxyHostGatewayEnabled() {
		return u
	}
	host := strings.TrimSpace(u.Hostname())
	if !isLoopbackProxyHost(host) {
		return u
	}

	mapped := *u
	if port := u.Port(); port != "" {
		mapped.Host = net.JoinHostPort(dockerHostGateway, port)
	} else {
		mapped.Host = dockerHostGateway
	}
	return &mapped
}

func isLoopbackProxyHost(host string) bool {
	if strings.EqualFold(host, "localhost") || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
