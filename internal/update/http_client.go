//go:build !updatetest

package update

import (
	"net/http"

	"javboss/internal/util"
)

func newUpdateHTTPClient() *http.Client { return util.NewHTTPClient(0) }
