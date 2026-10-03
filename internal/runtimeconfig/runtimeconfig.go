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

// ContainerMode reports whether JavBoss is running in a container-oriented mode.
func ContainerMode() bool {
	return envBool("JAVBOSS_CONTAINER")
}

func HostPathPrefixEnabled() bool {
	return envBool("JAVBOSS_HOST_PATH_PREFIX")
}

func ProxyHostGatewayEnabled() bool {
	return envBool("JAVBOSS_PROXY_HOST_GATEWAY")
}
