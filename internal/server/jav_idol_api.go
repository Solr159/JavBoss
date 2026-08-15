package server

import (
	"context"
	"errors"
	"math"
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
)

func listJavIdols(c *gin.Context) {
	limit := queryInt(c, "limit", 100)
	offset := queryInt(c, "offset", 0)
	search := strings.TrimSpace(c.Query("search"))
	sort := strings.TrimSpace(c.Query("sort"))
	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))
	favoriteGroupID := int64(0)
	if favoriteGroupParam := strings.TrimSpace(c.Query("favorite_group_id")); favoriteGroupParam != "" {
		parsed, err := strconv.ParseInt(favoriteGroupParam, 10, 64)
		if err != nil || parsed <= 0 {
			respondLocalizedError(c, http.StatusBadRequest, "收藏夹 ID 无效", "Invalid favorite group ID")
			return
		}
		favoriteGroupID = parsed
	}
	filters, ok := parseJavIdolFilters(c)
	if !ok {
		return
	}

	items, total, err := dbpkg.ListJavIdols(c.Request.Context(), search, sort, limit, offset, directoryIDs, favoriteGroupID, filters)
	if err != nil {
		logging.Error("list jav idols: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载女优列表失败", "Failed to load idols")
		return
	}

	enrichJavIdolSummaries(c.Request.Context(), items, directoryIDs)

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

func parseJavIdolFilters(c *gin.Context) (dbpkg.JavIdolFilters, bool) {
	filters := dbpkg.JavIdolFilters{}
	ranges := []struct {
		key     string
		labelCN string
		labelEN string
		minimum int
		maximum int
		target  *dbpkg.JavIdolIntRange
	}{
		{key: "height", labelCN: "身高", labelEN: "height", minimum: 130, maximum: 190, target: &filters.Height},
		{key: "age", labelCN: "年龄", labelEN: "age", minimum: 18, maximum: 60, target: &filters.Age},
		{key: "cup", labelCN: "罩杯", labelEN: "cup", minimum: 1, maximum: 11, target: &filters.Cup},
		{key: "bust", labelCN: "胸围", labelEN: "bust", minimum: 60, maximum: 130, target: &filters.Bust},
		{key: "waist", labelCN: "腰围", labelEN: "waist", minimum: 45, maximum: 100, target: &filters.Waist},
		{key: "hips", labelCN: "臀围", labelEN: "hips", minimum: 65, maximum: 130, target: &filters.Hips},
	}
	for _, item := range ranges {
		parsed, ok := parseJavIdolIntRange(c, item.key, item.labelCN, item.labelEN, item.minimum, item.maximum)
		if !ok {
			return filters, false
		}
		*item.target = parsed
	}
	return filters, true
}

func parseJavIdolIntRange(c *gin.Context, key, labelCN, labelEN string, minimum, maximum int) (dbpkg.JavIdolIntRange, bool) {
	result := dbpkg.JavIdolIntRange{}
	minParam := strings.TrimSpace(c.Query("idol_" + key + "_min"))
	maxParam := strings.TrimSpace(c.Query("idol_" + key + "_max"))
	if minParam == "" && maxParam == "" {
		return result, true
	}
	if minParam == "" || maxParam == "" {
		respondLocalizedError(c, http.StatusBadRequest, labelCN+"范围无效", "Invalid idol "+labelEN+" range")
		return result, false
	}
	parsedMin, minErr := strconv.Atoi(minParam)
	parsedMax, maxErr := strconv.Atoi(maxParam)
	if minErr != nil || maxErr != nil || parsedMin < minimum || parsedMax > maximum || parsedMin > parsedMax {
		respondLocalizedError(c, http.StatusBadRequest, labelCN+"范围无效", "Invalid idol "+labelEN+" range")
		return result, false
	}
	result.Min = &parsedMin
	result.Max = &parsedMax
	return result, true
}

