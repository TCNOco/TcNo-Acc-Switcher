package platform

import "sync"

var (
	discordRefreshMu   sync.RWMutex
	discordRefreshHook func()
)

func SetDiscordPresenceRefreshHook(fn func()) {
	discordRefreshMu.Lock()
	discordRefreshHook = fn
	discordRefreshMu.Unlock()
}

func TriggerDiscordPresenceRefresh() {
	discordRefreshMu.RLock()
	fn := discordRefreshHook
	discordRefreshMu.RUnlock()
	if fn != nil {
		fn()
	}
}
