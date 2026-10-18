package metadata

import (
	"errors"
)

var (
	ErrNotFound             = errors.New("jav: resource not found")
	ErrUnsupportedProvider  = errors.New("jav: unsupported provider")
	ErrUnsupportedOperation = errors.New("jav: unsupported operation")
)
