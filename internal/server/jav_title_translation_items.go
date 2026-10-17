package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav/translation"
)

// POST /jav/title-translation/items accepts 1–20 IDs from the current page.
// Only missing titles are translated, with at most three provider requests globally.
// Accept: application/x-ndjson streams {item}, {error_zh,error_en} and {done:true}.
// Other clients receive a JSON summary. On error, queued work stops; in-flight work finishes.
func translateJavTitles(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 || len(req.IDs) > 20 {
		respondLocalizedError(c, 400, "每批需提供 1–20 个影片 ID", "Provide 1–20 JAV IDs per request")
		return
	}
	for _, id := range req.IDs {
		if id <= 0 {
			respondLocalizedError(c, 400, "影片 ID 无效", "Invalid JAV ID")
			return
		}
	}
	cfg, err := dbpkg.ListConfig(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, 500, "读取翻译配置失败", "Failed to load translation settings")
		return
	}
	enabled, _ := strconv.ParseBool(cfg["jav_title_translation_enabled"])
	if !enabled {
		respondLocalizedError(c, 400, "标题翻译未开启", "Title translation is disabled")
		return
	}
	if err := http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		logging.Error("disable grouped translation write deadline: %v", err)
	}
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	waitCtx, stopQueue := context.WithCancel(ctx)
	defer stopQueue()
	type outcome struct {
		item gin.H
		err  error
	}
	jobs := make(chan int64, len(req.IDs))
	seen := make(map[int64]bool)
	for _, id := range req.IDs {
		if !seen[id] {
			jobs <- id
			seen[id] = true
		}
	}
	close(jobs)
	results := make(chan outcome, len(req.IDs))
	var workers sync.WaitGroup
	for i := 0; i < titleTranslationConcurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for id := range jobs {
				release, err := acquireTitleTranslation(waitCtx, id)
				if err != nil {
					return
				}
				item, err := translateMissingJavTitle(ctx, cfg, id)
				if err != nil {
					stopQueue()
				}
				release()
				results <- outcome{item: item, err: err}
			}
		}()
	}
	go func() { workers.Wait(); close(results) }()

	stream := strings.Contains(c.GetHeader("Accept"), "application/x-ndjson")
	if stream {
		c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
		c.Header("Cache-Control", "no-cache, no-transform")
		c.Header("X-Accel-Buffering", "no")
		c.Writer.Flush()
	}
	send := func(event gin.H) bool {
		if !stream {
			return true
		}
		if err := json.NewEncoder(c.Writer).Encode(event); err != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}
	completed := make(map[int64]gin.H)
	summary := gin.H{}
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if !send(gin.H{"heartbeat": true}) {
				return
			}
		case result, ok := <-results:
			if !ok {
				if stream {
					send(gin.H{"done": true})
				} else {
					items := make([]gin.H, 0, len(completed))
					for _, id := range req.IDs {
						if item := completed[id]; item != nil {
							items = append(items, item)
							delete(completed, id)
						}
					}
					summary["items"] = items
					c.JSON(http.StatusOK, summary)
				}
				return
			}
			if result.err != nil && summary["error_en"] == nil {
				_, summary["error_zh"], summary["error_en"] = titleTranslationErrorDetails(result.err)
				if !send(summary) {
					return
				}
			}
			if result.item != nil {
				completed[result.item["id"].(int64)] = result.item
				if !send(gin.H{"item": result.item}) {
					return
				}
			}
		}
	}
}

// The caller holds the per-title lock and a global provider slot.
func translateMissingJavTitle(ctx context.Context, cfg map[string]string, id int64) (gin.H, error) {
	item, err := dbpkg.GetJavTitleTranslationItem(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(item.ZhTitle) == "" && strings.TrimSpace(item.Title) != "" {
		title, err := titleTranslationClient.Translate(ctx, translation.ActiveProvider(cfg), translation.ActiveProfile(cfg), cfg["jav_title_translation_prompt"], item.Title, nil)
		if err != nil {
			return nil, err
		}
		if _, err = dbpkg.SaveJavTitleTranslation(ctx, item, title); err != nil {
			return nil, err
		}
		item, err = dbpkg.GetJavTitleTranslationItem(ctx, id)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
	}
	return gin.H{"id": item.ID, "title": item.Title, "zh_title": item.ZhTitle}, nil
}
