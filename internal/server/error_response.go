package server

import "github.com/gin-gonic/gin"

// respondLocalizedError returns both supported UI languages so clients can
// select the message that matches their environment.
func respondLocalizedError(c *gin.Context, status int, messageZH, messageEN string) {
	c.JSON(status, gin.H{
		"error_zh": messageZH,
		"error_en": messageEN,
	})
}

func abortLocalizedError(c *gin.Context, status int, messageZH, messageEN string) {
	c.Abort()
	respondLocalizedError(c, status, messageZH, messageEN)
}
