package common

import "sync/atomic"

var applicationConfig atomic.Pointer[Config]

// SetConfig installs a copy of startup configuration and returns the previous snapshot.
// Configure paths before starting consumers; serial tests may restore the previous value.
func SetConfig(cfg *Config) *Config {
	if cfg != nil {
		copy := *cfg
		cfg = &copy
	}
	if previous := applicationConfig.Swap(cfg); previous != nil {
		copy := *previous
		return &copy
	}
	return nil
}

// CurrentConfig returns a copy of the configured application paths, or nil before setup.
func CurrentConfig() *Config {
	if cfg := applicationConfig.Load(); cfg != nil {
		copy := *cfg
		return &copy
	}
	return nil
}
