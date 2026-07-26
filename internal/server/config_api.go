package server

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/mpv"
	"javboss/internal/runtimeconfig"
	"javboss/internal/util"
)

const maxPageSize = 500
const maxJavDisplayRows = 12

func getConfig(c *gin.Context) {
	cfg, err := dbpkg.ListConfig(c.Request.Context())
	if err != nil {
		logging.Error("list config error: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载配置失败", "Failed to load configuration")
		return
	}
	applyRuntimeConfigFields(cfg)
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
		JavTitleMaxRows        *int                  `json:"jav_title_max_rows"`
		JavIdolTagMaxRows      *int                  `json:"jav_idol_tag_max_rows"`
		JavTagMaxRows          *int                  `json:"jav_tag_max_rows"`
		JavHideSeries          *bool                 `json:"jav_hide_series"`
		JavHideIdols           *bool                 `json:"jav_hide_idols"`
		JavHideTags            *bool                 `json:"jav_hide_tags"`
		JavHideActions         *bool                 `json:"jav_hide_actions"`
		JavWaterfallDefault    *bool                 `json:"jav_waterfall_default"`
		IdolPageSize           *int                  `json:"idol_page_size"`
		IdolWaterfallDefault   *bool                 `json:"idol_waterfall_default"`
		StudioPageSize         *int                  `json:"studio_page_size"`
		StudioWaterfallDefault *bool                 `json:"studio_waterfall_default"`
		SeriesPageSize         *int                  `json:"series_page_size"`
		SeriesWaterfallDefault *bool                 `json:"series_waterfall_default"`
		VideoHideJav           *bool                 `json:"video_hide_jav"`
		VideoSort              string                `json:"video_sort"`
		JavSort                string                `json:"jav_sort"`
		IdolSort               string                `json:"idol_sort"`
		JavIdolPreferChinese   *bool                 `json:"jav_idol_prefer_chinese_name"`
		JavTagShowSimplified   *bool                 `json:"jav_tag_show_simplified"`
		DefaultPlayer          string                `json:"default_player"`
		InitialViewMode        string                `json:"initial_view_mode"`
		ShowTopBarTooltips     *bool                 `json:"show_top_bar_button_tooltips"`
		AllowLANAccess         *bool                 `json:"allow_lan_access"`
		ProxyHost              *string               `json:"proxy_host"`
		ProxyPort              *int                  `json:"proxy_port"`
		PlayerWindowSize       *int                  `json:"player_window_size"`
		PlayerWindowWidth      *int                  `json:"player_window_width"`
		PlayerWindowHeight     *int                  `json:"player_window_height"`
		PlayerVolume           *int                  `json:"player_volume"`
		PlayerOntop            *bool                 `json:"player_ontop"`
		PlayerReuseWindow      *bool                 `json:"player_reuse_window"`
		PlayerResumePlayback   *bool                 `json:"player_resume_playback"`
		PlayerShowHotkeyHint   *bool                 `json:"player_show_hotkey_hint"`
		PlayerHotkeys          []playerHotkeyPayload `json:"player_hotkeys"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "配置请求无效", "Invalid configuration request")
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
	if req.JavTitleMaxRows != nil {
		rows := *req.JavTitleMaxRows
		if rows < 0 {
			rows = 0
		}
		if rows > maxJavDisplayRows {
			rows = maxJavDisplayRows
		}
		entries["jav_title_max_rows"] = strconv.Itoa(rows)
	}
	if req.JavIdolTagMaxRows != nil {
		rows := *req.JavIdolTagMaxRows
		if rows < 0 {
			rows = 0
		}
		if rows > maxJavDisplayRows {
			rows = maxJavDisplayRows
		}
		entries["jav_idol_tag_max_rows"] = strconv.Itoa(rows)
	}
	if req.JavTagMaxRows != nil {
		rows := *req.JavTagMaxRows
		if rows < 0 {
			rows = 0
		}
		if rows > maxJavDisplayRows {
			rows = maxJavDisplayRows
		}
		entries["jav_tag_max_rows"] = strconv.Itoa(rows)
	}
	if req.JavHideSeries != nil {
		entries["jav_hide_series"] = strconv.FormatBool(*req.JavHideSeries)
	}
	if req.JavHideIdols != nil {
		entries["jav_hide_idols"] = strconv.FormatBool(*req.JavHideIdols)
	}
	if req.JavHideTags != nil {
		entries["jav_hide_tags"] = strconv.FormatBool(*req.JavHideTags)
	}
	if req.JavHideActions != nil {
		entries["jav_hide_actions"] = strconv.FormatBool(*req.JavHideActions)
	}
	if req.JavWaterfallDefault != nil {
		entries["jav_waterfall_default"] = strconv.FormatBool(*req.JavWaterfallDefault)
	}
	if req.IdolPageSize != nil {
		if v, ok := clampSize(*req.IdolPageSize); ok {
			entries["idol_page_size"] = v
		}
	}
	if req.IdolWaterfallDefault != nil {
		entries["idol_waterfall_default"] = strconv.FormatBool(*req.IdolWaterfallDefault)
	}
	if req.StudioPageSize != nil {
		if v, ok := clampSize(*req.StudioPageSize); ok {
			entries["studio_page_size"] = v
		}
	}
	if req.StudioWaterfallDefault != nil {
		entries["studio_waterfall_default"] = strconv.FormatBool(*req.StudioWaterfallDefault)
	}
	if req.SeriesPageSize != nil {
		if v, ok := clampSize(*req.SeriesPageSize); ok {
			entries["series_page_size"] = v
		}
	}
	if req.SeriesWaterfallDefault != nil {
		entries["series_waterfall_default"] = strconv.FormatBool(*req.SeriesWaterfallDefault)
	}
	if req.VideoHideJav != nil {
		entries["video_hide_jav"] = strconv.FormatBool(*req.VideoHideJav)
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
		case "recent", "recent_asc", "work", "work_asc", "birth", "birth_asc", "height", "height_desc", "bust", "bust_asc", "hips", "hips_asc", "waist", "waist_desc", "measurements", "cup", "cup_asc":
			entries["idol_sort"] = s
		default:
			// ignore invalid values
		}
	}
	if req.JavIdolPreferChinese != nil {
		entries["jav_idol_prefer_chinese_name"] = strconv.FormatBool(*req.JavIdolPreferChinese)
	}
	if req.JavTagShowSimplified != nil {
		entries["jav_tag_show_simplified"] = strconv.FormatBool(*req.JavTagShowSimplified)
	}
	if s := strings.ToLower(strings.TrimSpace(req.DefaultPlayer)); s != "" {
		switch s {
		case "browser", "mpv", "system":
			entries["default_player"] = s
		default:
			respondLocalizedError(c, http.StatusBadRequest, "默认播放器无效", "Invalid default player")
			return
		}
	}
	if s := strings.ToLower(strings.TrimSpace(req.InitialViewMode)); s != "" {
		switch s {
		case "video", "jav":
			entries["initial_view_mode"] = s
		default:
			respondLocalizedError(c, http.StatusBadRequest, "初始页面模式无效", "Invalid initial page mode")
			return
		}
	}
	if req.ShowTopBarTooltips != nil {
		entries["show_top_bar_button_tooltips"] = strconv.FormatBool(*req.ShowTopBarTooltips)
	}
	if req.AllowLANAccess != nil {
		entries["allow_lan_access"] = strconv.FormatBool(*req.AllowLANAccess)
	}
	if req.ProxyPort != nil {
		port := *req.ProxyPort
		if port <= 0 {
			entries["proxy_port"] = ""
		} else if port <= 65535 {
			entries["proxy_port"] = strconv.Itoa(port)
		} else {
			respondLocalizedError(c, http.StatusBadRequest, "代理端口必须在 1-65535 之间", "Proxy port must be between 1 and 65535")
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
			respondLocalizedError(c, http.StatusBadRequest, "代理主机无效", "Invalid proxy host")
			return
		}
	}
	if req.PlayerWindowSize != nil {
		size := *req.PlayerWindowSize
		if size < 10 || size > 100 {
			respondLocalizedError(c, http.StatusBadRequest, "播放器窗口大小必须在 10-100 之间", "Player window size must be between 10 and 100")
			return
		}
		entries["player_window_size"] = strconv.Itoa(size)
	}
	if req.PlayerWindowWidth != nil {
		width := *req.PlayerWindowWidth
		if width < 10 || width > 100 {
			respondLocalizedError(c, http.StatusBadRequest, "播放器窗口宽度必须在 10-100 之间", "Player window width must be between 10 and 100")
			return
		}
		entries["player_window_width"] = strconv.Itoa(width)
	}
	if req.PlayerWindowHeight != nil {
		height := *req.PlayerWindowHeight
		if height < 10 || height > 100 {
			respondLocalizedError(c, http.StatusBadRequest, "播放器窗口高度必须在 10-100 之间", "Player window height must be between 10 and 100")
			return
		}
		entries["player_window_height"] = strconv.Itoa(height)
	}
	if req.PlayerVolume != nil {
		volume := *req.PlayerVolume
		if volume < 0 || volume > 130 {
			respondLocalizedError(c, http.StatusBadRequest, "播放器音量必须在 0-130 之间", "Player volume must be between 0 and 130")
			return
		}
		entries["player_volume"] = strconv.Itoa(volume)
	}
	if req.PlayerOntop != nil {
		entries["player_ontop"] = strconv.FormatBool(*req.PlayerOntop)
	}
	if req.PlayerReuseWindow != nil {
		entries["player_reuse_window"] = strconv.FormatBool(*req.PlayerReuseWindow)
	}
	if req.PlayerResumePlayback != nil {
		entries["player_resume_playback"] = strconv.FormatBool(*req.PlayerResumePlayback)
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
				respondLocalizedError(c, http.StatusBadRequest, "播放器快捷键不能为空", "Player hotkey key is required")
				return
			}
			if key == " " || strings.EqualFold(key, "spacebar") || strings.EqualFold(key, "escape") {
				respondLocalizedError(c, http.StatusBadRequest, "Space 和 Escape 是保留按键", "Space and Escape are reserved")
				return
			}
			if _, ok := seen[key]; ok {
				respondLocalizedError(c, http.StatusBadRequest, "播放器快捷键重复", "Duplicate player hotkeys")
				return
			}
			if action != "seek" && action != "volume" && action != "screenshot" {
				respondLocalizedError(c, http.StatusBadRequest, "播放器快捷键动作无效", "Invalid player hotkey action")
				return
			}
			if action != "screenshot" && item.Amount == 0 {
				respondLocalizedError(c, http.StatusBadRequest, "播放器快捷键数值不能为空", "Player hotkey amount is required")
				return
			}
			if action == "volume" && (item.Amount < -100 || item.Amount > 100) {
				respondLocalizedError(c, http.StatusBadRequest, "音量快捷键数值必须在 -100 到 100 之间", "Player volume hotkey amount must be between -100 and 100")
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
			respondLocalizedError(c, http.StatusInternalServerError, "保存播放器快捷键失败", "Failed to save player hotkeys")
			return
		}
		entries["player_hotkeys"] = string(raw)
	}

	if err := dbpkg.UpsertConfig(c.Request.Context(), entries); err != nil {
		logging.Error("update config error: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "保存配置失败", "Failed to save configuration")
		return
	}
	playerSessionResetNeeded := false
	if req.PlayerHotkeys != nil {
		mpv.InvalidateHotkeysCache()
		playerSessionResetNeeded = true
	}
	if req.PlayerReuseWindow != nil {
		mpv.InvalidateHotkeysCache()
		playerSessionResetNeeded = true
	}
	if req.PlayerResumePlayback != nil {
		mpv.InvalidateHotkeysCache()
		playerSessionResetNeeded = true
	}
	if req.PlayerWindowSize != nil ||
		req.PlayerWindowWidth != nil ||
		req.PlayerWindowHeight != nil ||
		req.PlayerVolume != nil ||
		req.PlayerOntop != nil ||
		req.PlayerReuseWindow != nil ||
		req.PlayerResumePlayback != nil {
		mpv.InvalidatePlayerConfigCache()
		playerSessionResetNeeded = true
	}
	if req.PlayerShowHotkeyHint != nil {
		playerSessionResetNeeded = true
	}
	if playerSessionResetNeeded {
		mpv.ResetPlayerSession()
	}

	cfg, err := dbpkg.ListConfig(c.Request.Context())
	if err != nil {
		logging.Error("list config after update error: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "读取已保存的配置失败", "Failed to load the saved configuration")
		return
	}
	util.SetProxyFromStrings(cfg["proxy_host"], cfg["proxy_port"])
	applyRuntimeConfigFields(cfg)
	c.JSON(http.StatusOK, cfg)
}

func applyRuntimeConfigFields(cfg map[string]string) {
	cfg["runtime_container"] = strconv.FormatBool(runtimeconfig.ContainerMode())
	cfg["directory_picker_enabled"] = strconv.FormatBool(!runtimeconfig.DisableDirectoryPicker())
	cfg["desktop_integration_enabled"] = strconv.FormatBool(!runtimeconfig.DisableDesktopIntegration())
	cfg["mpv_enabled"] = strconv.FormatBool(!runtimeconfig.DisableMPVPlayback())
	cfg["browser_playback_only"] = strconv.FormatBool(runtimeconfig.DisableMPVPlayback() && runtimeconfig.DisableDesktopIntegration())
	cfg["host_path_prefix_enabled"] = strconv.FormatBool(runtimeconfig.HostPathPrefixEnabled())
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
