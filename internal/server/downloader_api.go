package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"javboss/internal/db"
	downloaderopenlist "javboss/internal/downloader/openlist"
	"javboss/internal/jav"
	"javboss/internal/models"
	"javboss/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type providerSettingsResponse struct {
	Address         string `json:"address"`
	RemoteFolder    string `json:"remote_folder"`
	TokenConfigured bool   `json:"token_configured"`
}

type downloaderSettingsResponse struct {
	ActiveProvider   string                              `json:"active_provider"`
	DirectoryID      *int64                              `json:"directory_id"`
	LocalConcurrency int                                 `json:"local_concurrency"`
	Providers        map[string]providerSettingsResponse `json:"providers"`
}

func getDownloaderSettings(c *gin.Context) {
	payload, err := loadDownloaderSettingsPayload(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取下载器配置失败", "Failed to load downloader settings")
		return
	}
	c.JSON(http.StatusOK, payload)
}

func updateDownloaderSettings(c *gin.Context) {
	var request struct {
		ActiveProvider   string `json:"active_provider"`
		DirectoryID      *int64 `json:"directory_id"`
		LocalConcurrency int    `json:"local_concurrency"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "下载器配置格式不正确", "Invalid downloader settings")
		return
	}
	request.ActiveProvider = strings.TrimSpace(request.ActiveProvider)
	if request.ActiveProvider != "" && !validDownloaderProvider(request.ActiveProvider) {
		respondLocalizedError(c, http.StatusBadRequest, "下载器类型不正确", "Invalid downloader provider")
		return
	}
	if request.LocalConcurrency < 1 || request.LocalConcurrency > 5 {
		respondLocalizedError(c, http.StatusBadRequest, "本地下载并发数必须在 1 到 5 之间", "Local download concurrency must be between 1 and 5")
		return
	}
	if request.DirectoryID != nil {
		directory, err := db.GetDirectory(c.Request.Context(), *request.DirectoryID)
		if err != nil || directory == nil || directory.IsDelete {
			respondLocalizedError(c, http.StatusBadRequest, "本地下载目录不存在", "The local download directory does not exist")
			return
		}
	}
	if request.ActiveProvider != "" {
		if request.DirectoryID == nil {
			respondLocalizedError(c, http.StatusBadRequest, "请选择本地下载目录", "Select a local download directory")
			return
		}
		configured, err := downloaderProviderConfigured(c.Request.Context(), request.ActiveProvider)
		if err != nil {
			respondLocalizedError(c, http.StatusInternalServerError, "读取下载器配置失败", "Failed to load downloader settings")
			return
		}
		if !configured {
			respondLocalizedError(c, http.StatusBadRequest, "请先完整配置所选下载器", "Configure the selected downloader first")
			return
		}
		if request.ActiveProvider == models.DownloaderProviderOpenList {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
			_, testErr := service.TestDownloader(ctx, request.ActiveProvider)
			cancel()
			if testErr != nil {
				if errors.Is(testErr, downloaderopenlist.ErrTemporaryDirectoryNotConfigured) {
					respondLocalizedError(
						c,
						http.StatusBadGateway,
						"尚未配置 115 Open 临时目录，请先前往 OpenList 管理后台完成配置后再试",
						"The 115 Open temporary directory is not configured. Configure it in the OpenList admin panel, then try again.",
					)
					return
				}
				respondLocalizedError(c, http.StatusBadGateway, "OpenList 115 Open 配置校验失败："+testErr.Error(), "OpenList 115 Open validation failed: "+testErr.Error())
				return
			}
		}
	}
	settings := models.DownloaderSettings{
		ID: 1, ActiveProvider: request.ActiveProvider, DirectoryID: request.DirectoryID,
		LocalConcurrency: request.LocalConcurrency,
	}
	if err := db.SaveDownloaderSettings(c.Request.Context(), &settings); err != nil {
		if errors.Is(err, db.ErrDownloaderProviderHasActiveJobs) {
			respondLocalizedError(c, http.StatusConflict, "队列中仍有未结束任务，暂时不能切换下载器", "The downloader cannot be switched while jobs are unfinished")
			return
		}
		respondLocalizedError(c, http.StatusInternalServerError, "保存下载器配置失败", "Failed to save downloader settings")
		return
	}
	service.WakeDownloadManager()
	payload, err := loadDownloaderSettingsPayload(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取下载器配置失败", "Failed to load downloader settings")
		return
	}
	c.JSON(http.StatusOK, payload)
}

func updateDownloaderProviderSettings(c *gin.Context) {
	provider := strings.TrimSpace(c.Param("provider"))
	if !validDownloaderProvider(provider) {
		respondLocalizedError(c, http.StatusNotFound, "下载器不存在", "Downloader provider not found")
		return
	}
	var request struct {
		Address       string  `json:"address"`
		APIToken      *string `json:"api_token"`
		ClearAPIToken bool    `json:"clear_api_token"`
		RemoteFolder  string  `json:"remote_folder"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "下载器配置格式不正确", "Invalid provider settings")
		return
	}
	address := strings.TrimSpace(request.Address)
	remoteFolder := strings.TrimSpace(request.RemoteFolder)
	if len(address) > 500 || len(remoteFolder) > 2000 {
		respondLocalizedError(c, http.StatusBadRequest, "下载器地址或目录过长", "Downloader address or folder is too long")
		return
	}
	if request.APIToken != nil && len(*request.APIToken) > 16384 {
		respondLocalizedError(c, http.StatusBadRequest, "API Token 过长", "The API token is too long")
		return
	}
	current, err := db.GetDownloaderProviderSettings(c.Request.Context(), provider)
	if err == nil {
		err = db.SaveDownloaderProviderSettings(c.Request.Context(), &models.DownloaderProviderSettings{
			Provider: provider, Address: address,
			APIToken:     updatedProviderToken(current.APIToken, request.APIToken, request.ClearAPIToken),
			RemoteFolder: remoteFolder,
		})
	}
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "保存下载器配置失败", "Failed to save provider settings")
		return
	}
	payload, err := loadDownloaderSettingsPayload(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取下载器配置失败", "Failed to load downloader settings")
		return
	}
	c.JSON(http.StatusOK, payload.Providers[provider])
}

