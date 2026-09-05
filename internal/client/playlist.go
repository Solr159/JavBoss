package client

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"javboss/internal/common/logging"
	"javboss/internal/mpv"
)

// handlePlaylist resolves remote files to local grants just like handlePlay.
// Never forward this endpoint: playback belongs to the client machine.
func (c *Client) handlePlaylist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		respondJSONError(w, http.StatusMethodNotAllowed, "请求方法无效", "Method not allowed")
		return
	}
	var request struct {
		Items []playRequest `json:"items"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxClientRequestBody))
	if err := decoder.Decode(&request); err != nil || len(request.Items) == 0 {
		respondJSONError(w, http.StatusBadRequest, "播放列表请求无效", "Invalid playlist request")
		return
	}
	cookie := r.Header.Get("Cookie")
	if cookie == "" {
		respondJSONError(w, http.StatusUnauthorized, "远端登录状态不存在，请重新登录", "Remote authentication is missing; please sign in again")
		return
	}
	for _, item := range request.Items {
		if item.VideoID <= 0 || item.LocationID < 0 || item.StartTimeSec < 0 {
			respondJSONError(w, http.StatusBadRequest, "播放列表包含无效视频", "Playlist contains an invalid video")
			return
		}
	}
	dataDir := c.clientDataDir()
	remote := c.remoteState()
	items := make([]mpv.PlaylistItem, 0, len(request.Items))
	for _, item := range request.Items {
		query := make(url.Values)
		if item.LocationID > 0 {
			query.Set("location_id", strconv.FormatInt(item.LocationID, 10))
		}
		remotePath := "/videos/" + strconv.FormatInt(item.VideoID, 10) + "/stream"
		if err := c.probeRemoteMedia(r, remotePath, query.Encode(), cookie); err != nil {
			status := http.StatusBadGateway
			var statusErr *remoteStatusError
			if errors.As(err, &statusErr) && statusErr.status >= 400 && statusErr.status <= 599 {
				status = statusErr.status
			}
			respondJSONError(w, status, "远端视频不可用或登录已失效", "The remote video is unavailable or authentication has expired")
			return
		}
		token, err := c.mediaToken(remotePath, query.Encode(), cookie)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "创建本机播放会话失败", "Could not create the local playback session")
			return
		}
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = "Video #" + strconv.FormatInt(item.VideoID, 10)
		}
		videoID := item.VideoID
		items = append(items, mpv.PlaylistItem{
			Path:  c.localBaseURL + "/__client/media/" + token,
			Title: title,
			OnStarted: func() {
				if c.remoteState() == remote {
					c.incrementRemotePlayCount(videoID, cookie)
				}
			},
			Options: mpv.PlayOptions{
				DataDir: dataDir, VideoID: item.VideoID,
				StartTimeSec: item.StartTimeSec, EnableNetworkThumbnail: true,
			},
		})
	}
	if err := c.playPlaylist(items); err != nil {
		logging.Error("client MPV playlist failed: %v", err)
		respondJSONError(w, http.StatusInternalServerError, "使用本机 MPV 播放列表失败", "Could not play the playlist with local MPV")
		return
	}
	for _, item := range items {
		if err := c.startScreenshotSync(item.Options.VideoID, cookie, dataDir); err != nil {
			logging.Error("start client screenshot sync failed: %v", err)
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "count": len(items)})
}
