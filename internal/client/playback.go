package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/mpv"
)

const mediaGrantTTL = 24 * time.Hour

type playRequest struct {
	VideoID      int64   `json:"video_id"`
	LocationID   int64   `json:"location_id"`
	Path         string  `json:"path"`
	DirPath      string  `json:"dir_path"`
	StartTimeSec float64 `json:"start_time"`
}

type mediaGrant struct {
	RemotePath string
	RawQuery   string
	Cookie     string
	LastUsed   time.Time
}

func (c *Client) handlePlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		respondJSONError(w, http.StatusMethodNotAllowed, "请求方法无效", "Method not allowed")
		return
	}
	if !localRequestOriginAllowed(r) {
		respondJSONError(w, http.StatusForbidden, "请求来源无效", "Invalid request origin")
		return
	}
	var request playRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxClientRequestBody))
	if err := decoder.Decode(&request); err != nil || request.VideoID <= 0 || request.StartTimeSec < 0 || request.LocationID < 0 {
		respondJSONError(w, http.StatusBadRequest, "播放请求无效", "Invalid playback request")
		return
	}
	query := make(url.Values)
	if request.LocationID > 0 {
		query.Set("location_id", strconv.FormatInt(request.LocationID, 10))
	} else {
		if strings.TrimSpace(request.Path) == "" || strings.TrimSpace(request.DirPath) == "" {
			respondJSONError(w, http.StatusBadRequest, "播放请求缺少视频位置", "The playback request is missing a video location")
			return
		}
		query.Set("path", request.Path)
		query.Set("dir_path", request.DirPath)
	}
	remotePath := "/videos/" + strconv.FormatInt(request.VideoID, 10) + "/stream"
	cookie := r.Header.Get("Cookie")
	if cookie == "" {
		respondJSONError(w, http.StatusUnauthorized, "远端登录状态不存在，请重新登录", "Remote authentication is missing; please sign in again")
		return
	}
	if err := c.probeRemoteMedia(r, remotePath, query.Encode(), cookie); err != nil {
		var statusErr *remoteStatusError
		if errors.As(err, &statusErr) {
			status := statusErr.status
			if status < 400 || status > 599 {
				status = http.StatusBadGateway
			}
			respondJSONError(w, status, "远端视频不可用或登录已失效", "The remote video is unavailable or authentication has expired")
			return
		}
		respondJSONError(w, http.StatusBadGateway, "连接远端视频失败", "Could not connect to the remote video")
		return
	}
	token, err := c.mediaToken(remotePath, query.Encode(), cookie)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "创建本机播放会话失败", "Could not create the local playback session")
		return
	}
	mediaURL := c.localBaseURL + "/__client/media/" + token
	dataDir := c.clientDataDir()
	if err := c.playVideo(mediaURL, mpv.PlayOptions{
		DataDir:                dataDir,
		VideoID:                request.VideoID,
		StartTimeSec:           request.StartTimeSec,
		EnableNetworkThumbnail: true,
	}); err != nil {
		logging.Error("client MPV playback failed: %v", err)
		if strings.Contains(err.Error(), "mpv not found") {
			respondJSONError(w, http.StatusServiceUnavailable, "未找到本机 MPV 播放器", err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "使用本机 MPV 播放失败", "Could not play the video with local MPV")
		return
	}
	if err := c.startScreenshotSync(request.VideoID, cookie, dataDir); err != nil {
		logging.Error("start client screenshot sync failed: %v", err)
	}
	go c.incrementRemotePlayCount(request.VideoID, cookie)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type remoteStatusError struct {
	status int
}

func (e *remoteStatusError) Error() string {
	return "remote media returned status " + strconv.Itoa(e.status)
}

func (c *Client) probeRemoteMedia(r *http.Request, remotePath, rawQuery, cookie string) error {
	headers := make(http.Header)
	headers.Set("Cookie", cookie)
	headers.Set("Range", "bytes=0-0")
	response, err := c.remoteRequest(r.Context(), http.MethodGet, remotePath, rawQuery, nil, headers)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &remoteStatusError{status: response.StatusCode}
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1))
	return nil
}

func (c *Client) mediaToken(remotePath, rawQuery, cookie string) (string, error) {
	keyDigest := sha256.Sum256([]byte(remotePath + "?" + rawQuery + "\x00" + cookie))
	key := hex.EncodeToString(keyDigest[:])
	now := time.Now()
	c.grantsMu.Lock()
	defer c.grantsMu.Unlock()
	c.cleanupMediaGrantsLocked(now)
	if token := c.grantKeys[key]; token != "" {
		if grant := c.grants[token]; grant != nil {
			grant.LastUsed = now
			return token, nil
		}
	}
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	c.grants[token] = &mediaGrant{RemotePath: remotePath, RawQuery: rawQuery, Cookie: cookie, LastUsed: now}
	c.grantKeys[key] = token
	return token, nil
}

func (c *Client) handleMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := strings.TrimPrefix(r.URL.Path, "/__client/media/")
	grant := c.lookupMediaGrant(token)
	if grant == nil {
		http.Error(w, "playback session expired", http.StatusGone)
		return
	}
	headers := make(http.Header)
	for _, key := range []string{"Range", "If-Range", "If-Modified-Since", "If-None-Match", "User-Agent"} {
		if value := r.Header.Get(key); value != "" {
			headers.Set(key, value)
		}
	}
	headers.Set("Cookie", grant.Cookie)
	response, err := c.remoteRequest(r.Context(), r.Method, grant.RemotePath, grant.RawQuery, nil, headers)
	if err != nil {
		logging.Error("client media proxy failed: %v", err)
		http.Error(w, "remote media unavailable", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	copyResponseHeaders(w.Header(), response.Header)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if controller := http.NewResponseController(w); controller != nil {
		_ = controller.SetWriteDeadline(time.Time{})
	}
	w.WriteHeader(response.StatusCode)
	if r.Method != http.MethodHead {
		_, _ = io.CopyBuffer(w, response.Body, make([]byte, 256*1024))
	}
}

func (c *Client) lookupMediaGrant(token string) *mediaGrant {
	if token == "" {
		return nil
	}
	now := time.Now()
	c.grantsMu.Lock()
	defer c.grantsMu.Unlock()
	c.cleanupMediaGrantsLocked(now)
	grant := c.grants[token]
	if grant == nil {
		return nil
	}
	grant.LastUsed = now
	copy := *grant
	return &copy
}

func (c *Client) cleanupMediaGrantsLocked(now time.Time) {
	for token, grant := range c.grants {
		if now.Sub(grant.LastUsed) <= mediaGrantTTL {
			continue
		}
		delete(c.grants, token)
	}
	for key, token := range c.grantKeys {
		if c.grants[token] == nil {
			delete(c.grantKeys, key)
		}
	}
}

func (c *Client) incrementRemotePlayCount(videoID int64, cookie string) {
	headers := make(http.Header)
	headers.Set("Cookie", cookie)
	headers.Set("Content-Type", "application/json")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	response, err := c.remoteRequest(ctx, http.MethodPost, "/videos/"+strconv.FormatInt(videoID, 10)+"/play", "", bytes.NewReader(nil), headers)
	if err != nil {
		logging.Error("increment remote play count failed: %v", err)
		return
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		logging.Error("increment remote play count returned HTTP %d", response.StatusCode)
	}
}