func getDownloaderProviderToken(c *gin.Context) {
	provider := strings.TrimSpace(c.Param("provider"))
	if !validDownloaderProvider(provider) {
		respondLocalizedError(c, http.StatusNotFound, "下载器不存在", "Downloader provider not found")
		return
	}
	settings, err := db.GetDownloaderProviderSettings(c.Request.Context(), provider)
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取 API Token 失败", "Failed to load the API token")
		return
	}
	c.JSON(http.StatusOK, gin.H{"api_token": settings.APIToken})
}

func testDownloaderProvider(c *gin.Context) {
	provider := strings.TrimSpace(c.Param("provider"))
	if !validDownloaderProvider(provider) {
		respondLocalizedError(c, http.StatusNotFound, "下载器不存在", "Downloader provider not found")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	result, err := service.TestDownloader(ctx, provider)
	if err != nil {
		if errors.Is(err, downloaderopenlist.ErrTemporaryDirectoryNotConfigured) {
			respondLocalizedError(
				c,
				http.StatusBadGateway,
				"尚未配置 115 Open 临时目录，请先前往 OpenList 管理后台完成配置后再试",
				"The 115 Open temporary directory is not configured. Configure it in the OpenList admin panel, then try again.",
			)
			return
		}
		respondLocalizedError(c, http.StatusBadGateway, "下载器连接测试失败："+err.Error(), "Downloader connection test failed: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}

func loadDownloaderSettingsPayload(ctx context.Context) (*downloaderSettingsResponse, error) {
	settings, err := db.GetDownloaderSettings(ctx)
	if err != nil {
		return nil, err
	}
	providers := make(map[string]providerSettingsResponse, 2)
	for _, provider := range []string{models.DownloaderProviderCloudDrive2, models.DownloaderProviderOpenList} {
		providerSettings, loadErr := db.GetDownloaderProviderSettings(ctx, provider)
		if loadErr != nil {
			return nil, loadErr
		}
		providers[provider] = providerSettingsPayload(
			providerSettings.Address, providerSettings.RemoteFolder, providerSettings.APIToken,
		)
	}
	return &downloaderSettingsResponse{
		ActiveProvider: settings.ActiveProvider, DirectoryID: settings.DirectoryID,
		LocalConcurrency: settings.LocalConcurrency,
		Providers:        providers,
	}, nil
}

func providerSettingsPayload(address, folder, token string) providerSettingsResponse {
	return providerSettingsResponse{
		Address: address, RemoteFolder: folder, TokenConfigured: strings.TrimSpace(token) != "",
	}
}

func updatedProviderToken(current string, requested *string, clear bool) string {
	if clear {
		return ""
	}
	if requested != nil && strings.TrimSpace(*requested) != "" {
		return strings.TrimSpace(*requested)
	}
	return current
}

func validDownloaderProvider(provider string) bool {
	return provider == models.DownloaderProviderCloudDrive2 || provider == models.DownloaderProviderOpenList
}

func downloaderProviderConfigured(ctx context.Context, provider string) (bool, error) {
	if !validDownloaderProvider(provider) {
		return false, nil
	}
	settings, err := db.GetDownloaderProviderSettings(ctx, provider)
	return err == nil && strings.TrimSpace(settings.Address) != "" && strings.TrimSpace(settings.APIToken) != "" && strings.TrimSpace(settings.RemoteFolder) != "", err
}

func listDownloadJobs(c *gin.Context) {
	jobs, err := db.ListDownloadJobs(c.Request.Context(), queryInt(c, "limit", 100))
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取下载队列失败", "Failed to load the download queue")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": jobs})
}

func createDiscoveryDownload(c *gin.Context) {
	itemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || itemID <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "作品 ID 不正确", "Invalid item ID")
		return
	}
	var request struct {
		MagnetURL   string `json:"magnet_url"`
		DirectoryID *int64 `json:"directory_id"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "下载请求格式不正确", "Invalid download request")
		return
	}
	item, err := db.GetJavDiscoveryItem(c.Request.Context(), itemID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), gorm.ErrRecordNotFound.Error()) {
			respondLocalizedError(c, http.StatusNotFound, "发现作品不存在", "Discovery item not found")
			return
		}
		respondLocalizedError(c, http.StatusInternalServerError, "读取发现作品失败", "Failed to read the discovery item")
		return
	}
	magnetURL := strings.TrimSpace(request.MagnetURL)
	var magnets []jav.JavBusMagnetLink
	if err := json.Unmarshal([]byte(item.MagnetLinksJSON), &magnets); err != nil {
		respondLocalizedError(c, http.StatusConflict, "请先打开作品详情加载磁力链接", "Open the item details to load magnet links first")
		return
	}
	magnetName := ""
	allowed := false
	for _, magnet := range magnets {
		if strings.TrimSpace(magnet.URL) == magnetURL {
			allowed = true
			magnetName = strings.TrimSpace(magnet.Name)
			break
		}
	}
	if !allowed {
		respondLocalizedError(c, http.StatusBadRequest, "磁力链接不属于该发现作品", "The magnet link does not belong to this discovery item")
		return
	}
	sourceID := itemID
	enqueueDownloadJob(c, item.Code, models.DownloadSourceDiscovery, &sourceID, magnetURL, magnetName, request.DirectoryID)
}

func createSourceFreeDownload(c *gin.Context) {
	var request struct {
		Code        string `json:"code"`
		MagnetURL   string `json:"magnet_url"`
		MagnetName  string `json:"magnet_name"`
		DirectoryID *int64 `json:"directory_id"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "下载请求格式不正确", "Invalid download request")
		return
	}
	enqueueDownloadJob(c, request.Code, "", nil, request.MagnetURL, request.MagnetName, request.DirectoryID)
}

