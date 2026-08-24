package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"javboss/internal/common"
	"javboss/internal/jav"
	"javboss/internal/models"
	"javboss/internal/util"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// JavTagCount represents a JAV tag with associated work count.
type JavTagCount struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	SimplifiedName string `json:"simplified_name,omitempty"`
	CategoryID     *int64 `json:"category_id,omitempty"`
	Category       string `json:"category,omitempty"`
	Provider       int    `json:"provider"`
	Count          int64  `json:"count"`
}

// JavTagOrganizeResult summarizes a JavBus category import.
type JavTagOrganizeResult struct {
	RemoteTagCount    int `json:"remote_tag_count"`
	MatchedTagCount   int `json:"matched_tag_count"`
	UpdatedTagCount   int `json:"updated_tag_count"`
	UnmatchedTagCount int `json:"unmatched_tag_count"`
}

// JavPrefixSummary represents an aggregated JAV code prefix.
type JavPrefixSummary struct {
	Prefix       string `json:"prefix"`
	StudioID     *int64 `json:"studio_id"`
	StudioName   string `json:"studio_name"`
	IsUncensored *bool  `json:"is_uncensored"`
	WorkCount    int64  `json:"work_count"`
	SampleCode   string `json:"sample_code"`
}

// JavStudioCodePrefixSummary represents a code prefix attached to a studio.
type JavStudioCodePrefixSummary struct {
	Prefix    string `json:"prefix"`
	WorkCount int64  `json:"work_count"`
}

// JavFilterOptionSearches contains independent searches for each filter facet.
type JavFilterOptionSearches struct {
	Prefix string
	Idol   string
	Tag    string
	Studio string
	Series string
}

// JavFilterOptions contains filter candidates whose counts are scoped to the
// currently matching JAV works. A candidate count is the result size after
// adding that candidate to the current filters.
type JavFilterOptions struct {
	Total     int64              `json:"total"`
	SoloCount int64              `json:"solo_count"`
	Prefixes  []JavPrefixSummary `json:"prefixes"`
	Idols     []JavIdolSummary   `json:"idols"`
	Tags      []JavTagCount      `json:"tags"`
	Studios   []JavStudioSummary `json:"studios"`
	Series    []JavSeriesSummary `json:"series"`
}

func javCodePrefixSQL(column string) string {
	return fmt.Sprintf("CASE WHEN INSTR(%[1]s, '-') > 1 AND (INSTR(%[1]s, '_') = 0 OR INSTR(%[1]s, '-') < INSTR(%[1]s, '_')) THEN UPPER(SUBSTR(%[1]s, 1, INSTR(%[1]s, '-') - 1)) WHEN INSTR(%[1]s, '_') > 1 THEN UPPER(SUBSTR(%[1]s, 1, INSTR(%[1]s, '_') - 1)) ELSE '' END", column)
}

