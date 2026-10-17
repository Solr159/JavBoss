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
// Always streams NDJSON {item}, {error_zh,error_en}, {heartbeat:true} and {done:true}.
// On error, queued work stops; in-flight work finishes.
func translateJavTitles(c *gin.Context) {
	translateTexts(c, javTitleDomain)
}

func translateTexts(c *gin.Context, domain textTranslationDomain) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 || len(req.IDs) > 20 {
		domain.error(c, 400, "每批需提供 1–20 个影片 ID", "Provide 1–20 JAV IDs per request")
		return
	}
	for _, id := range req.IDs {
		if id <= 0 {
			domain.error(c, 400, "影片 ID 无效", "Invalid JAV ID")
			return
		}
	}
	cfg, err := dbpkg.ListConfig(c.Request.Context())
	if err != nil {
		domain.error(c, 500, "读取翻译配置失败", "Failed to load translation settings")
		return
	}
	enabled, _ := strconv.ParseBool(cfg[domain.enabledKey()])
	if !enabled {
		domain.error(c, 400, "标题翻译未开启", "Title translation is disabled")
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
				release, err := domain.acquire(waitCtx, id)
				if err != nil {
					return
				}
				item, err := translateMissingText(ctx, cfg, id, domain)
				if err != nil {
					stopQueue()
				}
				release()
				results <- outcome{item: item, err: err}
			}
		}()
	}
	go func() { workers.Wait(); close(results) }()

	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.Flush()
	encoder := json.NewEncoder(c.Writer)
	send := func(event gin.H) bool {
		if err := encoder.Encode(event); err != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}
	errorSent := false
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
				send(gin.H{"done": true})
				return
			}
			if result.err != nil && !errorSent {
				_, zh, en := titleTranslationErrorDetails(result.err)
				if !send(gin.H{"error_zh": zh, "error_en": en}) {
					return
				}
				errorSent = true
			}
			if result.item != nil {
				if !send(gin.H{"item": result.item}) {
					return
				}
			}
		}
	}
}

// The caller holds the per-title lock and a global provider slot.
func translateMissingText(ctx context.Context, cfg map[string]string, id int64, domain textTranslationDomain) (gin.H, error) {
	item, err := domain.Get(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(item.Translation) == "" && strings.TrimSpace(item.Source) != "" {
		title, err := titleTranslationClient.Translate(ctx, translation.ActiveProvider(cfg), translation.ActiveProfile(cfg), domain.prompt(cfg), item.Source, nil)
		if err != nil {
			return nil, err
		}
		if _, err = domain.Save(ctx, item, title); err != nil {
			return nil, err
		}
		item, err = domain.Get(ctx, id)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
	}
	return domain.response(item), nil
}
