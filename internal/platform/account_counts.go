package platform

// startupAccountCountResolver returns per-platform saved account totals for UI skeleton hints.
// When nil, GetStartup omits disk-backed counts.
var startupAccountCountResolver func(platformNames []string, statsEnabled bool) map[string]int

// SetStartupAccountCountResolver wires startup account totals from basic/steam (registered from main).
func SetStartupAccountCountResolver(fn func(platformNames []string, statsEnabled bool) map[string]int) {
	startupAccountCountResolver = fn
}

func resolveStartupAccountCounts(platformNames []string, statsEnabled bool) map[string]int {
	if startupAccountCountResolver == nil {
		return map[string]int{}
	}
	return startupAccountCountResolver(platformNames, statsEnabled)
}
