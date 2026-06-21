package basic

import "strings"

type liveAccountIDResolver func(platformKey string) (string, error)

var resolveLiveAccountID liveAccountIDResolver

// SetLiveAccountIDResolver configures how the current signed-in account is resolved per platform (avoids basic<->steam import cycle).
func SetLiveAccountIDResolver(fn liveAccountIDResolver) {
	resolveLiveAccountID = fn
}

func currentLiveAccountID(b *BasicService, platformKey string) string {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return ""
	}
	if resolveLiveAccountID != nil {
		if id, err := resolveLiveAccountID(platformKey); err == nil {
			return strings.TrimSpace(id)
		}
	}
	if b != nil {
		if id, err := CurrentLiveUniqueID(b.deps(), platformKey); err == nil {
			return strings.TrimSpace(id)
		}
	}
	return ""
}
