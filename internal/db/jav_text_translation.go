package db

import (
	"context"
	"fmt"

	"javboss/internal/models"

	"gorm.io/gorm"
)

// JavTextTranslationItem keeps source text separate from its saved translation.
type JavTextTranslationItem struct {
	ID          int64
	Source      string
	Translation string
}

// JavTextTranslationStore selects one of the two supported translation domains.
// Column names are internal constants, never request input.
type JavTextTranslationStore struct {
	store  *Store
	Series bool
}

func (s JavTextTranslationStore) columns() (string, string) {
	if s.Series {
		return "name", "zh_name"
	}
	return "title", "zh_title"
}

func (s JavTextTranslationStore) query(ctx context.Context) *gorm.DB {
	if s.Series {
		return s.store.conn.WithContext(ctx).Model(&models.JavSeries{}).Where("is_english = ?", false)
	}
	return s.store.conn.WithContext(ctx).Model(&models.Jav{})
}

func (s JavTextTranslationStore) selectQuery(ctx context.Context) *gorm.DB {
	source, translated := s.columns()
	return s.query(ctx).Select("id, " + source + " AS source, " + translated + " AS translation")
}

func (s JavTextTranslationStore) Get(ctx context.Context, id int64) (*JavTextTranslationItem, error) {
	var item JavTextTranslationItem
	if err := s.selectQuery(ctx).Where("id = ?", id).Take(&item).Error; err != nil {
		return nil, fmt.Errorf("get translation text: %w", err)
	}
	return &item, nil
}

func (s JavTextTranslationStore) List(ctx context.Context) ([]JavTextTranslationItem, error) {
	var items []JavTextTranslationItem
	if err := s.selectQuery(ctx).Order("id").Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("list translation texts: %w", err)
	}
	return items, nil
}

// Save rejects stale results if either the source or translation changed in flight.
func (s JavTextTranslationStore) Save(ctx context.Context, item *JavTextTranslationItem, text string) (bool, error) {
	source, translated := s.columns()
	result := s.query(ctx).
		Where("id = ? AND "+source+" = ? AND "+translated+" = ?", item.ID, item.Source, item.Translation).
		Update(translated, text)
	if result.Error != nil {
		return false, fmt.Errorf("save translation text: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// TextTranslations selects the title or series translation store.
func (s *Store) TextTranslations(series bool) JavTextTranslationStore {
	return JavTextTranslationStore{store: s, Series: series}
}
