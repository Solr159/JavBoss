package translation

import (
	"context"
	dbpkg "javboss/internal/db"
	"strconv"
	"strings"
)

type Domain struct {
	Series bool
}

func (d Domain) Get(ctx context.Context, id int64) (*dbpkg.JavTextTranslationItem, error) {
	return dbpkg.Default().TextTranslations(d.Series).Get(ctx, id)
}

func (d Domain) List(ctx context.Context) ([]dbpkg.JavTextTranslationItem, error) {
	return dbpkg.Default().TextTranslations(d.Series).List(ctx)
}

func (d Domain) Save(ctx context.Context, item *dbpkg.JavTextTranslationItem, text string) (bool, error) {
	return dbpkg.Default().TextTranslations(d.Series).Save(ctx, item, text)
}

func (d Domain) EnabledKey() string {
	if d.Series {
		return "jav_series_translation_enabled"
	}
	return "jav_title_translation_enabled"
}

func (d Domain) Enabled(cfg map[string]string) bool {
	enabled, _ := strconv.ParseBool(cfg[d.EnabledKey()])
	return enabled
}

func (d Domain) AutoKey() string {
	return strings.TrimSuffix(d.EnabledKey(), "enabled") + "auto_enabled"
}

func (d Domain) Acquire(ctx context.Context, id int64) (func(), error) {
	// Positive keys identify titles; negative keys identify series with the same ID.
	if d.Series {
		id = -id
	}
	return Acquire(ctx, id)
}
