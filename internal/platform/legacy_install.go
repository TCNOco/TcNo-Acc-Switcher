package platform

import (
	"errors"

	"TcNo-Acc-Switcher/internal/legacyinstall"
	"TcNo-Acc-Switcher/internal/winutil"
)

// LegacyInstallInfo describes files the C# release (4.x and earlier) left in the
// install folder when the Go build was installed over the top of it.
type LegacyInstallInfo struct {
	Found   bool   `json:"found"`
	Entries int    `json:"entries"`
	Size    string `json:"size"`
	Dir     string `json:"dir"`
}

// LegacyCleanupResult is the outcome of one cleanup pass.
type LegacyCleanupResult struct {
	Failed int    `json:"failed"`
	Freed  string `json:"freed"`
	// Declined is set when the user dismissed the elevation prompt. Not an
	// error: the offer just comes back next launch.
	Declined bool `json:"declined"`
}

// PendingLegacyInstall reports leftovers worth asking about - present, needing
// elevation this process does not have, and not already declined. Leftovers the
// app can delete unaided are cleared at startup without a prompt.
func (*PlatformService) PendingLegacyInstall() (LegacyInstallInfo, error) {
	exeDir, err := ResolveExeDir()
	if err != nil {
		return LegacyInstallInfo{}, err
	}
	rep, prompt := legacyinstall.ShouldPrompt(exeDir)
	if !prompt {
		return LegacyInstallInfo{}, nil
	}
	return LegacyInstallInfo{
		Found:   true,
		Entries: rep.Count(),
		Size:    legacyinstall.HumanBytes(rep.Bytes),
		Dir:     rep.ExeDir,
	}, nil
}

// CleanLegacyInstall removes the leftovers, asking for elevation when the
// install folder is not writable.
func (*PlatformService) CleanLegacyInstall() (LegacyCleanupResult, error) {
	exeDir, err := ResolveExeDir()
	if err != nil {
		return LegacyCleanupResult{}, err
	}
	res, err := legacyinstall.CleanElevated(exeDir)
	if errors.Is(err, winutil.ErrElevationDeclined) {
		return LegacyCleanupResult{Declined: true}, nil
	}
	if err != nil {
		return LegacyCleanupResult{}, err
	}
	return LegacyCleanupResult{
		Failed: len(res.Failed),
		Freed:  legacyinstall.HumanBytes(res.Bytes),
	}, nil
}

// DismissLegacyInstallPrompt records that the user would rather keep the old files.
func (*PlatformService) DismissLegacyInstallPrompt() error {
	return legacyinstall.Dismiss()
}
