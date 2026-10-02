package db

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"javboss/internal/common"
	"javboss/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Accept Latin-script names, but not mixed Japanese names or punctuation alone.
func isEnglishStudioName(name string) bool {
	hasLetter := false
	for _, r := range strings.TrimSpace(name) {
		if unicode.IsLetter(r) {
			if !unicode.In(r, unicode.Latin) {
				return false
			}
			hasLetter = true
		}
	}
	return hasLetter
}

// ReconcileJavStudioNames merges names returned for the same movie in provider
// priority order. English names take precedence; all other names become aliases.
// expectedStudioID prevents an in-flight lookup from replacing a manual assignment.
func ReconcileJavStudioNames(ctx context.Context, javID int64, expectedStudioID *int64, names []string) (bool, error) {
	if javID <= 0 {
		return false, fmt.Errorf("jav id must be positive")
	}
	var cleanNames []string
	seenNames := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" && !seenNames[name] {
			seenNames[name] = true
			cleanNames = append(cleanNames, name)
		}
	}
	if len(cleanNames) == 0 {
		return false, nil
	}
	updated := false
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item models.Jav
		if err := tx.Select("id", "studio_id").First(&item, javID).Error; err != nil {
			return fmt.Errorf("load jav for studio reconciliation: %w", err)
		}
		if (item.StudioID == nil) != (expectedStudioID == nil) ||
			(item.StudioID != nil && *item.StudioID != *expectedStudioID) {
			return nil
		}
		var current *models.JavStudio
		var candidates []models.JavStudio
		seenIDs := map[int64]bool{}
		if item.StudioID != nil {
			current = &models.JavStudio{}
			if err := tx.First(current, *item.StudioID).Error; err != nil {
				return fmt.Errorf("load current studio: %w", err)
			}
			// Another scan or manual edit may already have promoted this studio.
			if isEnglishStudioName(current.Name) {
				return nil
			}
			candidates = append(candidates, *current)
			seenIDs[current.ID] = true
		}
		var target *models.JavStudio
		for _, name := range cleanNames {
			studio, err := findJavStudioByNameOrAliasTx(tx, name)
			if err != nil {
				return err
			}
			if studio == nil {
				continue
			}
			if !seenIDs[studio.ID] {
				candidates = append(candidates, *studio)
				seenIDs[studio.ID] = true
			}
			// Preserve an existing English canonical name even when matched by alias.
			if target == nil && isEnglishStudioName(studio.Name) {
				target = studio
			}
		}
		preferred := cleanNames[0]
		for _, name := range cleanNames {
			if isEnglishStudioName(name) {
				preferred = name
				break
			}
		}
		if target == nil {
			if current != nil {
				target = current
			} else if len(candidates) > 0 {
				target = &candidates[0]
			} else {
				var err error
				target, err = ensureStudioTx(tx, preferred)
				if err != nil {
					return err
				}
				updated = true
			}
		}
		var sources []models.JavStudio
		for _, candidate := range candidates {
			if candidate.ID != target.ID {
				sources = append(sources, candidate)
			}
		}
		if len(sources) > 0 {
			if err := mergeJavStudiosTx(tx, *target, sources); err != nil {
				return err
			}
			updated = true
		}
		if !isEnglishStudioName(target.Name) && isEnglishStudioName(preferred) {
			oldName := target.Name
			if err := tx.Model(target).Update("name", preferred).Error; err != nil {
				return fmt.Errorf("promote studio name: %w", err)
			}
			target.Name = preferred
			cleanNames = append(cleanNames, oldName)
			updated = true
		}
		if err := tx.Where("jav_studio_id = ? AND alias = ?", target.ID, target.Name).Delete(&models.JavStudioAlias{}).Error; err != nil {
			return fmt.Errorf("remove canonical studio alias: %w", err)
		}
		for _, name := range cleanNames {
			if name == "" || name == target.Name {
				continue
			}
			alias := models.JavStudioAlias{JavStudioID: target.ID, Alias: name}
			res := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "alias"}}, DoNothing: true}).Create(&alias)
			if res.Error != nil {
				return fmt.Errorf("save studio alias: %w", res.Error)
			}
			updated = updated || res.RowsAffected > 0
		}
		if item.StudioID == nil {
			if err := tx.Model(&models.Jav{}).Where("id = ?", item.ID).Update("studio_id", target.ID).Error; err != nil {
				return fmt.Errorf("set reconciled studio: %w", err)
			}
			updated = true
		}
		return nil
	})
	return updated, err
}
