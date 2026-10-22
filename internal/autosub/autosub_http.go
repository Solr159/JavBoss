// autosub_http provides the HTTP handler that bridges the host app's REST API
// to the local AutoSub subtitle gateway. The gateway is a separate Python
// service (auto_gateway.py) listening on loopback; the handler translates the
// browser's flat payload into the gateway's nested shape.
package autosub

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler bridges the app's REST API to the AutoSub gateway.
type Handler struct {
	client *Client
}

// NewHandler constructs a Handler backed by the given client.
func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

// AutoSubDownloadResponse is the JSON shape returned to the browser.
type AutoSubDownloadResponse struct {
	Success       bool     `json:"success"`
	SubtitlePath  string   `json:"subtitle_path"`
	Source        string   `json:"source"`
	Error         string   `json:"error"`
	Language      string   `json:"language"`
	Failures      []string `json:"failures"`
	FailureDetail string   `json:"failure_detail"`
}

// AutoSubTranslateResponse is the JSON shape returned to the browser for a
// subtitle translation request.
type AutoSubTranslateResponse struct {
	Success      bool   `json:"success"`
	SubtitlePath string `json:"subtitle_path"`
	Language     string `json:"language"`
	Error        string `json:"error"`
}

// AutoSubDownloadRequest is the JSON shape accepted by the handler.
type AutoSubDownloadRequest struct {
	Path       string  `json:"path"`
	Name       string  `json:"name"`
	SizeBytes  int64   `json:"size_bytes"`
	ModifiedAt float64 `json:"modified_time"`
	Duration   float64 `json:"duration_seconds,omitempty"`
	Language   string  `json:"language"`
}

// AutoSubTranslateRequest is the JSON shape accepted by the handler for a
// subtitle translation request. Params are passed through to the gateway's
// translation engine (target_language, source_language, engine).
type AutoSubTranslateRequest struct {
	Path       string            `json:"path"`
	Name       string            `json:"name"`
	SizeBytes  int64             `json:"size_bytes"`
	ModifiedAt float64           `json:"modified_time"`
	Duration   float64           `json:"duration_seconds,omitempty"`
	Subtitle   string            `json:"subtitle_path"`
	Params     map[string]string `json:"params"`
}

func (req AutoSubDownloadRequest) video() Video {
	return Video{
		Path:       req.Path,
		Name:       req.Name,
		SizeBytes:  req.SizeBytes,
		ModifiedAt: req.ModifiedAt,
		Duration:   req.Duration,
	}
}

func (req AutoSubTranslateRequest) video() Video {
	path := req.Subtitle
	if path == "" {
		path = req.Path
	}
	name := filepath.Base(path)
	return Video{
		Path:       path,
		Name:       name,
		SizeBytes:  req.SizeBytes,
		ModifiedAt: req.ModifiedAt,
		Duration:   req.Duration,
	}
}

// ServeAutoSubStatus handles GET /autosub/status and reports gateway reachability.
func (h *Handler) ServeAutoSubStatus(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := h.client.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ServeAutoSubDownload handles POST /autosub/download.
func (h *Handler) ServeAutoSubDownload(c *gin.Context) {
	var req AutoSubDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "video path is required"})
		return
	}
	if req.Name == "" {
		req.Name = filepath.Base(req.Path)
	}

	language := req.Language
	if language == "" {
		language = "Chinese Simplified"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
	defer cancel()

	result, err := h.client.Download(ctx, DownloadRequest{
		Video:    req.video(),
		Language: language,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, AutoSubDownloadResponse{
		Success:       result.Success,
		SubtitlePath:  result.SubtitlePath,
		Source:        result.Source,
		Error:         result.Error,
		Language:      result.Language,
		Failures:      result.Failures,
		FailureDetail: result.FailureDetail,
	})
}

// ServeAutoSubTranslate handles POST /autosub/translate.
func (h *Handler) ServeAutoSubTranslate(c *gin.Context) {
	var req AutoSubTranslateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.video().Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "subtitle path is required"})
		return
	}

	params := map[string]string{}
	for key, value := range req.Params {
		params[key] = value
	}
	if params["target_language"] == "" {
		params["target_language"] = "English"
	}
	if params["source_language"] == "" {
		params["source_language"] = "auto"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Minute)
	defer cancel()

	result, err := h.client.Translate(ctx, TranslateRequest{
		Video:  req.video(),
		Params: params,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, AutoSubTranslateResponse{
		Success:      result.Success,
		SubtitlePath: result.SubtitlePath,
		Language:     result.Language,
		Error:        result.Error,
	})
}
