package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"javboss/internal/common"
	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/manager"
	"javboss/internal/models"
)

func searchJav(c *gin.Context) {
	limit := queryInt(c, "limit", 100)
	offset := queryInt(c, "offset", 0)
	idolIDs := parseInt64CSV(c.Query("idol_ids"))
	tagIDs := parseInt64CSV(c.Query("tag_ids"))
	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))
	studioID := int64(-1)
	if studioParam := strings.TrimSpace(c.Query("studio_id")); studioParam != "" {
		parsed, err := strconv.ParseInt(studioParam, 10, 64)
		if err != nil || parsed < 0 {
			respondLocalizedError(c, http.StatusBadRequest, "片商 ID 无效", "Invalid studio ID")
			return
		}
		studioID = parsed
	}
	seriesID := int64(0)
	if seriesParam := strings.TrimSpace(c.Query("series_id")); seriesParam != "" {
		parsed, err := strconv.ParseInt(seriesParam, 10, 64)
		if err != nil || parsed <= 0 {
			respondLocalizedError(c, http.StatusBadRequest, "系列 ID 无效", "Invalid series ID")
			return
		}
		seriesID = parsed
	}
	search := strings.TrimSpace(c.Query("search"))
	prefix := strings.TrimSpace(c.Query("prefix"))
	sort := strings.TrimSpace(c.Query("sort"))
	soloOnly := queryBool(c, "solo", false)
	favoriteGroupID := int64(0)
	if favoriteGroupParam := strings.TrimSpace(c.Query("favorite_group_id")); favoriteGroupParam != "" {
		parsed, err := strconv.ParseInt(favoriteGroupParam, 10, 64)
		if err != nil || parsed <= 0 {
			respondLocalizedError(c, http.StatusBadRequest, "收藏夹 ID 无效", "Invalid favorite group ID")
			return
		}
		favoriteGroupID = parsed
	}
	seedParam := strings.TrimSpace(c.Query("seed"))
	var seed *int64
	if seedParam != "" {
		parsed, err := strconv.ParseInt(seedParam, 10, 64)
		if err != nil || parsed <= 0 {
			respondLocalizedError(c, http.StatusBadRequest, "随机种子无效", "Invalid random seed")
			return
		}
		seed = &parsed
	}

	items, total, err := dbpkg.SearchJavWithPrefix(c.Request.Context(), idolIDs, tagIDs, search, prefix, sort, limit, offset, seed, directoryIDs, studioID, seriesID, boolInt64(soloOnly), favoriteGroupID)
	if err != nil {
		logging.Error("SearchJav: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "搜索 JAV 作品失败", "Failed to search JAV items")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

func listJavPrefixes(c *gin.Context) {
	items, err := dbpkg.ListJavPrefixes(c.Request.Context(), parseDirectoryIDs(c.Query("directory_ids")))
	if err != nil {
		logging.Error("list jav prefixes error: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载 JAV 番号前缀失败", "Failed to load JAV code prefixes")
		return
	}
	if items == nil {
		items = []dbpkg.JavPrefixSummary{}
	}
	c.JSON(http.StatusOK, items)
}

func boolInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func getJavJavDBURL(c *gin.Context) {
	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		respondLocalizedError(c, http.StatusBadRequest, "番号不能为空", "JAV code is required")
		return
	}

	javdbURL, err := jav.LookupJavDBURLByCode(code)
	if err != nil {
		if errors.Is(err, jav.ResourceNotFonud) {
			respondLocalizedError(c, http.StatusNotFound, "未找到对应的 JavDB 页面", "JavDB page was not found")
			return
		}
		logging.Error("lookup javdb url code=%s: %v", code, err)
		respondLocalizedError(c, http.StatusInternalServerError, "查询 JavDB 页面失败", "Failed to look up the JavDB page")
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": javdbURL})
}

func resolveJavSampleImages(c *gin.Context) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "JAV 作品 ID 无效", "Invalid JAV item ID")
		return
	}

	item, err := dbpkg.GetJav(c.Request.Context(), id, parseDirectoryIDs(c.Query("directory_ids")))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "JAV 作品不存在", "JAV item was not found")
			return
		}
		logging.Error("get JAV sample images item id=%d: %v", id, err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载样品图失败", "Failed to load sample images")
		return
	}
	if len(item.SampleImages) > 0 {
		c.JSON(http.StatusOK, gin.H{"sample_images": item.SampleImages})
		return
	}

	images, lookupErr := lookupJavSampleImagesByProvider(item.Code, jav.LookupJavByCode)
	if len(images) == 0 {
		if lookupErr != nil {
			logging.Error("lookup JAV sample images code=%s: %v", item.Code, lookupErr)
			respondLocalizedError(c, http.StatusBadGateway, "样品图来源暂时不可用，请稍后重试", "Sample image providers are temporarily unavailable; try again later")
			return
		}
		if err := dbpkg.MarkJavSampleImagesNotFound(c.Request.Context(), item.ID); err != nil {
			logging.Error("mark JAV sample images not found id=%d code=%s: %v", item.ID, item.Code, err)
			respondLocalizedError(c, http.StatusInternalServerError, "保存样品图查询状态失败", "Failed to save sample image lookup state")
			return
		}
		c.JSON(http.StatusOK, gin.H{"sample_images": models.NewJavSampleImagesNotFound()})
		return
	}

	stored, err := dbpkg.SetJavSampleImagesIfEmpty(c.Request.Context(), item.ID, images)
	if err != nil {
		logging.Error("save JAV sample images id=%d code=%s: %v", item.ID, item.Code, err)
		respondLocalizedError(c, http.StatusInternalServerError, "保存样品图失败", "Failed to save sample images")
		return
	}
	c.JSON(http.StatusOK, gin.H{"sample_images": stored})
}

