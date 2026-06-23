package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"javboss/internal/common"
	"javboss/internal/jav"
	"javboss/internal/models"

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
	Title       *string
	StudioID    *int64
	SeriesID    *int64
	IdolIDs     *[]int64
	UserTagIDs  *[]int64
	ReleaseUnix *int64
	DurationMin *int
}

// JavMetadataScanItem contains a JAV row that needs studio or series metadata.
type JavMetadataScanItem struct {
	ID         int64  `gorm:"column:id"`
	Code       string `gorm:"column:code"`
	TitleEn    string `gorm:"column:title_en"`
	StudioID   *int64 `gorm:"column:studio_id"`
	SeriesID   *int64 `gorm:"column:series_id"`
	SeriesEnID *int64 `gorm:"column:series_en_id"`
}

// GetJav returns one JAV record with display-language associations and visible files/tags.
func GetJav(ctx context.Context, javID int64, directoryIDs []int64) (*models.Jav, error) {
	if javID <= 0 {
		return nil, errors.New("jav id must be positive")
	}

	var item models.Jav
	query := common.DB.WithContext(ctx).
		Preload("Studio").
		Preload("Idols", "COALESCE(is_english, 0) = ?", jav.CurrentMetadataLanguageIsEnglish()).
		Where("id = ?", javID)
	if jav.CurrentMetadataLanguageIsEnglish() {
		query = query.Preload("SeriesEn")
	} else {
		query = query.Preload("Series")
	}
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

	studioID, seriesID, soloOnly, favoriteGroupID := javFilterOptions(filterIDs)
	filtered := buildJavFilter(ctx, idolIDs, tagIDs, search, directoryIDs, studioID, seriesID, soloOnly, favoriteGroupID)

	// Count on a cloned query to avoid mutating the main one.
	countBase := buildJavFilter(ctx, idolIDs, tagIDs, search, directoryIDs, studioID, seriesID, soloOnly, favoriteGroupID)
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
	query := filtered.
		Preload("Studio").
		Preload("Idols", "COALESCE(is_english, 0) = ?", jav.CurrentMetadataLanguageIsEnglish()).
		Limit(limit).
		Offset(offset)
	if jav.CurrentMetadataLanguageIsEnglish() {
		query = query.Preload("SeriesEn")
	} else {
		query = query.Preload("Series")
	}
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
			ID:       r.ID,
			Name:     r.Name,
			Provider: r.Provider,
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

// UpdateJav applies user edits to one JAV record in the current metadata language.
func UpdateJav(ctx context.Context, javID int64, input JavUpdateInput, directoryIDs []int64) (*models.Jav, error) {
	if javID <= 0 {
		return nil, errors.New("jav id must be positive")
	}
	isEnglish := jav.CurrentMetadataLanguageIsEnglish()
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var javRec models.Jav
		if err := tx.Select("id", "studio_id").Where("id = ?", javID).First(&javRec).Error; err != nil {
			return fmt.Errorf("find jav: %w", err)
		}

		updates := map[string]any{}
		if input.Title != nil {
			titleColumn := "title"
			if isEnglish {
				titleColumn = "title_en"
			}
			updates[titleColumn] = strings.TrimSpace(*input.Title)
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
			column := "series_id"
			if isEnglish {
				column = "series_en_id"
			}
			if seriesID <= 0 {
				updates[column] = nil
			} else {
				var series models.JavSeries
				if err := tx.Select("id").Where("id = ? AND COALESCE(is_english, 0) = ?", seriesID, isEnglish).First(&series).Error; err != nil {
					return fmt.Errorf("find jav series: %w", err)
				}
				updates[column] = series.ID
			}
		}
		if len(updates) > 0 {
			if err := tx.Model(&models.Jav{}).Where("id = ?", javID).Updates(updates).Error; err != nil {
				return fmt.Errorf("update jav metadata: %w", err)
			}
		}
		if input.IdolIDs != nil {
			if err := replaceJavIdolsForLanguageTx(tx, javID, *input.IdolIDs, isEnglish); err != nil {
				return err
			}
		}
		if input.UserTagIDs != nil {
			if err := replaceJavUserTagsTx(tx, []int64{javID}, *input.UserTagIDs); err != nil {
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
	scrapedTags, err := listJavTagsForProviders(ctx, directoryIDs, visibleScrapedJavTagProviders(), int(jav.PreferredProvider()))
	if err != nil {
		return nil, err
	}
	userTags, err := listJavTagsForProviders(ctx, directoryIDs, []int{int(jav.ProviderUser)}, int(jav.ProviderUser))
	if err != nil {
		return nil, err
	}
	tags := append(scrapedTags, userTags...)
	return tags, nil
}

func listJavTagsForProviders(ctx context.Context, directoryIDs []int64, providers []int, outputProvider int) ([]JavTagCount, error) {
	if len(providers) == 0 {
		return nil, nil
	}
	var tags []JavTagCount
	activeLocationSQL := activeLocationWhereSQL("vl", "d") + directoryFilterSQL("vl", directoryIDs)
	isUser := outputProvider == int(jav.ProviderUser)
	tagMapJoin := "JOIN jav_tag_map jtm ON jtm.jav_tag_id = jt.id AND jtm.provider IN ?"
	if isUser {
		tagMapJoin = "LEFT JOIN jav_tag_map jtm ON jtm.jav_tag_id = jt.id AND jtm.provider IN ?"
	}
	if err := common.DB.WithContext(ctx).
		Table("jav_tag jt").
		Select("jt.id, jt.name, ? AS provider, COUNT(DISTINCT CASE WHEN "+activeLocationSQL+" THEN jtm.jav_id END) AS count", outputProvider).
		Joins(tagMapJoin, providers).
		Joins("LEFT JOIN video_location vl ON vl.jav_id = jtm.jav_id").
		Joins("LEFT JOIN directory d ON d.id = vl.directory_id").
		Where("COALESCE(jt.is_user, 0) = ?", isUser).
		Group("jt.id, jt.name").
		Order("jt.name").
		Scan(&tags).Error; err != nil {
		return nil, fmt.Errorf("list jav tags: %w", err)
	}
	return tags, nil
}

func visibleScrapedJavTagProviders() []int {
	currentLanguage := jav.CurrentMetadataLanguage()
	candidates := []jav.Provider{
		jav.ProviderJavBus,
		jav.ProviderJavDB,
		jav.ProviderAvmoo,
		jav.ProviderAvsox,
		jav.ProviderJavDatabase,
		jav.ProviderThePornDB,
	}
	providers := make([]int, 0, len(candidates)+1)
	for _, provider := range candidates {
		if jav.ProviderMetadataLanguage(provider) == currentLanguage {
			providers = append(providers, int(provider))
		}
	}
	return providers
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

func buildJavFilter(ctx context.Context, idolIDs []int64, tagIDs []int64, search string, directoryIDs []int64, studioID, seriesID int64, soloOnly bool, favoriteGroupID int64) *gorm.DB {
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
	if seriesID > 0 {
		q = q.Where(javSeriesColumn()+" = ?", seriesID)
	}
	if favoriteGroupID > 0 {
		q = q.Joins("JOIN jav_favorite_map jfm_filter ON jfm_filter.entity_id = jav.id AND jfm_filter.entity_type = ? AND jfm_filter.jav_favorite_group_id = ?", JavFavoriteEntityJav, favoriteGroupID)
	}
	if soloOnly {
		soloJavs := common.DB.WithContext(ctx).
			Table("jav_idol_map jim_solo_count").
			Select("jim_solo_count.jav_id").
			Joins("JOIN jav_idol ji_solo_count ON ji_solo_count.id = jim_solo_count.jav_idol_id").
			Where("COALESCE(ji_solo_count.is_english, 0) = ?", jav.CurrentMetadataLanguageIsEnglish()).
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

func javFilterOptions(values []int64) (int64, int64, bool, int64) {
	studioID := int64(0)
	seriesID := int64(0)
	soloOnly := false
	favoriteGroupID := int64(0)
	if len(values) > 0 && values[0] > 0 {
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

func javSeriesColumn() string {
	if jav.CurrentMetadataLanguageIsEnglish() {
		return "series_en_id"
	}
	return "series_id"
}

// JavStudioSummary represents studio info with aggregated work count and a sample code for cover lookup.
type JavStudioSummary struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	WorkCount     int64  `json:"work_count"`
	SampleCode    string `json:"sample_code"`
	FavoriteCount int64  `json:"favorite_count"`
}

// JavSeriesSummary represents series info with aggregated work count and a sample code for cover lookup.
type JavSeriesSummary struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	IsEnglish     bool   `json:"is_english"`
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
	return q.Where("js.name LIKE ?", like)
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
	return &item, nil
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
	isEnglish := jav.CurrentMetadataLanguageIsEnglish()
	seriesColumn := javSeriesColumn()

	countBase := common.DB.WithContext(ctx).
		Table("jav_series js").
		Joins("JOIN jav j ON j."+seriesColumn+" = js.id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where("COALESCE(js.is_english, 0) = ?", isEnglish).
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
		Joins("JOIN jav j ON j."+seriesColumn+" = js.id").
		Joins("LEFT JOIN jav_studio jst ON jst.id = js.studio_id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where("COALESCE(js.is_english, 0) = ?", isEnglish).
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
		Select("js.id, js.name, js.is_english, js.studio_id, jst.name AS studio_name, COUNT(DISTINCT j.id) AS work_count, MIN(j.code) AS sample_code, COALESCE(favorite_counts.favorite_count, 0) AS favorite_count").
		Group("js.id, js.name, js.is_english, js.studio_id, jst.name, favorite_counts.favorite_count").
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

	isEnglish := jav.CurrentMetadataLanguageIsEnglish()
	seriesColumn := javSeriesColumn()
	var item JavSeriesSummary
	query := common.DB.WithContext(ctx).
		Table("jav_series js").
		Joins("JOIN jav j ON j."+seriesColumn+" = js.id").
		Joins("LEFT JOIN jav_studio jst ON jst.id = js.studio_id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where("js.id = ?", seriesID).
		Where("COALESCE(js.is_english, 0) = ?", isEnglish).
		Where(activeLocationWhereSQL("vl", "d"))
	query = applyDirectoryFilter(query, "vl", directoryIDs)
	tx := query.
		Joins("LEFT JOIN (?) favorite_counts ON favorite_counts.entity_id = js.id", buildFavoriteCountQuery(ctx, JavFavoriteEntitySeries)).
		Select("js.id, js.name, js.is_english, js.studio_id, jst.name AS studio_name, COUNT(DISTINCT j.id) AS work_count, MIN(j.code) AS sample_code, COALESCE(favorite_counts.favorite_count, 0) AS favorite_count").
		Group("js.id, js.name, js.is_english, js.studio_id, jst.name, favorite_counts.favorite_count").
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
	seriesColumn := javSeriesColumn()
	query := common.DB.WithContext(ctx).
		Table("jav j").
		Select("j.code").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where("j."+seriesColumn+" = ?", seriesID).
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

// JavIdolCoverOption represents one visible JAV work that can be used as an idol card cover.
type JavIdolCoverOption struct {
	ID      int64  `json:"id"`
	Code    string `json:"code"`
	Title   string `json:"title"`
	TitleEn string `json:"title_en"`
	Solo    bool   `json:"solo"`
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

func buildVisibleSoloIdolCoverQuery(ctx context.Context, directoryIDs []int64, language ...bool) *gorm.DB {
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
		Select("jim_solo.jav_idol_id, MIN(j_solo.code) AS cover_code").
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
		Select("ji.id, ji.name, ji.roman_name, ji.japanese_name, ji.chinese_name, ji.height_cm, ji.birth_date, ji.bust, ji.waist, ji.hips, ji.cup, COALESCE(idol_work_counts.work_count, 0) AS work_count, ji.cover_jav_id, COALESCE(NULLIF(cover_jav.code, ''), solo_idols.cover_code) AS cover_code, COALESCE(ji.cover_crop_left, 0.53) AS cover_crop_left, COALESCE(favorite_counts.favorite_count, 0) AS favorite_count").
		Joins("LEFT JOIN (?) idol_work_counts ON idol_work_counts.jav_idol_id = ji.id", buildVisibleIdolWorkCountQuery(ctx, directoryIDs)).
		Joins("LEFT JOIN (?) solo_idols ON solo_idols.jav_idol_id = ji.id", buildVisibleSoloIdolCoverQuery(ctx, directoryIDs, isEnglish)).
		Joins("LEFT JOIN jav cover_jav ON cover_jav.id = ji.cover_jav_id").
		Joins("LEFT JOIN (?) favorite_counts ON favorite_counts.jav_idol_id = ji.id", buildIdolFavoriteCountQuery(ctx)).
		Where("ji.id = ?", idolID).
		Where("COALESCE(ji.is_english, 0) = ?", isEnglish).
		Where("solo_idols.cover_code IS NOT NULL").
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

// ListJavIdolOptions returns all idols in the active metadata language for edit selectors.
func ListJavIdolOptions(ctx context.Context, search string, limit, offset int) ([]JavIdolSummary, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	base := common.DB.WithContext(ctx).
		Table("jav_idol ji").
		Where("COALESCE(ji.is_english, 0) = ?", jav.CurrentMetadataLanguageIsEnglish())
	base = applyJavIdolSearch(base, search)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count jav idol options: %w", err)
	}

	var items []JavIdolSummary
	if err := base.
		Select("ji.id, ji.name, ji.roman_name, ji.japanese_name, ji.chinese_name, ji.height_cm, ji.birth_date, ji.bust, ji.waist, ji.hips, ji.cup, COALESCE(ji.cover_crop_left, 0.53) AS cover_crop_left").
		Order("ji.name ASC, ji.id ASC").
		Limit(limit).
		Offset(offset).
		Scan(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list jav idol options: %w", err)
	}

	return items, total, nil
}

// ListJavIdols returns idols ordered by selected sort with pagination.
func ListJavIdols(ctx context.Context, search, sort string, limit, offset int, directoryIDs []int64, favoriteGroupID int64) ([]JavIdolSummary, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	sort = strings.ToLower(strings.TrimSpace(sort))
	isEnglish := jav.CurrentMetadataLanguageIsEnglish()
	soloIdols := buildVisibleSoloIdolCoverQuery(ctx, directoryIDs, isEnglish)

	countBase := common.DB.WithContext(ctx).
		Table("jav_idol ji").
		Joins("JOIN (?) solo_idols ON solo_idols.jav_idol_id = ji.id", soloIdols).
		Where("COALESCE(ji.is_english, 0) = ?", isEnglish)
	if favoriteGroupID > 0 {
		countBase = countBase.Joins("JOIN jav_favorite_map jifm_filter ON jifm_filter.entity_id = ji.id AND jifm_filter.entity_type = ? AND jifm_filter.jav_favorite_group_id = ?", JavFavoriteEntityIdol, favoriteGroupID)
	}
	countBase = applyJavIdolSearch(countBase, search)

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
	if favoriteGroupID > 0 {
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
		Where("COALESCE(ji.is_english, 0) = ?", isEnglish).
		Where(activeLocationWhereSQL("vl", "d"))
	if favoriteGroupID > 0 {
		base = base.Joins("JOIN jav_favorite_map jifm_filter ON jifm_filter.entity_id = ji.id AND jifm_filter.entity_type = ? AND jifm_filter.jav_favorite_group_id = ?", JavFavoriteEntityIdol, favoriteGroupID)
	}
	base = applyDirectoryFilter(base, "vl", directoryIDs)
	base = applyJavIdolSearch(base, search)
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

// ListIdolCoverOptions returns visible works that can be selected as an idol cover.
func ListIdolCoverOptions(ctx context.Context, idolID int64, directoryIDs []int64) ([]JavIdolCoverOption, error) {
	if idolID <= 0 {
		return nil, errors.New("idol id must be positive")
	}

	sub := common.DB.WithContext(ctx).
		Table("jav_idol_map jim_count").
		Select("jim_count.jav_id, COUNT(*) as c").
		Joins("JOIN jav_idol ji_count ON ji_count.id = jim_count.jav_idol_id").
		Where("COALESCE(ji_count.is_english, 0) = (SELECT COALESCE(is_english, 0) FROM jav_idol WHERE id = ?)", idolID).
		Group("jim_count.jav_id")

	var rows []struct {
		ID      int64
		Code    string
		Title   string
		TitleEn string
		Solo    int
	}
	query := common.DB.WithContext(ctx).
		Table("jav_idol_map jim").
		Select("j.id, j.code, j.title, j.title_en, CASE WHEN s.c = 1 THEN 1 ELSE 0 END AS solo").
		Joins("JOIN jav j ON j.id = jim.jav_id").
		Joins("JOIN video_location vl ON vl.jav_id = j.id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Joins("LEFT JOIN (?) s ON s.jav_id = jim.jav_id", sub).
		Where("jim.jav_idol_id = ?", idolID).
		Where(activeLocationWhereSQL("vl", "d"))
	query = applyDirectoryFilter(query, "vl", directoryIDs)
	if err := query.
		Group("j.id, j.code, j.title, j.title_en, solo").
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
			ID:      row.ID,
			Code:    code,
			Title:   strings.TrimSpace(row.Title),
			TitleEn: strings.TrimSpace(row.TitleEn),
			Solo:    row.Solo == 1,
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
	soloIdols := buildVisibleSoloIdolCoverQuery(ctx, nil, isEnglish)
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

// ListJavCodes returns all jav codes.
func ListJavCodes(ctx context.Context) ([]string, error) {
	var codes []string
	if err := common.DB.WithContext(ctx).Model(&models.Jav{}).Pluck("code", &codes).Error; err != nil {
		return nil, fmt.Errorf("list jav codes: %w", err)
	}
	return codes, nil
}

// ListJavsMissingMetadata returns JAV rows whose English title, studio, or series relation is empty.
func ListJavsMissingMetadata(ctx context.Context) ([]JavMetadataScanItem, error) {
	var items []JavMetadataScanItem
	if err := common.DB.WithContext(ctx).
		Model(&models.Jav{}).
		Select("id, code, title_en, studio_id, series_id, series_en_id").
		Where("COALESCE(code, '') <> ''").
		Where("COALESCE(title_en, '') = '' OR studio_id IS NULL OR series_id IS NULL OR series_en_id IS NULL").
		Order("created_at ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list javs missing metadata: %w", err)
	}
	return items, nil
}

// ListJavsMissingTitle returns JAV rows whose primary title is empty.
func ListJavsMissingTitle(ctx context.Context) ([]JavMetadataScanItem, error) {
	var items []JavMetadataScanItem
	if err := common.DB.WithContext(ctx).
		Model(&models.Jav{}).
		Select("id, code, title_en, studio_id, series_id, series_en_id").
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
		Joins("JOIN jav_idol ji ON ji.id = jim.jav_idol_id").
		Where("jim.jav_id = jav.id").
		Where("COALESCE(ji.is_english, 0) = ?", jav.ProviderIsEnglish(jav.ProviderAvsox))
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

// UpdateJavSeries records the series lookup result for a JAV row.
func UpdateJavSeries(ctx context.Context, javID int64, series string, isEnglish bool) error {
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
		rec, err := ensureSeriesWithStudioTx(tx, series, isEnglish, javRec.StudioID)
		if err != nil {
			return err
		}
		column := "series_id"
		if isEnglish {
			column = "series_en_id"
		}
		if err := tx.Model(&models.Jav{}).
			Where("id = ?", javID).
			Update(column, rec.ID).Error; err != nil {
			return fmt.Errorf("update jav series: %w", err)
		}
		return nil
	})
}

// AppendJavIdolsIfMissingForProvider appends provider-language idol mappings when none exist yet.
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
	javRec.Code = info.Code
	if jav.ProviderIsEnglish(provider) {
		javRec.TitleEn = info.Title
	} else {
		javRec.Title = info.Title
	}
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
		seriesRec, err := ensureSeriesTx(tx, series, jav.ProviderIsEnglish(provider))
		if err != nil {
			return nil, err
		}
		if jav.ProviderIsEnglish(provider) {
			javRec.SeriesEnID = &seriesRec.ID
		} else {
			javRec.SeriesID = &seriesRec.ID
		}
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

func ensureSeriesTx(tx *gorm.DB, name string, isEnglish bool) (*models.JavSeries, error) {
	return ensureSeriesWithStudioTx(tx, name, isEnglish, nil)
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
	unique := normalizeNames(names)
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

func appendJavIdolsIfMissingForProvider(ctx context.Context, javID int64, names []string, provider jav.Provider) (bool, error) {
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
		isEnglish := jav.ProviderIsEnglish(provider)
		var existingCount int64
		if err := tx.Model(&models.JavIdolMap{}).
			Joins("JOIN jav_idol ji ON ji.id = jav_idol_map.jav_idol_id").
			Where("jav_idol_map.jav_id = ?", javID).
			Where("COALESCE(ji.is_english, 0) = ?", isEnglish).
			Count(&existingCount).Error; err != nil {
			return fmt.Errorf("count jav idol maps: %w", err)
		}
		if existingCount > 0 {
			return nil
		}

		idols, err := ensureJavIdolsTx(tx, unique, isEnglish)
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

func replaceJavIdolsForLanguageTx(tx *gorm.DB, javID int64, idolIDs []int64, isEnglish bool) error {
	if javID <= 0 {
		return errors.New("jav id cannot be zero")
	}
	cleanIDs := uniqueInt64s(idolIDs)
	idolIDsForLanguage := tx.
		Model(&models.JavIdol{}).
		Select("id").
		Where("COALESCE(is_english, 0) = ?", isEnglish)
	if err := tx.
		Where("jav_id = ? AND jav_idol_id IN (?)", javID, idolIDsForLanguage).
		Delete(&models.JavIdolMap{}).Error; err != nil {
		return fmt.Errorf("delete jav idol maps: %w", err)
	}

	if len(cleanIDs) == 0 {
		return nil
	}
	var count int64
	if err := tx.Model(&models.JavIdol{}).
		Where("id IN ?", cleanIDs).
		Where("COALESCE(is_english, 0) = ?", isEnglish).
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
