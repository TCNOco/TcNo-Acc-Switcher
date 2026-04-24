package platform

import (
	"errors"
	"os"
	"strings"

	"TcNo-Acc-Switcher/internal/winutil"
)

// Must match steam.steamKillNames (admin pre-flight uses the same process list).
var steamKillNamesForAdmin = []string{
	"steam.exe",
	"SERVICE:Steam Client Service",
	"steamwebhelper.exe",
	"GameOverlayUI.exe",
}

type AdminCheckResult struct {
	NeedsAdmin bool   `json:"needsAdmin"`
	Blocker    string `json:"blocker,omitempty"`
}

func (p *PlatformService) CheckAdminForPlatform(platformKey string) (AdminCheckResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return AdminCheckResult{}, errors.New("empty platform")
	}

	ps, err := LoadPlatformSettings(platformKey)
	if err != nil {
		return AdminCheckResult{}, err
	}
	method := winutil.ClosingMethod(strings.TrimSpace(ps.ClosingMethod))
	if method == "" {
		method = winutil.ClosingCombined
	}

	if strings.EqualFold(platformKey, "Steam") {
		blocker, ok := winutil.CanKillProcesses(steamKillNamesForAdmin, method)
		if !ok {
			return AdminCheckResult{NeedsAdmin: true, Blocker: blocker}, nil
		}
		return AdminCheckResult{}, nil
	}

	exeDir, err := ResolveExeDir()
	if err != nil {
		return AdminCheckResult{}, err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return AdminCheckResult{}, err
	}
	raw, err := os.ReadFile(p.resolvePlatformsPath(exeDir, settings))
	if err != nil {
		return AdminCheckResult{}, err
	}
	d, err := ParseDescriptor(raw, platformKey)
	if err != nil {
		return AdminCheckResult{}, err
	}
	blocker, ok := winutil.CanKillProcesses(d.ExesToEnd, method)
	if !ok {
		return AdminCheckResult{NeedsAdmin: true, Blocker: blocker}, nil
	}
	return AdminCheckResult{}, nil
}

// RestartAsAdmin re-execs elevated with args and exits; main should register winutil.RegisterSingletonReleaser first.
func (p *PlatformService) RestartAsAdmin(args []string) error {
	_ = p // satisfy staticcheck if receiver unused in future
	return winutil.RestartElevated(args)
}
