package api

import (
	"net/url"
	"strings"
)

const baseURL = "https://api.tcno.co/sw"

func UserAgent(version string) string {
	return "TcNo-Acc-Switcher/" + strings.TrimSpace(version)
}

func AnonymousStatsUploadURL() string {
	return baseURL + "/stats/"
}

func CrashURL() string {
	return baseURL + "/crashes/"
}

func VersionCheckURL(version string, debug bool) string {
	v := url.QueryEscape(strings.TrimSpace(version))
	if debug {
		return "https://tcno.co/Projects/AccSwitcher/api?debug&v=" + v
	}
	return "https://tcno.co/Projects/AccSwitcher/api?v=" + v
}