type javSampleImageLookupFunc func(string, jav.Provider) (*jav.JavInfo, error)

func lookupJavSampleImagesByProvider(code string, lookup javSampleImageLookupFunc) (models.JavSampleImages, error) {
	if strings.TrimSpace(code) == "" || lookup == nil {
		return models.JavSampleImages{}, nil
	}

	var lookupErrors []error
	for _, provider := range []jav.Provider{jav.ProviderJavMenu, jav.ProviderJavDB} {
		info, err := lookup(code, provider)
		if err != nil {
			if !errors.Is(err, jav.ResourceNotFonud) {
				lookupErrors = append(lookupErrors, fmt.Errorf("%s: %w", provider.String(), err))
			}
			continue
		}
		images := javSampleImagesToModel(info)
		if len(images) > 0 {
			return images, nil
		}
	}
	return models.JavSampleImages{}, errors.Join(lookupErrors...)
}

func javSampleImagesToModel(info *jav.JavInfo) models.JavSampleImages {
	if info == nil {
		return models.JavSampleImages{}
	}
	images := make(models.JavSampleImages, 0, len(info.SampleImages))
	seen := make(map[string]struct{}, len(info.SampleImages))
	for _, image := range info.SampleImages {
		thumbnailURL := strings.TrimSpace(image.ThumbnailURL)
		detailURL := strings.TrimSpace(image.DetailURL)
		if thumbnailURL == "" {
			thumbnailURL = detailURL
		}
		if detailURL == "" {
			detailURL = thumbnailURL
		}
		if thumbnailURL == "" {
			continue
		}
		key := thumbnailURL + "\x00" + detailURL
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		images = append(images, models.JavSampleImage{
			ThumbnailURL: thumbnailURL,
			DetailURL:    detailURL,
		})
	}
	return images
}

func listJavTags(c *gin.Context) {
	tags, err := dbpkg.ListJavTags(c.Request.Context(), parseDirectoryIDs(c.Query("directory_ids")))
	if err != nil {
		logging.Error("list jav tags error: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载 JAV 标签失败", "Failed to load JAV tags")
		return
	}
	if tags == nil {
		tags = []dbpkg.JavTagCount{}
	}
	c.JSON(http.StatusOK, tags)
}

type javItemUpdateRequest struct {
	Title       *string  `json:"title"`
	CoverURL    *string  `json:"cover_url"`
	TagIDs      *[]int64 `json:"tag_ids"`
	IdolIDs     *[]int64 `json:"idol_ids"`
	StudioID    *int64   `json:"studio_id"`
	SeriesID    *int64   `json:"series_id"`
	ReleaseDate *string  `json:"release_date"`
	DurationMin *int     `json:"duration_min"`
}

func updateJavItem(c *gin.Context) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "JAV 作品 ID 无效", "Invalid JAV item ID")
		return
	}

	var req javItemUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "修改 JAV 信息请求无效", "Invalid JAV item update request")
		return
	}

	var releaseUnix *int64
	if req.ReleaseDate != nil {
		parsed, err := parseJavEditReleaseUnix(*req.ReleaseDate)
		if err != nil {
			respondLocalizedError(c, http.StatusBadRequest, "发行日期格式必须为 YYYY-MM-DD", "Release date must use the YYYY-MM-DD format")
			return
		}
		releaseUnix = &parsed
	}

	if req.CoverURL != nil {
		coverURL := strings.TrimSpace(*req.CoverURL)
		if coverURL != "" {
			cfg := common.AppConfig
			if cfg == nil {
				respondLocalizedError(c, http.StatusInternalServerError, "应用配置尚未加载", "Application configuration is not loaded")
				return
			}
			item, err := dbpkg.GetJav(c.Request.Context(), id, parseDirectoryIDs(c.Query("directory_ids")))
			if err != nil {
				logging.Error("get jav for cover update error: %v", err)
				respondLocalizedError(c, http.StatusBadRequest, "读取 JAV 作品信息失败", "Failed to load the JAV item")
				return
			}
			ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
			defer cancel()
			if err := manager.DownloadCoverFromURL(ctx, cfg.JavCoverDir, item.Code, coverURL); err != nil {
				respondLocalizedError(c, http.StatusBadRequest, "下载 JAV 封面失败，请检查图片地址", "Failed to download the JAV cover; check the image URL")
				return
			}
		}
	}

	updated, err := dbpkg.UpdateJav(c.Request.Context(), id, dbpkg.JavUpdateInput{
		Title:       req.Title,
		StudioID:    req.StudioID,
		SeriesID:    req.SeriesID,
		IdolIDs:     req.IdolIDs,
		UserTagIDs:  req.TagIDs,
		ReleaseUnix: releaseUnix,
		DurationMin: req.DurationMin,
	}, parseDirectoryIDs(c.Query("directory_ids")))
	if err != nil {
		logging.Error("update jav item error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "保存 JAV 作品信息失败", "Failed to save JAV item information")
		return
	}
	c.JSON(http.StatusOK, updated)
}

