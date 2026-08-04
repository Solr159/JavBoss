// Package client implements the lightweight local JavBoss client runtime. It
// deliberately does not initialize the server database, scanners, or managers.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/mpv"
)

const maxClientRequestBody = 1 << 20

type Options struct {
	BaseDir      string
	LocalBaseURL string
	RemoteURL    string
	Transport    http.RoundTripper
	PlayVideo    func(path string, options mpv.PlayOptions) error
}

type remoteState struct {
	base  *url.URL
	proxy *httputil.ReverseProxy
}

type Client struct {
	baseDir      string
	localBaseURL string
	transport    http.RoundTripper
	playVideo    func(path string, options mpv.PlayOptions) error
	settings     *settingsStore

	remoteMu sync.RWMutex
	remote   *remoteState
	configMu sync.RWMutex
	config   map[string]string

	grantsMu  sync.Mutex
	grants    map[string]*mediaGrant
	grantKeys map[string]string

	screenshotMu         sync.Mutex
	screenshotCtx        context.Context
	screenshotCancel     context.CancelFunc
	screenshotJobs       map[int64]*screenshotSyncJob
	screenshotClosed     bool
	screenshotCookie     string
	screenshotLastResume time.Time
	screenshotWG         sync.WaitGroup
}

func New(options Options) (*Client, error) {
	if strings.TrimSpace(options.BaseDir) == "" {
		return nil, errors.New("create JavBoss client: base directory is required")
	}
	localBaseURL := strings.TrimRight(strings.TrimSpace(options.LocalBaseURL), "/")
	parsedLocal, err := url.Parse(localBaseURL)
	if err != nil || parsedLocal.Scheme != "http" || parsedLocal.Host == "" {
		return nil, errors.New("create JavBoss client: local base URL is invalid")
	}
	if strings.TrimSpace(options.RemoteURL) == "" {
		return nil, errors.New("create JavBoss client: remote server URL is required")
	}
	remoteURL, err := NormalizeServerURL(options.RemoteURL)
	if err != nil {
		return nil, err
	}
	transport := options.Transport
	if transport == nil {
		defaultTransport := http.DefaultTransport.(*http.Transport).Clone()
		defaultTransport.MaxIdleConns = 100
		defaultTransport.MaxIdleConnsPerHost = 20
		defaultTransport.IdleConnTimeout = 90 * time.Second
		defaultTransport.DisableCompression = true
		transport = defaultTransport
	}
	playVideo := options.PlayVideo
	if playVideo == nil {
		playVideo = mpv.PlayVideo
	}
	settings, err := loadSettingsStore(options.BaseDir)
	if err != nil {
		return nil, err
	}
	screenshotCtx, screenshotCancel := context.WithCancel(context.Background())
	client := &Client{
		baseDir:          options.BaseDir,
		localBaseURL:     localBaseURL,
		transport:        transport,
		playVideo:        playVideo,
		settings:         settings,
		grants:           make(map[string]*mediaGrant),
		grantKeys:        make(map[string]string),
		screenshotCtx:    screenshotCtx,
		screenshotCancel: screenshotCancel,
		screenshotJobs:   make(map[int64]*screenshotSyncJob),
	}
	if err := client.setRemoteURL(remoteURL); err != nil {
		return nil, err
	}
	client.startScreenshotResumeLoop()
	mpv.SetPlayerConfigProvider(settings.playerConfig)
	return client, nil
}

func (c *Client) Close() {
	mpv.ResetPlayerSession()
	c.closeScreenshotSync()
	mpv.SetPlayerConfigProvider(nil)
	if transport, ok := c.transport.(interface{ CloseIdleConnections() }); ok {
		transport.CloseIdleConnections()
	}
}

func (c *Client) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions && !localRequestOriginAllowed(r) {
		respondJSONError(w, http.StatusForbidden, "请求来源无效", "Invalid request origin")
		return
	}
	switch {
	case r.URL.Path == "/healthz":
		c.handleHealth(w)
	case strings.HasPrefix(r.URL.Path, "/__client/media/"):
		c.handleMedia(w, r)
	case r.URL.Path == "/videos/play":
		c.handlePlay(w, r)
	case r.URL.Path == "/videos/open" || r.URL.Path == "/videos/reveal":
		respondJSONError(w, http.StatusNotImplemented, "Client 模式不支持打开远端文件或所在目录", "Client mode cannot open or reveal a remote file")
	case r.URL.Path == "/config":
		c.handleConfig(w, r)
	default:
		state := c.remoteState()
		if state == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "远程 JavBoss Server 未配置", "The remote JavBoss server is not configured")
			return
		}
		state.proxy.ServeHTTP(w, r)
	}
}

func (c *Client) handleHealth(w http.ResponseWriter) {
	remoteURL := ""
	if state := c.remoteState(); state != nil {
		remoteURL = state.base.String()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":     "ok",
		"mode":       "client",
		"remote_url": remoteURL,
	})
}

