package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav/translation"
)

type titleTranslationBatchStatus struct {
	Stopping   bool   `json:"stopping"`
	Scope      string `json:"scope"`
	Running    bool   `json:"running"`
	Total      int    `json:"total"`
	Processed  int    `json:"processed"`
	Translated int    `json:"translated"`
	Skipped    int    `json:"skipped"`
	ErrorZH    string `json:"error_zh,omitempty"`
	ErrorEN    string `json:"error_en,omitempty"`
}

type titleTranslationBatch struct {
	domain textTranslationDomain
	mu     sync.Mutex
	ctx    context.Context
	status titleTranslationBatchStatus
}

var allTitleTranslations = &titleTranslationBatch{ctx: context.Background()}
var allSeriesTranslations = &titleTranslationBatch{ctx: context.Background(), domain: javSeriesDomain}

// BindTitleTranslationContext stops background translation when the server shuts down.
func BindTitleTranslationContext(ctx context.Context) {
	for _, job := range []*titleTranslationBatch{allTitleTranslations, allSeriesTranslations} {
		job.mu.Lock()
		job.ctx = ctx
		job.mu.Unlock()
	}
}

func getTitleTranslationBatch(c *gin.Context) { allTitleTranslations.get(c) }

func (job *titleTranslationBatch) get(c *gin.Context) {
	job.mu.Lock()
	defer job.mu.Unlock()
	c.JSON(http.StatusOK, job.status)
}

// GET /jav/title-translation/batch/preview?scope=missing|all counts the selected library titles.
func previewTitleTranslationBatch(c *gin.Context) { allTitleTranslations.preview(c) }

func (job *titleTranslationBatch) preview(c *gin.Context) {
	scope := c.DefaultQuery("scope", "missing")
	if scope != "missing" && scope != "all" {
		job.domain.error(c, 400, "翻译范围无效", "Invalid translation scope")
		return
	}
	items, err := job.domain.List(c.Request.Context())
	if err != nil {
		job.domain.error(c, 500, "读取影片标题失败", "Failed to load JAV titles")
		return
	}
	selected := selectTitleTranslationItems(items, scope)
	existing := 0
	for _, item := range selected {
		if strings.TrimSpace(item.Translation) != "" {
			existing++
		}
	}
	c.JSON(200, gin.H{"scope": scope, "total": len(selected), "existing": existing})
}

func selectTitleTranslationItems(items []dbpkg.JavTextTranslationItem, scope string) []dbpkg.JavTextTranslationItem {
	selected := make([]dbpkg.JavTextTranslationItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Source) == "" || (scope == "missing" && strings.TrimSpace(item.Translation) != "") {
			continue
		}
		selected = append(selected, item)
	}
	return selected
}

// DELETE /jav/title-translation/batch stops after the current provider request completes.
func stopTitleTranslationBatch(c *gin.Context) { allTitleTranslations.stop(c) }

func (job *titleTranslationBatch) stop(c *gin.Context) {
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.status.Running {
		job.status.Stopping = true
	}
	c.JSON(200, job.status)
}

// POST /jav/title-translation/batch uses saved settings. Scope defaults to missing;
// scope=all requires confirm_overwrite=true because it can replace manual corrections.
func startTitleTranslationBatch(c *gin.Context) { allTitleTranslations.start(c) }

func (job *titleTranslationBatch) start(c *gin.Context) {
	var req struct {
		Scope            string `json:"scope"`
		ConfirmOverwrite bool   `json:"confirm_overwrite"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		job.domain.error(c, 400, "批量翻译请求无效", "Invalid batch translation request")
		return
	}
	if req.Scope == "" {
		req.Scope = "missing"
	}
	if (req.Scope != "missing" && req.Scope != "all") || (req.Scope == "all" && !req.ConfirmOverwrite) {
		job.domain.error(c, 400, "覆盖已有译名需要明确确认", "Confirm before overwriting existing translations")
		return
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.status.Running {
		c.JSON(http.StatusAccepted, job.status)
		return
	}
	cfg, err := dbpkg.ListConfig(c.Request.Context())
	if err != nil {
		job.domain.error(c, 500, "读取翻译配置失败", "Failed to load translation settings")
		return
	}
	profile := translation.ActiveProfile(cfg)
	provider := translation.ActiveProvider(cfg)
	if profile.Model == "" || (profile.Endpoint(provider) == "" && provider != "custom") || (profile.APIKey == "" && provider != "custom") {
		job.domain.error(c, 400, "请先保存 API Key 和模型配置", "Save an API key and model first")
		return
	}
	if err := profile.Validate(provider); err != nil {
		job.domain.error(c, 400, "翻译配置无效", "Invalid translation settings")
		return
	}
	items, err := job.domain.List(c.Request.Context())
	if err != nil {
		job.domain.error(c, 500, "读取影片标题失败", "Failed to load JAV titles")
		return
	}
	items = selectTitleTranslationItems(items, req.Scope)
	job.status = titleTranslationBatchStatus{Running: len(items) > 0, Total: len(items), Scope: req.Scope}
	if job.status.Running {
		go job.run(items, cfg)
	}
	c.JSON(http.StatusAccepted, job.status)
}

func (job *titleTranslationBatch) run(items []dbpkg.JavTextTranslationItem, cfg map[string]string) {
	defer func() {
		job.mu.Lock()
		job.status.Running = false
		job.mu.Unlock()
	}()
	provider := translation.ActiveProvider(cfg)
	profile := translation.ActiveProfile(cfg)
	for _, item := range items {
		if job.ctx.Err() != nil {
			return
		}
		release, err := job.domain.acquire(job.ctx, item.ID)
		if err != nil {
			return
		}
		job.mu.Lock()
		stopping := job.status.Stopping
		job.mu.Unlock()
		if stopping {
			release()
			return
		}
		// Skip titles changed since the batch started, before spending a request.
		current, err := job.domain.Get(job.ctx, item.ID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			release()
			job.fail("读取待翻译内容失败，任务已停止", "Failed to load text; translation stopped")
			return
		}
		if err != nil || current.Source != item.Source || current.Translation != item.Translation {
			release()
			job.record(false)
			continue
		}
		title, err := titleTranslationClient.Translate(job.ctx, provider, profile, job.domain.prompt(cfg), item.Source, nil)
		var saved bool
		if err == nil {
			saved, err = job.domain.Save(job.ctx, &item, title)
		}
		release()
		if err != nil {
			_, zh, en := titleTranslationErrorDetails(err)
			job.fail("重新翻译失败，未完成的内容保留原译文。"+zh, "Retranslation failed; unfinished titles are unchanged. "+en)
			return
		}
		job.record(saved)
	}
}

func (job *titleTranslationBatch) record(saved bool) {
	job.mu.Lock()
	defer job.mu.Unlock()
	job.status.Processed++
	if saved {
		job.status.Translated++
	} else {
		job.status.Skipped++
	}
}

func (job *titleTranslationBatch) fail(zh, en string) {
	job.mu.Lock()
	defer job.mu.Unlock()
	job.status.ErrorZH, job.status.ErrorEN = zh, en
}
