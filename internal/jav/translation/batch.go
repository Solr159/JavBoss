package translation

import (
	"context"
	"errors"
	dbpkg "javboss/internal/db"
	"strings"
	"sync"

	"gorm.io/gorm"
)

type BatchStatus struct {
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

type BatchPreview struct {
	Scope    string `json:"scope"`
	Total    int    `json:"total"`
	Existing int    `json:"existing"`
}

type BatchError struct {
	Status int
	ZH, EN string
}

func (e *BatchError) Error() string              { return e.EN }
func batchError(status int, zh, en string) error { return &BatchError{status, zh, en} }

type batch struct {
	domain Domain
	mu     sync.Mutex
	ctx    context.Context
	status BatchStatus
	done   chan struct{}
}

var titleBatch = batch{ctx: context.Background()}
var seriesBatch = batch{ctx: context.Background(), domain: Domain{Series: true}}

func batchFor(series bool) *batch {
	if series {
		return &seriesBatch
	}
	return &titleBatch
}

// Init binds background tasks to application lifetime. Call before consumers start,
// after any previous jobs have finished.
func Init(ctx context.Context) {
	for _, job := range []*batch{&titleBatch, &seriesBatch} {
		job.mu.Lock()
		job.ctx = ctx
		job.status = BatchStatus{}
		job.done = nil
		job.mu.Unlock()
	}
}

// WaitBatches waits for jobs to finish; cancel the application context first on shutdown.
func WaitBatches() {
	for _, job := range []*batch{&titleBatch, &seriesBatch} {
		job.mu.Lock()
		done := job.done
		job.mu.Unlock()
		if done != nil {
			<-done
		}
	}
}

func GetBatchStatus(series bool) BatchStatus {
	job := batchFor(series)
	job.mu.Lock()
	defer job.mu.Unlock()
	return job.status
}

func StopBatch(series bool) BatchStatus {
	job := batchFor(series)
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.status.Running {
		job.status.Stopping = true
	}
	return job.status
}

func PreviewBatch(ctx context.Context, series bool, scope string) (BatchPreview, error) {
	if scope != "missing" && scope != "all" {
		return BatchPreview{}, batchError(400, "翻译范围无效", "Invalid translation scope")
	}
	items, err := (Domain{Series: series}).List(ctx)
	if err != nil {
		return BatchPreview{}, batchError(500, "读取影片标题失败", "Failed to load JAV titles")
	}
	selected := selectItems(items, scope)
	result := BatchPreview{Scope: scope, Total: len(selected)}
	for _, item := range selected {
		if strings.TrimSpace(item.Translation) != "" {
			result.Existing++
		}
	}
	return result, nil
}

func selectItems(items []dbpkg.JavTextTranslationItem, scope string) []dbpkg.JavTextTranslationItem {
	selected := make([]dbpkg.JavTextTranslationItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Source) == "" || (scope == "missing" && strings.TrimSpace(item.Translation) != "") {
			continue
		}
		selected = append(selected, item)
	}
	return selected
}

func StartBatch(ctx context.Context, series bool, scope string, confirmOverwrite bool) (BatchStatus, error) {
	if scope == "" {
		scope = "missing"
	}
	if (scope != "missing" && scope != "all") || (scope == "all" && !confirmOverwrite) {
		return BatchStatus{}, batchError(400, "覆盖已有译名需要明确确认", "Confirm before overwriting existing translations")
	}
	job := batchFor(series)
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.status.Running {
		return job.status, nil
	}
	if err := job.ctx.Err(); err != nil {
		return BatchStatus{}, batchError(503, "翻译服务已停止", "Translation service has stopped")
	}
	cfg, err := dbpkg.Default().ListConfig(ctx)
	if err != nil {
		return BatchStatus{}, batchError(500, "读取翻译配置失败", "Failed to load translation settings")
	}
	if !job.domain.Enabled(cfg) {
		return BatchStatus{}, batchError(400, "标题翻译未开启", "Title translation is disabled")
	}
	profile, provider := ActiveProfile(cfg), ActiveProvider(cfg)
	if profile.Model == "" || (profile.Endpoint(provider) == "" && provider != "custom") || (profile.APIKey == "" && provider != "custom") {
		return BatchStatus{}, batchError(400, "请先保存 API Key 和模型配置", "Save an API key and model first")
	}
	if err := profile.Validate(provider); err != nil {
		return BatchStatus{}, batchError(400, "翻译配置无效", "Invalid translation settings")
	}
	items, err := job.domain.List(ctx)
	if err != nil {
		return BatchStatus{}, batchError(500, "读取影片标题失败", "Failed to load JAV titles")
	}
	items = selectItems(items, scope)
	job.status = BatchStatus{Running: len(items) > 0, Total: len(items), Scope: scope}
	if job.status.Running {
		job.done = make(chan struct{})
		go job.run(job.ctx, items, cfg, job.done)
	}
	return job.status, nil
}

func (job *batch) run(ctx context.Context, items []dbpkg.JavTextTranslationItem, cfg map[string]string, done chan struct{}) {
	defer func() {
		job.mu.Lock()
		job.status.Running = false
		close(done)
		job.mu.Unlock()
	}()
	provider := ActiveProvider(cfg)
	profile := ActiveProfile(cfg)
	for _, item := range items {
		if ctx.Err() != nil {
			return
		}
		release, err := job.domain.Acquire(ctx, item.ID)
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
		current, err := job.domain.Get(ctx, item.ID)
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
		title, err := Translate(ctx, provider, profile, cfg["jav_title_translation_prompt"], item.Source, nil)
		var saved bool
		if err == nil {
			saved, err = job.domain.Save(ctx, &item, title)
		}
		release()
		if err != nil {
			_, zh, en := ErrorDetails(err)
			job.fail("重新翻译失败，未完成的内容保留原译文。"+zh, "Retranslation failed; unfinished titles are unchanged. "+en)
			return
		}
		job.record(saved)
	}
}

func (job *batch) record(saved bool) {
	job.mu.Lock()
	defer job.mu.Unlock()
	job.status.Processed++
	if saved {
		job.status.Translated++
	} else {
		job.status.Skipped++
	}
}

func (job *batch) fail(zh, en string) {
	job.mu.Lock()
	defer job.mu.Unlock()
	job.status.ErrorZH, job.status.ErrorEN = zh, en
}
