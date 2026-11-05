package steamguard

import (
	"sync"

	"TcNo-Acc-Switcher/internal/battlenetauth"
	"TcNo-Acc-Switcher/internal/steambrowser"
)

func battleNetEnrollmentNavigation(onSessionToken func(string)) func(string) bool {
	var consume sync.Once
	return func(raw string) bool {
		if token, err := battlenetauth.SessionTokenFromRedirect(raw); err == nil {
			consume.Do(func() { onSessionToken(token) })
			return true
		}
		return !steambrowser.Classify(steambrowser.PlatformBattleNet, raw).Trusted
	}
}
