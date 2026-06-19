package mpv

import "javboss/internal/util"

// ResolvePath returns the path to the mpv binary.
func ResolvePath() (string, error) {
	return util.ResolveMPVPath()
}
