package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
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
	"javboss/internal/util"
)

type javFilterQuery struct {
	IdolIDs           []int64
	TagIDs            []int64
	DirectoryIDs      []int64
	Search            string
	Prefix            string
	StudioID          int64
	SeriesID          int64
	SoloOnly          bool
	FavoriteGroupID   int64
	FavoriteRatingMin *float64
	FavoriteRatingMax *float64
}

func parseJavFilterQuery(c *gin.Context) (javFilterQuery, bool) {
	query := javFilterQuery{
		IdolIDs:      parseInt64CSV(c.Query("idol_ids")),
		TagIDs:       parseInt64CSV(c.Query("tag_ids")),
		DirectoryIDs: parseDirectoryIDs(c.Query("directory_ids")),
		Search:       strings.TrimSpace(c.Query("search")),
		Prefix:       strings.TrimSpace(c.Query("prefix")),
		StudioID:     -1,
		SoloOnly:     queryBool(c, "solo", false),
	}
	if studioParam := strings.TrimSpace(c.Query("studio_id")); studioParam != "" {
		parsed, err := strconv.ParseInt(studioParam, 10, 64)
		if err != nil || parsed < 0 {
			respondLocalizedError(c, http.StatusBadRequest, "片商 ID 无效", "Invalid studio ID")
			return query, false
		}
		query.StudioID = parsed
	}
	if seriesParam := strings.TrimSpace(c.Query("series_id")); seriesParam != "" {
		parsed, err := strconv.ParseInt(seriesParam, 10, 64)
		if err != nil || parsed <= 0 {
			respondLocalizedError(c, http.StatusBadRequest, "系列 ID 无效", "Invalid series ID")
			return query, false
		}
		query.SeriesID = parsed
	}
	favoriteRatingMinParam := strings.TrimSpace(c.Query("favorite_rating_min"))
	favoriteRatingMaxParam := strings.TrimSpace(c.Query("favorite_rating_max"))
	if favoriteRatingMinParam != "" || favoriteRatingMaxParam != "" {
		if favoriteRatingMinParam == "" || favoriteRatingMaxParam == "" {
			respondLocalizedError(c, http.StatusBadRequest, "喜爱度范围无效", "Invalid favorite rating range")
			return query, false
		}
		parsedMin, minErr := strconv.ParseFloat(favoriteRatingMinParam, 64)
		parsedMax, maxErr := strconv.ParseFloat(favoriteRatingMaxParam, 64)
		if minErr != nil || maxErr != nil || math.IsNaN(parsedMin) || math.IsNaN(parsedMax) || math.IsInf(parsedMin, 0) || math.IsInf(parsedMax, 0) || parsedMin < 0.5 || parsedMax > 5 || parsedMin > parsedMax || math.Abs(parsedMin*2-math.Round(parsedMin*2)) > 1e-9 || math.Abs(parsedMax*2-math.Round(parsedMax*2)) > 1e-9 {
			respondLocalizedError(c, http.StatusBadRequest, "喜爱度范围无效", "Invalid favorite rating range")
			return query, false
		}
		query.FavoriteRatingMin = &parsedMin
		query.FavoriteRatingMax = &parsedMax
	}
	if favoriteGroupParam := strings.TrimSpace(c.Query("favorite_group_id")); favoriteGroupParam != "" {
		parsed, err := strconv.ParseInt(favoriteGroupParam, 10, 64)
		if err != nil || parsed <= 0 {
			respondLocalizedError(c, http.StatusBadRequest, "收藏夹 ID 无效", "Invalid favorite group ID")
			return query, false
		}
		query.FavoriteGroupID = parsed
	}
	return query, true
}

