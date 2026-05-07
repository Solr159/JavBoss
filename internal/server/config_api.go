package server

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"pornboss/internal/common/logging"
	dbpkg "pornboss/internal/db"
	"pornboss/internal/jav"
	"pornboss/internal/mpv"
	"pornboss/internal/util"
)

const maxPageSize = 500

func getConfig(c *gin.Context) {
	cfg, err := dbpkg.ListConfig(c.Request.Context())
	if err != nil {
		logging.Error("list config error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

func updateConfig(c *gin.Context) {
	type playerHotkeyPayload struct {
		Key    string  `json:"key"`
		Action string  `json:"action"`
		Amount float64 `json:"amount"`
	}

	var req struct {
		VideoPageSize          *int                  `json:"video_page_size"`
		JavPageSize            *int                  `json:"jav_page_size"`
		JavGridColumns         *int                  `json:"jav_grid_columns"`
		IdolPageSize           *int                  `json:"idol_page_size"`
		VideoSort              string                `json:"video_sort"`
		JavSort                string                `json:"jav_sort"`
		IdolSort               string                `json:"idol_sort"`
		JavMetadataLanguage    string                `json:"jav_metadata_language"`
		DefaultPlayer          string                `json:"default_player"`
		ProxyHost              *string               `json:"proxy_host"`
		ProxyPort              *int                  `json:"proxy_port"`
		PlayerWindowSize       *int                  `json:"player_window_size"`
		PlayerWindowWidth      *int                  `json:"player_window_width"`
		PlayerWindowHeight     *int                  `json:"player_window_height"`
		PlayerWindowUseAutofit *bool                 `json:"player_window_use_autofit"`
		PlayerVolume           *int                  `json:"player_volume"`
		PlayerOntop            *bool                 `json:"player_ontop"`
		PlayerShowHotkeyHint   *bool                 `json:"player_show_hotkey_hint"`
		PlayerHotkeys          []playerHotkeyPayload `json:"player_hotkeys"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	entries := make(map[string]string)
	clampSize := func(n int) (string, bool) {
		if n <= 0 {
			return "", false
		}
		if n > maxPageSize {
			n = maxPageSize
		}
		return strconv.Itoa(n), true
	}

	if req.VideoPageSize != nil {
		if v, ok := clampSize(*req.VideoPageSize); ok {
			entries["video_page_size"] = v
		}
	}
	if req.JavPageSize != nil {
		if v, ok := clampSize(*req.JavPageSize); ok {
			entries["jav_page_size"] = v
		}
	}
	if req.JavGridColumns != nil {
		columns := *req.JavGridColumns
		if columns < 0 {
			columns = 0
		}
		if columns > 12 {
			columns = 12
		}
		entries["jav_grid_columns"] = strconv.Itoa(columns)
	}
	if req.IdolPageSize != nil {
		if v, ok := clampSize(*req.IdolPageSize); ok {
			entries["idol_page_size"] = v
		}
	}
	if s := strings.ToLower(strings.TrimSpace(req.VideoSort)); s != "" {
		switch s {
		case "recent", "recent_asc", "filename", "filename_desc", "duration", "duration_asc", "play_count", "play_count_asc":
			entries["video_sort"] = s
		default:
			// ignore invalid values
		}
	}
	if s := strings.ToLower(strings.TrimSpace(req.JavSort)); s != "" {
		switch s {
		case "recent", "recent_asc", "code", "code_desc", "duration", "duration_asc", "release", "release_asc", "play_count", "play_count_asc":
			entries["jav_sort"] = s
		default:
			// ignore invalid values
		}
	}
	if s := strings.ToLower(strings.TrimSpace(req.IdolSort)); s != "" {
		switch s {
		case "work", "work_asc", "birth", "birth_asc", "height", "height_desc", "bust", "bust_asc", "hips", "hips_asc", "waist", "waist_desc", "measurements", "cup", "cup_asc":
			entries["idol_sort"] = s
		default:
			// ignore invalid values
		}
	}
	if s := strings.TrimSpace(req.JavMetadataLanguage); s != "" {
		lang, ok := jav.ParseMetadataLanguage(s)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid jav metadata language"})
			return
		}
		entries["jav_metadata_language"] = string(lang)
	}
	if s := strings.ToLower(strings.TrimSpace(req.DefaultPlayer)); s != "" {
		switch s {
		case "mpv", "system":
			entries["default_player"] = s
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid default player"})
			return
		}
	}
	if req.ProxyPort != nil {
		port := *req.ProxyPort
		if port <= 0 {
			entries["proxy_port"] = ""
		} else if port <= 65535 {
			entries["proxy_port"] = strconv.Itoa(port)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "proxy port out of range"})
			return
		}
	}
	if req.ProxyHost != nil {
		host := strings.TrimSpace(*req.ProxyHost)
		if host == "" {
			entries["proxy_host"] = ""
		} else if cleanHost, ok := normalizeProxyHost(host); ok {
			entries["proxy_host"] = cleanHost
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid proxy host"})
			return
		}
	}
	if req.PlayerWindowSize != nil {
		size := *req.PlayerWindowSize
		if size < 10 || size > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "player window size out of range"})
			return
		}
		entries["player_window_size"] = strconv.Itoa(size)
	}
	if req.PlayerWindowWidth != nil {
		width := *req.PlayerWindowWidth
		if width < 10 || width > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "player window width out of range"})
			return
		}
		entries["player_window_width"] = strconv.Itoa(width)
	}
	if req.PlayerWindowHeight != nil {
		height := *req.PlayerWindowHeight
		if height < 10 || height > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "player window height out of range"})
			return
		}
		entries["player_window_height"] = strconv.Itoa(height)
	}
	if req.PlayerWindowUseAutofit != nil {
		entries["player_window_use_autofit"] = strconv.FormatBool(*req.PlayerWindowUseAutofit)
	}
	if req.PlayerVolume != nil {
		volume := *req.PlayerVolume
		if volume < 0 || volume > 130 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "player volume out of range"})
			return
		}
		entries["player_volume"] = strconv.Itoa(volume)
	}
	if req.PlayerOntop != nil {
		entries["player_ontop"] = strconv.FormatBool(*req.PlayerOntop)
	}
	if req.PlayerShowHotkeyHint != nil {
		entries["player_show_hotkey_hint"] = strconv.FormatBool(*req.PlayerShowHotkeyHint)
	}
	if req.PlayerHotkeys != nil {
		clean := make([]playerHotkeyPayload, 0, len(req.PlayerHotkeys))
		seen := make(map[string]struct{}, len(req.PlayerHotkeys))
		for _, item := range req.PlayerHotkeys {
			key := strings.TrimSpace(item.Key)
			action := strings.ToLower(strings.TrimSpace(item.Action))
			if len(key) == 1 {
				key = strings.ToLower(key)
			}
			if key == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "player hotkey key required"})
				return
			}
			if key == " " || strings.EqualFold(key, "spacebar") || strings.EqualFold(key, "escape") {
				c.JSON(http.StatusBadRequest, gin.H{"error": "space and escape are reserved"})
				return
			}
			if _, ok := seen[key]; ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": "duplicate player hotkeys"})
				return
			}
			if action != "seek" && action != "volume" && action != "screenshot" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid player hotkey action"})
				return
			}
			if action != "screenshot" && item.Amount == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "player hotkey amount required"})
				return
			}
			if action == "volume" && (item.Amount < -100 || item.Amount > 100) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "player volume hotkey out of range"})
				return
			}
			seen[key] = struct{}{}
			clean = append(clean, playerHotkeyPayload{
				Key:    key,
				Action: action,
				Amount: normalizedPlayerHotkeyAmount(action, item.Amount),
			})
		}
		raw, err := json.Marshal(clean)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		entries["player_hotkeys"] = string(raw)
	}

	if err := dbpkg.UpsertConfig(c.Request.Context(), entries); err != nil {
		logging.Error("update config error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if req.PlayerHotkeys != nil {
		mpv.InvalidateHotkeysCache()
	}
	if req.PlayerWindowSize != nil ||
		req.PlayerWindowWidth != nil ||
		req.PlayerWindowHeight != nil ||
		req.PlayerWindowUseAutofit != nil ||
		req.PlayerVolume != nil ||
		req.PlayerOntop != nil {
		mpv.InvalidatePlayerConfigCache()
	}

	cfg, err := dbpkg.ListConfig(c.Request.Context())
	if err != nil {
		logging.Error("list config after update error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	util.SetProxyFromStrings(cfg["proxy_host"], cfg["proxy_port"])
	jav.SetMetadataLanguage(cfg["jav_metadata_language"])
	c.JSON(http.StatusOK, cfg)
}

func normalizeProxyHost(host string) (string, bool) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", true
	}
	if strings.Contains(host, "://") {
		u, err := url.Parse(host)
		if err != nil || u.Hostname() == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return "", false
		}
		host = u.Hostname()
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	}
	if strings.ContainsAny(host, "/\\?#@ \t\r\n") {
		return "", false
	}
	if strings.Contains(host, ":") && net.ParseIP(host) == nil {
		return "", false
	}
	return host, true
}

func normalizedPlayerHotkeyAmount(action string, amount float64) float64 {
	if action == "screenshot" {
		return 0
	}
	return amount
}
