package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ThumbnailQueue abstracts the ability to enqueue thumbnail generation tasks.
// RegisterRoutes wires handlers onto the provided router.
func RegisterRoutes(router *gin.Engine) {
	router.GET("/healthz", handleHealth)
	router.GET("/config", getConfig)
	router.PATCH("/config", updateConfig)
	router.GET("/videos", listVideos)
	router.GET("/videos/:id", getVideo)
	router.GET("/videos/:id/streams", getVideoStreams)
	router.GET("/videos/:id/stream", streamVideo)
	router.GET("/videos/:id/thumbnail", getThumbnail)
	router.GET("/videos/:id/screenshots", listVideoScreenshots)
	router.GET("/videos/:id/screenshots/:name", getVideoScreenshot)
	router.PATCH("/videos/:id/jav-scrape", updateVideoJavScrapeSettings)
	router.GET("/videos/:id/jav-scrape/possible-codes", getVideoJavScrapePossibleCodes)
	router.GET("/videos/:id/jav-scrape/javdb", lookupVideoJavScrapeJavDB)
	router.POST("/videos/:id/jav-scrape/manual", manualVideoJavScrape)
	router.DELETE("/videos/:id/screenshots/:name", deleteVideoScreenshot)
	router.PATCH("/videos/:id/locations/:location_id", renameVideoLocation)
	router.DELETE("/videos/:id/locations/:location_id", deleteVideoLocation)
	router.POST("/videos/:id/play", incrementVideoPlayCount)
	router.POST("/videos/play", playVideoFile)
	router.POST("/videos/open", openVideoFile)
	router.POST("/videos/reveal", revealVideoLocation)

	router.GET("/directories", listDirectories)
	router.POST("/directories", createDirectory)
	router.POST("/directories/pick", pickDirectory)
	router.PATCH("/directories/:id", updateDirectory)

	router.GET("/tags", listTags)
	router.POST("/tags", createTag)
	router.PATCH("/tags/:id", renameTag)
	router.DELETE("/tags/:id", deleteTag)
	router.POST("/tags/batch_delete", deleteTagsBatch)

	router.POST("/videos/tags/add", addTagsToVideos)
	router.POST("/videos/tags/remove", removeTagsFromVideos)
	router.POST("/videos/tags/replace", replaceTagsForVideos)

	router.GET("/jav", searchJav)
	router.GET("/jav/javdb-url", getJavJavDBURL)
	router.GET("/jav/tags", listJavTags)
	router.GET("/jav/studios", listJavStudios)
	router.GET("/jav/studios/javdb-url", getJavStudioJavDBURL)
	router.GET("/jav/studios/:id", getJavStudio)
	router.GET("/jav/series", listJavSeries)
	router.GET("/jav/series/javdb-url", getJavSeriesJavDBURL)
	router.GET("/jav/series/:id", getJavSeries)
	router.PUT("/jav/items/:id", updateJavItem)
	router.POST("/jav/tags", createJavTag)
	router.PATCH("/jav/tags/:id", renameJavTag)
	router.DELETE("/jav/tags/:id", deleteJavTag)
	router.POST("/jav/tags/batch_delete", deleteJavTagsBatch)
	router.POST("/jav/tags/add", addJavTagsToItems)
	router.POST("/jav/tags/remove", removeJavTagsFromItems)
	router.POST("/jav/tags/replace", replaceJavTagsForItems)
	router.GET("/jav/:code/cover", getJavCover)
	router.PUT("/jav/:code/cover", updateJavCover)
	registerJavFavoriteRoutes(router, "jav", dbFavoriteEntityJav)
	registerJavFavoriteRoutes(router, "idol", dbFavoriteEntityIdol)
	registerJavFavoriteRoutes(router, "studio", dbFavoriteEntityStudio)
	registerJavFavoriteRoutes(router, "series", dbFavoriteEntitySeries)
	router.GET("/jav/idols", listJavIdols)
	router.GET("/jav/idols/options", listJavIdolOptions)
	router.GET("/jav/idols/resolve", resolveJavIdols)
	router.GET("/jav/idols/javdb-url", getJavIdolJavDBURL)
	router.GET("/jav/idols/:id/cover-options", listJavIdolCoverOptions)
	router.PUT("/jav/idols/:id/cover", updateJavIdolCover)
	router.GET("/jav/idols/:id", getJavIdol)
}

const (
	dbFavoriteEntityJav    = "jav"
	dbFavoriteEntityIdol   = "idol"
	dbFavoriteEntityStudio = "studio"
	dbFavoriteEntitySeries = "series"
)

func registerJavFavoriteRoutes(router *gin.Engine, routeEntity string, dbEntity string) {
	base := "/jav/" + routeEntity + "-favorite-groups"
	router.GET(base, listJavFavoriteGroupsFor(dbEntity))
	router.POST(base, createJavFavoriteGroupFor(dbEntity))
	router.PUT(base+"/order", reorderJavFavoriteGroupsFor(dbEntity))
	router.PATCH(base+"/:id", renameJavFavoriteGroupFor(dbEntity))
	router.DELETE(base+"/:id", deleteJavFavoriteGroupFor(dbEntity))
	router.GET(base+"/:id/items", listJavFavoriteGroupItemsFor(dbEntity))
	router.PUT(base+"/:id/item-order", reorderJavFavoriteGroupItemsFor(dbEntity))
	router.POST(base+"/:id/items/remove", removeJavFavoriteGroupItemsFor(dbEntity))
	if routeEntity == "jav" {
		router.GET("/jav/items/:id/favorite-groups", listJavFavoriteGroupIDsFor(dbEntity))
		router.PUT("/jav/items/:id/favorite-groups", replaceJavFavoriteGroupsFor(dbEntity))
		return
	}
	if routeEntity == "series" {
		router.GET("/jav/series/:id/favorite-groups", listJavFavoriteGroupIDsFor(dbEntity))
		router.PUT("/jav/series/:id/favorite-groups", replaceJavFavoriteGroupsFor(dbEntity))
		return
	}
	router.GET("/jav/"+routeEntity+"s/:id/favorite-groups", listJavFavoriteGroupIDsFor(dbEntity))
	router.PUT("/jav/"+routeEntity+"s/:id/favorite-groups", replaceJavFavoriteGroupsFor(dbEntity))
}

func handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
