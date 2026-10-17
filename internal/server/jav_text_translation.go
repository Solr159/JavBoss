package server

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	dbpkg "javboss/internal/db"
)

type textTranslationDomain struct {
	dbpkg.JavTextTranslationStore
}

var javTitleDomain = textTranslationDomain{}
var javSeriesDomain = textTranslationDomain{JavTextTranslationStore: dbpkg.JavTextTranslationStore{Series: true}}

func (d textTranslationDomain) enabledKey() string {
	if d.Series {
		return "jav_series_translation_enabled"
	}
	return "jav_title_translation_enabled"
}

func (d textTranslationDomain) acquire(ctx context.Context, id int64) (func(), error) {
	// Positive keys identify titles; negative keys identify series with the same ID.
	if d.Series {
		id = -id
	}
	return acquireTitleTranslation(ctx, id)
}

func (d textTranslationDomain) response(item *dbpkg.JavTextTranslationItem) gin.H {
	if d.Series {
		return gin.H{"id": item.ID, "name": item.Source, "zh_name": item.Translation}
	}
	return gin.H{"id": item.ID, "title": item.Source, "zh_title": item.Translation}
}

func (d textTranslationDomain) error(c *gin.Context, status int, zh, en string) {
	if d.Series {
		zh = strings.NewReplacer("影片", "系列", "标题", "名称").Replace(zh)
		en = strings.NewReplacer("JAV item", "Series", "JAV IDs", "series IDs", "JAV ID", "series ID", "JAV titles", "series names", "title", "name", "Title", "Name").Replace(en)
	}
	respondLocalizedError(c, status, zh, en)
}
