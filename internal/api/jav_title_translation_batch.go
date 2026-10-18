package api

import (
	"errors"
	"io"
	"javboss/internal/jav/translation"
	"net/http"

	"github.com/gin-gonic/gin"
)

func getTitleTranslationBatch(c *gin.Context)     { getTranslationBatch(c, javTitleDomain) }
func previewTitleTranslationBatch(c *gin.Context) { previewTranslationBatch(c, javTitleDomain) }
func startTitleTranslationBatch(c *gin.Context)   { startTranslationBatch(c, javTitleDomain) }
func stopTitleTranslationBatch(c *gin.Context)    { stopTranslationBatch(c, javTitleDomain) }

func getTranslationBatch(c *gin.Context, domain textTranslationDomain) {
	c.JSON(http.StatusOK, translation.GetBatchStatus(domain.Series))
}

func previewTranslationBatch(c *gin.Context, domain textTranslationDomain) {
	result, err := translation.PreviewBatch(c.Request.Context(), domain.Series, c.DefaultQuery("scope", "missing"))
	if err != nil {
		status, zh, en := translation.ErrorDetails(err)
		domain.error(c, status, zh, en)
		return
	}
	c.JSON(http.StatusOK, result)
}

func stopTranslationBatch(c *gin.Context, domain textTranslationDomain) {
	c.JSON(http.StatusOK, translation.StopBatch(domain.Series))
}

func startTranslationBatch(c *gin.Context, domain textTranslationDomain) {
	var req struct {
		Scope            string `json:"scope"`
		ConfirmOverwrite bool   `json:"confirm_overwrite"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		domain.error(c, 400, "批量翻译请求无效", "Invalid batch translation request")
		return
	}
	status, err := translation.StartBatch(c.Request.Context(), domain.Series, req.Scope, req.ConfirmOverwrite)
	if err != nil {
		code, zh, en := translation.ErrorDetails(err)
		domain.error(c, code, zh, en)
		return
	}
	c.JSON(http.StatusAccepted, status)
}
