package api

import (
	"strings"

	dbpkg "javboss/internal/db"
	"javboss/internal/jav/translation"

	"github.com/gin-gonic/gin"
)

type textTranslationDomain struct{ translation.Domain }

var javTitleDomain = textTranslationDomain{}
var javSeriesDomain = textTranslationDomain{Domain: translation.Domain{Series: true}}

func (d textTranslationDomain) response(item *dbpkg.JavTextTranslationItem) gin.H {
	if d.Series {
		return gin.H{"id": item.ID, "name": item.Source, "zh_name": item.Translation}
	}
	if d.Language == "en" {
		return gin.H{"id": item.ID, "title": item.Source, "en_title": item.Translation}
	}
	return gin.H{"id": item.ID, "title": item.Source, "zh_title": item.Translation}
}

// withConfig copies the domain with the configured title translation language.
// The shared package-level domains are never mutated.
func (d textTranslationDomain) withConfig(cfg map[string]string) textTranslationDomain {
	if !d.Series {
		d.Language = translation.TargetLanguage(cfg)
	}
	return d
}

func (d textTranslationDomain) error(c *gin.Context, status int, zh, en string) {
	if d.Series {
		zh = strings.NewReplacer("影片", "系列", "标题", "名称").Replace(zh)
		en = strings.NewReplacer("JAV item", "Series", "JAV IDs", "series IDs", "JAV ID", "series ID", "JAV titles", "series names", "title", "name", "Title", "Name").Replace(en)
	}
	respondLocalizedError(c, status, zh, en)
}
