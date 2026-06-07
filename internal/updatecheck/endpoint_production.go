//go:build production

package updatecheck

import "TcNo-Acc-Switcher/internal/api"

func updateAPIURL(version string) string {
	return api.VersionCheckURL(version, false)
}