func searchJav(c *gin.Context) {
	limit := queryInt(c, "limit", 100)
	offset := queryInt(c, "offset", 0)
	filterQuery, ok := parseJavFilterQuery(c)
	if !ok {
		return
	}
	sort := strings.TrimSpace(c.Query("sort"))
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

	items, total, err := dbpkg.SearchJavWithPrefixFilters(c.Request.Context(), filterQuery.IdolIDs, filterQuery.TagIDs, filterQuery.Search, filterQuery.Prefix, sort, limit, offset, seed, filterQuery.DirectoryIDs, dbpkg.JavSearchFilters{
		StudioID:          filterQuery.StudioID,
		SeriesID:          filterQuery.SeriesID,
		SoloOnly:          filterQuery.SoloOnly,
		FavoriteGroupID:   filterQuery.FavoriteGroupID,
		FavoriteRatingMin: filterQuery.FavoriteRatingMin,
		FavoriteRatingMax: filterQuery.FavoriteRatingMax,
	})
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

func listJavFilterOptions(c *gin.Context) {
	filterQuery, ok := parseJavFilterQuery(c)
	if !ok {
		return
	}
	options, err := dbpkg.ListJavFilterOptions(
		c.Request.Context(),
		filterQuery.IdolIDs,
		filterQuery.TagIDs,
		filterQuery.Search,
		filterQuery.Prefix,
		filterQuery.DirectoryIDs,
		dbpkg.JavSearchFilters{
			StudioID:          filterQuery.StudioID,
			SeriesID:          filterQuery.SeriesID,
			SoloOnly:          filterQuery.SoloOnly,
			FavoriteGroupID:   filterQuery.FavoriteGroupID,
			FavoriteRatingMin: filterQuery.FavoriteRatingMin,
			FavoriteRatingMax: filterQuery.FavoriteRatingMax,
		},
		dbpkg.JavFilterOptionSearches{
			Prefix: strings.TrimSpace(c.Query("prefix_search")),
			Idol:   strings.TrimSpace(c.Query("idol_search")),
			Tag:    strings.TrimSpace(c.Query("tag_search")),
			Studio: strings.TrimSpace(c.Query("studio_search")),
			Series: strings.TrimSpace(c.Query("series_search")),
		},
		queryInt(c, "option_limit", 120),
	)
	if err != nil {
		logging.Error("list JAV filter options: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载 JAV 筛选候选项失败", "Failed to load JAV filter options")
		return
	}
	c.JSON(http.StatusOK, options)
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

func redirectJavAvsox(c *gin.Context) {
	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		respondLocalizedError(c, http.StatusBadRequest, "番号不能为空", "JAV code is required")
		return
	}

	detailURL, err := jav.LookupAvsoxURLByCode(code)
	if err != nil {
		if errors.Is(err, jav.ResourceNotFonud) {
			respondLocalizedError(c, http.StatusNotFound, "未找到对应的 Avsox 详情页", "Avsox detail page was not found")
			return
		}
		logging.Error("lookup avsox redirect code=%s: %v", code, err)
		respondLocalizedError(c, http.StatusBadGateway, "查询 Avsox 详情页失败", "Failed to look up the Avsox detail page")
		return
	}
	c.Redirect(http.StatusFound, detailURL)
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

	images, lookupErr := lookupJavSampleImagesByProvider(
		c.Request.Context(),
		item.Code,
		jav.LookupJavByCode,
		validateJavSampleImageDetailURL,
	)
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
type javSampleImageURLValidator func(context.Context, string) (bool, error)

func lookupJavSampleImagesByProvider(
	ctx context.Context,
	code string,
	lookup javSampleImageLookupFunc,
	validateURL javSampleImageURLValidator,
) (models.JavSampleImages, error) {
	if strings.TrimSpace(code) == "" || lookup == nil {
		return models.JavSampleImages{}, nil
	}

	var lookupErrors []error
	for _, provider := range []jav.Provider{jav.ProviderJavMenu, jav.ProviderJavBus} {
		info, err := lookup(code, provider)
		if err != nil {
			if !errors.Is(err, jav.ResourceNotFonud) {
				lookupErrors = append(lookupErrors, fmt.Errorf("%s: %w", provider.String(), err))
			}
			continue
		}
		images := javSampleImagesToModel(info)
		if len(images) == 0 {
			continue
		}

		detailURL := lastJavSampleImageDetailURL(images)
		if detailURL == "" || validateURL == nil {
			continue
		}
		valid, err := validateURL(ctx, detailURL)
		if err != nil {
			lookupErrors = append(lookupErrors, fmt.Errorf("%s sample image validation: %w", provider.String(), err))
			continue
		}
		if !valid {
			logging.Info("skip invalid JAV sample images provider=%s detail_url=%s", provider.String(), detailURL)
			continue
		}
		return images, nil
	}
	return models.JavSampleImages{}, errors.Join(lookupErrors...)
}

func lastJavSampleImageDetailURL(images models.JavSampleImages) string {
	for index := len(images) - 1; index >= 0; index-- {
		if detailURL := strings.TrimSpace(images[index].DetailURL); detailURL != "" {
			return detailURL
		}
	}
	return ""
}

func validateJavSampleImageDetailURL(ctx context.Context, detailURL string) (bool, error) {
	detailURL = strings.TrimSpace(detailURL)
	parsed, err := url.Parse(detailURL)
	if err != nil || parsed == nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, detailURL, nil)
	if err != nil {
		return false, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")
	if host := strings.ToLower(parsed.Hostname()); host == "pics.dmm.co.jp" || strings.HasSuffix(host, ".dmm.co.jp") {
		req.Header.Set("Referer", "https://www.dmm.co.jp/")
	}

	resp, err := util.DoRequest(req)
	if err != nil {
		if errors.Is(err, util.ErrCachedNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("request image: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent:
	case http.StatusNotFound, http.StatusGone:
		return false, nil
	default:
		return false, fmt.Errorf("image returned %s", resp.Status)
	}

	header, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil {
		return false, fmt.Errorf("read image header: %w", err)
	}
	if len(header) == 0 {
		return false, nil
	}
	detectedType := strings.ToLower(http.DetectContentType(header))
	if strings.HasPrefix(detectedType, "image/") {
		return true, nil
	}
	declaredType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	return strings.HasPrefix(declaredType, "image/") && detectedType == "application/octet-stream", nil
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

func organizeJavTags(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()

	genres, err := jav.FetchJavBusGenreCategories(ctx)
	if err != nil {
		logging.Error("fetch javbus tag categories error: %v", err)
		respondLocalizedError(c, http.StatusBadGateway, "读取 JavBus 标签分类失败，请确认网络或代理可访问 JavBus", "Failed to read JavBus tag categories; check JavBus network or proxy access")
		return
	}
	result, err := dbpkg.OrganizeJavTagCategories(ctx, genres)
	if err != nil {
		logging.Error("organize jav tag categories error: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "整理 JAV 标签分类失败", "Failed to organize JAV tag categories")
		return
	}
	c.JSON(http.StatusOK, result)
}

func listJavTagCategories(c *gin.Context) {
	categories, err := dbpkg.ListJavTagCategories(c.Request.Context())
	if err != nil {
		logging.Error("list jav tag categories error: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载 JAV 标签分类失败", "Failed to load JAV tag categories")
		return
	}
	if categories == nil {
		categories = []models.JavTagCategory{}
	}
	c.JSON(http.StatusOK, categories)
}

func createJavTagCategory(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "创建标签分类请求无效", "Invalid tag category creation request")
		return
	}
	category, err := dbpkg.CreateJavTagCategory(c.Request.Context(), req.Name)
	if err != nil {
		logging.Error("create jav tag category error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "创建标签分类失败，名称可能为空或已存在", "Failed to create tag category; the name may be empty or already exist")
		return
	}
	c.JSON(http.StatusCreated, category)
}

func reorderJavTagCategories(c *gin.Context) {
	var req struct {
		CategoryIDs []int64 `json:"category_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "调整标签分类顺序请求无效", "Invalid tag category reorder request")
		return
	}
	if err := dbpkg.ReorderJavTagCategories(c.Request.Context(), req.CategoryIDs); err != nil {
		logging.Error("reorder jav tag categories error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "调整标签分类顺序失败", "Failed to reorder tag categories")
		return
	}
	c.Status(http.StatusNoContent)
}

func renameJavTagCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "标签分类 ID 无效", "Invalid tag category ID")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "修改标签分类请求无效", "Invalid tag category update request")
		return
	}
	if err := dbpkg.RenameJavTagCategory(c.Request.Context(), id, req.Name); err != nil {
		logging.Error("rename jav tag category error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "修改标签分类失败，名称可能为空或已存在", "Failed to rename tag category; the name may be empty or already exist")
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteJavTagCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "标签分类 ID 无效", "Invalid tag category ID")
		return
	}
	if err := dbpkg.DeleteJavTagCategory(c.Request.Context(), id); err != nil {
		logging.Error("delete jav tag category error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "删除标签分类失败", "Failed to delete tag category")
		return
	}
	c.Status(http.StatusNoContent)
}

func assignJavTagsCategory(c *gin.Context) {
	var req struct {
		TagIDs     []int64 `json:"tag_ids"`
		CategoryID *int64  `json:"category_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "批量调整标签分类请求无效", "Invalid batch tag category request")
		return
	}
	if err := dbpkg.AssignJavTagsCategory(c.Request.Context(), req.TagIDs, req.CategoryID); err != nil {
		logging.Error("assign jav tag category error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "批量调整标签分类失败", "Failed to assign tag categories")
		return
	}
	c.Status(http.StatusNoContent)
}

type javItemUpdateRequest struct {
	Title          *string  `json:"title"`
	CoverURL       *string  `json:"cover_url"`
	TagIDs         *[]int64 `json:"tag_ids"`
	IdolIDs        *[]int64 `json:"idol_ids"`
	ScrapedTagIDs  *[]int64 `json:"scraped_tag_ids"`
	StudioID       *int64   `json:"studio_id"`
	SeriesID       *int64   `json:"series_id"`
	ReleaseDate    *string  `json:"release_date"`
	DurationMin    *int     `json:"duration_min"`
	FavoriteRating *float64 `json:"favorite_rating"`
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
		Title:          req.Title,
		StudioID:       req.StudioID,
		SeriesID:       req.SeriesID,
		IdolIDs:        req.IdolIDs,
		UserTagIDs:     req.TagIDs,
		ScrapedTagIDs:  req.ScrapedTagIDs,
		ReleaseUnix:    releaseUnix,
		DurationMin:    req.DurationMin,
		FavoriteRating: req.FavoriteRating,
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
		ID:             tag.ID,
		Name:           tag.Name,
		SimplifiedName: util.SimplifyChineseName(tag.Name),
		Provider:       tag.Provider,
		Count:          0,
	})
}

func createJavScrapedTag(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "创建刮削标签请求无效", "Invalid scraped tag creation request")
		return
	}

	tag, err := dbpkg.CreateJavScrapedTag(c.Request.Context(), req.Name)
	if err != nil {
		logging.Error("create scraped jav tag error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "创建刮削标签失败，标签名称可能为空", "Failed to create scraped tag; the name may be empty")
		return
	}
	c.JSON(http.StatusCreated, dbpkg.JavTagCount{
		ID:             tag.ID,
		Name:           tag.Name,
		SimplifiedName: util.SimplifyChineseName(tag.Name),
		Provider:       int(jav.ProviderManualScrape),
		Count:          0,
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
