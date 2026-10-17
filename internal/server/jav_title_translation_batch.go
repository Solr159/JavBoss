package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav/translation"
	"javboss/internal/models"
)

type titleTranslationBatchStatus struct {
	Running    bool   `json:"running"`
	Total      int    `json:"total"`
	Processed  int    `json:"processed"`
	Translated int    `json:"translated"`
	Skipped    int    `json:"skipped"`
	ErrorZH    string `json:"error_zh,omitempty"`
	ErrorEN    string `json:"error_en,omitempty"`
}

type titleTranslationBatch struct {
	mu     sync.Mutex
	ctx    context.Context
	status titleTranslationBatchStatus
}

var allTitleTranslations = &titleTranslationBatch{ctx: context.Background()}

// BindTitleTranslationContext stops background translation when the server shuts down.
func BindTitleTranslationContext(ctx context.Context) {
	allTitleTranslations.mu.Lock()
	defer allTitleTranslations.mu.Unlock()
	allTitleTranslations.ctx = ctx
}

func getTitleTranslationBatch(c *gin.Context) {
	allTitleTranslations.mu.Lock()
	defer allTitleTranslations.mu.Unlock()
	c.JSON(http.StatusOK, allTitleTranslations.status)
}

// POST /jav/title-translation/batch snapshots every title and starts one background job.
func startTitleTranslationBatch(c *gin.Context) {
	job := allTitleTranslations
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.status.Running {
		c.JSON(http.StatusAccepted, job.status)
		return
	}
	cfg, err := dbpkg.ListConfig(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取翻译配置失败", "Failed to load translation settings")
		return
	}
	enabled, _ := strconv.ParseBool(cfg["jav_title_translation_enabled"])
	profile := translation.ActiveProfile(cfg)
	provider := translation.ActiveProvider(cfg)
	if !enabled || profile.Model == "" || (profile.Endpoint(provider) == "" && provider != "custom") || (profile.APIKey == "" && provider != "custom") {
		respondLocalizedError(c, http.StatusBadRequest, "请开启标题翻译并配置 API Key 和模型", "Enable title translation and configure an API key and model")
		return
	}
	items, err := dbpkg.ListJavTitleTranslationItems(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取影片标题失败", "Failed to load JAV titles")
		return
	}
	job.status = titleTranslationBatchStatus{Running: len(items) > 0, Total: len(items)}
	if job.status.Running {
		go job.run(items, cfg)
	}
	c.JSON(http.StatusAccepted, job.status)
}

func (job *titleTranslationBatch) run(items []models.Jav, cfg map[string]string) {
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
		// Share the paid-request lock with automatic and manual single-title translation.
		titleTranslationMu.Lock()
		current, err := dbpkg.ListConfig(job.ctx)
		enabled, _ := strconv.ParseBool(current["jav_title_translation_enabled"])
		if err != nil || !enabled {
			titleTranslationMu.Unlock()
			job.fail("重新翻译已停止，请检查标题翻译开关", "Retranslation stopped; check the title translation switch")
			return
		}
		if strings.TrimSpace(item.Title) == "" {
			titleTranslationMu.Unlock()
			job.record(false)
			continue
		}
		title, err := titleTranslationClient.Translate(job.ctx, provider, profile, cfg["jav_title_translation_prompt"], item.Title, nil)
		var saved bool
		if err == nil {
			saved, err = dbpkg.SaveJavTitleTranslation(job.ctx, &item, title)
		}
		titleTranslationMu.Unlock()
		if err != nil {
			_, zh, en := titleTranslationErrorDetails(err)
			job.fail("重新翻译失败，未完成的影片保留原译文。"+zh, "Retranslation failed; unfinished titles are unchanged. "+en)
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
