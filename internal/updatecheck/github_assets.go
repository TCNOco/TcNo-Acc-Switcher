package updatecheck

import (
	"strings"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

// Updater release asset basenames on GitHub, keyed by GOOS/GOARCH.
// These are the files Wails downloads and swaps in — not necessarily every
// asset on the release page (installer, 7z, etc. are user-facing only).
//
// Add a row when you ship a new platform. SHA256SUMS must list the same basename.
var updaterReleaseAssets = map[string]string{
	"windows/amd64": "TcNo-Acc-Switcher.exe",
	// "darwin/arm64":  "TcNo-Acc-Switcher-macos-universal.zip",
	// "darwin/amd64":  "TcNo-Acc-Switcher-macos-universal.zip",
	// "linux/amd64":   "TcNo-Acc-Switcher-linux-amd64.tar.gz",
}

// GitHubAssetMatcher selects the GitHub release asset for the running platform.
//
// Order: explicit project map → Wails DefaultAssetMatcher (app-{os}-{arch}.*).
// Prefer adding to updaterReleaseAssets over renaming user-facing downloads.
func GitHubAssetMatcher(req updater.CheckRequest, assets []github.ReleaseAsset) int {
	if idx := matchUpdaterAsset(req, assets); idx >= 0 {
		return idx
	}
	return github.DefaultAssetMatcher(req, assets)
}

func matchUpdaterAsset(req updater.CheckRequest, assets []github.ReleaseAsset) int {
	want, ok := updaterReleaseAssets[req.Platform+"/"+req.Arch]
	if !ok {
		return -1
	}
	wantLower := strings.ToLower(want)
	for i, a := range assets {
		if strings.EqualFold(a.Name, want) {
			return i
		}
		// Guard against accidental partial matches (e.g. foo.exe vs foo.exe.sig).
		name := strings.ToLower(a.Name)
		if name == wantLower {
			return i
		}
	}
	return -1
}
