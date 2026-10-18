package directory

import (
	"context"
	"sync"
)

type Options struct {
	CoverDir string
}

// Manager coordinates directory tasks and owns their shared progress and cancellation state.
type Manager struct {
	coverDir string
	ctx      context.Context

	dirScanMu                 sync.Mutex
	dirScanActive             map[int64]*directoryScanSession
	directoryProcessingMu     sync.Mutex
	directoryProcessingStatus map[int64]string
}

func New(options Options) *Manager {
	return &Manager{
		ctx:                       context.Background(),
		coverDir:                  options.CoverDir,
		dirScanActive:             make(map[int64]*directoryScanSession),
		directoryProcessingStatus: make(map[int64]string),
	}
}
