package winutil

import "sync"

var (
	statusReporterMu sync.RWMutex
	statusReporter   func(key string, vars map[string]string)
)

// SetStatusReporter lets higher-level packages surface long-running native steps.
func SetStatusReporter(fn func(key string, vars map[string]string)) {
	statusReporterMu.Lock()
	statusReporter = fn
	statusReporterMu.Unlock()
}

func emitStatus(key string, vars map[string]string) {
	statusReporterMu.RLock()
	fn := statusReporter
	statusReporterMu.RUnlock()
	if fn != nil {
		fn(key, vars)
	}
}