func enqueueDownloadJob(c *gin.Context, code, sourceType string, sourceID *int64, magnetURL, magnetName string, requestedDirectoryID *int64) {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 200 {
		respondLocalizedError(c, http.StatusBadRequest, "作品番号不能为空且不能超过 200 个字符", "The work code is required and must not exceed 200 characters")
		return
	}
	magnetURL = strings.TrimSpace(magnetURL)
	infoHash, err := service.ParseMagnetInfoHash(magnetURL)
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "磁力链接格式不正确", "Invalid magnet link")
		return
	}
	settings, err := db.GetDownloaderSettings(c.Request.Context())
	if err != nil || !validDownloaderProvider(settings.ActiveProvider) {
		respondLocalizedError(c, http.StatusConflict, "尚未激活下载器", "No downloader is active")
		return
	}
	directoryID := settings.DirectoryID
	if requestedDirectoryID != nil {
		directoryID = requestedDirectoryID
	}
	if directoryID == nil || *directoryID <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "请选择本地下载目录", "Select a local download directory")
		return
	}
	directory, err := db.GetDirectory(c.Request.Context(), *directoryID)
	if err != nil || directory == nil || directory.IsDelete {
		respondLocalizedError(c, http.StatusBadRequest, "本地下载目录不存在", "The local download directory does not exist")
		return
	}
	var sourceTypeValue *string
	if sourceType = strings.TrimSpace(sourceType); sourceType != "" {
		sourceTypeValue = &sourceType
	}
	job := models.DownloadJob{
		SourceType: sourceTypeValue, SourceID: sourceID, Code: code,
		DirectoryID: *directoryID, InfoHash: infoHash, MagnetURL: magnetURL,
		MagnetName: strings.TrimSpace(magnetName), Provider: settings.ActiveProvider,
	}
	if err := db.CreateDownloadJob(c.Request.Context(), &job); err != nil {
		switch {
		case errors.Is(err, db.ErrDownloadJobExists):
			respondLocalizedError(c, http.StatusConflict, "该磁力已经在此目录的下载队列中", "This magnet is already queued for the selected directory")
		case errors.Is(err, db.ErrDownloaderProviderChanged):
			respondLocalizedError(c, http.StatusConflict, "下载器配置已经变化，请重试", "The active downloader changed; try again")
		default:
			respondLocalizedError(c, http.StatusInternalServerError, "创建下载任务失败", "Failed to create the download job")
		}
		return
	}
	service.WakeDownloadManager()
	c.JSON(http.StatusCreated, job)
}

