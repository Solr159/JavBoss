package runtimeconfig

import (
	"os"
	"strings"
)

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// BuildMode reports the current build mode ("development" or "release").
// It falls back to "development" when JAVBOSS_BUILD_MODE is unset.
func BuildMode() string {
	mode := strings.TrimSpace(os.Getenv("JAVBOSS_BUILD_MODE"))
	if mode == "" {
		return "development"
	}
	return mode
}

// DevelopmentMode reports whether JavBoss is running outside of a release build.
func DevelopmentMode() bool {
	return BuildMode() != "release"
}

// ContainerMode reports whether JavBoss is running in a container-oriented mode.
func ContainerMode() bool {
	return envBool("JAVBOSS_CONTAINER") || envBool("JAVBOSS_DOCKER")
}

func DisableDirectoryPicker() bool {
	return envBool("JAVBOSS_DISABLE_DIRECTORY_PICKER")
}

func DisableDesktopIntegration() bool {
	return ContainerMode() || envBool("JAVBOSS_DISABLE_DESKTOP_INTEGRATION")
}

func DisableMPVPlayback() bool {
	return ContainerMode() || envBool("JAVBOSS_DISABLE_MPV")
}

func UseFFmpegScreenshots() bool {
	return ContainerMode() || envBool("JAVBOSS_USE_FFMPEG_SCREENSHOTS")
}

func HostPathPrefixEnabled() bool {
	return envBool("JAVBOSS_HOST_PATH_PREFIX")
}

func ProxyHostGatewayEnabled() bool {
	return envBool("JAVBOSS_PROXY_HOST_GATEWAY")
}