func (c *Client) setRemoteURL(raw string) error {
	normalized, err := NormalizeServerURL(raw)
	if err != nil {
		return err
	}
	base, err := url.Parse(normalized)
	if err != nil {
		return err
	}
	proxy := httputil.NewSingleHostReverseProxy(base)
	proxy.Transport = c.transport
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = base.Host
		sanitizeForwardingHeaders(req.Header)
		rewriteRequestOrigin(req, base)
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		c.refreshScreenshotCookies(response.Cookies())
		rewriteResponseCookiesForLoopback(response)
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, proxyErr error) {
		logging.Error("client proxy request failed: %v", proxyErr)
		respondJSONError(w, http.StatusBadGateway, "无法连接远端 JavBoss Server", "Could not connect to the remote JavBoss server")
	}
	c.remoteMu.Lock()
	c.remote = &remoteState{base: base, proxy: proxy}
	c.remoteMu.Unlock()
	c.configMu.Lock()
	c.config = nil
	c.configMu.Unlock()
	return nil
}

func (c *Client) remoteState() *remoteState {
	c.remoteMu.RLock()
	defer c.remoteMu.RUnlock()
	return c.remote
}

func (c *Client) remoteRequest(ctx context.Context, method, requestPath, rawQuery string, body io.Reader, headers http.Header) (*http.Response, error) {
	state := c.remoteState()
	if state == nil {
		return nil, errors.New("remote JavBoss server is not configured")
	}
	target := *state.base
	target.Path = requestPath
	target.RawQuery = rawQuery
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, err
	}
	copyRequestHeaders(req.Header, headers)
	if body != nil && req.ContentLength == 0 {
		rawLength := strings.TrimSpace(headers.Get("Content-Length"))
		if contentLength, parseErr := strconv.ParseInt(rawLength, 10, 64); parseErr == nil && contentLength >= 0 {
			req.ContentLength = contentLength
		}
	}
	req.Host = state.base.Host
	sanitizeForwardingHeaders(req.Header)
	rewriteRequestOrigin(req, state.base)
	return c.transport.RoundTrip(req)
}

func copyRequestHeaders(target, source http.Header) {
	for key, values := range source {
		if isHopByHopHeader(key) || strings.EqualFold(key, "Accept-Encoding") || strings.EqualFold(key, "Content-Length") {
			continue
		}
		for _, value := range values {
			target.Add(key, value)
		}
	}
}

func copyResponseHeaders(target, source http.Header) {
	for key, values := range source {
		if isHopByHopHeader(key) || strings.EqualFold(key, "Set-Cookie") {
			continue
		}
		for _, value := range values {
			target.Add(key, value)
		}
	}
}

func isHopByHopHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "proxy-connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	default:
		return false
	}
}

func sanitizeForwardingHeaders(headers http.Header) {
	for _, key := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto"} {
		headers.Del(key)
	}
}

func rewriteRequestOrigin(req *http.Request, base *url.URL) {
	if req.Method == http.MethodGet || req.Method == http.MethodHead || req.Method == http.MethodOptions {
		return
	}
	origin := base.Scheme + "://" + base.Host
	if req.Header.Get("Origin") != "" {
		req.Header.Set("Origin", origin)
	}
	if req.Header.Get("Referer") != "" {
		req.Header.Set("Referer", origin+"/")
	}
	if req.Header.Get("Sec-Fetch-Site") != "" {
		req.Header.Set("Sec-Fetch-Site", "same-origin")
	}
}

func rewriteResponseCookiesForLoopback(response *http.Response) {
	cookies := response.Cookies()
	if len(cookies) == 0 {
		return
	}
	response.Header.Del("Set-Cookie")
	for _, cookie := range cookies {
		cookie.Domain = ""
		cookie.Secure = false
		response.Header.Add("Set-Cookie", cookie.String())
	}
}

func localRequestOriginAllowed(req *http.Request) bool {
	switch strings.ToLower(strings.TrimSpace(req.Header.Get("Sec-Fetch-Site"))) {
	case "cross-site", "same-site":
		return false
	case "same-origin", "none":
		return true
	}
	origin := strings.TrimSpace(req.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Host, req.Host) && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func respondJSONError(w http.ResponseWriter, status int, zh, en string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error_zh": zh, "error_en": en})
}

func writeRemoteResponse(w http.ResponseWriter, response *http.Response) {
	defer response.Body.Close()
	copyResponseHeaders(w.Header(), response.Header)
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

func (c *Client) clientDataDir() string {
	remoteKey := "unconfigured"
	if state := c.remoteState(); state != nil {
		remoteKey = remoteStorageKey(state.base.String())
	}
	return filepath.Join(c.baseDir, "data", "client", remoteKey)
}