func listJavIdolOptions(c *gin.Context) {
	limit := queryInt(c, "limit", 100)
	offset := queryInt(c, "offset", 0)
	search := strings.TrimSpace(c.Query("search"))

	items, total, err := dbpkg.ListJavIdolOptions(c.Request.Context(), search, limit, offset)
	if err != nil {
		logging.Error("list jav idol options: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载女优选项失败", "Failed to load idol options")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

func resolveJavIdols(c *gin.Context) {
	ids := parseInt64CSV(c.Query("ids"))
	items, err := dbpkg.ResolveJavIdols(c.Request.Context(), ids)
	if err != nil {
		logging.Error("resolve jav idols: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "解析女优信息失败", "Failed to resolve idol information")
		return
	}
	if items == nil {
		items = []dbpkg.JavIdolSummary{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func mergeJavIdols(c *gin.Context) {
	var req struct {
		CanonicalID int64   `json:"canonical_id"`
		MergeIDs    []int64 `json:"merge_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "合并女优请求无效", "Invalid idol merge request")
		return
	}
	if req.CanonicalID <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "主女优 ID 不能为空", "Canonical idol ID is required")
		return
	}
	if len(req.MergeIDs) == 0 {
		respondLocalizedError(c, http.StatusBadRequest, "待合并女优 ID 不能为空", "Idol IDs to merge are required")
		return
	}

	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))
	item, err := dbpkg.MergeJavIdols(c.Request.Context(), req.CanonicalID, req.MergeIDs, directoryIDs)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "女优不存在", "Idol was not found")
			return
		}
		logging.Error("merge jav idols canonical=%d merge=%v: %v", req.CanonicalID, req.MergeIDs, err)
		respondLocalizedError(c, http.StatusBadRequest, "合并女优失败，请检查所选女优是否有效", "Failed to merge idols; check the selected idols")
		return
	}
	items := []dbpkg.JavIdolSummary{*item}
	enrichJavIdolSummaries(c.Request.Context(), items, directoryIDs)
	c.JSON(http.StatusOK, items[0])
}

func updateJavIdol(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "女优 ID 无效", "Invalid idol ID")
		return
	}

	var req struct {
		Name         string   `json:"name"`
		RomanName    string   `json:"roman_name"`
		JapaneseName string   `json:"japanese_name"`
		ChineseName  string   `json:"chinese_name"`
		HeightCM     *int     `json:"height_cm"`
		BirthDate    *string  `json:"birth_date"`
		Bust         *int     `json:"bust"`
		Waist        *int     `json:"waist"`
		Hips         *int     `json:"hips"`
		Cup          *int     `json:"cup"`
		Aliases      []string `json:"aliases"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "修改女优信息请求无效", "Invalid idol update request")
		return
	}

	var birthDate *time.Time
	if req.BirthDate != nil {
		raw := strings.TrimSpace(*req.BirthDate)
		if raw != "" {
			parsed, err := time.Parse("2006-01-02", raw)
			if err != nil {
				respondLocalizedError(c, http.StatusBadRequest, "出生日期格式必须为 YYYY-MM-DD", "Birth date must use the YYYY-MM-DD format")
				return
			}
			birthDate = &parsed
		}
	}

	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))
	item, err := dbpkg.UpdateJavIdol(c.Request.Context(), id, dbpkg.JavIdolUpdateInput{
		Name:         req.Name,
		RomanName:    req.RomanName,
		JapaneseName: req.JapaneseName,
		ChineseName:  req.ChineseName,
		HeightCM:     req.HeightCM,
		BirthDate:    birthDate,
		Bust:         req.Bust,
		Waist:        req.Waist,
		Hips:         req.Hips,
		Cup:          req.Cup,
		Aliases:      req.Aliases,
	}, directoryIDs)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "女优不存在", "Idol was not found")
			return
		}
		logging.Error("update jav idol id=%d: %v", id, err)
		respondLocalizedError(c, http.StatusBadRequest, "保存女优信息失败", "Failed to save idol information")
		return
	}
	items := []dbpkg.JavIdolSummary{*item}
	enrichJavIdolSummaries(c.Request.Context(), items, directoryIDs)
	c.JSON(http.StatusOK, items[0])
}

func listJavIdolCoverOptions(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "女优 ID 无效", "Invalid idol ID")
		return
	}

	options, err := dbpkg.ListIdolCoverOptions(
		c.Request.Context(),
		id,
		parseDirectoryIDs(c.Query("directory_ids")),
	)
	if err != nil {
		logging.Error("list jav idol cover options id=%d: %v", id, err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载女优封面选项失败", "Failed to load idol cover options")
		return
	}
	if options == nil {
		options = []dbpkg.JavIdolCoverOption{}
	}
	c.JSON(http.StatusOK, gin.H{"items": options})
}

