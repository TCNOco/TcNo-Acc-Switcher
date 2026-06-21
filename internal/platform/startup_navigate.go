package platform

import (
	"sync/atomic"
)

var startupNavigateJSON atomic.Value // string

// SetStartupNavigateHint stores a one-shot JSON route for the SPA (from CLI open-page).
func SetStartupNavigateHint(jsonRoute string) {
	startupNavigateJSON.Store(jsonRoute)
}

// ConsumeStartupNavigateHint returns and clears the CLI-provided startup route, if any.
func ConsumeStartupNavigateHint() string {
	v := startupNavigateJSON.Swap("")
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
