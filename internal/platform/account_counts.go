package platform

// startupCountResolver returns per-platform saved account totals and tag totals
// for UI skeleton hints.
//
// Accounts and tags come back from one hook rather than two so the resolver can
// answer both from a single read of each platform's ids.json. When nil,
// GetStartup omits disk-backed counts.
var startupCountResolver func(platformNames []string, statsEnabled bool) (map[string]int, map[string]PlatformTagCountInfo)

// SetStartupCountResolver wires startup account and tag totals from basic/steam (registered from main).
func SetStartupCountResolver(fn func(platformNames []string, statsEnabled bool) (map[string]int, map[string]PlatformTagCountInfo)) {
	startupCountResolver = fn
}

func resolveStartupCounts(platformNames []string, statsEnabled bool) (map[string]int, map[string]PlatformTagCountInfo) {
	if startupCountResolver == nil {
		return map[string]int{}, map[string]PlatformTagCountInfo{}
	}
	return startupCountResolver(platformNames, statsEnabled)
}