func retryDownloadJob(c *gin.Context) {
	id, ok := downloadJobID(c)
	if !ok {
		return
	}
	job, err := db.GetDownloadJob(c.Request.Context(), id)
	if err != nil {
		respondLocalizedError(c, http.StatusNotFound, "下载任务不存在", "Download job not found")
		return
	}
	settings, err := db.GetDownloaderSettings(c.Request.Context())
	if err != nil || settings.ActiveProvider != job.Provider {
		respondLocalizedError(c, http.StatusConflict, "请先激活该任务使用的下载器", "Activate the downloader used by this job first")
		return
	}
	if err := db.RetryDownloadJob(c.Request.Context(), id); err != nil {
		respondLocalizedError(c, http.StatusConflict, "该任务当前不能重试", "The job cannot be retried in its current state")
		return
	}
	service.WakeDownloadManager()
	c.Status(http.StatusNoContent)
}

func cancelDownloadJob(c *gin.Context) {
	id, ok := downloadJobID(c)
	if !ok {
		return
	}
	if err := db.CancelDownloadJob(c.Request.Context(), id); err != nil {
		respondLocalizedError(c, http.StatusConflict, "该任务当前不能取消", "The job cannot be canceled in its current state")
		return
	}
	service.CancelDownloadJob(id)
	c.Status(http.StatusNoContent)
}

func deleteDownloadJob(c *gin.Context) {
	id, ok := downloadJobID(c)
	if !ok {
		return
	}
	if err := db.DeleteDownloadJob(c.Request.Context(), id); err != nil {
		respondLocalizedError(c, http.StatusConflict, "只能删除已结束的下载任务", "Only finished download jobs can be deleted")
		return
	}
	c.Status(http.StatusNoContent)
}

func downloadJobID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "下载任务 ID 不正确", "Invalid download job ID")
		return 0, false
	}
	return id, true
}