// JavScanVideo contains the fields the scanner needs to resolve or refresh JAV metadata.
type JavScanVideo struct {
	LocationID        int64     `gorm:"column:location_id"`
	VideoID           int64     `gorm:"column:video_id"`
	Filename          string    `gorm:"column:filename"`
	JavID             *int64    `gorm:"column:jav_id"`
	JavCode           string    `gorm:"column:jav_code"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
	DurationSec       int64     `gorm:"column:duration_sec"`
	JavScrapeOverride string    `gorm:"column:jav_scrape_override"`
}

// JavUpdateInput contains user-editable JAV metadata fields.
type JavUpdateInput struct {
	Title          *string
	StudioID       *int64
	SeriesID       *int64
	IdolIDs        *[]int64
	UserTagIDs     *[]int64
	ScrapedTagIDs  *[]int64
	ReleaseUnix    *int64
	DurationMin    *int
	FavoriteRating *float64
}

// JavIdolUpdateInput contains user-editable JAV idol profile fields.
type JavIdolUpdateInput struct {
	Name         string
	RomanName    string
	JapaneseName string
	ChineseName  string
	HeightCM     *int
	BirthDate    *time.Time
	Bust         *int
	Waist        *int
	Hips         *int
	Cup          *int
	Aliases      []string
}

// JavStudioUpdateInput contains user-editable JAV studio fields.
type JavStudioUpdateInput struct {
	Name    string
	Aliases []string
}

// JavMetadataScanItem contains a JAV row that needs studio or series metadata.
type JavMetadataScanItem struct {
	ID         int64  `gorm:"column:id"`
	Code       string `gorm:"column:code"`
	StudioID   *int64 `gorm:"column:studio_id"`
	SeriesID   *int64 `gorm:"column:series_id"`
	SeriesEnID *int64 `gorm:"column:series_en_id"`
}

// GetJav returns one JAV record with visible files and tags.
func GetJav(ctx context.Context, javID int64, directoryIDs []int64) (*models.Jav, error) {
	if javID <= 0 {
		return nil, errors.New("jav id must be positive")
	}

	var item models.Jav
	query := common.DB.WithContext(ctx).
		Preload("Studio").
		Preload("Idols").
		Preload("Series").
		Where("id = ?", javID)
	if err := query.First(&item).Error; err != nil {
		return nil, fmt.Errorf("get jav: %w", err)
	}
	items := []models.Jav{item}
	if err := attachJavLocationVideos(ctx, items, directoryIDs); err != nil {
		return nil, err
	}
	if err := attachVisibleJavTags(ctx, items); err != nil {
		return nil, err
	}
	if err := attachJavFavoriteCounts(ctx, items); err != nil {
		return nil, err
	}
	return &items[0], nil
}

// SearchJav lists Jav metadata filtered by idol IDs/tag IDs/search with pagination and sorting.
func SearchJav(ctx context.Context, idolIDs []int64, tagIDs []int64, search, sort string, limit, offset int, seed *int64, directoryIDs []int64, filterIDs ...int64) ([]models.Jav, int64, error) {
	return SearchJavWithPrefix(ctx, idolIDs, tagIDs, search, "", sort, limit, offset, seed, directoryIDs, filterIDs...)
}

// JavSearchFilters contains optional filters for a JAV list query.
type JavSearchFilters struct {
	StudioID          int64
	SeriesID          int64
	SoloOnly          bool
	FavoriteGroupID   int64
	FavoriteRatingMin *float64
	FavoriteRatingMax *float64
}

// SearchJavWithPrefix lists Jav metadata filtered by an exact code prefix plus other filters.
func SearchJavWithPrefix(ctx context.Context, idolIDs []int64, tagIDs []int64, search, prefix, sort string, limit, offset int, seed *int64, directoryIDs []int64, filterIDs ...int64) ([]models.Jav, int64, error) {
	studioID, seriesID, soloOnly, favoriteGroupID := javFilterOptions(filterIDs)
	return SearchJavWithPrefixFilters(ctx, idolIDs, tagIDs, search, prefix, sort, limit, offset, seed, directoryIDs, JavSearchFilters{
		StudioID:        studioID,
		SeriesID:        seriesID,
		SoloOnly:        soloOnly,
		FavoriteGroupID: favoriteGroupID,
	})
}

// SearchJavWithPrefixFilters lists JAV metadata using the complete filter set.
func SearchJavWithPrefixFilters(ctx context.Context, idolIDs []int64, tagIDs []int64, search, prefix, sort string, limit, offset int, seed *int64, directoryIDs []int64, filters JavSearchFilters) ([]models.Jav, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	idolIDs = uniqueInt64s(idolIDs)
	tagIDs = uniqueInt64s(tagIDs)
	search = strings.TrimSpace(search)
	prefix = normalizeJavCodePrefix(prefix)
	sort = strings.ToLower(strings.TrimSpace(sort))

	filtered := buildJavFilter(ctx, idolIDs, tagIDs, search, prefix, directoryIDs, filters)

	// Count on a cloned query to avoid mutating the main one.
	countBase := buildJavFilter(ctx, idolIDs, tagIDs, search, prefix, directoryIDs, filters)
	countQuery := countBase.Select("DISTINCT jav.id")
	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count jav: %w", err)
	}

	var items []models.Jav
	order := "jav.created_at DESC, jav.id DESC"
	var orderExpr clause.Expr
	useExpr := false
	switch sort {
	case "code", "code_asc":
		order = "jav.code ASC, jav.id ASC"
	case "code_desc":
		order = "jav.code DESC, jav.id DESC"
	case "duration", "duration_desc":
		order = "jav.duration_min DESC, jav.created_at DESC, jav.id DESC"
	case "duration_asc":
		order = "jav.duration_min ASC, jav.created_at ASC, jav.id ASC"
	case "release", "release_desc":
		order = "jav.release_unix IS NULL, jav.release_unix DESC, jav.code ASC, jav.id ASC"
	case "release_asc":
		order = "jav.release_unix IS NULL, jav.release_unix ASC, jav.code ASC, jav.id ASC"
	case "play_count", "play_count_desc":
		order = "COALESCE((SELECT SUM(COALESCE(v.play_count, 0)) FROM video_location vl JOIN directory d ON d.id = vl.directory_id JOIN video v ON v.id = vl.video_id WHERE vl.jav_id = jav.id AND " + activeLocationWhereSQL("vl", "d") + directoryFilterSQL("vl", directoryIDs) + "), 0) DESC, jav.created_at DESC, jav.id DESC"
	case "play_count_asc":
		order = "COALESCE((SELECT SUM(COALESCE(v.play_count, 0)) FROM video_location vl JOIN directory d ON d.id = vl.directory_id JOIN video v ON v.id = vl.video_id WHERE vl.jav_id = jav.id AND " + activeLocationWhereSQL("vl", "d") + directoryFilterSQL("vl", directoryIDs) + "), 0) ASC, jav.created_at ASC, jav.id ASC"
	case "favorite_rating", "favorite_rating_desc":
		order = "jav.favorite_rating DESC, jav.created_at DESC, jav.id DESC"
	case "favorite_rating_asc":
		order = "jav.favorite_rating = 0 ASC, jav.favorite_rating ASC, jav.created_at DESC, jav.id DESC"
	case "recent_asc":
		order = "jav.created_at ASC, jav.id ASC"
	case "random":
		if seed != nil && *seed > 0 {
			orderExpr = clause.Expr{
				SQL:  "stable_random_rank(jav.id, ?), jav.id",
				Vars: []any{*seed},
			}
			useExpr = true
		} else {
			order = "RANDOM()"
		}
	case "recent", "":
		// default order
	default:
		// ignore unknown values
	}
	query := filtered.
		Preload("Studio").
		Preload("Idols").
		Preload("Series").
		Limit(limit).
		Offset(offset)
	if useExpr {
		query = query.Order(clause.OrderBy{Expression: orderExpr})
	} else {
		query = query.Order(order)
	}
	if err := query.Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list jav: %w", err)
	}
	if err := attachJavLocationVideos(ctx, items, directoryIDs); err != nil {
		return nil, 0, err
	}
	if err := attachVisibleJavTags(ctx, items); err != nil {
		return nil, 0, err
	}
	if err := attachJavFavoriteCounts(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListJavPrefixes returns visible JAV code prefixes with studio, censor status, and work count.
func ListJavPrefixes(ctx context.Context, directoryIDs []int64) ([]JavPrefixSummary, error) {
	prefixExpr := javCodePrefixSQL("j.code")
	query := common.DB.WithContext(ctx).
		Table("jav j").
		Select(prefixExpr + " AS prefix, j.studio_id, COALESCE(js.name, '') AS studio_name, j.is_uncensored, COUNT(DISTINCT j.id) AS work_count, MIN(j.code) AS sample_code").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Joins("LEFT JOIN jav_studio js ON js.id = j.studio_id").
		Where(prefixExpr + " <> ''").
		Where(activeLocationWhereSQL("vl", "d")).
		Group(prefixExpr + ", j.studio_id, js.name, j.is_uncensored").
		Order("work_count DESC, prefix ASC, studio_name ASC")
	query = applyDirectoryFilter(query, "vl", directoryIDs)

	var rows []JavPrefixSummary
	if err := query.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list jav prefixes: %w", err)
	}
	return rows, nil
}

func attachVisibleJavTags(ctx context.Context, items []models.Jav) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(items))
	indexByID := make(map[int64]int, len(items))
	for i, item := range items {
		if item.ID > 0 {
			ids = append(ids, item.ID)
			indexByID[item.ID] = i
		}
	}
	if len(ids) == 0 {
		return nil
	}

	type row struct {
		JavID    int64  `gorm:"column:jav_id"`
		ID       int64  `gorm:"column:id"`
		Name     string `gorm:"column:name"`
		Provider int    `gorm:"column:provider"`
	}
	var rows []row
	if err := common.DB.WithContext(ctx).
		Table("jav_tag_map jtm").
		Select("jtm.jav_id, jt.id, jt.name, jtm.provider").
		Joins("JOIN jav_tag jt ON jt.id = jtm.jav_tag_id").
		Where("jtm.jav_id IN ?", ids).
		Where("jtm.provider IN ?", visibleJavTagProviders()).
		Order("jtm.jav_id, jt.name, jtm.provider").
		Scan(&rows).Error; err != nil {
		return fmt.Errorf("load jav tags: %w", err)
	}
	for _, r := range rows {
		i, ok := indexByID[r.JavID]
		if !ok {
			continue
		}
		items[i].Tags = append(items[i].Tags, models.JavTag{
			ID:             r.ID,
			Name:           r.Name,
			SimplifiedName: util.SimplifyChineseName(r.Name),
			Provider:       r.Provider,
		})
	}
	return nil
}

func attachJavLocationVideos(ctx context.Context, items []models.Jav, directoryIDs []int64) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		if item.ID > 0 {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	var locations []models.VideoLocation
	query := common.DB.WithContext(ctx).
		Model(&models.VideoLocation{}).
		Joins("JOIN directory ON directory.id = video_location.directory_id").
		Where("video_location.jav_id IN ?", ids).
		Where(activeLocationWhereSQL("video_location", "directory")).
		Order("video_location.jav_id, video_location.id").
		Preload("DirectoryRef").
		Preload("Video").
		Preload("Video.Tags")
	query = applyDirectoryFilter(query, "video_location", directoryIDs)
	if err := query.
		Find(&locations).Error; err != nil {
		return fmt.Errorf("load jav video locations: %w", err)
	}

	byJavID := make(map[int64][]models.Video, len(ids))
	for _, loc := range locations {
		if loc.JavID == nil || *loc.JavID == 0 {
			continue
		}
		if loc.Video.ID == 0 {
			continue
		}
		video := videoFromLocation(loc)
		byJavID[*loc.JavID] = append(byJavID[*loc.JavID], video)
	}
	for i := range items {
		items[i].Videos = byJavID[items[i].ID]
	}
	return nil
}

// ListJavsForDirectoryProcessing returns every JAV with an active video location
// in directoryID, including the metadata and locations needed by filesystem jobs.
func ListJavsForDirectoryProcessing(ctx context.Context, directoryID int64) ([]models.Jav, error) {
	if directoryID <= 0 {
		return nil, errors.New("directory id must be positive")
	}

	var items []models.Jav
	if err := common.DB.WithContext(ctx).
		Model(&models.Jav{}).
		Preload("Studio").
		Preload("Idols").
		Preload("Series").
		Where(`EXISTS (
			SELECT 1
			FROM video_location vl
			JOIN directory d ON d.id = vl.directory_id
			WHERE vl.jav_id = jav.id
				AND vl.directory_id = ?
				AND COALESCE(vl.is_delete, 0) = 0
				AND COALESCE(d.is_delete, 0) = 0
				AND COALESCE(d.missing, 0) = 0
		)`, directoryID).
		Order("jav.code, jav.id").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list JAVs for directory processing: %w", err)
	}
	if err := attachJavLocationVideos(ctx, items, []int64{directoryID}); err != nil {
		return nil, err
	}
	if err := attachVisibleJavTags(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

// UpdateJav applies user edits to one JAV record.
func UpdateJav(ctx context.Context, javID int64, input JavUpdateInput, directoryIDs []int64) (*models.Jav, error) {
	if javID <= 0 {
		return nil, errors.New("jav id must be positive")
	}
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var javRec models.Jav
		if err := tx.Select("id", "studio_id").Where("id = ?", javID).First(&javRec).Error; err != nil {
			return fmt.Errorf("find jav: %w", err)
		}

		updates := map[string]any{}
		if input.Title != nil {
			updates["title"] = strings.TrimSpace(*input.Title)
		}
		if input.ReleaseUnix != nil {
			releaseUnix := *input.ReleaseUnix
			if releaseUnix < 0 {
				releaseUnix = 0
			}
			updates["release_unix"] = releaseUnix
		}
		if input.DurationMin != nil {
			durationMin := *input.DurationMin
			if durationMin < 0 {
				durationMin = 0
			}
			updates["duration_min"] = durationMin
		}
		if input.FavoriteRating != nil {
			favoriteRating := *input.FavoriteRating
			if math.IsNaN(favoriteRating) || math.IsInf(favoriteRating, 0) ||
				favoriteRating < 0 || favoriteRating > 5 || favoriteRating*2 != math.Trunc(favoriteRating*2) {
				return errors.New("favorite rating must be 0 or between 0.5 and 5 in 0.5 increments")
			}
			updates["favorite_rating"] = favoriteRating
		}
		if input.StudioID != nil {
			studioID := *input.StudioID
			if studioID <= 0 {
				updates["studio_id"] = nil
				javRec.StudioID = nil
			} else {
				var studio models.JavStudio
				if err := tx.Select("id").Where("id = ?", studioID).First(&studio).Error; err != nil {
					return fmt.Errorf("find jav studio: %w", err)
				}
				updates["studio_id"] = studio.ID
				javRec.StudioID = &studio.ID
			}
		}
		if input.SeriesID != nil {
			seriesID := *input.SeriesID
			if seriesID <= 0 {
				updates["series_id"] = nil
			} else {
				var series models.JavSeries
				if err := tx.Select("id").Where("id = ? AND is_english = ?", seriesID, false).First(&series).Error; err != nil {
					return fmt.Errorf("find jav series: %w", err)
				}
				updates["series_id"] = series.ID
			}
		}
		if len(updates) > 0 {
			if err := tx.Model(&models.Jav{}).Where("id = ?", javID).Updates(updates).Error; err != nil {
				return fmt.Errorf("update jav metadata: %w", err)
			}
		}
		if input.IdolIDs != nil {
			if err := replaceJavIdolsTx(tx, javID, *input.IdolIDs); err != nil {
				return err
			}
		}
		if input.UserTagIDs != nil {
			if err := replaceJavUserTagsTx(tx, []int64{javID}, *input.UserTagIDs); err != nil {
				return err
			}
		}
		if input.ScrapedTagIDs != nil {
			if err := replaceJavScrapedTagsTx(tx, javID, *input.ScrapedTagIDs); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return GetJav(ctx, javID, directoryIDs)
}

// ListJavTags returns JAV tags with the number of works for each tag.
func ListJavTags(ctx context.Context, directoryIDs []int64) ([]JavTagCount, error) {
	scrapedTags, err := listJavTagsForProviders(ctx, directoryIDs, visibleScrapedJavTagProviders(), int(jav.ProviderJavBus))
	if err != nil {
		return nil, err
	}
	userTags, err := listJavTagsForProviders(ctx, directoryIDs, []int{int(jav.ProviderUser)}, int(jav.ProviderUser))
	if err != nil {
		return nil, err
	}
	tags := append(scrapedTags, userTags...)
	for i := range tags {
		tags[i].SimplifiedName = util.SimplifyChineseName(tags[i].Name)
	}
	return tags, nil
}

func listJavTagsForProviders(ctx context.Context, directoryIDs []int64, providers []int, outputProvider int) ([]JavTagCount, error) {
	if len(providers) == 0 {
		return nil, nil
	}
	var tags []JavTagCount
	activeLocationSQL := activeLocationWhereSQL("vl", "d") + directoryFilterSQL("vl", directoryIDs)
	isUser := outputProvider == int(jav.ProviderUser)
	tagMapJoin := "LEFT JOIN jav_tag_map jtm ON jtm.jav_tag_id = jt.id AND jtm.provider IN ?"
	query := common.DB.WithContext(ctx).
		Table("jav_tag jt").
		Select("jt.id, jt.name, jt.category_id, jtc.name AS category, ? AS provider, COUNT(DISTINCT CASE WHEN "+activeLocationSQL+" THEN jtm.jav_id END) AS count", outputProvider).
		Joins(tagMapJoin, providers).
		Joins("LEFT JOIN jav_tag_category jtc ON jtc.id = jt.category_id").
		Joins("LEFT JOIN video_location vl ON vl.jav_id = jtm.jav_id").
		Joins("LEFT JOIN directory d ON d.id = vl.directory_id").
		Where("COALESCE(jt.is_user, 0) = ?", isUser)
	if !isUser {
		query = query.Where("jtm.jav_tag_id IS NOT NULL OR NOT EXISTS (SELECT 1 FROM jav_tag_map jtm_any WHERE jtm_any.jav_tag_id = jt.id)")
	}
	if err := query.
		Group("jt.id, jt.name, jt.category_id, jtc.name").
		Order("jt.name").
		Scan(&tags).Error; err != nil {
		return nil, fmt.Errorf("list jav tags: %w", err)
	}
	return tags, nil
}

// ListJavTagCategories returns every manually or automatically created category.
func ListJavTagCategories(ctx context.Context) ([]models.JavTagCategory, error) {
	var categories []models.JavTagCategory
	if err := common.DB.WithContext(ctx).Order("sort_order, id").Find(&categories).Error; err != nil {
		return nil, fmt.Errorf("list jav tag categories: %w", err)
	}
	return categories, nil
}

// CreateJavTagCategory creates an empty category that tags can be moved into.
func CreateJavTagCategory(ctx context.Context, name string) (*models.JavTagCategory, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("category name cannot be empty")
	}
	category := models.JavTagCategory{Name: name}
	if err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maxSortOrder int
		if err := tx.Model(&models.JavTagCategory{}).
			Select("COALESCE(MAX(sort_order), -1)").
			Scan(&maxSortOrder).Error; err != nil {
			return fmt.Errorf("find last jav tag category position: %w", err)
		}
		category.SortOrder = maxSortOrder + 1
		return tx.Create(&category).Error
	}); err != nil {
		return nil, fmt.Errorf("create jav tag category %q: %w", name, err)
	}
	return &category, nil
}

// ReorderJavTagCategories saves the complete category order. ID 0 reserves a
// sortable position for the virtual default category without storing a row.
func ReorderJavTagCategories(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return errors.New("category ids are required")
	}
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id < 0 {
			return errors.New("category ids cannot be negative")
		}
		if _, exists := seen[id]; exists {
			return errors.New("category ids must be unique")
		}
		seen[id] = struct{}{}
	}
	if _, hasDefaultCategory := seen[0]; !hasDefaultCategory {
		return errors.New("category order must include the default category")
	}
	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var storedIDs []int64
		if err := tx.Model(&models.JavTagCategory{}).Pluck("id", &storedIDs).Error; err != nil {
			return fmt.Errorf("list jav tag categories for reorder: %w", err)
		}
		if len(storedIDs)+1 != len(ids) {
			return errors.New("category order must include every category")
		}
		stored := make(map[int64]struct{}, len(storedIDs))
		for _, id := range storedIDs {
			stored[id] = struct{}{}
		}
		for sortOrder, id := range ids {
			if id == 0 {
				continue
			}
			if _, exists := stored[id]; !exists {
				return fmt.Errorf("jav tag category %d not found", id)
			}
			if err := tx.Model(&models.JavTagCategory{}).
				Where("id = ?", id).
				Update("sort_order", sortOrder).Error; err != nil {
				return fmt.Errorf("update jav tag category %d position: %w", id, err)
			}
		}
		return nil
	})
}

// RenameJavTagCategory changes a category name without changing tag membership.
func RenameJavTagCategory(ctx context.Context, id int64, name string) error {
	name = strings.TrimSpace(name)
	if id <= 0 {
		return errors.New("category id must be positive")
	}
	if name == "" {
		return errors.New("category name cannot be empty")
	}
	result := common.DB.WithContext(ctx).
		Model(&models.JavTagCategory{}).
		Where("id = ?", id).
		Update("name", name)
	if result.Error != nil {
		return fmt.Errorf("rename jav tag category: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteJavTagCategory removes a category and leaves its tags uncategorized.
func DeleteJavTagCategory(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("category id must be positive")
	}
	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var categories []models.JavTagCategory
		if err := tx.Order("sort_order, id").Find(&categories).Error; err != nil {
			return fmt.Errorf("list jav tag categories for delete: %w", err)
		}
		categoryOrder := javTagCategoryOrderWithDefault(categories)
		found := false
		nextOrder := make([]int64, 0, len(categoryOrder)-1)
		for _, categoryID := range categoryOrder {
			if categoryID == id {
				found = true
				continue
			}
			nextOrder = append(nextOrder, categoryID)
		}
		if !found {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Model(&models.JavTag{}).Where("category_id = ?", id).Update("category_id", nil).Error; err != nil {
			return fmt.Errorf("clear jav tag category: %w", err)
		}
		result := tx.Delete(&models.JavTagCategory{}, id)
		if result.Error != nil {
			return fmt.Errorf("delete jav tag category: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		for sortOrder, categoryID := range nextOrder {
			if categoryID == 0 {
				continue
			}
			if err := tx.Model(&models.JavTagCategory{}).
				Where("id = ?", categoryID).
				Update("sort_order", sortOrder).Error; err != nil {
				return fmt.Errorf("normalize jav tag category %d position after delete: %w", categoryID, err)
			}
		}
		return nil
	})
}

func javTagCategoryOrderWithDefault(categories []models.JavTagCategory) []int64 {
	occupiedSortOrders := make(map[int]struct{}, len(categories))
	for _, category := range categories {
		if category.SortOrder >= 0 {
			occupiedSortOrders[category.SortOrder] = struct{}{}
		}
	}
	defaultSortOrder := 0
	for {
		if _, occupied := occupiedSortOrders[defaultSortOrder]; !occupied {
			break
		}
		defaultSortOrder++
	}

	type orderedCategory struct {
		id        int64
		sortOrder int
	}
	ordered := make([]orderedCategory, 0, len(categories)+1)
	for _, category := range categories {
		ordered = append(ordered, orderedCategory{id: category.ID, sortOrder: category.SortOrder})
	}
	ordered = append(ordered, orderedCategory{id: 0, sortOrder: defaultSortOrder})
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].sortOrder != ordered[j].sortOrder {
			return ordered[i].sortOrder < ordered[j].sortOrder
		}
		return ordered[i].id < ordered[j].id
	})

	ids := make([]int64, 0, len(ordered))
	for _, category := range ordered {
		ids = append(ids, category.id)
	}
	return ids
}

// AssignJavTagsCategory moves multiple tags into one category. A nil category
// leaves the selected tags uncategorized.
func AssignJavTagsCategory(ctx context.Context, tagIDs []int64, categoryID *int64) error {
	cleanTagIDs := uniqueInt64s(tagIDs)
	if len(cleanTagIDs) == 0 {
		return errors.New("tag ids are required")
	}
	if categoryID != nil {
		if *categoryID <= 0 {
			return errors.New("category id must be positive")
		}
		var count int64
		if err := common.DB.WithContext(ctx).Model(&models.JavTagCategory{}).Where("id = ?", *categoryID).Count(&count).Error; err != nil {
			return fmt.Errorf("find jav tag category: %w", err)
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
	}
	if err := common.DB.WithContext(ctx).
		Model(&models.JavTag{}).
		Where("id IN ?", cleanTagIDs).
		Update("category_id", categoryID).Error; err != nil {
		return fmt.Errorf("assign jav tag category: %w", err)
	}
	return nil
}

// OrganizeJavTagCategories applies the category map fetched from JavBus to
// matching JAV tags while preserving manual categories on unmatched tags.
func OrganizeJavTagCategories(ctx context.Context, genres []jav.JavBusGenreCategory) (*JavTagOrganizeResult, error) {
	exactCategoryNames := make(map[string]string, len(genres))
	normalizedCategoryNames := make(map[string]string, len(genres))
	remoteNames := make(map[string]struct{}, len(genres))
	categoryNames := make(map[string]struct{})
	categoryNameOrder := make([]string, 0)
	for _, genre := range genres {
		name := strings.TrimSpace(genre.Name)
		category := util.SimplifyChineseName(genre.Category)
		if name == "" || category == "" {
			continue
		}
		if _, exists := exactCategoryNames[name]; !exists {
			exactCategoryNames[name] = category
		}
		normalizedName := normalizeJavTagCategoryName(name)
		if _, exists := normalizedCategoryNames[normalizedName]; normalizedName != "" && !exists {
			normalizedCategoryNames[normalizedName] = category
		}
		remoteNames[name] = struct{}{}
		if _, exists := categoryNames[category]; !exists {
			categoryNames[category] = struct{}{}
			categoryNameOrder = append(categoryNameOrder, category)
		}
	}
	if len(exactCategoryNames) == 0 {
		return nil, errors.New("javbus category map is empty")
	}

	result := &JavTagOrganizeResult{RemoteTagCount: len(remoteNames)}
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var storedCategories []models.JavTagCategory
		if err := tx.Where("name IN ?", categoryNameOrder).Find(&storedCategories).Error; err != nil {
			return fmt.Errorf("load jav tag categories: %w", err)
		}
		categoryIDs := make(map[string]int64, len(storedCategories))
		for _, category := range storedCategories {
			categoryIDs[category.Name] = category.ID
		}
		var maxSortOrder int
		if err := tx.Model(&models.JavTagCategory{}).
			Select("COALESCE(MAX(sort_order), -1)").
			Scan(&maxSortOrder).Error; err != nil {
			return fmt.Errorf("find last jav tag category position: %w", err)
		}
		for _, name := range categoryNameOrder {
			if _, exists := categoryIDs[name]; exists {
				continue
			}
			maxSortOrder++
			category := models.JavTagCategory{Name: name, SortOrder: maxSortOrder}
			if err := tx.Create(&category).Error; err != nil {
				return fmt.Errorf("create jav tag category %q: %w", name, err)
			}
			categoryIDs[name] = category.ID
		}

		var tags []models.JavTag
		if err := tx.Order("id").Find(&tags).Error; err != nil {
			return fmt.Errorf("list jav tags for category organization: %w", err)
		}
		for _, tag := range tags {
			categoryName, matched := exactCategoryNames[strings.TrimSpace(tag.Name)]
			if !matched {
				categoryName, matched = normalizedCategoryNames[normalizeJavTagCategoryName(tag.Name)]
			}
			if !matched {
				result.UnmatchedTagCount++
				continue
			}
			result.MatchedTagCount++
			id := categoryIDs[categoryName]
			categoryID := &id
			if equalOptionalInt64(tag.CategoryID, categoryID) {
				continue
			}
			if err := tx.Model(&models.JavTag{}).Where("id = ?", tag.ID).Update("category_id", categoryID).Error; err != nil {
				return fmt.Errorf("update jav tag %d category: %w", tag.ID, err)
			}
			result.UpdatedTagCount++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func equalOptionalInt64(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func normalizeJavTagCategoryName(name string) string {
	return strings.ToLower(util.SimplifyChineseName(strings.TrimSpace(name)))
}

func visibleScrapedJavTagProviders() []int {
	return []int{
		int(jav.ProviderJavBus),
		int(jav.ProviderJavDB),
		int(jav.ProviderAvmoo),
		int(jav.ProviderAvsox),
		int(jav.ProviderJavMenu),
		int(jav.ProviderManualScrape),
	}
}

func visibleJavTagProviders() []int {
	providers := visibleScrapedJavTagProviders()
	providers = append(providers, int(jav.ProviderUser))
	return providers
}

// CreateJavTag creates a user-defined JAV tag.
func CreateJavTag(ctx context.Context, name string) (*models.JavTag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("tag name cannot be empty")
	}

	var tag models.JavTag
	err := common.DB.WithContext(ctx).Where("name = ? AND is_user = ?", name, true).First(&tag).Error
	if err == nil {
		tag.Provider = int(jav.ProviderUser)
		return &tag, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find jav tag %q: %w", name, err)
	}
	tag = models.JavTag{Name: name, IsUser: true}
	if err := common.DB.WithContext(ctx).Create(&tag).Error; err != nil {
		return nil, fmt.Errorf("create jav tag %q: %w", name, err)
	}
	tag.Provider = int(jav.ProviderUser)
	return &tag, nil
}

// CreateJavScrapedTag creates or returns a manually entered scraped JAV tag.
func CreateJavScrapedTag(ctx context.Context, name string) (*models.JavTag, error) {
	names, err := normalizeScrapedJavTagNames([]string{name})
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, errors.New("tag name cannot be empty")
	}

	tag := models.JavTag{Name: names[0], IsUser: false}
	if err := common.DB.WithContext(ctx).
		Where("name = ? AND is_user = ?", tag.Name, false).
		FirstOrCreate(&tag).Error; err != nil {
		return nil, fmt.Errorf("create scraped jav tag %q: %w", tag.Name, err)
	}
	tag.Provider = int(jav.ProviderManualScrape)
	return &tag, nil
}

// CreateJavIdol creates an idol or returns the existing idol matching the name or an alias.
func CreateJavIdol(ctx context.Context, name string) (*models.JavIdol, error) {
	var idol models.JavIdol
	if err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		resolved, err := findOrCreateJavIdolByNameOrAliasTx(tx, name)
		if err != nil {
			return err
		}
		idol = resolved
		return nil
	}); err != nil {
		return nil, fmt.Errorf("create jav idol %q: %w", strings.TrimSpace(name), err)
	}
	return &idol, nil
}

// RenameJavTag renames a user-created JAV tag.
func RenameJavTag(ctx context.Context, id int64, newName string) error {
	newName = strings.TrimSpace(newName)
	if id == 0 {
		return errors.New("tag id cannot be zero")
	}
	if newName == "" {
		return errors.New("tag name cannot be empty")
	}

	var tag models.JavTag
	if err := common.DB.WithContext(ctx).First(&tag, id).Error; err != nil {
		return fmt.Errorf("find jav tag: %w", err)
	}
	if !tag.IsUser {
		return errors.New("tag is not user-defined")
	}

	if err := common.DB.WithContext(ctx).
		Model(&models.JavTag{}).
		Where("id = ?", id).
		Updates(map[string]any{"name": newName, "category_id": nil}).Error; err != nil {
		return fmt.Errorf("rename jav tag: %w", err)
	}
	return nil
}

// DeleteJavTag removes a user-created JAV tag and detaches it from any associated entries.
func DeleteJavTag(ctx context.Context, id int64) error {
	if id == 0 {
		return errors.New("tag id cannot be zero")
	}

	var tag models.JavTag
	if err := common.DB.WithContext(ctx).First(&tag, id).Error; err != nil {
		return fmt.Errorf("find jav tag: %w", err)
	}
	if !tag.IsUser {
		return errors.New("tag is not user-defined")
	}

	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("jav_tag_id = ? AND provider = ?", id, int(jav.ProviderUser)).Delete(&models.JavTagMap{}).Error; err != nil {
			return fmt.Errorf("delete jav tag relations: %w", err)
		}
		if err := deleteJavTagIfUnusedTx(tx, id); err != nil {
			return err
		}
		return nil
	})
}

// DeleteJavTags removes multiple user-created JAV tags and detaches them.
func DeleteJavTags(ctx context.Context, ids []int64) error {
	cleanIDs := uniqueInt64s(ids)
	if len(cleanIDs) == 0 {
		return nil
	}

	var count int64
	if err := common.DB.WithContext(ctx).
		Table("jav_tag jt").
		Where("jt.id IN ?", cleanIDs).
		Where("COALESCE(jt.is_user, 0) = ?", true).
		Count(&count).Error; err != nil {
		return fmt.Errorf("find jav tags: %w", err)
	}
	if count != int64(len(cleanIDs)) {
		return errors.New("tag is not user-defined")
	}

	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("jav_tag_id IN ? AND provider = ?", cleanIDs, int(jav.ProviderUser)).Delete(&models.JavTagMap{}).Error; err != nil {
			return fmt.Errorf("delete jav tag relations: %w", err)
		}
		for _, id := range cleanIDs {
			if err := deleteJavTagIfUnusedTx(tx, id); err != nil {
				return err
			}
		}
		return nil
	})
}

// AddJavTagToJavs associates a user-created tag with JAV entries.
func AddJavTagToJavs(ctx context.Context, tagID int64, javIDs []int64) error {
	if tagID == 0 || len(javIDs) == 0 {
		return nil
	}
	cleanIDs := uniqueInt64s(javIDs)
	if len(cleanIDs) == 0 {
		return nil
	}

	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tag models.JavTag
		if err := tx.First(&tag, tagID).Error; err != nil {
			return fmt.Errorf("find jav tag: %w", err)
		}
		if !tag.IsUser {
			return errors.New("tag is not user-defined")
		}

		now := time.Now()
		rows := make([]models.JavTagMap, 0, len(cleanIDs))
		for _, javID := range cleanIDs {
			rows = append(rows, models.JavTagMap{JavID: javID, JavTagID: tagID, Provider: int(jav.ProviderUser), CreatedAt: now})
		}
		if len(rows) == 0 {
			return nil
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
			return fmt.Errorf("insert jav tag map: %w", err)
		}
		return nil
	})
}

// RemoveJavTagFromJavs detaches a user-created tag from JAV entries.
func RemoveJavTagFromJavs(ctx context.Context, tagID int64, javIDs []int64) error {
	if tagID == 0 || len(javIDs) == 0 {
		return nil
	}
	cleanIDs := uniqueInt64s(javIDs)
	if len(cleanIDs) == 0 {
		return nil
	}

	var tag models.JavTag
	if err := common.DB.WithContext(ctx).First(&tag, tagID).Error; err != nil {
		return fmt.Errorf("find jav tag: %w", err)
	}
	if !tag.IsUser {
		return errors.New("tag is not user-defined")
	}

	if err := common.DB.WithContext(ctx).
		Where("jav_id IN ? AND jav_tag_id = ? AND provider = ?", cleanIDs, tagID, int(jav.ProviderUser)).
		Delete(&models.JavTagMap{}).Error; err != nil {
		return fmt.Errorf("delete jav tag map: %w", err)
	}
	return nil
}

// ReplaceJavUserTags replaces user-defined tags for the given JAV entries.
func ReplaceJavUserTags(ctx context.Context, javIDs, tagIDs []int64) error {
	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return replaceJavUserTagsTx(tx, javIDs, tagIDs)
	})
}

func replaceJavUserTagsTx(tx *gorm.DB, javIDs, tagIDs []int64) error {
	cleanJavIDs := uniqueInt64s(javIDs)
	if len(cleanJavIDs) == 0 {
		return nil
	}
	cleanTagIDs := uniqueInt64s(tagIDs)

	if len(cleanTagIDs) > 0 {
		var count int64
		if err := tx.
			Model(&models.JavTag{}).
			Where("id IN ?", cleanTagIDs).
			Where("is_user = ?", true).
			Count(&count).Error; err != nil {
			return fmt.Errorf("find jav tags: %w", err)
		}
		if count != int64(len(cleanTagIDs)) {
			return errors.New("invalid tag_id")
		}
	}

	if err := tx.
		Where("jav_id IN ? AND provider = ?", cleanJavIDs, int(jav.ProviderUser)).
		Delete(&models.JavTagMap{}).Error; err != nil {
		return fmt.Errorf("delete jav tag map: %w", err)
	}
	if len(cleanTagIDs) == 0 {
		return nil
	}
	now := time.Now()
	rows := make([]models.JavTagMap, 0, len(cleanJavIDs)*len(cleanTagIDs))
	for _, javID := range cleanJavIDs {
		for _, tagID := range cleanTagIDs {
			rows = append(rows, models.JavTagMap{JavID: javID, JavTagID: tagID, Provider: int(jav.ProviderUser), CreatedAt: now})
		}
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
		return fmt.Errorf("insert jav tag map: %w", err)
	}
	return nil
}

func buildJavFilter(ctx context.Context, idolIDs []int64, tagIDs []int64, search, prefix string, directoryIDs []int64, filters JavSearchFilters) *gorm.DB {
	q := common.DB.WithContext(ctx).Model(&models.Jav{})
	visibleTagProviders := visibleJavTagProviders()
	// Only include JAV entries that have at least one active file location.
	validLocation := common.DB.WithContext(ctx).
		Table("video_location vl").
		Select("1").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where("vl.jav_id = jav.id").
		Where(activeLocationWhereSQL("vl", "d"))
	validLocation = applyDirectoryFilter(validLocation, "vl", directoryIDs)
	q = q.Where("EXISTS (?)", validLocation)
	if search != "" {
		like := fmt.Sprintf("%%%s%%", search)
		q = q.Where("code LIKE ? OR title LIKE ?", like, like)
	}
	if filters.StudioID == 0 {
		q = q.Where("studio_id IS NULL")
	} else if filters.StudioID > 0 {
		q = q.Where("studio_id = ?", filters.StudioID)
	}
	if prefix != "" {
		q = q.Where(javCodePrefixSQL("code")+" = ?", prefix)
	}
	if filters.SeriesID > 0 {
		q = q.Where("series_id = ?", filters.SeriesID)
	}
	if filters.FavoriteGroupID > 0 {
		q = q.Joins("JOIN jav_favorite_map jfm_filter ON jfm_filter.entity_id = jav.id AND jfm_filter.entity_type = ? AND jfm_filter.jav_favorite_group_id = ?", JavFavoriteEntityJav, filters.FavoriteGroupID)
	}
	if filters.FavoriteRatingMin != nil {
		q = q.Where("jav.favorite_rating >= ?", *filters.FavoriteRatingMin)
	}
	if filters.FavoriteRatingMax != nil {
		q = q.Where("jav.favorite_rating <= ?", *filters.FavoriteRatingMax)
	}
	if filters.SoloOnly {
		soloJavs := common.DB.WithContext(ctx).
			Table("jav_idol_map jim_solo_count").
			Select("jim_solo_count.jav_id").
			Group("jim_solo_count.jav_id").
			Having("COUNT(DISTINCT jim_solo_count.jav_idol_id) = 1")
		q = q.Where("jav.id IN (?)", soloJavs)
	}
	if len(tagIDs) > 0 {
		q = q.
			Joins("JOIN jav_tag_map jtm ON jtm.jav_id = jav.id").
			Where("jtm.provider IN ?", visibleTagProviders).
			Where("jtm.jav_tag_id IN ?", tagIDs).
			Group("jav.id").
			Having("COUNT(DISTINCT jtm.jav_tag_id) = ?", len(tagIDs))
	}
	if len(idolIDs) > 0 {
		q = q.
			Joins("JOIN jav_idol_map jim ON jim.jav_id = jav.id").
			Where("jim.jav_idol_id IN ?", idolIDs).
			Group("jav.id").
			Having("COUNT(DISTINCT jim.jav_idol_id) = ?", len(idolIDs))
	}
	return q
}

// ListJavFilterOptions returns faceted filter candidates for the current JAV
// result set. All counts use the same AND semantics as SearchJav.
func ListJavFilterOptions(ctx context.Context, idolIDs []int64, tagIDs []int64, search, prefix string, directoryIDs []int64, filters JavSearchFilters, optionSearches JavFilterOptionSearches, limit int) (JavFilterOptions, error) {
	if limit <= 0 {
		limit = 120
	}
	if limit > 500 {
		limit = 500
	}
	idolIDs = uniqueInt64s(idolIDs)
	tagIDs = uniqueInt64s(tagIDs)
	search = strings.TrimSpace(search)
	prefix = normalizeJavCodePrefix(prefix)
	matched := func() *gorm.DB {
		return buildJavFilter(ctx, idolIDs, tagIDs, search, prefix, directoryIDs, filters).
			Select("jav.id")
	}

	result := JavFilterOptions{
		Prefixes: []JavPrefixSummary{},
		Idols:    []JavIdolSummary{},
		Tags:     []JavTagCount{},
		Studios:  []JavStudioSummary{},
		Series:   []JavSeriesSummary{},
	}
	if err := common.DB.WithContext(ctx).Table("(?) matched", matched()).Count(&result.Total).Error; err != nil {
		return result, fmt.Errorf("count JAV filter matches: %w", err)
	}

	soloJavs := common.DB.WithContext(ctx).
		Table("jav_idol_map jim_solo_option").
		Select("jim_solo_option.jav_id").
		Group("jim_solo_option.jav_id").
		Having("COUNT(DISTINCT jim_solo_option.jav_idol_id) = 1")
	if !filters.SoloOnly {
		if err := common.DB.WithContext(ctx).
			Table("(?) matched", matched()).
			Joins("JOIN (?) solo_jav ON solo_jav.jav_id = matched.id", soloJavs).
			Count(&result.SoloCount).Error; err != nil {
			return result, fmt.Errorf("count solo JAV filter option: %w", err)
		}
	}

	idolQuery := common.DB.WithContext(ctx).
		Table("(?) matched", matched()).
		Joins("JOIN jav_idol_map jim_option ON jim_option.jav_id = matched.id").
		Joins("JOIN jav_idol ji ON ji.id = jim_option.jav_idol_id")
	idolQuery = applyJavIdolSearch(idolQuery, optionSearches.Idol)
	if err := idolQuery.
		Select("ji.id, ji.name, ji.roman_name, ji.japanese_name, ji.chinese_name, COUNT(DISTINCT matched.id) AS work_count").
		Group("ji.id, ji.name, ji.roman_name, ji.japanese_name, ji.chinese_name").
		Order("work_count DESC, ji.name ASC, ji.id ASC").
		Limit(limit).
		Scan(&result.Idols).Error; err != nil {
		return result, fmt.Errorf("list JAV idol filter options: %w", err)
	}
	if err := attachJavIdolAliases(ctx, result.Idols); err != nil {
		return result, err
	}

	visibleProviders := visibleJavTagProviders()
	tagQuery := common.DB.WithContext(ctx).
		Table("(?) matched", matched()).
		Joins("JOIN jav_tag_map jtm_option ON jtm_option.jav_id = matched.id AND jtm_option.provider IN ?", visibleProviders).
		Joins("JOIN jav_tag jt ON jt.id = jtm_option.jav_tag_id")
	if tagSearch := strings.TrimSpace(optionSearches.Tag); tagSearch != "" {
		tagQuery = tagQuery.Where("jt.name LIKE ?", fmt.Sprintf("%%%s%%", tagSearch))
	}
	if err := tagQuery.
		Select("jt.id, jt.name, CASE WHEN COALESCE(jt.is_user, 0) = 1 THEN ? ELSE ? END AS provider, COUNT(DISTINCT matched.id) AS count", int(jav.ProviderUser), int(jav.ProviderJavBus)).
		Group("jt.id, jt.name, jt.is_user").
		Order("count DESC, jt.name ASC, jt.id ASC").
		Limit(limit).
		Scan(&result.Tags).Error; err != nil {
		return result, fmt.Errorf("list JAV tag filter options: %w", err)
	}
	for i := range result.Tags {
		result.Tags[i].SimplifiedName = util.SimplifyChineseName(result.Tags[i].Name)
	}

	studioQuery := common.DB.WithContext(ctx).
		Table("(?) matched", matched()).
		Joins("JOIN jav j_option_studio ON j_option_studio.id = matched.id").
		Joins("LEFT JOIN jav_studio js ON js.id = j_option_studio.studio_id")
	if studioSearch := strings.TrimSpace(optionSearches.Studio); studioSearch != "" {
		studioQuery = applyJavStudioSearch(studioQuery, studioSearch)
	}
	if err := studioQuery.
		Select("COALESCE(js.id, 0) AS id, COALESCE(js.name, '') AS name, COUNT(DISTINCT matched.id) AS work_count").
		Group("js.id, js.name").
		Order("work_count DESC, name ASC, id ASC").
		Limit(limit).
		Scan(&result.Studios).Error; err != nil {
		return result, fmt.Errorf("list JAV studio filter options: %w", err)
	}
	if err := attachJavStudioAliases(ctx, result.Studios); err != nil {
		return result, err
	}

	seriesQuery := common.DB.WithContext(ctx).
		Table("(?) matched", matched()).
		Joins("JOIN jav j_option_series ON j_option_series.id = matched.id").
		Joins("JOIN jav_series js ON js.id = j_option_series.series_id")
	seriesQuery = applyJavSeriesSearch(seriesQuery, optionSearches.Series)
	if err := seriesQuery.
		Select("js.id, js.name, js.studio_id, COUNT(DISTINCT matched.id) AS work_count").
		Group("js.id, js.name, js.studio_id").
		Order("work_count DESC, js.name ASC, js.id ASC").
		Limit(limit).
		Scan(&result.Series).Error; err != nil {
		return result, fmt.Errorf("list JAV series filter options: %w", err)
	}

	prefixExpr := javCodePrefixSQL("j_option_prefix.code")
	prefixQuery := common.DB.WithContext(ctx).
		Table("(?) matched", matched()).
		Joins("JOIN jav j_option_prefix ON j_option_prefix.id = matched.id").
		Joins("LEFT JOIN jav_studio js ON js.id = j_option_prefix.studio_id").
		Where(prefixExpr + " <> ''")
	if prefixSearch := strings.TrimSpace(optionSearches.Prefix); prefixSearch != "" {
		like := fmt.Sprintf("%%%s%%", strings.ToUpper(prefixSearch))
		prefixQuery = prefixQuery.Where(prefixExpr+" LIKE ? OR js.name LIKE ?", like, fmt.Sprintf("%%%s%%", prefixSearch))
	}
	if err := prefixQuery.
		Select(prefixExpr + " AS prefix, GROUP_CONCAT(DISTINCT js.name) AS studio_name, COUNT(DISTINCT matched.id) AS work_count, MIN(j_option_prefix.code) AS sample_code").
		Group(prefixExpr).
		Order("work_count DESC, prefix ASC").
		Limit(limit).
		Scan(&result.Prefixes).Error; err != nil {
		return result, fmt.Errorf("list JAV prefix filter options: %w", err)
	}

	return result, nil
}

func normalizeJavCodePrefix(prefix string) string {
	prefix = strings.ToUpper(strings.TrimSpace(prefix))
	if prefix == "" {
		return ""
	}
	for _, r := range prefix {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		return ""
	}
	return prefix
}

func javFilterOptions(values []int64) (int64, int64, bool, int64) {
	studioID := int64(-1)
	seriesID := int64(0)
	soloOnly := false
	favoriteGroupID := int64(0)
	if len(values) > 0 && values[0] >= 0 {
		studioID = values[0]
	}
	if len(values) > 1 && values[1] > 0 {
		seriesID = values[1]
	}
	if len(values) > 2 && values[2] > 0 {
		soloOnly = true
	}
	if len(values) > 3 && values[3] > 0 {
		favoriteGroupID = values[3]
	}
	return studioID, seriesID, soloOnly, favoriteGroupID
}

// JavStudioSummary represents studio info with aggregated work count and a sample code for cover lookup.
type JavStudioSummary struct {
	ID            int64                        `json:"id"`
	Name          string                       `json:"name"`
	Aliases       []string                     `json:"aliases,omitempty" gorm:"-"`
	WorkCount     int64                        `json:"work_count"`
	SampleCode    string                       `json:"sample_code"`
	FavoriteCount int64                        `json:"favorite_count"`
	CodePrefixes  []JavStudioCodePrefixSummary `json:"code_prefixes" gorm:"-"`
	Series        []JavSeriesSummary           `json:"series" gorm:"-"`
}

// JavSeriesSummary represents series info with aggregated work count and a sample code for cover lookup.
type JavSeriesSummary struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	StudioID      *int64 `json:"studio_id"`
	StudioName    string `json:"studio_name"`
	WorkCount     int64  `json:"work_count"`
	SampleCode    string `json:"sample_code"`
	FavoriteCount int64  `json:"favorite_count"`
}

func applyJavStudioSearch(q *gorm.DB, search string) *gorm.DB {
	search = strings.TrimSpace(search)
	if search == "" {
		return q
	}
	like := fmt.Sprintf("%%%s%%", search)
	return q.Where(
		"js.name LIKE ? OR EXISTS (SELECT 1 FROM jav_studio_alias jsa WHERE jsa.jav_studio_id = js.id AND jsa.alias LIKE ?)",
		like,
		like,
	)
}

func applyJavSeriesSearch(q *gorm.DB, search string) *gorm.DB {
	search = strings.TrimSpace(search)
	if search == "" {
		return q
	}
	like := fmt.Sprintf("%%%s%%", search)
	return q.Where("js.name LIKE ?", like)
}

// ListJavStudios returns studios ordered by visible work count descending.
func ListJavStudios(ctx context.Context, search string, limit, offset int, directoryIDs []int64, favoriteGroupIDs ...int64) ([]JavStudioSummary, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	favoriteGroupID := int64(0)
	if len(favoriteGroupIDs) > 0 && favoriteGroupIDs[0] > 0 {
		favoriteGroupID = favoriteGroupIDs[0]
	}

	countBase := common.DB.WithContext(ctx).
		Table("jav_studio js").
		Joins("JOIN jav j ON j.studio_id = js.id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where(activeLocationWhereSQL("vl", "d"))
	countBase = applyDirectoryFilter(countBase, "vl", directoryIDs)
	countBase = applyJavStudioSearch(countBase, search)
	if favoriteGroupID > 0 {
		countBase = countBase.Joins("JOIN jav_favorite_map jfm_filter ON jfm_filter.entity_id = js.id AND jfm_filter.entity_type = ? AND jfm_filter.jav_favorite_group_id = ?", JavFavoriteEntityStudio, favoriteGroupID)
	}

	var total int64
	if err := countBase.Distinct("js.id").Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count jav studios: %w", err)
	}

	var items []JavStudioSummary
	base := common.DB.WithContext(ctx).
		Table("jav_studio js").
		Joins("JOIN jav j ON j.studio_id = js.id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where(activeLocationWhereSQL("vl", "d"))
	base = applyDirectoryFilter(base, "vl", directoryIDs)
	base = applyJavStudioSearch(base, search)
	if favoriteGroupID > 0 {
		base = base.Joins("JOIN jav_favorite_map jfm_filter ON jfm_filter.entity_id = js.id AND jfm_filter.entity_type = ? AND jfm_filter.jav_favorite_group_id = ?", JavFavoriteEntityStudio, favoriteGroupID)
	}
	order := "work_count DESC, js.name ASC"
	if favoriteGroupID > 0 {
		order = "jfm_filter.sort_order ASC, js.name ASC, js.id ASC"
	}
	if err := base.
		Joins("LEFT JOIN (?) favorite_counts ON favorite_counts.entity_id = js.id", buildFavoriteCountQuery(ctx, JavFavoriteEntityStudio)).
		Select("js.id, js.name, COUNT(DISTINCT j.id) AS work_count, MIN(j.code) AS sample_code, COALESCE(favorite_counts.favorite_count, 0) AS favorite_count").
		Group("js.id, js.name, favorite_counts.favorite_count").
		Order(order).
		Limit(limit).
		Offset(offset).
		Scan(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list jav studios: %w", err)
	}
	if err := attachJavStudioCodePrefixes(ctx, items, directoryIDs); err != nil {
		return nil, 0, err
	}
	if err := attachJavStudioSeries(ctx, items, directoryIDs); err != nil {
		return nil, 0, err
	}
	if err := attachJavStudioAliases(ctx, items); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// GetJavStudioSummary returns one studio summary for hover preview usage.
func GetJavStudioSummary(ctx context.Context, studioID int64, directoryIDs []int64) (*JavStudioSummary, error) {
	if studioID <= 0 {
		return nil, errors.New("studio id must be positive")
	}

	var item JavStudioSummary
	query := common.DB.WithContext(ctx).
		Table("jav_studio js").
		Joins("JOIN jav j ON j.studio_id = js.id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where("js.id = ?", studioID).
		Where(activeLocationWhereSQL("vl", "d"))
	query = applyDirectoryFilter(query, "vl", directoryIDs)
	tx := query.
		Joins("LEFT JOIN (?) favorite_counts ON favorite_counts.entity_id = js.id", buildFavoriteCountQuery(ctx, JavFavoriteEntityStudio)).
		Select("js.id, js.name, COUNT(DISTINCT j.id) AS work_count, MIN(j.code) AS sample_code, COALESCE(favorite_counts.favorite_count, 0) AS favorite_count").
		Group("js.id, js.name, favorite_counts.favorite_count").
		Limit(1).
		Scan(&item)
	if tx.Error != nil {
		return nil, fmt.Errorf("get jav studio summary: %w", tx.Error)
	}
	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	items := []JavStudioSummary{item}
	if err := attachJavStudioCodePrefixes(ctx, items, directoryIDs); err != nil {
		return nil, err
	}
	if err := attachJavStudioSeries(ctx, items, directoryIDs); err != nil {
		return nil, err
	}
	if err := attachJavStudioAliases(ctx, items); err != nil {
		return nil, err
	}
	item = items[0]
	return &item, nil
}

// ListJavStudioOptions returns all studios for edit and merge selectors.
func ListJavStudioOptions(ctx context.Context, search string, limit, offset int) ([]JavStudioSummary, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	base := common.DB.WithContext(ctx).Table("jav_studio js")
	base = applyJavStudioSearch(base, search)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count jav studio options: %w", err)
	}

	var items []JavStudioSummary
	if err := base.
		Select("js.id, js.name").
		Order("js.name ASC, js.id ASC").
		Limit(limit).
		Offset(offset).
		Scan(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list jav studio options: %w", err)
	}
	if err := attachJavStudioAliases(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func attachJavStudioAliases(ctx context.Context, items []JavStudioSummary) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(items))
	indexByID := make(map[int64]int, len(items))
	for i, item := range items {
		if item.ID > 0 {
			ids = append(ids, item.ID)
			indexByID[item.ID] = i
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var rows []struct {
		JavStudioID int64  `gorm:"column:jav_studio_id"`
		Alias       string `gorm:"column:alias"`
	}
	if err := common.DB.WithContext(ctx).
		Model(&models.JavStudioAlias{}).
		Select("jav_studio_id, alias").
		Where("jav_studio_id IN ?", ids).
		Order("alias ASC").
		Scan(&rows).Error; err != nil {
		return fmt.Errorf("load jav studio aliases: %w", err)
	}
	for _, row := range rows {
		index, ok := indexByID[row.JavStudioID]
		if !ok {
			continue
		}
		alias := strings.TrimSpace(row.Alias)
		if alias != "" {
			items[index].Aliases = append(items[index].Aliases, alias)
		}
	}
	return nil
}

// UpdateJavStudioProfile updates a studio name and replaces its aliases.
func UpdateJavStudioProfile(ctx context.Context, studioID int64, input JavStudioUpdateInput, directoryIDs []int64) (*JavStudioSummary, error) {
	if studioID <= 0 {
		return nil, errors.New("studio id must be positive")
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return nil, errors.New("studio name cannot be empty")
	}

	if err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.JavStudio
		if err := tx.Where("id = ?", studioID).First(&existing).Error; err != nil {
			return fmt.Errorf("find jav studio: %w", err)
		}

		var duplicateNameCount int64
		if err := tx.Model(&models.JavStudio{}).
			Where("id <> ? AND name = ?", studioID, input.Name).
			Count(&duplicateNameCount).Error; err != nil {
			return fmt.Errorf("check jav studio name: %w", err)
		}
		if duplicateNameCount > 0 {
			return errors.New("studio name already exists")
		}
		var nameAliasConflict int64
		if err := tx.Model(&models.JavStudioAlias{}).
			Where("jav_studio_id <> ? AND alias = ?", studioID, input.Name).
			Count(&nameAliasConflict).Error; err != nil {
			return fmt.Errorf("check jav studio name aliases: %w", err)
		}
		if nameAliasConflict > 0 {
			return errors.New("studio name conflicts with another studio alias")
		}

		aliases := normalizeJavStudioAliases(input.Aliases, input.Name)
		if err := validateJavStudioAliasesTx(tx, studioID, aliases); err != nil {
			return err
		}
		if err := tx.Model(&models.JavStudio{}).
			Where("id = ?", studioID).
			Update("name", input.Name).Error; err != nil {
			return fmt.Errorf("update jav studio: %w", err)
		}
		return replaceJavStudioAliasesTx(tx, studioID, aliases)
	}); err != nil {
		return nil, err
	}

	return GetJavStudioSummary(ctx, studioID, directoryIDs)
}

func normalizeJavStudioAliases(values []string, ownName string) []string {
	ownName = strings.TrimSpace(ownName)
	seen := map[string]bool{}
	aliases := make([]string, 0, len(values))
	for _, value := range values {
		alias := strings.TrimSpace(value)
		if alias == "" || alias == ownName || seen[alias] {
			continue
		}
		seen[alias] = true
		aliases = append(aliases, alias)
	}
	return aliases
}

func validateJavStudioAliasesTx(tx *gorm.DB, studioID int64, aliases []string) error {
	if len(aliases) == 0 {
		return nil
	}
	var nameConflict int64
	if err := tx.Model(&models.JavStudio{}).
		Where("id <> ? AND name IN ?", studioID, aliases).
		Count(&nameConflict).Error; err != nil {
		return fmt.Errorf("check jav studio alias names: %w", err)
	}
	if nameConflict > 0 {
		return errors.New("studio alias conflicts with another studio name")
	}
	var aliasConflict int64
	if err := tx.Model(&models.JavStudioAlias{}).
		Where("jav_studio_id <> ? AND alias IN ?", studioID, aliases).
		Count(&aliasConflict).Error; err != nil {
		return fmt.Errorf("check jav studio aliases: %w", err)
	}
	if aliasConflict > 0 {
		return errors.New("studio alias already exists")
	}
	return nil
}

func replaceJavStudioAliasesTx(tx *gorm.DB, studioID int64, aliases []string) error {
	if err := tx.Where("jav_studio_id = ?", studioID).Delete(&models.JavStudioAlias{}).Error; err != nil {
		return fmt.Errorf("delete jav studio aliases: %w", err)
	}
	if len(aliases) == 0 {
		return nil
	}
	now := time.Now()
	rows := make([]models.JavStudioAlias, 0, len(aliases))
	for _, alias := range aliases {
		rows = append(rows, models.JavStudioAlias{
			JavStudioID: studioID,
			Alias:       alias,
			CreatedAt:   now,
		})
	}
	if err := tx.Create(&rows).Error; err != nil {
		return fmt.Errorf("create jav studio aliases: %w", err)
	}
	return nil
}

func attachJavStudioCodePrefixes(ctx context.Context, items []JavStudioSummary, directoryIDs []int64) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(items))
	indexByID := make(map[int64]int, len(items))
	for i, item := range items {
		if item.ID > 0 {
			ids = append(ids, item.ID)
			indexByID[item.ID] = i
		}
	}
	if len(ids) == 0 {
		return nil
	}

	prefixExpr := javCodePrefixSQL("j.code")
	type row struct {
		StudioID  int64  `gorm:"column:studio_id"`
		Prefix    string `gorm:"column:prefix"`
		WorkCount int64  `gorm:"column:work_count"`
	}
	var rows []row
	query := common.DB.WithContext(ctx).
		Table("jav j").
		Select("j.studio_id, "+prefixExpr+" AS prefix, COUNT(DISTINCT j.id) AS work_count").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where("j.studio_id IN ?", ids).
		Where(prefixExpr + " <> ''").
		Where(activeLocationWhereSQL("vl", "d")).
		Group("j.studio_id, " + prefixExpr).
		Order("j.studio_id, prefix")
	query = applyDirectoryFilter(query, "vl", directoryIDs)
	if err := query.Scan(&rows).Error; err != nil {
		return fmt.Errorf("load jav studio code prefixes: %w", err)
	}
	for _, r := range rows {
		i, ok := indexByID[r.StudioID]
		if !ok {
			continue
		}
		prefix := strings.TrimSpace(r.Prefix)
		if prefix != "" {
			items[i].CodePrefixes = append(items[i].CodePrefixes, JavStudioCodePrefixSummary{
				Prefix:    prefix,
				WorkCount: r.WorkCount,
			})
		}
	}
	return nil
}

func attachJavStudioSeries(ctx context.Context, items []JavStudioSummary, directoryIDs []int64) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(items))
	indexByID := make(map[int64]int, len(items))
	for i, item := range items {
		if item.ID > 0 {
			ids = append(ids, item.ID)
			indexByID[item.ID] = i
		}
	}
	if len(ids) == 0 {
		return nil
	}

	type row struct {
		ParentStudioID int64  `gorm:"column:parent_studio_id"`
		ID             int64  `gorm:"column:id"`
		Name           string `gorm:"column:name"`
		StudioID       *int64 `gorm:"column:studio_id"`
		StudioName     string `gorm:"column:studio_name"`
		WorkCount      int64  `gorm:"column:work_count"`
		SampleCode     string `gorm:"column:sample_code"`
		FavoriteCount  int64  `gorm:"column:favorite_count"`
	}
	var rows []row
	query := common.DB.WithContext(ctx).
		Table("jav j").
		Select("j.studio_id AS parent_studio_id, js.id, js.name, js.studio_id, COALESCE(jst.name, '') AS studio_name, COUNT(DISTINCT j.id) AS work_count, MIN(j.code) AS sample_code, COALESCE(favorite_counts.favorite_count, 0) AS favorite_count").
		Joins("JOIN jav_series js ON j.series_id = js.id").
		Joins("LEFT JOIN jav_studio jst ON jst.id = js.studio_id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Joins("LEFT JOIN (?) favorite_counts ON favorite_counts.entity_id = js.id", buildFavoriteCountQuery(ctx, JavFavoriteEntitySeries)).
		Where("j.studio_id IN ?", ids).
		Where(activeLocationWhereSQL("vl", "d")).
		Group("j.studio_id, js.id, js.name, js.studio_id, jst.name, favorite_counts.favorite_count").
		Order("j.studio_id, work_count DESC, js.name ASC")
	query = applyDirectoryFilter(query, "vl", directoryIDs)
	if err := query.Scan(&rows).Error; err != nil {
		return fmt.Errorf("load jav studio series: %w", err)
	}
	for _, r := range rows {
		i, ok := indexByID[r.ParentStudioID]
		if !ok {
			continue
		}
		items[i].Series = append(items[i].Series, JavSeriesSummary{
			ID:            r.ID,
			Name:          strings.TrimSpace(r.Name),
			StudioID:      r.StudioID,
			StudioName:    strings.TrimSpace(r.StudioName),
			WorkCount:     r.WorkCount,
			SampleCode:    strings.TrimSpace(r.SampleCode),
			FavoriteCount: r.FavoriteCount,
		})
	}
	return nil
}

// ListStudioCoverCodes returns a prioritized list of codes for a studio.
func ListStudioCoverCodes(ctx context.Context, studioID int64, directoryIDs []int64) ([]string, error) {
	if studioID <= 0 {
		return nil, errors.New("studio id must be positive")
	}
	var codes []string
	query := common.DB.WithContext(ctx).
		Table("jav j").
		Select("j.code").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where("j.studio_id = ?", studioID).
		Where(activeLocationWhereSQL("vl", "d"))
	query = applyDirectoryFilter(query, "vl", directoryIDs)
	if err := query.
		Group("j.code").
		Order("j.code").
		Pluck("j.code", &codes).Error; err != nil {
		return nil, fmt.Errorf("list studio cover codes: %w", err)
	}
	return codes, nil
}

// ListJavSeries returns series ordered by visible work count descending.
func ListJavSeries(ctx context.Context, search string, limit, offset int, directoryIDs []int64, favoriteGroupIDs ...int64) ([]JavSeriesSummary, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	favoriteGroupID := int64(0)
	if len(favoriteGroupIDs) > 0 && favoriteGroupIDs[0] > 0 {
		favoriteGroupID = favoriteGroupIDs[0]
	}
	countBase := common.DB.WithContext(ctx).
		Table("jav_series js").
		Joins("JOIN jav j ON j.series_id = js.id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where("COALESCE(js.is_english, 0) = 0").
		Where(activeLocationWhereSQL("vl", "d"))
	countBase = applyDirectoryFilter(countBase, "vl", directoryIDs)
	countBase = applyJavSeriesSearch(countBase, search)
	if favoriteGroupID > 0 {
		countBase = countBase.Joins("JOIN jav_favorite_map jfm_filter ON jfm_filter.entity_id = js.id AND jfm_filter.entity_type = ? AND jfm_filter.jav_favorite_group_id = ?", JavFavoriteEntitySeries, favoriteGroupID)
	}

	var total int64
	if err := countBase.Distinct("js.id").Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count jav series: %w", err)
	}

	var items []JavSeriesSummary
	base := common.DB.WithContext(ctx).
		Table("jav_series js").
		Joins("JOIN jav j ON j.series_id = js.id").
		Joins("LEFT JOIN jav_studio jst ON jst.id = js.studio_id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where("COALESCE(js.is_english, 0) = 0").
		Where(activeLocationWhereSQL("vl", "d"))
	base = applyDirectoryFilter(base, "vl", directoryIDs)
	base = applyJavSeriesSearch(base, search)
	if favoriteGroupID > 0 {
		base = base.Joins("JOIN jav_favorite_map jfm_filter ON jfm_filter.entity_id = js.id AND jfm_filter.entity_type = ? AND jfm_filter.jav_favorite_group_id = ?", JavFavoriteEntitySeries, favoriteGroupID)
	}
	order := "work_count DESC, js.name ASC"
	if favoriteGroupID > 0 {
		order = "jfm_filter.sort_order ASC, js.name ASC, js.id ASC"
	}
	if err := base.
		Joins("LEFT JOIN (?) favorite_counts ON favorite_counts.entity_id = js.id", buildFavoriteCountQuery(ctx, JavFavoriteEntitySeries)).
		Select("js.id, js.name, js.studio_id, jst.name AS studio_name, COUNT(DISTINCT j.id) AS work_count, MIN(j.code) AS sample_code, COALESCE(favorite_counts.favorite_count, 0) AS favorite_count").
		Group("js.id, js.name, js.studio_id, jst.name, favorite_counts.favorite_count").
		Order(order).
		Limit(limit).
		Offset(offset).
		Scan(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list jav series: %w", err)
	}

	return items, total, nil
}

// GetJavSeriesSummary returns one series summary for hover preview usage.
func GetJavSeriesSummary(ctx context.Context, seriesID int64, directoryIDs []int64) (*JavSeriesSummary, error) {
	if seriesID <= 0 {
		return nil, errors.New("series id must be positive")
	}

	var item JavSeriesSummary
	query := common.DB.WithContext(ctx).
		Table("jav_series js").
		Joins("JOIN jav j ON j.series_id = js.id").
		Joins("LEFT JOIN jav_studio jst ON jst.id = js.studio_id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where("js.id = ?", seriesID).
		Where("COALESCE(js.is_english, 0) = 0").
		Where(activeLocationWhereSQL("vl", "d"))
	query = applyDirectoryFilter(query, "vl", directoryIDs)
	tx := query.
		Joins("LEFT JOIN (?) favorite_counts ON favorite_counts.entity_id = js.id", buildFavoriteCountQuery(ctx, JavFavoriteEntitySeries)).
		Select("js.id, js.name, js.studio_id, jst.name AS studio_name, COUNT(DISTINCT j.id) AS work_count, MIN(j.code) AS sample_code, COALESCE(favorite_counts.favorite_count, 0) AS favorite_count").
		Group("js.id, js.name, js.studio_id, jst.name, favorite_counts.favorite_count").
		Limit(1).
		Scan(&item)
	if tx.Error != nil {
		return nil, fmt.Errorf("get jav series summary: %w", tx.Error)
	}
	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &item, nil
}

// ListSeriesCoverCodes returns a prioritized list of codes for a series.
func ListSeriesCoverCodes(ctx context.Context, seriesID int64, directoryIDs []int64) ([]string, error) {
	if seriesID <= 0 {
		return nil, errors.New("series id must be positive")
	}
	var codes []string
	query := common.DB.WithContext(ctx).
		Table("jav j").
		Select("j.code").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where("j.series_id = ?", seriesID).
		Where(activeLocationWhereSQL("vl", "d"))
	query = applyDirectoryFilter(query, "vl", directoryIDs)
	if err := query.
		Group("j.code").
		Order("j.code").
		Pluck("j.code", &codes).Error; err != nil {
		return nil, fmt.Errorf("list series cover codes: %w", err)
	}
	return codes, nil
}

// JavIdolSummary represents idol info with aggregated work count and cover selection.
type JavIdolSummary struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Aliases       []string   `json:"aliases,omitempty" gorm:"-"`
	RomanName     string     `json:"roman_name"`
	JapaneseName  string     `json:"japanese_name"`
	ChineseName   string     `json:"chinese_name"`
	HeightCM      *int       `json:"height_cm"`
	BirthDate     *time.Time `json:"birth_date"`
	Bust          *int       `json:"bust"`
	Waist         *int       `json:"waist"`
	Hips          *int       `json:"hips"`
	Cup           *int       `json:"cup"`
	WorkCount     int64      `json:"work_count"`
	CoverJavID    *int64     `json:"cover_jav_id"`
	CoverCode     string     `json:"cover_code"`
	CoverCropLeft float64    `json:"cover_crop_left"`
	FavoriteCount int64      `json:"favorite_count"`
}

// JavIdolIntRange is an inclusive numeric range used by idol profile filters.
type JavIdolIntRange struct {
	Min *int
	Max *int
}

// JavIdolFilters contains optional profile ranges for the idol list.
type JavIdolFilters struct {
	Height JavIdolIntRange
	Age    JavIdolIntRange
	Cup    JavIdolIntRange
	Bust   JavIdolIntRange
	Waist  JavIdolIntRange
	Hips   JavIdolIntRange
}

// JavIdolCoverOption represents one visible JAV work that can be used as an idol card cover.
type JavIdolCoverOption struct {
	ID    int64  `json:"id"`
	Code  string `json:"code"`
	Title string `json:"title"`
	Solo  bool   `json:"solo"`
}

func applyJavIdolSearch(q *gorm.DB, search string) *gorm.DB {
	search = strings.TrimSpace(search)
	if search == "" {
		return q
	}
	like := fmt.Sprintf("%%%s%%", search)
	return q.Where(
		"ji.name LIKE ? OR ji.roman_name LIKE ? OR ji.japanese_name LIKE ? OR ji.chinese_name LIKE ? OR EXISTS (SELECT 1 FROM jav_idol_alias jia WHERE jia.jav_idol_id = ji.id AND jia.alias LIKE ?)",
		like,
		like,
		like,
		like,
		like,
	)
}

func buildVisibleSoloIdolCoverQuery(ctx context.Context, directoryIDs []int64) *gorm.DB {
	soloJavs := common.DB.WithContext(ctx).
		Table("jav_idol_map jim_count").
		Select("jim_count.jav_id").
		Group("jim_count.jav_id").
		Having("COUNT(*) = 1")

	query := common.DB.WithContext(ctx).
		Table("jav_idol_map jim_solo").
		Select("jim_solo.jav_idol_id, MIN(j_solo.code) AS cover_code").
		Joins("JOIN (?) solo_jav ON solo_jav.jav_id = jim_solo.jav_id", soloJavs).
		Joins("JOIN jav j_solo ON j_solo.id = jim_solo.jav_id").
		Joins("JOIN video_location vl_solo ON vl_solo.jav_id = jim_solo.jav_id").
		Joins("JOIN directory d_solo ON d_solo.id = vl_solo.directory_id").
		Where(activeLocationWhereSQL("vl_solo", "d_solo"))
	query = applyDirectoryFilter(query, "vl_solo", directoryIDs)
	return query.
		Group("jim_solo.jav_idol_id")
}

func buildVisibleIdolWorkCountQuery(ctx context.Context, directoryIDs []int64) *gorm.DB {
	query := common.DB.WithContext(ctx).
		Table("jav_idol_map jim").
		Select("jim.jav_idol_id, COUNT(DISTINCT jim.jav_id) AS work_count").
		Joins("JOIN video_location vl ON vl.jav_id = jim.jav_id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where(activeLocationWhereSQL("vl", "d"))
	query = applyDirectoryFilter(query, "vl", directoryIDs)
	return query.
		Group("jim.jav_idol_id")
}

// GetJavIdolSummary returns one idol summary for hover preview usage.
func GetJavIdolSummary(ctx context.Context, idolID int64, directoryIDs []int64) (*JavIdolSummary, error) {
	if idolID <= 0 {
		return nil, errors.New("idol id must be positive")
	}

	var item JavIdolSummary
	tx := common.DB.WithContext(ctx).
		Table("jav_idol ji").
		Select("ji.id, ji.name, ji.roman_name, ji.japanese_name, ji.chinese_name, ji.height_cm, ji.birth_date, ji.bust, ji.waist, ji.hips, ji.cup, COALESCE(idol_work_counts.work_count, 0) AS work_count, ji.cover_jav_id, COALESCE(NULLIF(cover_jav.code, ''), solo_idols.cover_code) AS cover_code, COALESCE(ji.cover_crop_left, 0.53) AS cover_crop_left, COALESCE(favorite_counts.favorite_count, 0) AS favorite_count").
		Joins("LEFT JOIN (?) idol_work_counts ON idol_work_counts.jav_idol_id = ji.id", buildVisibleIdolWorkCountQuery(ctx, directoryIDs)).
		Joins("LEFT JOIN (?) solo_idols ON solo_idols.jav_idol_id = ji.id", buildVisibleSoloIdolCoverQuery(ctx, directoryIDs)).
		Joins("LEFT JOIN jav cover_jav ON cover_jav.id = ji.cover_jav_id").
		Joins("LEFT JOIN (?) favorite_counts ON favorite_counts.jav_idol_id = ji.id", buildIdolFavoriteCountQuery(ctx)).
		Where("ji.id = ?", idolID).
		Where("solo_idols.cover_code IS NOT NULL").
		Limit(1).
		Scan(&item)
	if tx.Error != nil {
		return nil, fmt.Errorf("get jav idol summary: %w", tx.Error)
	}
	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	items := []JavIdolSummary{item}
	if err := attachJavIdolAliases(ctx, items); err != nil {
		return nil, err
	}
	return &items[0], nil
}

// ResolveJavIdols returns lightweight idol labels for URL/filter display.
func ResolveJavIdols(ctx context.Context, ids []int64) ([]JavIdolSummary, error) {
	cleanIDs := uniqueInt64s(ids)
	if len(cleanIDs) == 0 {
		return []JavIdolSummary{}, nil
	}

	var items []JavIdolSummary
	if err := common.DB.WithContext(ctx).
		Table("jav_idol ji").
		Select("ji.id, ji.name").
		Where("ji.id IN ?", cleanIDs).
		Order("ji.name").
		Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("resolve jav idols: %w", err)
	}
	if err := attachJavIdolAliases(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

// ListJavIdolOptions returns all idols for edit selectors.
func ListJavIdolOptions(ctx context.Context, search string, limit, offset int, directoryIDs []int64) ([]JavIdolSummary, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	base := common.DB.WithContext(ctx).Table("jav_idol ji")
	base = applyJavIdolSearch(base, search)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count jav idol options: %w", err)
	}

	var items []JavIdolSummary
	if err := base.
		Select("ji.id, ji.name, ji.roman_name, ji.japanese_name, ji.chinese_name, ji.height_cm, ji.birth_date, ji.bust, ji.waist, ji.hips, ji.cup, COALESCE(idol_work_counts.work_count, 0) AS work_count, COALESCE(ji.cover_crop_left, 0.53) AS cover_crop_left").
		Joins("LEFT JOIN (?) idol_work_counts ON idol_work_counts.jav_idol_id = ji.id", buildVisibleIdolWorkCountQuery(ctx, directoryIDs)).
		Order("ji.name ASC, ji.id ASC").
		Limit(limit).
		Offset(offset).
		Scan(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list jav idol options: %w", err)
	}
	if err := attachJavIdolAliases(ctx, items); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// ListJavIdols returns idols ordered by selected sort with pagination.
func ListJavIdols(ctx context.Context, search, sort string, limit, offset int, directoryIDs []int64, favoriteGroupID int64, filterOptions ...JavIdolFilters) ([]JavIdolSummary, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	sort = strings.ToLower(strings.TrimSpace(sort))
	filters := JavIdolFilters{}
	if len(filterOptions) > 0 {
		filters = filterOptions[0]
	}
	filterDate := time.Now().UTC()
	soloIdols := buildVisibleSoloIdolCoverQuery(ctx, directoryIDs)

	countBase := common.DB.WithContext(ctx).
		Table("jav_idol ji").
		Joins("JOIN (?) solo_idols ON solo_idols.jav_idol_id = ji.id", soloIdols)
	if favoriteGroupID > 0 {
		countBase = countBase.Joins("JOIN jav_favorite_map jifm_filter ON jifm_filter.entity_id = ji.id AND jifm_filter.entity_type = ? AND jifm_filter.jav_favorite_group_id = ?", JavFavoriteEntityIdol, favoriteGroupID)
	}
	countBase = applyJavIdolSearch(countBase, search)
	countBase = applyJavIdolFilters(countBase, filters, filterDate)

	var total int64
	if err := countBase.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count jav idols: %w", err)
	}

	var items []JavIdolSummary
	order := "work_count DESC, ji.name ASC"
	switch sort {
	case "recent", "recent_desc", "added", "created", "created_at":
		order = "ji.created_at DESC, ji.id DESC"
	case "recent_asc", "added_asc", "created_asc", "created_at_asc":
		order = "ji.created_at ASC, ji.id ASC"
	case "birth", "birth_date", "age", "birth_desc", "birth_date_desc", "age_asc":
		order = "ji.birth_date IS NULL, ji.birth_date DESC, ji.name ASC"
	case "birth_asc", "birth_date_asc", "age_desc":
		order = "ji.birth_date IS NULL, ji.birth_date ASC, ji.name ASC"
	case "height", "height_asc":
		order = "ji.height_cm IS NULL, ji.height_cm ASC, ji.name ASC"
	case "height_desc":
		order = "ji.height_cm IS NULL, ji.height_cm DESC, ji.name ASC"
	case "bust", "bust_desc":
		order = "ji.bust IS NULL, ji.bust DESC, ji.name ASC"
	case "bust_asc":
		order = "ji.bust IS NULL, ji.bust ASC, ji.name ASC"
	case "hips", "hip", "hips_desc", "hip_desc":
		order = "ji.hips IS NULL, ji.hips DESC, ji.name ASC"
	case "hips_asc", "hip_asc":
		order = "ji.hips IS NULL, ji.hips ASC, ji.name ASC"
	case "waist", "waist_asc":
		order = "ji.waist IS NULL, ji.waist ASC, ji.name ASC"
	case "waist_desc":
		order = "ji.waist IS NULL, ji.waist DESC, ji.name ASC"
	case "measurements", "measure", "bwh":
		order = "ji.bust IS NULL, ji.bust DESC, ji.hips IS NULL, ji.hips DESC, ji.waist IS NULL, ji.waist ASC, ji.name ASC"
	case "cup", "cup_desc":
		order = "ji.cup IS NULL, ji.cup DESC, ji.name ASC"
	case "cup_asc":
		order = "ji.cup IS NULL, ji.cup ASC, ji.name ASC"
	case "work_asc", "work_count_asc", "count_asc":
		order = "work_count ASC, ji.name ASC"
	case "work", "work_desc", "work_count", "work_count_desc", "count", "count_desc", "":
		// default order
	default:
		// ignore unknown values
	}
	useFavoriteOrder := favoriteGroupID > 0 && (sort == "" || sort == "favorite_order" || sort == "favorite" || sort == "manual_order")
	if useFavoriteOrder {
		order = "jifm_filter.sort_order ASC, ji.name ASC, ji.id ASC"
	}
	base := common.DB.WithContext(ctx).
		Table("jav_idol ji").
		Joins("JOIN (?) solo_idols ON solo_idols.jav_idol_id = ji.id", soloIdols).
		Joins("LEFT JOIN (?) favorite_counts ON favorite_counts.jav_idol_id = ji.id", buildIdolFavoriteCountQuery(ctx)).
		Joins("JOIN jav_idol_map jim ON jim.jav_idol_id = ji.id").
		Joins("JOIN jav j ON j.id = jim.jav_id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where(activeLocationWhereSQL("vl", "d"))
	if favoriteGroupID > 0 {
		base = base.Joins("JOIN jav_favorite_map jifm_filter ON jifm_filter.entity_id = ji.id AND jifm_filter.entity_type = ? AND jifm_filter.jav_favorite_group_id = ?", JavFavoriteEntityIdol, favoriteGroupID)
	}
	base = applyDirectoryFilter(base, "vl", directoryIDs)
	base = applyJavIdolSearch(base, search)
	base = applyJavIdolFilters(base, filters, filterDate)
	if err := base.
		Joins("LEFT JOIN jav cover_jav ON cover_jav.id = ji.cover_jav_id").
		Select("ji.id, ji.name, ji.roman_name, ji.japanese_name, ji.chinese_name, ji.height_cm, ji.birth_date, ji.bust, ji.waist, ji.hips, ji.cup, COUNT(DISTINCT j.id) AS work_count, ji.cover_jav_id, COALESCE(NULLIF(cover_jav.code, ''), solo_idols.cover_code) AS cover_code, COALESCE(ji.cover_crop_left, 0.53) AS cover_crop_left, COALESCE(favorite_counts.favorite_count, 0) AS favorite_count").
		Group("ji.id, ji.name, ji.roman_name, ji.japanese_name, ji.chinese_name, ji.height_cm, ji.birth_date, ji.bust, ji.waist, ji.hips, ji.cup, ji.cover_jav_id, cover_jav.code, ji.cover_crop_left, solo_idols.cover_code, favorite_counts.favorite_count").
		Order(order).
		Limit(limit).
		Offset(offset).
		Scan(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list jav idols: %w", err)
	}
	if err := attachJavIdolAliases(ctx, items); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func applyJavIdolFilters(q *gorm.DB, filters JavIdolFilters, now time.Time) *gorm.DB {
	q = applyJavIdolIntRange(q, "ji.height_cm", filters.Height)
	q = applyJavIdolIntRange(q, "ji.cup", filters.Cup)
	q = applyJavIdolIntRange(q, "ji.bust", filters.Bust)
	q = applyJavIdolIntRange(q, "ji.waist", filters.Waist)
	q = applyJavIdolIntRange(q, "ji.hips", filters.Hips)
	if filters.Age.Min != nil {
		latestBirthDate := now.AddDate(-*filters.Age.Min, 0, 0).Format("2006-01-02")
		q = q.Where("date(ji.birth_date) <= ?", latestBirthDate)
	}
	if filters.Age.Max != nil {
		earliestBirthDate := now.AddDate(-(*filters.Age.Max + 1), 0, 0).Format("2006-01-02")
		q = q.Where("date(ji.birth_date) > ?", earliestBirthDate)
	}
	return q
}

func applyJavIdolIntRange(q *gorm.DB, column string, value JavIdolIntRange) *gorm.DB {
	if value.Min != nil {
		q = q.Where(column+" >= ?", *value.Min)
	}
	if value.Max != nil {
		q = q.Where(column+" <= ?", *value.Max)
	}
	return q
}

func attachJavIdolAliases(ctx context.Context, items []JavIdolSummary) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(items))
	indexByID := make(map[int64]int, len(items))
	for i, item := range items {
		if item.ID > 0 {
			ids = append(ids, item.ID)
			indexByID[item.ID] = i
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var rows []struct {
		JavIdolID int64  `gorm:"column:jav_idol_id"`
		Alias     string `gorm:"column:alias"`
	}
	if err := common.DB.WithContext(ctx).
		Model(&models.JavIdolAlias{}).
		Select("jav_idol_id, alias").
		Where("jav_idol_id IN ?", ids).
		Order("alias ASC").
		Scan(&rows).Error; err != nil {
		return fmt.Errorf("load jav idol aliases: %w", err)
	}
	for _, row := range rows {
		index, ok := indexByID[row.JavIdolID]
		if !ok {
			continue
		}
		alias := strings.TrimSpace(row.Alias)
		if alias != "" {
			items[index].Aliases = append(items[index].Aliases, alias)
		}
	}
	return nil
}

// UpdateJavIdol updates editable idol profile fields and replaces aliases.
func UpdateJavIdol(ctx context.Context, idolID int64, input JavIdolUpdateInput, directoryIDs []int64) (*JavIdolSummary, error) {
	if idolID <= 0 {
		return nil, errors.New("idol id must be positive")
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return nil, errors.New("idol name cannot be empty")
	}

	if err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.JavIdol
		if err := tx.Where("id = ?", idolID).First(&existing).Error; err != nil {
			return fmt.Errorf("find jav idol: %w", err)
		}

		var duplicateNameCount int64
		if err := tx.Model(&models.JavIdol{}).
			Where("id <> ? AND name = ?", idolID, input.Name).
			Count(&duplicateNameCount).Error; err != nil {
			return fmt.Errorf("check jav idol name: %w", err)
		}
		if duplicateNameCount > 0 {
			return errors.New("idol name already exists")
		}

		ownNames := []string{input.Name, input.RomanName, input.JapaneseName, input.ChineseName}
		aliases := normalizeJavIdolAliases(input.Aliases, ownNames...)
		if err := validateJavIdolAliasesTx(tx, idolID, aliases); err != nil {
			return err
		}

		updates := map[string]any{
			"name":          input.Name,
			"roman_name":    strings.TrimSpace(input.RomanName),
			"japanese_name": strings.TrimSpace(input.JapaneseName),
			"chinese_name":  strings.TrimSpace(input.ChineseName),
			"height_cm":     input.HeightCM,
			"birth_date":    input.BirthDate,
			"bust":          input.Bust,
			"waist":         input.Waist,
			"hips":          input.Hips,
			"cup":           input.Cup,
		}
		if err := tx.Model(&models.JavIdol{}).Where("id = ?", idolID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update jav idol: %w", err)
		}
		if err := replaceJavIdolAliasesTx(tx, idolID, aliases); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return GetJavIdolSummary(ctx, idolID, directoryIDs)
}

func normalizeJavIdolAliases(values []string, ownNames ...string) []string {
	excluded := make(map[string]bool, len(ownNames))
	for _, ownName := range ownNames {
		ownName = strings.TrimSpace(ownName)
		if ownName != "" {
			excluded[javIdolAliasKey(ownName)] = true
		}
	}
	seen := map[string]bool{}
	aliases := make([]string, 0, len(values))
	for _, value := range values {
		alias := strings.TrimSpace(value)
		if alias == "" {
			continue
		}
		key := javIdolAliasKey(alias)
		if excluded[key] || seen[key] {
			continue
		}
		seen[key] = true
		aliases = append(aliases, alias)
	}
	return aliases
}

func javIdolAliasKey(value string) string {
	return strings.TrimSpace(value)
}

func validateJavIdolAliasesTx(tx *gorm.DB, idolID int64, aliases []string) error {
	if len(aliases) == 0 {
		return nil
	}
	var nameConflict int64
	if err := tx.Model(&models.JavIdol{}).
		Where("id <> ? AND name IN ?", idolID, aliases).
		Count(&nameConflict).Error; err != nil {
		return fmt.Errorf("check jav idol alias names: %w", err)
	}
	if nameConflict > 0 {
		return errors.New("idol alias conflicts with another idol name")
	}

	var aliasConflict int64
	if err := tx.Model(&models.JavIdolAlias{}).
		Where("jav_idol_id <> ? AND alias IN ?", idolID, aliases).
		Count(&aliasConflict).Error; err != nil {
		return fmt.Errorf("check jav idol aliases: %w", err)
	}
	if aliasConflict > 0 {
		return errors.New("idol alias already exists")
	}
	return nil
}

func replaceJavIdolAliasesTx(tx *gorm.DB, idolID int64, aliases []string) error {
	if err := tx.Where("jav_idol_id = ?", idolID).Delete(&models.JavIdolAlias{}).Error; err != nil {
		return fmt.Errorf("delete jav idol aliases: %w", err)
	}
	if len(aliases) == 0 {
		return nil
	}
	now := time.Now()
	rows := make([]models.JavIdolAlias, 0, len(aliases))
	for _, alias := range aliases {
		rows = append(rows, models.JavIdolAlias{
			JavIdolID: idolID,
			Alias:     alias,
			CreatedAt: now,
		})
	}
	if err := tx.Create(&rows).Error; err != nil {
		return fmt.Errorf("create jav idol aliases: %w", err)
	}
	return nil
}

// ListIdolCoverCodes returns a prioritized list of codes for an idol, preferring solo works first.
func ListIdolCoverCodes(ctx context.Context, idolID int64, directoryIDs []int64) ([]string, error) {
	var codes []string
	sub := common.DB.WithContext(ctx).
		Table("jav_idol_map").
		Select("jav_id, COUNT(*) as c").
		Group("jav_id")

	query := common.DB.WithContext(ctx).
		Table("jav_idol_map jim").
		Select("j.code, CASE WHEN s.c = 1 THEN 1 ELSE 0 END as solo").
		Joins("JOIN jav j ON j.id = jim.jav_id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Joins("LEFT JOIN (?) s ON s.jav_id = jim.jav_id", sub).
		Where("jim.jav_idol_id = ?", idolID).
		Where(activeLocationWhereSQL("vl", "d"))
	query = applyDirectoryFilter(query, "vl", directoryIDs)
	rows, err := query.
		Group("j.code, solo").
		Order("solo DESC, j.code").
		Rows()
	if err != nil {
		return nil, fmt.Errorf("list idol codes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var code string
		var solo int
		if err := rows.Scan(&code, &solo); err != nil {
			return nil, fmt.Errorf("scan idol codes: %w", err)
		}
		code = strings.TrimSpace(code)
		if code != "" {
			codes = append(codes, code)
		}
	}
	return codes, nil
}

// ListIdolCoverOptions returns visible works that can be selected as an idol cover.
func ListIdolCoverOptions(ctx context.Context, idolID int64, directoryIDs []int64) ([]JavIdolCoverOption, error) {
	if idolID <= 0 {
		return nil, errors.New("idol id must be positive")
	}

	sub := common.DB.WithContext(ctx).
		Table("jav_idol_map jim_count").
		Select("jim_count.jav_id, COUNT(*) as c").
		Group("jim_count.jav_id")

	var rows []struct {
		ID    int64
		Code  string
		Title string
		Solo  int
	}
	query := common.DB.WithContext(ctx).
		Table("jav_idol_map jim").
		Select("j.id, j.code, j.title, CASE WHEN s.c = 1 THEN 1 ELSE 0 END AS solo").
		Joins("JOIN jav j ON j.id = jim.jav_id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Joins("LEFT JOIN (?) s ON s.jav_id = jim.jav_id", sub).
		Where("jim.jav_idol_id = ?", idolID).
		Where(activeLocationWhereSQL("vl", "d"))
	query = applyDirectoryFilter(query, "vl", directoryIDs)
	if err := query.
		Group("j.id, j.code, j.title, solo").
		Order("solo DESC, j.code ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list idol cover options: %w", err)
	}

	options := make([]JavIdolCoverOption, 0, len(rows))
	for _, row := range rows {
		code := strings.TrimSpace(row.Code)
		if row.ID <= 0 || code == "" {
			continue
		}
		options = append(options, JavIdolCoverOption{
			ID:    row.ID,
			Code:  code,
			Title: strings.TrimSpace(row.Title),
			Solo:  row.Solo == 1,
		})
	}
	return options, nil
}

// UpdateJavIdolCoverSelection persists a custom idol card cover. javID <= 0 resets to automatic selection.
func UpdateJavIdolCoverSelection(ctx context.Context, idolID, javID int64, cropLeft float64, directoryIDs []int64) (*JavIdolSummary, error) {
	if idolID <= 0 {
		return nil, errors.New("idol id must be positive")
	}
	cropLeft = normalizeCoverCropLeft(cropLeft)

	if err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.JavIdol{}).Where("id = ?", idolID).Count(&count).Error; err != nil {
			return fmt.Errorf("find jav idol: %w", err)
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}

		updates := map[string]any{
			"cover_crop_left": cropLeft,
		}
		if javID <= 0 {
			updates["cover_jav_id"] = nil
			updates["cover_crop_left"] = normalizeCoverCropLeft(0.53)
			if err := tx.Model(&models.JavIdol{}).Where("id = ?", idolID).Updates(updates).Error; err != nil {
				return fmt.Errorf("reset jav idol cover selection: %w", err)
			}
			return nil
		}

		visible := tx.Table("jav_idol_map jim").
			Joins("JOIN jav j ON j.id = jim.jav_id").
			Joins("JOIN video_location vl ON vl.jav_id = j.id").
			Joins("JOIN directory d ON d.id = vl.directory_id").
			Where("jim.jav_idol_id = ? AND jim.jav_id = ?", idolID, javID).
			Where(activeLocationWhereSQL("vl", "d"))
		visible = applyDirectoryFilter(visible, "vl", directoryIDs)
		if err := visible.Count(&count).Error; err != nil {
			return fmt.Errorf("validate idol cover jav: %w", err)
		}
		if count == 0 {
			return errors.New("cover jav is not available for idol")
		}

		updates["cover_jav_id"] = javID
		if err := tx.Model(&models.JavIdol{}).Where("id = ?", idolID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update jav idol cover selection: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return GetJavIdolSummary(ctx, idolID, directoryIDs)
}

func normalizeCoverCropLeft(value float64) float64 {
	if value < 0 || !isFiniteFloat(value) {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func isFiniteFloat(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// FindIdolSoloCode returns one solo work code for the idol, when available.
func FindIdolSoloCode(ctx context.Context, idolID int64) (string, error) {
	if idolID == 0 {
		return "", errors.New("idol id cannot be zero")
	}
	sub := common.DB.WithContext(ctx).
		Table("jav_idol_map jim_count").
		Select("jim_count.jav_id, COUNT(*) as c").
		Group("jav_id")

	var codes []string
	if err := common.DB.WithContext(ctx).
		Table("jav_idol_map jim").
		Select("j.code").
		Joins("JOIN jav j ON j.id = jim.jav_id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Joins("LEFT JOIN (?) s ON s.jav_id = jim.jav_id", sub).
		Where("jim.jav_idol_id = ?", idolID).
		Where("s.c = 1").
		Where(activeLocationWhereSQL("vl", "d")).
		Group("j.code").
		Order("RANDOM()").
		Limit(1).
		Pluck("j.code", &codes).Error; err != nil {
		return "", fmt.Errorf("find idol solo code: %w", err)
	}
	if len(codes) == 0 {
		return "", nil
	}
	return strings.TrimSpace(codes[0]), nil
}

// ListIdolsMissingProfile returns idols that have no profile fields populated.
func ListIdolsMissingProfile(ctx context.Context) ([]models.JavIdol, error) {
	var idols []models.JavIdol
	soloIdols := buildVisibleSoloIdolCoverQuery(ctx, nil)
	if err := common.DB.WithContext(ctx).
		Joins("JOIN (?) solo_idols ON solo_idols.jav_idol_id = jav_idol.id", soloIdols).
		Where(`
(
  japanese_name IS NULL OR japanese_name = '' OR
  roman_name IS NULL OR roman_name = '' OR
  chinese_name IS NULL OR chinese_name = '' OR
  height_cm IS NULL OR
  birth_date IS NULL OR
  bust IS NULL OR
  waist IS NULL OR
  hips IS NULL OR
  cup IS NULL
)`).
		Order("id").
		Find(&idols).Error; err != nil {
		return nil, fmt.Errorf("list idols missing profile: %w", err)
	}
	return idols, nil
}

// UpdateIdolProfile updates missing idol profile fields with fetched info.
func UpdateIdolProfile(ctx context.Context, idolID int64, info *jav.ActressInfo) (bool, error) {
	if idolID == 0 {
		return false, errors.New("idol id cannot be zero")
	}
	if info == nil {
		return false, errors.New("actress info is nil")
	}
	var idol models.JavIdol
	if err := common.DB.WithContext(ctx).Where("id = ?", idolID).First(&idol).Error; err != nil {
		return false, fmt.Errorf("get idol profile: %w", err)
	}

	updates := make(map[string]any)
	addTextUpdate := func(column, current, value string) {
		value = strings.TrimSpace(value)
		if value == "" || strings.TrimSpace(current) != "" {
			return
		}
		updates[column] = value
	}
	addIntUpdate := func(column string, current *int, value int) {
		if value <= 0 || current != nil {
			return
		}
		updates[column] = value
	}

	addTextUpdate("japanese_name", idol.JapaneseName, info.JapaneseName)
	addTextUpdate("roman_name", idol.RomanName, info.RomanName)
	addTextUpdate("chinese_name", idol.ChineseName, info.ChineseName)
	addIntUpdate("height_cm", idol.HeightCM, info.HeightCM)
	if info.BirthDate > 0 && idol.BirthDate == nil {
		updates["birth_date"] = time.Unix(int64(info.BirthDate), 0).UTC()
	}
	addIntUpdate("bust", idol.Bust, info.Bust)
	addIntUpdate("waist", idol.Waist, info.Waist)
	addIntUpdate("hips", idol.Hips, info.Hips)
	addIntUpdate("cup", idol.Cup, info.Cup)

	if len(updates) == 0 {
		return false, nil
	}
	res := common.DB.WithContext(ctx).
		Model(&models.JavIdol{}).
		Where("id = ?", idolID).
		Updates(updates)
	if res.Error != nil {
		return false, fmt.Errorf("update idol profile: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

// ListVideosForJavScan loads fields used by the jav scanner.
func ListVideosForJavScan(ctx context.Context) ([]JavScanVideo, error) {
	var videos []JavScanVideo
	if err := videosForJavScanQuery(ctx).
		Order("vl.updated_at DESC, vl.id DESC").
		Find(&videos).Error; err != nil {
		return nil, fmt.Errorf("list videos for jav scan: %w", err)
	}
	return videos, nil
}

// GetVideoForJavScan loads one active video location snapshot for a jav link task.
func GetVideoForJavScan(ctx context.Context, locationID int64) (*JavScanVideo, error) {
	if locationID <= 0 {
		return nil, errors.New("location id cannot be zero")
	}
	var video JavScanVideo
	err := videosForJavScanQuery(ctx).
		Where("vl.id = ?", locationID).
		Take(&video).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get video for jav scan: %w", err)
	}
	return &video, nil
}

func videosForJavScanQuery(ctx context.Context) *gorm.DB {
	return common.DB.WithContext(ctx).
		Table("video_location vl").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Joins("JOIN video v ON v.id = vl.video_id").
		Joins("LEFT JOIN jav j ON j.id = vl.jav_id").
		Where("COALESCE(vl.is_delete, 0) = 0").
		Where("COALESCE(d.is_delete, 0) = 0").
		Where("COALESCE(d.missing, 0) = 0").
		Select("vl.id AS location_id, vl.video_id, COALESCE(NULLIF(vl.filename, ''), vl.relative_path) AS filename, vl.jav_id, j.code AS jav_code, vl.updated_at, v.duration_sec, v.jav_scrape_override")
}

// GetJavByCode fetches a jav record by code.
func GetJavByCode(ctx context.Context, code string) (*models.Jav, error) {
	var javRec models.Jav
	err := common.DB.WithContext(ctx).Where("code = ?", code).First(&javRec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get jav by code: %w", err)
	}
	return &javRec, nil
}

// SetVideoLocationJavID links a file location to a jav record, guarding against stale updates when expectedUpdatedAt is provided.
func SetVideoLocationJavID(ctx context.Context, locationID, javID int64, expectedUpdatedAt time.Time) error {
	return setVideoLocationJavIDTx(common.DB.WithContext(ctx), locationID, 0, javID, expectedUpdatedAt)
}

// SetVideoLocationJavIDForVideo links a file location to a jav record while ensuring the location still points at the scanned video.
func SetVideoLocationJavIDForVideo(ctx context.Context, locationID, videoID, javID int64, expectedUpdatedAt time.Time) error {
	return setVideoLocationJavIDTx(common.DB.WithContext(ctx), locationID, videoID, javID, expectedUpdatedAt)
}

// SaveJavInfoAndLinkLocation upserts jav metadata and associates the video location in one transaction.
func SaveJavInfoAndLinkLocation(ctx context.Context, info *jav.JavInfo, locationID int64, expectedUpdatedAt time.Time) (*models.Jav, error) {
	return SaveJavInfoAndLinkLocationForVideo(ctx, info, locationID, 0, expectedUpdatedAt)
}

// SaveJavInfoAndLinkLocationForVideo upserts jav metadata and associates the video location when it still belongs to the scanned video.
func SaveJavInfoAndLinkLocationForVideo(ctx context.Context, info *jav.JavInfo, locationID, videoID int64, expectedUpdatedAt time.Time) (*models.Jav, error) {
	if info == nil {
		return nil, errors.New("jav info is nil")
	}
	var javRec *models.Jav
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rec, err := saveJavInfoTx(tx, info)
		if err != nil {
			return err
		}
		if err := setVideoLocationJavIDTx(tx, locationID, videoID, rec.ID, expectedUpdatedAt); err != nil {
			return err
		}
		javRec = rec
		return nil
	})
	if err != nil {
		return nil, err
	}
	return javRec, nil
}

// SaveJavInfoAndLinkVideoLocations upserts jav metadata and associates every location for a video.
func SaveJavInfoAndLinkVideoLocations(ctx context.Context, info *jav.JavInfo, videoID int64) (*models.Jav, error) {
	if info == nil {
		return nil, errors.New("jav info is nil")
	}
	if videoID <= 0 {
		return nil, errors.New("video id cannot be zero")
	}
	var javRec *models.Jav
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rec, err := saveJavInfoTx(tx, info)
		if err != nil {
			return err
		}
		res := tx.Model(&models.VideoLocation{}).
			Where("video_id = ?", videoID).
			UpdateColumn("jav_id", rec.ID)
		if res.Error != nil {
			return fmt.Errorf("link video locations to jav: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		javRec = rec
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return javRec, nil
}

// SaveManualJavInfoAndLinkVideoLocations atomically upserts manually entered JAV
// metadata, records the manual scrape override, and associates every location
// for the video with the resulting JAV record.
func SaveManualJavInfoAndLinkVideoLocations(ctx context.Context, info *jav.JavInfo, videoID int64) (*models.Jav, error) {
	if info == nil {
		return nil, errors.New("jav info is nil")
	}
	if videoID <= 0 {
		return nil, errors.New("video id cannot be zero")
	}

	var javRec *models.Jav
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rec, err := saveJavInfoTx(tx, info)
		if err != nil {
			return err
		}
		res := tx.Model(&models.VideoLocation{}).
			Where("video_id = ?", videoID).
			UpdateColumn("jav_id", rec.ID)
		if res.Error != nil {
			return fmt.Errorf("link video locations to jav: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		override := models.JavScrapeOverrideManualPrefix + strings.ToUpper(strings.TrimSpace(info.Code))
		if err := updateVideoJavScrapeOverrideTx(tx, videoID, override); err != nil {
			return err
		}
		javRec = rec
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return javRec, nil
}

// LinkVideoLocationsToExistingJav atomically records a manual scrape override
// and associates every location for a video with an existing JAV record without
// changing that record's metadata.
func LinkVideoLocationsToExistingJav(ctx context.Context, code string, videoID int64) (*models.Jav, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, errors.New("jav code is required")
	}
	if videoID <= 0 {
		return nil, errors.New("video id cannot be zero")
	}

	var javRec *models.Jav
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rec, err := lockJavByCodeTx(tx, code)
		if err != nil {
			return err
		}
		if rec == nil {
			return nil
		}
		if err := tx.Model(&models.VideoLocation{}).
			Where("video_id = ?", videoID).
			UpdateColumn("jav_id", rec.ID).Error; err != nil {
			return fmt.Errorf("link video locations to existing jav: %w", err)
		}
		override := models.JavScrapeOverrideManualPrefix + code
		if err := updateVideoJavScrapeOverrideTx(tx, videoID, override); err != nil {
			return err
		}
		javRec = rec
		return nil
	})
	if err != nil {
		return nil, err
	}
	return javRec, nil
}

// SaveJavInfo upserts jav metadata without linking it to a video location.
func SaveJavInfo(ctx context.Context, info *jav.JavInfo) (*models.Jav, error) {
	if info == nil {
		return nil, errors.New("jav info is nil")
	}
	var javRec *models.Jav
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rec, err := saveJavInfoTx(tx, info)
		if err != nil {
			return err
		}
		javRec = rec
		return nil
	})
	if err != nil {
		return nil, err
	}
	return javRec, nil
}

// DeleteOrphanJavs removes JAV records that have no video referencing them.
func DeleteOrphanJavs(ctx context.Context) error {
	var orphanIDs []int64
	sub := common.DB.WithContext(ctx).Model(&models.VideoLocation{}).Select("DISTINCT jav_id").Where("jav_id IS NOT NULL")
	if err := common.DB.WithContext(ctx).Model(&models.Jav{}).
		Where("id NOT IN (?)", sub).
		Pluck("id", &orphanIDs).Error; err != nil {
		return fmt.Errorf("find orphan javs: %w", err)
	}
	if len(orphanIDs) == 0 {
		return nil
	}

	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("jav_id IN ?", orphanIDs).Delete(&models.JavTagMap{}).Error; err != nil {
			return fmt.Errorf("delete orphan jav tag maps: %w", err)
		}
		if err := tx.Where("jav_id IN ?", orphanIDs).Delete(&models.JavIdolMap{}).Error; err != nil {
			return fmt.Errorf("delete orphan jav idol maps: %w", err)
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Where("id IN ?", orphanIDs).Delete(&models.Jav{}).Error; err != nil {
			return fmt.Errorf("delete orphan javs: %w", err)
		}
		return nil
	})
}

// ListJavCodesForDirectory 返回指定目录中可见视频关联的去重 JAV 番号。
func ListJavCodesForDirectory(ctx context.Context, directoryID int64) ([]string, error) {
	if directoryID <= 0 {
		return nil, errors.New("directory id must be positive")
	}
	var codes []string
	if err := common.DB.WithContext(ctx).
		Table("jav j").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where("vl.directory_id = ?", directoryID).
		Where(activeLocationWhereSQL("vl", "d")).
		Where("COALESCE(j.code, '') <> ''").
		Distinct("j.code").
		Order("j.code").
		Pluck("j.code", &codes).Error; err != nil {
		return nil, fmt.Errorf("list jav codes for directory: %w", err)
	}
	return codes, nil
}

// ListJavsMissingStudioOrEnglishSeries returns non-uncensored JAV rows whose
// studio or internal English-series relation is empty.
func ListJavsMissingStudioOrEnglishSeries(ctx context.Context) ([]JavMetadataScanItem, error) {
	var items []JavMetadataScanItem
	if err := common.DB.WithContext(ctx).
		Model(&models.Jav{}).
		Select("id, code, studio_id, series_en_id").
		Where("COALESCE(code, '') <> ''").
		Where("COALESCE(is_uncensored, 0) = 0").
		Where("studio_id IS NULL OR series_en_id IS NULL").
		Order("created_at ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list javs missing studio or english series: %w", err)
	}
	return items, nil
}

// ListJavsMissingLocalSeries returns every non-uncensored JAV row whose
// frontend-visible series relation is empty, regardless of English-series hints.
func ListJavsMissingLocalSeries(ctx context.Context) ([]JavMetadataScanItem, error) {
	var items []JavMetadataScanItem
	if err := common.DB.WithContext(ctx).
		Model(&models.Jav{}).
		Select("id, code, series_id, series_en_id").
		Where("COALESCE(code, '') <> ''").
		Where("COALESCE(is_uncensored, 0) = 0").
		Where("series_id IS NULL").
		Order("created_at ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list javs missing local series: %w", err)
	}
	return items, nil
}

// ListJavsMissingLocalSeriesWithEnglishSeries returns non-uncensored JAV rows
// that have an English hint but are still missing the frontend-visible series.
func ListJavsMissingLocalSeriesWithEnglishSeries(ctx context.Context) ([]JavMetadataScanItem, error) {
	var items []JavMetadataScanItem
	if err := common.DB.WithContext(ctx).
		Model(&models.Jav{}).
		Select("id, code, series_id, series_en_id").
		Where("COALESCE(code, '') <> ''").
		Where("COALESCE(is_uncensored, 0) = 0").
		Where("series_id IS NULL").
		Where("series_en_id IS NOT NULL").
		Order("created_at ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list javs missing local series with english series: %w", err)
	}
	return items, nil
}

// ListJavsMissingTitle returns JAV rows whose primary title is empty.
func ListJavsMissingTitle(ctx context.Context) ([]JavMetadataScanItem, error) {
	var items []JavMetadataScanItem
	if err := common.DB.WithContext(ctx).
		Model(&models.Jav{}).
		Select("id, code, studio_id, series_id").
		Where("COALESCE(code, '') <> ''").
		Where("TRIM(COALESCE(title, '')) = ''").
		Order("created_at ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list javs missing title: %w", err)
	}
	return items, nil
}

// ListJavsMissingUncensored returns JAV rows whose censored/uncensored state is unknown.
func ListJavsMissingUncensored(ctx context.Context) ([]JavMetadataScanItem, error) {
	var items []JavMetadataScanItem
	if err := common.DB.WithContext(ctx).
		Model(&models.Jav{}).
		Select("id, code").
		Where("COALESCE(code, '') <> ''").
		Where("is_uncensored IS NULL").
		Order("created_at ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list javs missing uncensored state: %w", err)
	}
	return items, nil
}

// ListUncensoredJavsMissingAvsoxMetadata returns uncensored JAV rows missing fields avsox can fill.
func ListUncensoredJavsMissingAvsoxMetadata(ctx context.Context) ([]JavMetadataScanItem, error) {
	var items []JavMetadataScanItem
	localIdols := common.DB.WithContext(ctx).
		Table("jav_idol_map jim").
		Select("1").
		Where("jim.jav_id = jav.id")
	if err := common.DB.WithContext(ctx).
		Model(&models.Jav{}).
		Select("id, code, studio_id, series_id").
		Where("COALESCE(code, '') <> ''").
		Where("COALESCE(is_uncensored, 0) <> 0").
		Where("studio_id IS NULL OR series_id IS NULL OR NOT EXISTS (?)", localIdols).
		Order("created_at ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list uncensored javs missing avsox metadata: %w", err)
	}
	return items, nil
}

// UpdateMissingJavSeriesStudios assigns a studio to series that can be inferred from a linked JAV row.
func UpdateMissingJavSeriesStudios(ctx context.Context) (int64, error) {
	type candidate struct {
		SeriesID int64 `gorm:"column:series_id"`
		StudioID int64 `gorm:"column:studio_id"`
	}

	var candidates []candidate
	if err := common.DB.WithContext(ctx).
		Table("jav_series js").
		Select("js.id AS series_id, MIN(j.studio_id) AS studio_id").
		Joins(`JOIN jav j ON (
			(COALESCE(js.is_english, 0) = 0 AND j.series_id = js.id)
			OR (COALESCE(js.is_english, 0) <> 0 AND j.series_en_id = js.id)
		)`).
		Where("js.studio_id IS NULL").
		Where("j.studio_id IS NOT NULL").
		Group("js.id").
		Scan(&candidates).Error; err != nil {
		return 0, fmt.Errorf("list jav series studio candidates: %w", err)
	}
	if len(candidates) == 0 {
		return 0, nil
	}

	var updated int64
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range candidates {
			if item.SeriesID <= 0 || item.StudioID <= 0 {
				continue
			}
			res := tx.Model(&models.JavSeries{}).
				Where("id = ? AND studio_id IS NULL", item.SeriesID).
				Update("studio_id", item.StudioID)
			if res.Error != nil {
				return fmt.Errorf("update jav series studio id=%d: %w", item.SeriesID, res.Error)
			}
			updated += res.RowsAffected
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return updated, nil
}

// UpdateJavStudio records the studio lookup result for a JAV row.
func UpdateJavStudio(ctx context.Context, javID int64, studio string) error {
	if javID == 0 {
		return errors.New("jav id cannot be zero")
	}
	studio = strings.TrimSpace(studio)
	if studio == "" {
		return nil
	}
	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rec, err := ensureStudioTx(tx, studio)
		if err != nil {
			return err
		}
		if err := tx.Model(&models.Jav{}).
			Where("id = ?", javID).
			Updates(map[string]any{
				"studio_id": rec.ID,
			}).Error; err != nil {
			return fmt.Errorf("update jav studio: %w", err)
		}
		return nil
	})
}

// UpdateJavStudioIfMissing records the studio lookup result without overwriting an existing studio.
func UpdateJavStudioIfMissing(ctx context.Context, javID int64, studio string) (bool, error) {
	if javID == 0 {
		return false, errors.New("jav id cannot be zero")
	}
	studio = strings.TrimSpace(studio)
	if studio == "" {
		return false, nil
	}
	var updated bool
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var javRec models.Jav
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "studio_id").
			Where("id = ?", javID).
			First(&javRec).Error; err != nil {
			return fmt.Errorf("get jav studio: %w", err)
		}
		if javRec.StudioID != nil {
			return nil
		}
		rec, err := ensureStudioTx(tx, studio)
		if err != nil {
			return err
		}
		res := tx.Model(&models.Jav{}).
			Where("id = ? AND studio_id IS NULL", javID).
			Update("studio_id", rec.ID)
		if res.Error != nil {
			return fmt.Errorf("update missing jav studio: %w", res.Error)
		}
		updated = res.RowsAffected > 0
		return nil
	})
	if err != nil {
		return false, err
	}
	return updated, nil
}

// UpdateJavSeries records the series lookup result for a JAV row.
func UpdateJavSeries(ctx context.Context, javID int64, series string) error {
	if javID == 0 {
		return errors.New("jav id cannot be zero")
	}
	series = strings.TrimSpace(series)
	if series == "" {
		return nil
	}
	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var javRec models.Jav
		if err := tx.Select("id", "studio_id").Where("id = ?", javID).First(&javRec).Error; err != nil {
			return fmt.Errorf("get jav studio for series: %w", err)
		}
		rec, err := ensureSeriesWithStudioTx(tx, series, false, javRec.StudioID)
		if err != nil {
			return err
		}
		if err := tx.Model(&models.Jav{}).
			Where("id = ?", javID).
			Update("series_id", rec.ID).Error; err != nil {
			return fmt.Errorf("update jav series: %w", err)
		}
		return nil
	})
}

// UpdateJavSeriesIfMissing records the series lookup result without overwriting an existing series.
func UpdateJavSeriesIfMissing(ctx context.Context, javID int64, series string) (bool, error) {
	return updateJavSeriesIfMissing(ctx, javID, series, false)
}

// UpdateJavEnglishSeriesIfMissing records the internal JavDatabase series hint
// used to decide which rows need the slow Avmoo localized-series lookup.
func UpdateJavEnglishSeriesIfMissing(ctx context.Context, javID int64, series string) (bool, error) {
	return updateJavSeriesIfMissing(ctx, javID, series, true)
}

func updateJavSeriesIfMissing(ctx context.Context, javID int64, series string, isEnglish bool) (bool, error) {
	if javID == 0 {
		return false, errors.New("jav id cannot be zero")
	}
	series = strings.TrimSpace(series)
	if series == "" {
		return false, nil
	}
	var updated bool
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var javRec models.Jav
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "studio_id", "series_id", "series_en_id").
			Where("id = ?", javID).
			First(&javRec).Error; err != nil {
			return fmt.Errorf("get jav studio for series: %w", err)
		}
		if isEnglish && javRec.SeriesEnID != nil {
			return nil
		}
		if !isEnglish && javRec.SeriesID != nil {
			return nil
		}
		rec, err := ensureSeriesWithStudioTx(tx, series, isEnglish, javRec.StudioID)
		if err != nil {
			return err
		}
		column := "series_id"
		if isEnglish {
			column = "series_en_id"
		}
		res := tx.Model(&models.Jav{}).
			Where("id = ? AND "+column+" IS NULL", javID).
			Update(column, rec.ID)
		if res.Error != nil {
			return fmt.Errorf("update missing jav series: %w", res.Error)
		}
		updated = res.RowsAffected > 0
		return nil
	})
	if err != nil {
		return false, err
	}
	return updated, nil
}

// AppendJavIdolsIfMissingForProvider appends idol mappings when none exist yet.
func AppendJavIdolsIfMissingForProvider(ctx context.Context, javID int64, names []string, provider jav.Provider) (bool, error) {
	if javID == 0 {
		return false, errors.New("jav id cannot be zero")
	}
	return appendJavIdolsIfMissingForProvider(ctx, javID, names, provider)
}

func saveJavInfoTx(tx *gorm.DB, info *jav.JavInfo, now ...time.Time) (*models.Jav, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	ts := time.Now()
	if len(now) > 0 {
		ts = now[0]
	}

	javRec, err := lockJavByCodeTx(tx, info.Code)
	if err != nil {
		return nil, err
	}
	if javRec == nil {
		javRec = &models.Jav{Code: info.Code}
	}
	provider := jav.ParseProvider(int(info.Provider))
	if provider == jav.ProviderJavDatabase || provider == jav.ProviderThePornDB {
		return nil, errors.New("english JAV metadata cannot be persisted")
	}
	javRec.Code = info.Code
	javRec.Title = info.Title
	javRec.ReleaseUnix = info.ReleaseUnix
	javRec.DurationMin = info.DurationMin
	javRec.FetchedAt = ts
	if info.IsUncensored != nil {
		isUncensored := *info.IsUncensored
		javRec.IsUncensored = &isUncensored
	}
	if studio := strings.TrimSpace(info.Studio); studio != "" {
		studioRec, err := ensureStudioTx(tx, studio)
		if err != nil {
			return nil, err
		}
		javRec.StudioID = &studioRec.ID
	}
	if series := strings.TrimSpace(info.Series); series != "" {
		seriesRec, err := ensureSeriesTx(tx, series)
		if err != nil {
			return nil, err
		}
		javRec.SeriesID = &seriesRec.ID
	}
	// Sample images are resolved lazily by the detail API. Metadata scans must
	// neither import provider sample images nor overwrite a previously resolved
	// list.
	if err := tx.Omit("sample_images").Save(javRec).Error; err != nil {
		return nil, fmt.Errorf("save jav: %w", err)
	}

	tags, err := ensureJavTagsTx(tx, info.Tags, info.Provider)
	if err != nil {
		return nil, err
	}
	if err := replaceJavTagsForProviderTx(tx, javRec.ID, tags, info.Provider); err != nil {
		return nil, err
	}
	if err := appendJavIdolsTx(tx, javRec, info.Actors); err != nil {
		return nil, err
	}
	return javRec, nil
}

// SetJavSampleImagesIfEmpty stores sample images without replacing an existing list.
func SetJavSampleImagesIfEmpty(ctx context.Context, javID int64, images models.JavSampleImages) (models.JavSampleImages, error) {
	if javID <= 0 {
		return nil, errors.New("jav id must be positive")
	}
	images = normalizeJavSampleImages(images)
	if len(images) == 0 {
		return models.JavSampleImages{}, nil
	}

	result := common.DB.WithContext(ctx).
		Model(&models.Jav{}).
		Where("id = ?", javID).
		Where(`TRIM(COALESCE(sample_images, '')) IN ('', '[]', 'null')`).
		UpdateColumn("sample_images", images)
	if result.Error != nil {
		return nil, fmt.Errorf("update JAV sample images: %w", result.Error)
	}

	var stored models.Jav
	if err := common.DB.WithContext(ctx).
		Select("id", "sample_images").
		Where("id = ?", javID).
		First(&stored).Error; err != nil {
		return nil, fmt.Errorf("load JAV sample images: %w", err)
	}
	if stored.SampleImages == nil {
		stored.SampleImages = models.JavSampleImages{}
	}
	return stored.SampleImages, nil
}

// MarkJavSampleImagesNotFound stores a sentinel without replacing an existing result.
func MarkJavSampleImagesNotFound(ctx context.Context, javID int64) error {
	if javID <= 0 {
		return errors.New("jav id must be positive")
	}
	if err := common.DB.WithContext(ctx).
		Model(&models.Jav{}).
		Where("id = ?", javID).
		Where(`TRIM(COALESCE(sample_images, '')) IN ('', '[]', 'null')`).
		UpdateColumn("sample_images", models.NewJavSampleImagesNotFound()).Error; err != nil {
		return fmt.Errorf("mark JAV sample images not found: %w", err)
	}
	return nil
}

func normalizeJavSampleImages(images models.JavSampleImages) models.JavSampleImages {
	normalized := make(models.JavSampleImages, 0, len(images))
	seen := make(map[string]struct{}, len(images))
	for _, image := range images {
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
		normalized = append(normalized, models.JavSampleImage{
			ThumbnailURL: thumbnailURL,
			DetailURL:    detailURL,
		})
	}
	return normalized
}

// UpdateJavIsUncensoredIfUnknown records an uncensored/censored classification
// without overwriting an existing explicit value.
func UpdateJavIsUncensoredIfUnknown(ctx context.Context, javID int64, isUncensored bool) error {
	if javID == 0 {
		return errors.New("jav id cannot be zero")
	}
	if err := common.DB.WithContext(ctx).
		Model(&models.Jav{}).
		Where("id = ? AND is_uncensored IS NULL", javID).
		Update("is_uncensored", isUncensored).Error; err != nil {
		return fmt.Errorf("update jav is_uncensored: %w", err)
	}
	return nil
}

func normalizeJavTagProvider(provider jav.Provider) jav.Provider {
	provider = jav.ParseProvider(int(provider))
	if provider == jav.ProviderUnknown {
		return jav.ProviderJavBus
	}
	return provider
}

func lockJavByCodeTx(tx *gorm.DB, code string) (*models.Jav, error) {
	var javRec models.Jav
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", code).First(&javRec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get jav by code: %w", err)
	}
	return &javRec, nil
}

func ensureStudioTx(tx *gorm.DB, name string) (*models.JavStudio, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("studio name cannot be empty")
	}
	var studio models.JavStudio
	err := tx.Where("name = ?", name).First(&studio).Error
	if err == nil {
		return &studio, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("load studio %q: %w", name, err)
	}
	err = tx.
		Table("jav_studio_alias jsa").
		Select("js.*").
		Joins("JOIN jav_studio js ON js.id = jsa.jav_studio_id").
		Where("jsa.alias = ?", name).
		Limit(1).
		Scan(&studio).Error
	if err != nil {
		return nil, fmt.Errorf("load studio alias %q: %w", name, err)
	}
	if studio.ID > 0 {
		return &studio, nil
	}
	studio = models.JavStudio{Name: name}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&studio).Error; err != nil {
		return nil, fmt.Errorf("ensure studio %q: %w", name, err)
	}
	if studio.ID == 0 {
		if err := tx.Where("name = ?", name).First(&studio).Error; err != nil {
			return nil, fmt.Errorf("load studio %q: %w", name, err)
		}
	}
	return &studio, nil
}

func ensureSeriesTx(tx *gorm.DB, name string) (*models.JavSeries, error) {
	return ensureSeriesWithStudioTx(tx, name, false, nil)
}

func ensureSeriesWithStudioTx(tx *gorm.DB, name string, isEnglish bool, studioID *int64) (*models.JavSeries, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("series name cannot be empty")
	}
	series := models.JavSeries{Name: name, IsEnglish: isEnglish}
	if studioID != nil && *studioID > 0 {
		series.StudioID = studioID
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&series).Error; err != nil {
		return nil, fmt.Errorf("ensure series %q: %w", name, err)
	}
	if series.ID != 0 {
		return &series, nil
	}
	if err := tx.Where("name = ? AND is_english = ?", name, isEnglish).First(&series).Error; err != nil {
		return nil, fmt.Errorf("load series %q: %w", name, err)
	}
	return &series, nil
}

func ensureJavTagsTx(tx *gorm.DB, names []string, provider jav.Provider) ([]models.JavTag, error) {
	unique, err := normalizeScrapedJavTagNames(names)
	if err != nil {
		return nil, err
	}
	if len(unique) == 0 {
		return nil, nil
	}
	var tags []models.JavTag
	for _, name := range unique {
		tag := models.JavTag{Name: name, IsUser: false}
		if err := tx.Where("name = ? AND is_user = ?", name, false).FirstOrCreate(&tag).Error; err != nil {
			return nil, fmt.Errorf("ensure jav tag %q: %w", name, err)
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func normalizeScrapedJavTagNames(names []string) ([]string, error) {
	unique := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		traditional, err := util.TraditionalizeChineseName(name)
		if err != nil {
			return nil, fmt.Errorf("normalize scraped JAV tag %q: %w", name, err)
		}
		if _, ok := seen[traditional]; ok {
			continue
		}
		seen[traditional] = struct{}{}
		unique = append(unique, traditional)
	}
	return unique, nil
}

func replaceJavTagsForProviderTx(tx *gorm.DB, javID int64, tags []models.JavTag, provider jav.Provider) error {
	if javID == 0 {
		return errors.New("jav id cannot be zero")
	}
	provider = normalizeJavTagProvider(provider)
	if err := tx.
		Where("jav_id = ? AND provider = ?", javID, int(provider)).
		Delete(&models.JavTagMap{}).Error; err != nil {
		return fmt.Errorf("delete jav tag maps for provider: %w", err)
	}
	if len(tags) == 0 {
		return nil
	}
	now := time.Now()
	rows := make([]models.JavTagMap, 0, len(tags))
	for _, tag := range tags {
		if tag.ID == 0 {
			continue
		}
		rows = append(rows, models.JavTagMap{JavID: javID, JavTagID: tag.ID, Provider: int(provider), CreatedAt: now})
	}
	if len(rows) == 0 {
		return nil
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
		return fmt.Errorf("insert jav tag maps for provider: %w", err)
	}
	return nil
}

func deleteJavTagIfUnusedTx(tx *gorm.DB, tagID int64) error {
	if tagID <= 0 {
		return nil
	}
	var count int64
	if err := tx.Model(&models.JavTagMap{}).Where("jav_tag_id = ?", tagID).Count(&count).Error; err != nil {
		return fmt.Errorf("count jav tag maps: %w", err)
	}
	if count > 0 {
		return nil
	}
	if err := tx.Delete(&models.JavTag{}, tagID).Error; err != nil {
		return fmt.Errorf("delete jav tag: %w", err)
	}
	return nil
}

func appendJavIdolsTx(tx *gorm.DB, javRec *models.Jav, names []string) error {
	if javRec == nil || javRec.ID == 0 {
		return errors.New("jav record is missing")
	}

	var existingCount int64
	if err := tx.Model(&models.JavIdolMap{}).
		Where("jav_idol_map.jav_id = ?", javRec.ID).
		Count(&existingCount).Error; err != nil {
		return fmt.Errorf("count jav idol maps: %w", err)
	}
	if existingCount > 0 {
		return nil
	}

	idols, err := ensureJavIdolsTx(tx, names)
	if err != nil {
		return err
	}
	if len(idols) == 0 {
		return nil
	}
	if err := tx.Model(javRec).Association("Idols").Append(idols); err != nil {
		return fmt.Errorf("append jav idols: %w", err)
	}
	return nil
}

func appendJavIdolsIfMissingForProvider(ctx context.Context, javID int64, names []string, provider jav.Provider) (bool, error) {
	provider = jav.ParseProvider(int(provider))
	if provider == jav.ProviderJavDatabase || provider == jav.ProviderThePornDB {
		return false, errors.New("english JAV idols cannot be persisted")
	}
	unique := normalizeNames(names)
	if len(unique) == 0 {
		return false, nil
	}

	var updated bool
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var javRec models.Jav
		if err := tx.Select("id").Where("id = ?", javID).First(&javRec).Error; err != nil {
			return fmt.Errorf("get jav for idol append: %w", err)
		}
		var existingCount int64
		if err := tx.Model(&models.JavIdolMap{}).
			Where("jav_idol_map.jav_id = ?", javID).
			Count(&existingCount).Error; err != nil {
			return fmt.Errorf("count jav idol maps: %w", err)
		}
		if existingCount > 0 {
			return nil
		}

		idols, err := ensureJavIdolsTx(tx, unique)
		if err != nil {
			return err
		}
		if len(idols) == 0 {
			return nil
		}
		if err := tx.Model(&javRec).Association("Idols").Append(idols); err != nil {
			return fmt.Errorf("append jav idols: %w", err)
		}
		updated = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return updated, nil
}

func ensureJavIdolsTx(tx *gorm.DB, names []string) ([]models.JavIdol, error) {
	unique := normalizeNames(names)
	if len(unique) == 0 {
		return nil, nil
	}
	var idols []models.JavIdol
	for _, name := range unique {
		idol, err := findOrCreateJavIdolByNameOrAliasTx(tx, name)
		if err != nil {
			return nil, fmt.Errorf("ensure jav idol %q: %w", name, err)
		}
		idols = append(idols, idol)
	}
	return idols, nil
}

func findOrCreateJavIdolByNameOrAliasTx(tx *gorm.DB, name string) (models.JavIdol, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return models.JavIdol{}, errors.New("jav idol name cannot be empty")
	}
	var idol models.JavIdol
	err := tx.Where("name = ?", name).First(&idol).Error
	if err == nil {
		return idol, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.JavIdol{}, err
	}
	err = tx.
		Table("jav_idol_alias jia").
		Select("ji.*").
		Joins("JOIN jav_idol ji ON ji.id = jia.jav_idol_id").
		Where("jia.alias = ?", name).
		Limit(1).
		Scan(&idol).Error
	if err != nil {
		return models.JavIdol{}, err
	}
	if idol.ID > 0 {
		return idol, nil
	}
	idol = models.JavIdol{Name: name}
	if err := tx.Create(&idol).Error; err != nil {
		return models.JavIdol{}, err
	}
	return idol, nil
}

// MergeJavIdols physically moves source idol relationships onto canonicalID and records source names as aliases.
func MergeJavIdols(ctx context.Context, canonicalID int64, sourceIDs []int64, directoryIDs []int64) (*JavIdolSummary, error) {
	if canonicalID <= 0 {
		return nil, errors.New("canonical_id must be positive")
	}
	cleanSourceIDs := make([]int64, 0, len(sourceIDs))
	seen := map[int64]bool{}
	for _, id := range uniqueInt64s(sourceIDs) {
		if id <= 0 || id == canonicalID || seen[id] {
			continue
		}
		seen[id] = true
		cleanSourceIDs = append(cleanSourceIDs, id)
	}
	if len(cleanSourceIDs) == 0 {
		return nil, errors.New("merge_ids required")
	}

	if err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var canonical models.JavIdol
		if err := tx.Where("id = ?", canonicalID).First(&canonical).Error; err != nil {
			return fmt.Errorf("find canonical jav idol: %w", err)
		}

		var sources []models.JavIdol
		if err := tx.Where("id IN ?", cleanSourceIDs).Find(&sources).Error; err != nil {
			return fmt.Errorf("find source jav idols: %w", err)
		}
		if len(sources) != len(cleanSourceIDs) {
			return gorm.ErrRecordNotFound
		}
		if err := moveJavIdolAliasesTx(tx, canonical, sources); err != nil {
			return err
		}
		if err := moveJavIdolMapsTx(tx, canonicalID, cleanSourceIDs); err != nil {
			return err
		}
		if err := moveJavIdolFavoriteMapsTx(tx, canonicalID, cleanSourceIDs); err != nil {
			return err
		}
		if err := inheritJavIdolCoverTx(tx, canonical, sources); err != nil {
			return err
		}
		if err := tx.Where("id IN ?", cleanSourceIDs).Delete(&models.JavIdol{}).Error; err != nil {
			return fmt.Errorf("delete merged jav idols: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return GetJavIdolSummary(ctx, canonicalID, directoryIDs)
}

// MergeJavStudios moves source studio relationships onto canonicalID and records source names as aliases.
func MergeJavStudios(ctx context.Context, canonicalID int64, sourceIDs []int64, directoryIDs []int64) (*JavStudioSummary, error) {
	if canonicalID <= 0 {
		return nil, errors.New("canonical_id must be positive")
	}
	cleanSourceIDs := make([]int64, 0, len(sourceIDs))
	seen := map[int64]bool{}
	for _, id := range uniqueInt64s(sourceIDs) {
		if id <= 0 || id == canonicalID || seen[id] {
			continue
		}
		seen[id] = true
		cleanSourceIDs = append(cleanSourceIDs, id)
	}
	if len(cleanSourceIDs) == 0 {
		return nil, errors.New("merge_ids required")
	}

	if err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var canonical models.JavStudio
		if err := tx.Where("id = ?", canonicalID).First(&canonical).Error; err != nil {
			return fmt.Errorf("find canonical jav studio: %w", err)
		}
		var sources []models.JavStudio
		if err := tx.Where("id IN ?", cleanSourceIDs).Find(&sources).Error; err != nil {
			return fmt.Errorf("find source jav studios: %w", err)
		}
		if len(sources) != len(cleanSourceIDs) {
			return gorm.ErrRecordNotFound
		}
		if err := moveJavStudioAliasesTx(tx, canonical, sources); err != nil {
			return err
		}
		if err := tx.Model(&models.Jav{}).
			Where("studio_id IN ?", cleanSourceIDs).
			Update("studio_id", canonicalID).Error; err != nil {
			return fmt.Errorf("move jav studio works: %w", err)
		}
		if err := tx.Model(&models.JavSeries{}).
			Where("studio_id IN ?", cleanSourceIDs).
			Update("studio_id", canonicalID).Error; err != nil {
			return fmt.Errorf("move jav studio series: %w", err)
		}
		if err := moveJavStudioFavoriteMapsTx(tx, canonicalID, cleanSourceIDs); err != nil {
			return err
		}
		if err := tx.Where("id IN ?", cleanSourceIDs).Delete(&models.JavStudio{}).Error; err != nil {
			return fmt.Errorf("delete merged jav studios: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return GetJavStudioSummary(ctx, canonicalID, directoryIDs)
}

func moveJavStudioAliasesTx(tx *gorm.DB, canonical models.JavStudio, sources []models.JavStudio) error {
	sourceIDs := make([]int64, 0, len(sources))
	for _, source := range sources {
		sourceIDs = append(sourceIDs, source.ID)
	}
	if len(sourceIDs) > 0 {
		if err := tx.Exec(
			`DELETE FROM jav_studio_alias
			WHERE jav_studio_id IN ?
			AND EXISTS (
				SELECT 1
				FROM jav_studio_alias canonical_alias
				WHERE canonical_alias.jav_studio_id = ?
				AND canonical_alias.alias = jav_studio_alias.alias
			)`,
			sourceIDs,
			canonical.ID,
		).Error; err != nil {
			return fmt.Errorf("delete duplicate jav studio aliases: %w", err)
		}
		if err := tx.Model(&models.JavStudioAlias{}).
			Where("jav_studio_id IN ?", sourceIDs).
			Update("jav_studio_id", canonical.ID).Error; err != nil {
			return fmt.Errorf("move jav studio aliases: %w", err)
		}
	}

	aliases := make([]models.JavStudioAlias, 0, len(sources))
	seen := map[string]bool{}
	for _, source := range sources {
		alias := strings.TrimSpace(source.Name)
		if alias == "" || alias == canonical.Name || seen[alias] {
			continue
		}
		seen[alias] = true
		aliases = append(aliases, models.JavStudioAlias{
			JavStudioID: canonical.ID,
			Alias:       alias,
			CreatedAt:   time.Now(),
		})
	}
	if len(aliases) == 0 {
		return nil
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "alias"}},
		DoNothing: true,
	}).Create(&aliases).Error; err != nil {
		return fmt.Errorf("create jav studio aliases: %w", err)
	}
	return nil
}

func moveJavStudioFavoriteMapsTx(tx *gorm.DB, canonicalID int64, sourceIDs []int64) error {
	if err := tx.Exec(
		`INSERT OR IGNORE INTO jav_favorite_map (jav_favorite_group_id, entity_type, entity_id, created_at, sort_order)
		SELECT jav_favorite_group_id, entity_type, ?, MIN(created_at), MIN(sort_order)
		FROM jav_favorite_map
		WHERE entity_type = ? AND entity_id IN ?
		GROUP BY jav_favorite_group_id, entity_type`,
		canonicalID,
		JavFavoriteEntityStudio,
		sourceIDs,
	).Error; err != nil {
		return fmt.Errorf("move jav studio favorite maps: %w", err)
	}
	if err := tx.Where("entity_type = ? AND entity_id IN ?", JavFavoriteEntityStudio, sourceIDs).
		Delete(&models.JavFavoriteMap{}).Error; err != nil {
		return fmt.Errorf("delete source jav studio favorite maps: %w", err)
	}
	return nil
}

func moveJavIdolAliasesTx(tx *gorm.DB, canonical models.JavIdol, sources []models.JavIdol) error {
	sourceIDs := make([]int64, 0, len(sources))
	for _, source := range sources {
		sourceIDs = append(sourceIDs, source.ID)
	}
	if len(sourceIDs) > 0 {
		if err := tx.Exec(
			`DELETE FROM jav_idol_alias
			WHERE jav_idol_id IN ?
			AND EXISTS (
				SELECT 1
				FROM jav_idol_alias canonical_alias
				WHERE canonical_alias.jav_idol_id = ?
				AND canonical_alias.alias = jav_idol_alias.alias
			)`,
			sourceIDs,
			canonical.ID,
		).Error; err != nil {
			return fmt.Errorf("delete duplicate jav idol aliases: %w", err)
		}
		if err := tx.Model(&models.JavIdolAlias{}).
			Where("jav_idol_id IN ?", sourceIDs).
			Update("jav_idol_id", canonical.ID).Error; err != nil {
			return fmt.Errorf("move jav idol aliases: %w", err)
		}
	}

	aliases := make([]models.JavIdolAlias, 0, len(sources)*4)
	seen := map[string]bool{}
	addAlias := func(value string) {
		alias := strings.TrimSpace(value)
		if alias == "" || alias == canonical.Name || alias == canonical.RomanName || alias == canonical.JapaneseName || alias == canonical.ChineseName {
			return
		}
		key := strings.ToLower(alias)
		if seen[key] {
			return
		}
		seen[key] = true
		aliases = append(aliases, models.JavIdolAlias{
			JavIdolID: canonical.ID,
			Alias:     alias,
			CreatedAt: time.Now(),
		})
	}
	for _, source := range sources {
		addAlias(source.Name)
	}
	if len(aliases) == 0 {
		return nil
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "alias"}},
		DoNothing: true,
	}).Create(&aliases).Error; err != nil {
		return fmt.Errorf("create jav idol aliases: %w", err)
	}
	return nil
}

func moveJavIdolMapsTx(tx *gorm.DB, canonicalID int64, sourceIDs []int64) error {
	if err := tx.Exec(
		`INSERT OR IGNORE INTO jav_idol_map (jav_id, jav_idol_id, created_at)
		SELECT jav_id, ?, MIN(created_at)
		FROM jav_idol_map
		WHERE jav_idol_id IN ?
		GROUP BY jav_id`,
		canonicalID,
		sourceIDs,
	).Error; err != nil {
		return fmt.Errorf("move jav idol maps: %w", err)
	}
	if err := tx.Where("jav_idol_id IN ?", sourceIDs).Delete(&models.JavIdolMap{}).Error; err != nil {
		return fmt.Errorf("delete source jav idol maps: %w", err)
	}
	return nil
}

func moveJavIdolFavoriteMapsTx(tx *gorm.DB, canonicalID int64, sourceIDs []int64) error {
	if err := tx.Exec(
		`INSERT OR IGNORE INTO jav_favorite_map (jav_favorite_group_id, entity_type, entity_id, created_at, sort_order)
		SELECT jav_favorite_group_id, entity_type, ?, MIN(created_at), MIN(sort_order)
		FROM jav_favorite_map
		WHERE entity_type = ? AND entity_id IN ?
		GROUP BY jav_favorite_group_id, entity_type`,
		canonicalID,
		JavFavoriteEntityIdol,
		sourceIDs,
	).Error; err != nil {
		return fmt.Errorf("move jav idol favorite maps: %w", err)
	}
	if err := tx.Where("entity_type = ? AND entity_id IN ?", JavFavoriteEntityIdol, sourceIDs).Delete(&models.JavFavoriteMap{}).Error; err != nil {
		return fmt.Errorf("delete source jav idol favorite maps: %w", err)
	}
	return nil
}

func inheritJavIdolCoverTx(tx *gorm.DB, canonical models.JavIdol, sources []models.JavIdol) error {
	if canonical.CoverJavID != nil {
		return nil
	}
	for _, source := range sources {
		if source.CoverJavID == nil {
			continue
		}
		if err := tx.Model(&models.JavIdol{}).
			Where("id = ?", canonical.ID).
			Updates(map[string]any{
				"cover_jav_id":    source.CoverJavID,
				"cover_crop_left": source.CoverCropLeft,
			}).Error; err != nil {
			return fmt.Errorf("inherit jav idol cover: %w", err)
		}
		return nil
	}
	return nil
}

func replaceJavIdolsTx(tx *gorm.DB, javID int64, idolIDs []int64) error {
	if javID <= 0 {
		return errors.New("jav id cannot be zero")
	}
	cleanIDs := uniqueInt64s(idolIDs)
	if err := tx.
		Where("jav_id = ?", javID).
		Delete(&models.JavIdolMap{}).Error; err != nil {
		return fmt.Errorf("delete jav idol maps: %w", err)
	}

	if len(cleanIDs) == 0 {
		return nil
	}
	var count int64
	if err := tx.Model(&models.JavIdol{}).
		Where("id IN ?", cleanIDs).
		Count(&count).Error; err != nil {
		return fmt.Errorf("find jav idols: %w", err)
	}
	if count != int64(len(cleanIDs)) {
		return errors.New("invalid idol_id")
	}

	now := time.Now()
	rows := make([]models.JavIdolMap, 0, len(cleanIDs))
	for _, idolID := range cleanIDs {
		rows = append(rows, models.JavIdolMap{
			JavID:     javID,
			JavIdolID: idolID,
			CreatedAt: now,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
		return fmt.Errorf("insert jav idol maps: %w", err)
	}
	return nil
}

func replaceJavScrapedTagsTx(tx *gorm.DB, javID int64, tagIDs []int64) error {
	if javID <= 0 {
		return errors.New("jav id cannot be zero")
	}
	cleanTagIDs := uniqueInt64s(tagIDs)
	if len(cleanTagIDs) > 0 {
		var count int64
		if err := tx.Model(&models.JavTag{}).
			Where("id IN ? AND is_user = ?", cleanTagIDs, false).
			Count(&count).Error; err != nil {
			return fmt.Errorf("find scraped jav tags: %w", err)
		}
		if count != int64(len(cleanTagIDs)) {
			return errors.New("invalid scraped_tag_id")
		}
	}
	providers := visibleScrapedJavTagProviders()
	var oldTagIDs []int64
	if err := tx.Model(&models.JavTagMap{}).
		Where("jav_id = ? AND provider IN ?", javID, providers).
		Distinct().
		Pluck("jav_tag_id", &oldTagIDs).Error; err != nil {
		return fmt.Errorf("find scraped jav tag maps: %w", err)
	}
	if err := tx.
		Where("jav_id = ? AND provider IN ?", javID, providers).
		Delete(&models.JavTagMap{}).Error; err != nil {
		return fmt.Errorf("delete scraped jav tag maps: %w", err)
	}

	tags := make([]models.JavTag, 0, len(cleanTagIDs))
	for _, tagID := range cleanTagIDs {
		tags = append(tags, models.JavTag{ID: tagID})
	}
	if err := replaceJavTagsForProviderTx(tx, javID, tags, jav.ProviderManualScrape); err != nil {
		return err
	}
	for _, tagID := range uniqueInt64s(oldTagIDs) {
		if err := deleteJavTagIfUnusedTx(tx, tagID); err != nil {
			return err
		}
	}
	return nil
}

func setVideoLocationJavIDTx(tx *gorm.DB, locationID, expectedVideoID, javID int64, expectedUpdatedAt time.Time) error {
	if tx == nil {
		return errors.New("tx is nil")
	}
	q := tx.Model(&models.VideoLocation{}).Where("id = ?", locationID)
	if expectedVideoID > 0 {
		q = q.Where("video_id = ?", expectedVideoID).
			Where("jav_id IS NULL OR jav_id = ?", javID)
	} else if !expectedUpdatedAt.IsZero() {
		q = q.Where("updated_at = ?", expectedUpdatedAt)
	}
	res := q.Update("jav_id", javID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 && (expectedVideoID > 0 || !expectedUpdatedAt.IsZero()) {
		ok, err := videoLocationHasJavIDTx(tx, locationID, expectedVideoID, javID)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		return fmt.Errorf("video location %d stale or missing", locationID)
	}
	return nil
}

func videoLocationHasJavIDTx(tx *gorm.DB, locationID, expectedVideoID, javID int64) (bool, error) {
	if tx == nil {
		return false, errors.New("tx is nil")
	}
	var loc models.VideoLocation
	err := tx.Select("id", "video_id", "jav_id").Where("id = ?", locationID).First(&loc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("get video location jav id: %w", err)
	}
	if expectedVideoID > 0 && loc.VideoID != expectedVideoID {
		return false, nil
	}
	return loc.JavID != nil && *loc.JavID == javID, nil
}
