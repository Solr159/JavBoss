package scrape

import (
	"javboss/internal/jav/scrape/metadata"
)

var (
	ErrNotFound             = metadata.ErrNotFound
	ErrUnsupportedProvider  = metadata.ErrUnsupportedProvider
	ErrUnsupportedOperation = metadata.ErrUnsupportedOperation
)

// ResourceNotFonud is retained for source compatibility.
// Deprecated: use ErrNotFound.
var ResourceNotFonud = ErrNotFound
