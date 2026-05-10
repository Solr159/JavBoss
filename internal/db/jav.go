package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"pornboss/internal/common"
	"pornboss/internal/jav"
	"pornboss/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// JavTagCount represents a JAV tag with associated work count.
type JavTagCount struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Provider int    `json:"provider"`
	Count    int64  `json:"count"`
}

// JavScanVideo contains the fields the scanner needs to resolve or refresh JAV metadata.
type JavScanVideo struct {
	LocationID  int64     `gorm:"column:location_id"`
	VideoID     int64     `gorm:"column:video_id"`
	Filename    string    `gorm:"column:filename"`
	JavID       *int64    `gorm:"column:jav_id"`
	JavProvider int       `gorm:"column:jav_provider"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
	DurationSec int64     `gorm:"column:duration_sec"`
}

// JavStudioScanItem contains a JAV row that needs studio metadata.
type JavStudioScanItem struct {
	ID   int64  `gorm:"column:id"`
	Code string `gorm:"column:code"`
}

// SearchJav lists Jav metadata filtered by idol IDs/tag IDs/search with pagination and sorting.
func SearchJav(ctx context.Context, idolIDs []int64, tagIDs []int64, search, sort string, limit, offset int, seed *int64, directoryIDs []int64, studioIDs ...int64) ([]models.Jav, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	idolIDs = uniqueInt64s(idolIDs)
	tagIDs = uniqueInt64s(tagIDs)
	search = strings.TrimSpace(search)
	sort = strings.ToLower(strings.TrimSpace(sort))

	studioID := firstPositiveInt64(studioIDs)
	filtered := buildJavFilter(ctx, idolIDs, tagIDs, search, directoryIDs, studioID)

	// Count on a cloned query to avoid mutating the main one.
	countBase := buildJavFilter(ctx, idolIDs, tagIDs, search, directoryIDs, studioID)
	countQuery := countBase.Select("DISTINCT jav.id")
	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count jav: %w", err)
	}

	var items []models.Jav
	order := "jav.created_at DESC"
	var orderExpr clause.Expr
	useExpr := false
	switch sort {
	case "code", "code_asc":
		order = "jav.code"
	case "code_desc":
		order = "jav.code DESC"
	case "duration", "duration_desc":
		order = "jav.duration_min DESC, jav.created_at DESC, jav.id DESC"
	case "duration_asc":
		order = "jav.duration_min ASC, jav.created_at ASC, jav.id ASC"
	case "release", "release_desc":
		order = "jav.release_unix DESC, jav.code"
	case "release_asc":
		order = "jav.release_unix IS NULL, jav.release_unix ASC, jav.code"
	case "play_count", "play_count_desc":
		order = "COALESCE((SELECT SUM(COALESCE(v.play_count, 0)) FROM video_location vl JOIN directory d ON d.id = vl.directory_id JOIN video v ON v.id = vl.video_id WHERE vl.jav_id = jav.id AND " + activeLocationWhereSQL("vl", "d") + directoryFilterSQL("vl", directoryIDs) + "), 0) DESC, jav.created_at DESC, jav.id DESC"
	case "play_count_asc":
		order = "COALESCE((SELECT SUM(COALESCE(v.play_count, 0)) FROM video_location vl JOIN directory d ON d.id = vl.directory_id JOIN video v ON v.id = vl.video_id WHERE vl.jav_id = jav.id AND " + activeLocationWhereSQL("vl", "d") + directoryFilterSQL("vl", directoryIDs) + "), 0) ASC, jav.created_at ASC, jav.id ASC"
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
	visibleTagProviders := visibleJavTagProviders()
	query := filtered.
		Preload("Studio").
		Preload("Tags", "provider IN ?", visibleTagProviders).
		Preload("Idols", "COALESCE(is_english, 0) = ?", jav.CurrentMetadataLanguageIsEnglish()).
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
	return items, total, nil
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

// ListJavTags returns JAV tags with the number of works for each tag.
func ListJavTags(ctx context.Context, directoryIDs []int64) ([]JavTagCount, error) {
	var tags []JavTagCount
	visibleProviders := visibleJavTagProviders()
	query := common.DB.WithContext(ctx).
		Table("jav_tag jt").
		Select("jt.id, jt.name, jt.provider, COUNT(DISTINCT CASE WHEN "+activeLocationWhereSQL("vl", "d")+" THEN jtm.jav_id END) AS count").
		Joins("LEFT JOIN jav_tag_map jtm ON jtm.jav_tag_id = jt.id").
		Joins("LEFT JOIN video_location vl ON vl.jav_id = jtm.jav_id").
		Joins("LEFT JOIN directory d ON d.id = vl.directory_id").
		Where("jt.provider IN ?", visibleProviders)
	query = applyDirectoryFilter(query, "vl", directoryIDs)
	if err := query.
		Group("jt.id, jt.name, jt.provider").
		Order("jt.name, jt.provider").
		Scan(&tags).Error; err != nil {
		return nil, fmt.Errorf("list jav tags: %w", err)
	}
	return tags, nil
}

func visibleJavTagProviders() []int {
	current := int(jav.PreferredProvider())
	if current <= 0 {
		current = int(jav.ProviderJavBus)
	}
	return []int{current, int(jav.ProviderUser)}
}

// CreateJavTag creates a user-defined JAV tag.
func CreateJavTag(ctx context.Context, name string) (*models.JavTag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("tag name cannot be empty")
	}

	tag := models.JavTag{Name: name, Provider: int(jav.ProviderUser)}
	if err := common.DB.WithContext(ctx).Create(&tag).Error; err != nil {
		return nil, fmt.Errorf("create jav tag %q: %w", name, err)
	}
	return &tag, nil
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
	if !isUserJavTagProvider(tag.Provider) {
		return errors.New("tag is not user-defined")
	}

	if err := common.DB.WithContext(ctx).
		Model(&models.JavTag{}).
		Where("id = ? AND provider = ?", id, int(jav.ProviderUser)).
		Update("name", newName).Error; err != nil {
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
	if !isUserJavTagProvider(tag.Provider) {
		return errors.New("tag is not user-defined")
	}

	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("jav_tag_id = ?", id).Delete(&models.JavTagMap{}).Error; err != nil {
			return fmt.Errorf("delete jav tag relations: %w", err)
		}
		if err := tx.Delete(&models.JavTag{}, id).Error; err != nil {
			return fmt.Errorf("delete jav tag: %w", err)
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
		Model(&models.JavTag{}).
		Where("id IN ? AND provider = ?", cleanIDs, int(jav.ProviderUser)).
		Count(&count).Error; err != nil {
		return fmt.Errorf("find jav tags: %w", err)
	}
	if count != int64(len(cleanIDs)) {
		return errors.New("tag is not user-defined")
	}

	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("jav_tag_id IN ?", cleanIDs).Delete(&models.JavTagMap{}).Error; err != nil {
			return fmt.Errorf("delete jav tag relations: %w", err)
		}
		if err := tx.Where("id IN ?", cleanIDs).Delete(&models.JavTag{}).Error; err != nil {
			return fmt.Errorf("delete jav tag: %w", err)
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
		if !isUserJavTagProvider(tag.Provider) {
			return errors.New("tag is not user-defined")
		}

		now := time.Now()
		rows := make([]models.JavTagMap, 0, len(cleanIDs))
		for _, javID := range cleanIDs {
			rows = append(rows, models.JavTagMap{JavID: javID, JavTagID: tagID, CreatedAt: now})
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
	if !isUserJavTagProvider(tag.Provider) {
		return errors.New("tag is not user-defined")
	}

	if err := common.DB.WithContext(ctx).
		Where("jav_id IN ? AND jav_tag_id = ?", cleanIDs, tagID).
		Delete(&models.JavTagMap{}).Error; err != nil {
		return fmt.Errorf("delete jav tag map: %w", err)
	}
	return nil
}

// ReplaceJavUserTags replaces user-defined tags for the given JAV entries.
func ReplaceJavUserTags(ctx context.Context, javIDs, tagIDs []int64) error {
	cleanJavIDs := uniqueInt64s(javIDs)
	if len(cleanJavIDs) == 0 {
		return nil
	}
	cleanTagIDs := uniqueInt64s(tagIDs)

	if len(cleanTagIDs) > 0 {
		var count int64
		if err := common.DB.WithContext(ctx).
			Model(&models.JavTag{}).
			Where("id IN ? AND provider = ?", cleanTagIDs, int(jav.ProviderUser)).
			Count(&count).Error; err != nil {
			return fmt.Errorf("find jav tags: %w", err)
		}
		if count != int64(len(cleanTagIDs)) {
			return errors.New("invalid tag_id")
		}
	}

	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("jav_id IN ? AND jav_tag_id IN (SELECT id FROM jav_tag WHERE provider = ?)", cleanJavIDs, int(jav.ProviderUser)).
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
				rows = append(rows, models.JavTagMap{JavID: javID, JavTagID: tagID, CreatedAt: now})
			}
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
			return fmt.Errorf("insert jav tag map: %w", err)
		}
		return nil
	})
}

func buildJavFilter(ctx context.Context, idolIDs []int64, tagIDs []int64, search string, directoryIDs []int64, studioID int64) *gorm.DB {
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
		titleColumn := "title"
		if jav.CurrentMetadataLanguageIsEnglish() {
			titleColumn = "title_en"
		}
		q = q.Where("code LIKE ? OR "+titleColumn+" LIKE ?", like, like)
	}
	if studioID > 0 {
		q = q.Where("studio_id = ?", studioID)
	}
	if len(tagIDs) > 0 {
		q = q.
			Joins("JOIN jav_tag_map jtm ON jtm.jav_id = jav.id").
			Joins("JOIN jav_tag jt ON jt.id = jtm.jav_tag_id").
			Where("jt.provider IN ?", visibleTagProviders).
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

func firstPositiveInt64(values []int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

// JavStudioSummary represents studio info with aggregated work count and a sample code for cover lookup.
type JavStudioSummary struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	WorkCount  int64  `json:"work_count"`
	SampleCode string `json:"sample_code"`
}

func applyJavStudioSearch(q *gorm.DB, search string) *gorm.DB {
	search = strings.TrimSpace(search)
	if search == "" {
		return q
	}
	like := fmt.Sprintf("%%%s%%", search)
	return q.Where("js.name LIKE ?", like)
}

// ListJavStudios returns studios ordered by visible work count descending.
func ListJavStudios(ctx context.Context, search string, limit, offset int, directoryIDs []int64) ([]JavStudioSummary, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	countBase := common.DB.WithContext(ctx).
		Table("jav_studio js").
		Joins("JOIN jav j ON j.studio_id = js.id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where(activeLocationWhereSQL("vl", "d"))
	countBase = applyDirectoryFilter(countBase, "vl", directoryIDs)
	countBase = applyJavStudioSearch(countBase, search)

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
	if err := base.
		Select("js.id, js.name, COUNT(DISTINCT j.id) AS work_count, MIN(j.code) AS sample_code").
		Group("js.id, js.name").
		Order("work_count DESC, js.name ASC").
		Limit(limit).
		Offset(offset).
		Scan(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list jav studios: %w", err)
	}

	return items, total, nil
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

// JavIdolSummary represents idol info with aggregated work count and a sample code for cover lookup.
type JavIdolSummary struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	RomanName    string     `json:"roman_name"`
	JapaneseName string     `json:"japanese_name"`
	ChineseName  string     `json:"chinese_name"`
	HeightCM     *int       `json:"height_cm"`
	BirthDate    *time.Time `json:"birth_date"`
	Bust         *int       `json:"bust"`
	Waist        *int       `json:"waist"`
	Hips         *int       `json:"hips"`
	Cup          *int       `json:"cup"`
	WorkCount    int64      `json:"work_count"`
	SampleCode   string     `json:"sample_code"`
}

func applyJavIdolSearch(q *gorm.DB, search string) *gorm.DB {
	search = strings.TrimSpace(search)
	if search == "" {
		return q
	}
	like := fmt.Sprintf("%%%s%%", search)
	return q.Where(
		"ji.name LIKE ? OR ji.japanese_name LIKE ? OR ji.roman_name LIKE ? OR ji.chinese_name LIKE ?",
		like,
		like,
		like,
		like,
	)
}

func buildVisibleSoloIdolSampleQuery(ctx context.Context, directoryIDs []int64, language ...bool) *gorm.DB {
	soloJavs := common.DB.WithContext(ctx).
		Table("jav_idol_map jim_count").
		Select("jim_count.jav_id").
		Group("jim_count.jav_id").
		Having("COUNT(*) = 1")
	if len(language) > 0 {
		soloJavs = soloJavs.
			Joins("JOIN jav_idol ji_count ON ji_count.id = jim_count.jav_idol_id").
			Where("COALESCE(ji_count.is_english, 0) = ?", language[0])
	}

	query := common.DB.WithContext(ctx).
		Table("jav_idol_map jim_solo").
		Select("jim_solo.jav_idol_id, MIN(j_solo.code) AS sample_code").
		Joins("JOIN (?) solo_jav ON solo_jav.jav_id = jim_solo.jav_id", soloJavs).
		Joins("JOIN jav j_solo ON j_solo.id = jim_solo.jav_id").
		Joins("JOIN video_location vl_solo ON vl_solo.jav_id = jim_solo.jav_id").
		Joins("JOIN directory d_solo ON d_solo.id = vl_solo.directory_id").
		Where(activeLocationWhereSQL("vl_solo", "d_solo"))
	if len(language) > 0 {
		query = query.
			Joins("JOIN jav_idol ji_solo ON ji_solo.id = jim_solo.jav_idol_id").
			Where("COALESCE(ji_solo.is_english, 0) = ?", language[0])
	}
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
	isEnglish := jav.CurrentMetadataLanguageIsEnglish()
	tx := common.DB.WithContext(ctx).
		Table("jav_idol ji").
		Select("ji.id, ji.name, ji.roman_name, ji.japanese_name, ji.chinese_name, ji.height_cm, ji.birth_date, ji.bust, ji.waist, ji.hips, ji.cup, COALESCE(idol_work_counts.work_count, 0) AS work_count, solo_idols.sample_code").
		Joins("LEFT JOIN (?) idol_work_counts ON idol_work_counts.jav_idol_id = ji.id", buildVisibleIdolWorkCountQuery(ctx, directoryIDs)).
		Joins("LEFT JOIN (?) solo_idols ON solo_idols.jav_idol_id = ji.id", buildVisibleSoloIdolSampleQuery(ctx, directoryIDs, isEnglish)).
		Where("ji.id = ?", idolID).
		Where("COALESCE(ji.is_english, 0) = ?", isEnglish).
		Where("solo_idols.sample_code IS NOT NULL").
		Limit(1).
		Scan(&item)
	if tx.Error != nil {
		return nil, fmt.Errorf("get jav idol summary: %w", tx.Error)
	}
	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &item, nil
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
		Where("COALESCE(ji.is_english, 0) = ?", jav.CurrentMetadataLanguageIsEnglish()).
		Order("ji.name").
		Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("resolve jav idols: %w", err)
	}
	return items, nil
}

// ListJavIdols returns idols ordered by selected sort with pagination.
func ListJavIdols(ctx context.Context, search, sort string, limit, offset int, directoryIDs []int64) ([]JavIdolSummary, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	sort = strings.ToLower(strings.TrimSpace(sort))
	isEnglish := jav.CurrentMetadataLanguageIsEnglish()
	soloIdols := buildVisibleSoloIdolSampleQuery(ctx, directoryIDs, isEnglish)

	countBase := common.DB.WithContext(ctx).
		Table("jav_idol ji").
		Joins("JOIN (?) solo_idols ON solo_idols.jav_idol_id = ji.id", soloIdols).
		Where("COALESCE(ji.is_english, 0) = ?", isEnglish)
	countBase = applyJavIdolSearch(countBase, search)

	var total int64
	if err := countBase.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count jav idols: %w", err)
	}

	var items []JavIdolSummary
	order := "work_count DESC, ji.name ASC"
	switch sort {
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
	base := common.DB.WithContext(ctx).
		Table("jav_idol ji").
		Joins("JOIN (?) solo_idols ON solo_idols.jav_idol_id = ji.id", soloIdols).
		Joins("JOIN jav_idol_map jim ON jim.jav_idol_id = ji.id").
		Joins("JOIN jav j ON j.id = jim.jav_id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where("COALESCE(ji.is_english, 0) = ?", isEnglish).
		Where(activeLocationWhereSQL("vl", "d"))
	base = applyDirectoryFilter(base, "vl", directoryIDs)
	base = applyJavIdolSearch(base, search)
	if err := base.
		Select("ji.id, ji.name, ji.roman_name, ji.japanese_name, ji.chinese_name, ji.height_cm, ji.birth_date, ji.bust, ji.waist, ji.hips, ji.cup, COUNT(DISTINCT j.id) AS work_count, solo_idols.sample_code").
		Group("ji.id, ji.name, ji.roman_name, ji.japanese_name, ji.chinese_name, ji.height_cm, ji.birth_date, ji.bust, ji.waist, ji.hips, ji.cup, solo_idols.sample_code").
		Order(order).
		Limit(limit).
		Offset(offset).
		Scan(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list jav idols: %w", err)
	}

	return items, total, nil
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

// FindIdolSoloCode returns one solo work code for the idol, when available.
func FindIdolSoloCode(ctx context.Context, idolID int64) (string, error) {
	if idolID == 0 {
		return "", errors.New("idol id cannot be zero")
	}
	sub := common.DB.WithContext(ctx).
		Table("jav_idol_map jim_count").
		Select("jim_count.jav_id, COUNT(*) as c").
		Joins("JOIN jav_idol ji_count ON ji_count.id = jim_count.jav_idol_id").
		Where("COALESCE(ji_count.is_english, 0) = (SELECT COALESCE(is_english, 0) FROM jav_idol WHERE id = ?)", idolID).
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
	isEnglish := jav.CurrentMetadataLanguageIsEnglish()
	soloIdols := buildVisibleSoloIdolSampleQuery(ctx, nil, isEnglish)
	if err := common.DB.WithContext(ctx).
		Joins("JOIN (?) solo_idols ON solo_idols.jav_idol_id = jav_idol.id", soloIdols).
		Where("COALESCE(jav_idol.is_english, 0) = ?", isEnglish).
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
	if err := common.DB.WithContext(ctx).
		Table("video_location vl").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Joins("JOIN video v ON v.id = vl.video_id").
		Joins("LEFT JOIN jav ON jav.id = vl.jav_id").
		Where("COALESCE(vl.is_delete, 0) = 0").
		Where("COALESCE(d.is_delete, 0) = 0").
		Where("COALESCE(d.missing, 0) = 0").
		Select("vl.id AS location_id, vl.video_id, COALESCE(NULLIF(vl.filename, ''), vl.relative_path) AS filename, vl.jav_id, vl.updated_at, v.duration_sec, COALESCE(jav.provider, 0) AS jav_provider").
		Order("vl.updated_at DESC, vl.id DESC").
		Find(&videos).Error; err != nil {
		return nil, fmt.Errorf("list videos for jav scan: %w", err)
	}
	return videos, nil
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
	return setVideoLocationJavIDTx(common.DB.WithContext(ctx), locationID, javID, expectedUpdatedAt)
}

// SaveJavInfoAndLinkLocation upserts jav metadata and associates the video location in one transaction.
func SaveJavInfoAndLinkLocation(ctx context.Context, info *jav.JavInfo, locationID int64, expectedUpdatedAt time.Time) (*models.Jav, error) {
	if info == nil {
		return nil, errors.New("jav info is nil")
	}
	var javRec *models.Jav
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rec, err := saveJavInfoTx(tx, info)
		if err != nil {
			return err
		}
		if err := setVideoLocationJavIDTx(tx, locationID, rec.ID, expectedUpdatedAt); err != nil {
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

// ListJavCodes returns all jav codes.
func ListJavCodes(ctx context.Context) ([]string, error) {
	var codes []string
	if err := common.DB.WithContext(ctx).Model(&models.Jav{}).Pluck("code", &codes).Error; err != nil {
		return nil, fmt.Errorf("list jav codes: %w", err)
	}
	return codes, nil
}

// ListJavsMissingStudio returns JAV rows whose studio relation is empty.
func ListJavsMissingStudio(ctx context.Context) ([]JavStudioScanItem, error) {
	var items []JavStudioScanItem
	if err := common.DB.WithContext(ctx).
		Model(&models.Jav{}).
		Select("id, code").
		Where("COALESCE(code, '') <> ''").
		Where("studio_id IS NULL").
		Order("created_at ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list javs missing studio: %w", err)
	}
	return items, nil
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
	javRec.Code = info.Code
	if jav.ProviderIsEnglish(provider) {
		javRec.TitleEn = info.Title
	} else {
		javRec.Title = info.Title
	}
	javRec.ReleaseUnix = info.ReleaseUnix
	javRec.DurationMin = info.DurationMin
	javRec.Provider = int(provider)
	javRec.FetchedAt = ts
	if studio := strings.TrimSpace(info.Studio); studio != "" {
		studioRec, err := ensureStudioTx(tx, studio)
		if err != nil {
			return nil, err
		}
		javRec.StudioID = &studioRec.ID
	}
	if err := tx.Save(javRec).Error; err != nil {
		return nil, fmt.Errorf("save jav: %w", err)
	}

	tags, err := ensureJavTagsTx(tx, info.Tags, info.Provider)
	if err != nil {
		return nil, err
	}
	if err := replaceJavTagsForProviderTx(tx, javRec.ID, tags, info.Provider); err != nil {
		return nil, err
	}
	if err := appendJavIdolsForProviderLanguageTx(tx, javRec, info.Actors, info.Provider); err != nil {
		return nil, err
	}
	return javRec, nil
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
	studio := models.JavStudio{Name: name}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&studio).Error; err != nil {
		return nil, fmt.Errorf("ensure studio %q: %w", name, err)
	}
	if studio.ID != 0 {
		return &studio, nil
	}
	if err := tx.Where("name = ?", name).First(&studio).Error; err != nil {
		return nil, fmt.Errorf("load studio %q: %w", name, err)
	}
	return &studio, nil
}

func ensureJavTagsTx(tx *gorm.DB, names []string, provider jav.Provider) ([]models.JavTag, error) {
	unique := normalizeNames(names)
	if len(unique) == 0 {
		return nil, nil
	}
	provider = normalizeJavTagProvider(provider)
	var tags []models.JavTag
	for _, name := range unique {
		tag := models.JavTag{Name: name, Provider: int(provider)}
		if err := tx.Where("name = ? AND provider = ?", name, int(provider)).FirstOrCreate(&tag).Error; err != nil {
			return nil, fmt.Errorf("ensure jav tag %q: %w", name, err)
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func replaceJavTagsForProviderTx(tx *gorm.DB, javID int64, tags []models.JavTag, provider jav.Provider) error {
	if javID == 0 {
		return errors.New("jav id cannot be zero")
	}
	provider = normalizeJavTagProvider(provider)
	if err := tx.
		Where("jav_id = ? AND jav_tag_id IN (SELECT id FROM jav_tag WHERE provider = ?)", javID, int(provider)).
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
		rows = append(rows, models.JavTagMap{JavID: javID, JavTagID: tag.ID, CreatedAt: now})
	}
	if len(rows) == 0 {
		return nil
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
		return fmt.Errorf("insert jav tag maps for provider: %w", err)
	}
	return nil
}

func isUserJavTagProvider(provider int) bool {
	return jav.ParseProvider(provider) == jav.ProviderUser
}

func appendJavIdolsForProviderLanguageTx(tx *gorm.DB, javRec *models.Jav, names []string, provider jav.Provider) error {
	if javRec == nil || javRec.ID == 0 {
		return errors.New("jav record is missing")
	}

	isEnglish := jav.ProviderIsEnglish(provider)
	var existingCount int64
	if err := tx.Model(&models.JavIdolMap{}).
		Joins("JOIN jav_idol ji ON ji.id = jav_idol_map.jav_idol_id").
		Where("jav_idol_map.jav_id = ?", javRec.ID).
		Where("COALESCE(ji.is_english, 0) = ?", isEnglish).
		Count(&existingCount).Error; err != nil {
		return fmt.Errorf("count jav idol maps: %w", err)
	}
	if existingCount > 0 {
		return nil
	}

	idols, err := ensureJavIdolsTx(tx, names, isEnglish)
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

func ensureJavIdolsTx(tx *gorm.DB, names []string, isEnglish bool) ([]models.JavIdol, error) {
	unique := normalizeNames(names)
	if len(unique) == 0 {
		return nil, nil
	}
	var idols []models.JavIdol
	for _, name := range unique {
		idol := models.JavIdol{Name: name, IsEnglish: isEnglish}
		if err := tx.Where("name = ? AND is_english = ?", name, isEnglish).FirstOrCreate(&idol).Error; err != nil {
			return nil, fmt.Errorf("ensure jav idol %q: %w", name, err)
		}
		idols = append(idols, idol)
	}
	return idols, nil
}

func setVideoLocationJavIDTx(tx *gorm.DB, locationID, javID int64, expectedUpdatedAt time.Time) error {
	if tx == nil {
		return errors.New("tx is nil")
	}
	q := tx.Model(&models.VideoLocation{}).Where("id = ?", locationID)
	if !expectedUpdatedAt.IsZero() {
		q = q.Where("updated_at = ?", expectedUpdatedAt)
	}
	res := q.Update("jav_id", javID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 && !expectedUpdatedAt.IsZero() {
		ok, err := videoLocationHasJavIDTx(tx, locationID, javID)
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

func videoLocationHasJavIDTx(tx *gorm.DB, locationID, javID int64) (bool, error) {
	if tx == nil {
		return false, errors.New("tx is nil")
	}
	var loc models.VideoLocation
	err := tx.Select("id", "jav_id").Where("id = ?", locationID).First(&loc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("get video location jav id: %w", err)
	}
	return loc.JavID != nil && *loc.JavID == javID, nil
}
