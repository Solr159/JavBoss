package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"javboss/internal/common/logging"
)

// NewRouter constructs a gin router with API routes and optional static file serving.
func NewRouter(staticDir string, apiToken string) *gin.Engine {
	router := gin.New()
	router.Use(ginLogger(), gin.Recovery())
	if apiToken != "" {
		router.Use(apiTokenMiddleware(apiToken))
	}

	RegisterRoutes(router)

	if staticDir != "" {
		if fi, err := os.Stat(staticDir); err == nil && fi.IsDir() {
			router.Static("/assets", filepath.Join(staticDir, "assets"))
			router.Static("/ico", filepath.Join(staticDir, "ico"))
			router.StaticFile("/", filepath.Join(staticDir, "index.html"))

			router.NoRoute(func(c *gin.Context) {
				path := c.Request.URL.Path
				if strings.HasPrefix(path, "/videos") || strings.HasPrefix(path, "/tags") || strings.HasPrefix(path, "/sync") || strings.HasPrefix(path, "/healthz") {
					c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
					return
				}
				if strings.Contains(c.GetHeader("Accept"), "text/html") {
					c.File(filepath.Join(staticDir, "index.html"))
					return
				}
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			})

			logging.Info("serving frontend from %s", staticDir)
		} else if err == nil {
			logging.Error("static path %s is not a directory; frontend serving disabled", staticDir)
		} else if !errors.Is(err, os.ErrNotExist) {
			logging.Error("static path check error: %v", err)
		}
	}

	return router
}

func ginLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logging.Info("%s %s %d %dB %s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), c.Writer.Size(), time.Since(start))
	}
}