func updateJavIdolCover(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "女优 ID 无效", "Invalid idol ID")
		return
	}

	var req struct {
		JavID    int64   `json:"jav_id"`
		CropLeft float64 `json:"crop_left"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "更新女优封面请求无效", "Invalid idol cover update request")
		return
	}
	if math.IsNaN(req.CropLeft) || math.IsInf(req.CropLeft, 0) {
		respondLocalizedError(c, http.StatusBadRequest, "封面裁剪位置无效", "Invalid cover crop position")
		return
	}

	item, err := dbpkg.UpdateJavIdolCoverSelection(
		c.Request.Context(),
		id,
		req.JavID,
		req.CropLeft,
		parseDirectoryIDs(c.Query("directory_ids")),
	)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "女优或封面作品不存在", "Idol or cover item was not found")
			return
		}
		logging.Error("update jav idol cover id=%d: %v", id, err)
		respondLocalizedError(c, http.StatusBadRequest, "保存女优封面失败", "Failed to save idol cover")
		return
	}

	items := []dbpkg.JavIdolSummary{*item}
	enrichJavIdolSummaries(c.Request.Context(), items, parseDirectoryIDs(c.Query("directory_ids")))
	c.JSON(http.StatusOK, items[0])
}

func getJavIdol(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "女优 ID 无效", "Invalid idol ID")
		return
	}

	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))
	item, err := dbpkg.GetJavIdolSummary(c.Request.Context(), id, directoryIDs)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "女优不存在", "Idol was not found")
			return
		}
		logging.Error("get jav idol id=%d: %v", id, err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载女优信息失败", "Failed to load idol information")
		return
	}

	items := []dbpkg.JavIdolSummary{*item}
	enrichJavIdolSummaries(c.Request.Context(), items, directoryIDs)
	c.JSON(http.StatusOK, items[0])
}

func getJavIdolJavDBURL(c *gin.Context) {
	code := strings.TrimSpace(c.Query("code"))
	name := strings.TrimSpace(c.Query("name"))
	if code == "" || name == "" {
		respondLocalizedError(c, http.StatusBadRequest, "番号和女优名称不能为空", "JAV code and idol name are required")
		return
	}

	profileURL, err := jav.LookupActressURLByCodeAndName(code, name, jav.ProviderJavDB)
	if err != nil {
		if errors.Is(err, jav.ResourceNotFonud) {
			respondLocalizedError(c, http.StatusNotFound, "未找到对应的 JavDB 女优页面", "JavDB idol page was not found")
			return
		}
		logging.Error("lookup javdb actress url code=%s name=%s: %v", code, name, err)
		respondLocalizedError(c, http.StatusInternalServerError, "查询 JavDB 女优页面失败", "Failed to look up the JavDB idol page")
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": profileURL})
}

func enrichJavIdolSummaries(ctx context.Context, items []dbpkg.JavIdolSummary, directoryIDs []int64) {
	cfg := common.AppConfig
	coverDir := ""
	if cfg != nil {
		coverDir = cfg.JavCoverDir
	}
	for i := range items {
		enrichJavIdolSummary(ctx, &items[i], coverDir, directoryIDs)
	}
}

func enrichJavIdolSummary(ctx context.Context, item *dbpkg.JavIdolSummary, coverDir string, directoryIDs []int64) {
	item.Name = strings.TrimSpace(item.Name)
	item.RomanName = strings.TrimSpace(item.RomanName)
	item.JapaneseName = strings.TrimSpace(item.JapaneseName)
	item.ChineseName = strings.TrimSpace(item.ChineseName)
	item.CoverCode = strings.TrimSpace(item.CoverCode)

	if coverDir == "" {
		return
	}
	if item.CoverJavID != nil && item.CoverCode != "" {
		if common.CoverManager != nil && !common.CoverManager.Exists(item.CoverCode) {
			common.CoverManager.Enqueue(item.CoverCode)
		}
		return
	}
	if item.CoverCode != "" {
		if _, ok := manager.FindCoverPath(coverDir, item.CoverCode); ok {
			return
		}
	}
	codes, err := dbpkg.ListIdolCoverCodes(ctx, item.ID, directoryIDs)
	if err != nil {
		logging.Error("list idol cover codes id=%d: %v", item.ID, err)
		return
	}
	var chosen string
	for _, code := range codes {
		if _, ok := manager.FindCoverPath(coverDir, code); ok {
			chosen = code
			break
		}
	}
	if chosen == "" && len(codes) > 0 {
		chosen = codes[0]
	}
	if chosen != "" {
		item.CoverCode = chosen
		if common.CoverManager != nil && !common.CoverManager.Exists(chosen) {
			common.CoverManager.Enqueue(chosen)
		}
	}
}
