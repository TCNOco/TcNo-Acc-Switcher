//go:build production

package updatecheck

import "net/url"

func updateAPIURL(version string) string {
	return "https://tcno.co/Projects/AccSwitcher/api?v=" + url.QueryEscape(version)
}
