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
	"pornboss/internal/common/logging"
)

var (
	proxyOnce     sync.Once
	proxyFunc     func(*http.Request) (*url.URL, error)
	proxyOverride atomic.Value // stores *url.URL
)

// DetectProxyFunc returns a proxy function that prefers manual override, then env/system
// proxies provided by go-ieproxy (env has priority inside). The result is cached.
func DetectProxyFunc() func(*http.Request) (*url.URL, error) {
	proxyOnce.Do(func() {
		proxyFunc = resolveProxy()
	})
	return proxyFunc
}

// SetProxy configures the manual HTTP proxy. Use port <= 0 to disable.
func SetProxy(host string, port int) {
	if port <= 0 {
		proxyOverride.Store((*url.URL)(nil))
		logging.Info("proxy: cleared configured proxy")
		return
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	u := &url.URL{Scheme: "http", Host: net.JoinHostPort(host, strconv.Itoa(port))}
	proxyOverride.Store(u)
	logging.Info("proxy: using configured proxy %s", u.Redacted())
}

// SetProxyPort configures the local proxy port. Use <=0 to disable.
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
	systemProxy := ieproxy.GetProxyFunc()
	return func(req *http.Request) (*url.URL, error) {
		if u := loadProxyOverride(); u != nil {
			return u, nil
		}
		if systemProxy != nil {
			return systemProxy(req)
		}
		return nil, nil
	}
}

func loadProxyOverride() *url.URL {
	val := proxyOverride.Load()
	if val == nil {
		return nil
	}
	if u, ok := val.(*url.URL); ok {
		return u
	}
	return nil
}
