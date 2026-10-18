package directory

import "sync"

type Options struct {
	CoverDir string
}

// Manager coordinates directory tasks and owns their shared progress and cancellation state.
type Manager struct {
	coverDir string

	dirScanMu                 sync.Mutex
	dirScanActive             map[int64]*directoryScanSession
	directoryProcessingMu     sync.Mutex
	directoryProcessingStatus map[int64]string
}

func New(options Options) *Manager {
	return &Manager{
		coverDir:                  options.CoverDir,
		dirScanActive:             make(map[int64]*directoryScanSession),
		directoryProcessingStatus: make(map[int64]string),
	}
}