func parseJavEditReleaseUnix(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return 0, errors.New("release_date must be YYYY-MM-DD")
	}
	return t.Unix(), nil
}

func createJavTag(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "创建 JAV 标签请求无效", "Invalid JAV tag creation request")
		return
	}

	tag, err := dbpkg.CreateJavTag(c.Request.Context(), req.Name)
	if err != nil {
		logging.Error("create jav tag error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "创建 JAV 标签失败，标签名称可能为空或已存在", "Failed to create JAV tag; the name may be empty or already exist")
		return
	}
	c.JSON(http.StatusCreated, dbpkg.JavTagCount{
		ID:       tag.ID,
		Name:     tag.Name,
		Provider: tag.Provider,
		Count:    0,
	})
}

func renameJavTag(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "JAV 标签 ID 无效", "Invalid JAV tag ID")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "重命名 JAV 标签请求无效", "Invalid JAV tag rename request")
		return
	}

	if err := dbpkg.RenameJavTag(c.Request.Context(), id, req.Name); err != nil {
		logging.Error("rename jav tag error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "重命名 JAV 标签失败，标签名称可能为空或已存在", "Failed to rename JAV tag; the name may be empty or already exist")
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteJavTag(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "JAV 标签 ID 无效", "Invalid JAV tag ID")
		return
	}

	if err := dbpkg.DeleteJavTag(c.Request.Context(), id); err != nil {
		logging.Error("delete jav tag error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "删除 JAV 标签失败", "Failed to delete JAV tag")
		return
	}
	c.Status(http.StatusNoContent)
}

type javTagRequest struct {
	JavIDs []int64 `json:"jav_ids"`
	TagID  int64   `json:"tag_id"`
}

type javTagsReplaceRequest struct {
	JavIDs []int64 `json:"jav_ids"`
	TagIDs []int64 `json:"tag_ids"`
}

type javTagsBatchDeleteRequest struct {
	TagIDs []int64 `json:"tag_ids"`
}

func addJavTagsToItems(c *gin.Context) {
	var req javTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "添加 JAV 标签请求无效", "Invalid add-JAV-tag request")
		return
	}
	if req.TagID <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "JAV 标签 ID 无效", "Invalid JAV tag ID")
		return
	}
	if err := dbpkg.AddJavTagToJavs(c.Request.Context(), req.TagID, req.JavIDs); err != nil {
		logging.Error("add jav tag error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "添加 JAV 标签失败", "Failed to add JAV tag")
		return
	}
	c.Status(http.StatusNoContent)
}

func removeJavTagsFromItems(c *gin.Context) {
	var req javTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "移除 JAV 标签请求无效", "Invalid remove-JAV-tag request")
		return
	}
	if req.TagID <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "JAV 标签 ID 无效", "Invalid JAV tag ID")
		return
	}
	if err := dbpkg.RemoveJavTagFromJavs(c.Request.Context(), req.TagID, req.JavIDs); err != nil {
		logging.Error("remove jav tag error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "移除 JAV 标签失败", "Failed to remove JAV tag")
		return
	}
	c.Status(http.StatusNoContent)
}

func replaceJavTagsForItems(c *gin.Context) {
	var req javTagsReplaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "更新 JAV 标签请求无效", "Invalid JAV tag update request")
		return
	}
	if len(req.JavIDs) == 0 {
		respondLocalizedError(c, http.StatusBadRequest, "JAV 作品 ID 不能为空", "JAV item IDs are required")
		return
	}
	if err := dbpkg.ReplaceJavUserTags(c.Request.Context(), req.JavIDs, req.TagIDs); err != nil {
		logging.Error("replace jav tags error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "更新 JAV 标签失败", "Failed to update JAV tags")
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteJavTagsBatch(c *gin.Context) {
	var req javTagsBatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "批量删除 JAV 标签请求无效", "Invalid batch JAV tag deletion request")
		return
	}
	if len(req.TagIDs) == 0 {
		respondLocalizedError(c, http.StatusBadRequest, "JAV 标签 ID 不能为空", "JAV tag IDs are required")
		return
	}
	if err := dbpkg.DeleteJavTags(c.Request.Context(), req.TagIDs); err != nil {
		logging.Error("delete jav tags error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "批量删除 JAV 标签失败", "Failed to delete JAV tags")
		return
	}
	c.Status(http.StatusNoContent)
}
