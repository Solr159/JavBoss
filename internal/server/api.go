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
	router.DELETE("/videos/:id/screenshots/:name", deleteVideoScreenshot)
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
	router.GET("/jav/tags", listJavTags)
	router.GET("/jav/studios", listJavStudios)
	router.POST("/jav/tags", createJavTag)
	router.PATCH("/jav/tags/:id", renameJavTag)
	router.DELETE("/jav/tags/:id", deleteJavTag)
	router.POST("/jav/tags/batch_delete", deleteJavTagsBatch)
	router.POST("/jav/tags/add", addJavTagsToItems)
	router.POST("/jav/tags/remove", removeJavTagsFromItems)
	router.POST("/jav/tags/replace", replaceJavTagsForItems)
	router.GET("/jav/:code/cover", getJavCover)
	router.GET("/jav/idols", listJavIdols)
	router.GET("/jav/idols/:id", getJavIdol)
}

func handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
