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

// Accept Latin-script names from the English metadata source, not mixed Japanese
// names or punctuation-only placeholders. This does not translate studio names.
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

// PromoteJavStudioEnglishName preserves the original name as an alias. Existing
// English canonical names remain stable across subsequent scans and providers.
func PromoteJavStudioEnglishName(ctx context.Context, javID int64, name string) (bool, error) {
	if javID <= 0 {
		return false, fmt.Errorf("jav id must be positive")
	}
	name = strings.TrimSpace(name)
	if !isEnglishStudioName(name) {
		return false, nil
	}
	updated := false
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item models.Jav
		if err := tx.Select("id", "studio_id").First(&item, javID).Error; err != nil {
			return fmt.Errorf("load jav for English studio: %w", err)
		}
		var current *models.JavStudio
		if item.StudioID != nil {
			current = &models.JavStudio{}
			if err := tx.First(current, *item.StudioID).Error; err != nil {
				return fmt.Errorf("load current studio: %w", err)
			}
			if isEnglishStudioName(current.Name) {
				return nil
			}
		}
		target, err := findJavStudioByNameOrAliasTx(tx, name)
		if err != nil {
			return err
		}
		if target == nil {
			if current != nil {
				target = current
			} else {
				target, err = ensureStudioTx(tx, name)
				if err != nil {
					return err
				}
			}
		}
		if !isEnglishStudioName(target.Name) {
			old := *target
			// The requested English name can already be an alias of this studio.
			if err := tx.Where("jav_studio_id = ? AND alias = ?", target.ID, name).Delete(&models.JavStudioAlias{}).Error; err != nil {
				return fmt.Errorf("remove promoted studio alias: %w", err)
			}
			if err := tx.Model(target).Update("name", name).Error; err != nil {
				return fmt.Errorf("promote studio name: %w", err)
			}
			target.Name = name
			alias := models.JavStudioAlias{JavStudioID: target.ID, Alias: old.Name}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "alias"}}, DoNothing: true}).Create(&alias).Error; err != nil {
				return fmt.Errorf("preserve original studio name: %w", err)
			}
		}
		if current != nil && current.ID != target.ID {
			if err := mergeJavStudiosTx(tx, *target, []models.JavStudio{*current}); err != nil {
				return err
			}
		}
		if item.StudioID == nil {
			if err := tx.Model(&models.Jav{}).Where("id = ?", item.ID).Update("studio_id", target.ID).Error; err != nil {
				return fmt.Errorf("set English studio: %w", err)
			}
		}
		updated = true
		return nil
	})
	return updated, err
}
