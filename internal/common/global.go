package common

import (
	"javboss/internal/manager"

	"gorm.io/gorm"
)

// Shared application-wide dependencies.
var (
	DB                *gorm.DB
	ScreenshotManager *manager.ScreenshotManager
	CoverManager      *manager.CoverManager
	StreamManager     *manager.StreamManager
	AppConfig         *Config
)
