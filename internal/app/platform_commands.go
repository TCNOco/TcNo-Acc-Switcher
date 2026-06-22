package app

import (
	"strings"

	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/steam"
)

type platformCommandAdapter interface {
	AccountRows() ([]ListAccountRow, error)
	Swap(uniqueID string, personaState int, extraLaunchArgs []string) error
	Logout() error
}

type steamCommandAdapter struct {
	svc *steam.SteamService
}

func (a steamCommandAdapter) AccountRows() ([]ListAccountRow, error) {
	accs, err := a.svc.GetSteamAccounts()
	if err != nil {
		return nil, err
	}
	out := make([]ListAccountRow, 0, len(accs))
	for _, account := range accs {
		out = append(out, ListAccountRow{
			UniqueID:     account.SteamID64,
			DisplayName:  strings.TrimSpace(account.DisplayName),
			LastLoggedIn: strings.TrimSpace(account.LastLogin),
		})
	}
	return out, nil
}

func (a steamCommandAdapter) Swap(uniqueID string, personaState int, extraLaunchArgs []string) error {
	return a.svc.SwapToSteamAccount(uniqueID, personaState, extraLaunchArgs)
}

func (a steamCommandAdapter) Logout() error {
	return a.svc.SteamAddNew()
}

type basicCommandAdapter struct {
	platformKey string
	basicSvc    *basic.BasicService
	platformSvc *platform.PlatformService
}

func (a basicCommandAdapter) AccountRows() ([]ListAccountRow, error) {
	accs, err := a.basicSvc.GetAccounts(a.platformKey)
	if err != nil {
		return nil, err
	}
	out := make([]ListAccountRow, 0, len(accs))
	for _, account := range accs {
		out = append(out, ListAccountRow{
			UniqueID:     account.UniqueID,
			DisplayName:  strings.TrimSpace(account.DisplayName),
			LastLoggedIn: strings.TrimSpace(account.LastUsed),
		})
	}
	return out, nil
}

func (a basicCommandAdapter) Swap(uniqueID string, _ int, extraLaunchArgs []string) error {
	return basic.SwapTo(basic.FlowDeps{PS: a.platformSvc}, a.platformKey, uniqueID, extraLaunchArgs)
}

func (a basicCommandAdapter) Logout() error {
	deps := basic.FlowDeps{PS: a.platformSvc}
	fc, err := basic.PrepareFlow(deps, a.platformKey)
	if err != nil {
		return err
	}
	return basic.ClearCurrentLogin(deps, fc)
}

func (d *Dispatch) commandAdapter(platformKey string) platformCommandAdapter {
	if strings.EqualFold(strings.TrimSpace(platformKey), steam.PlatformKey) {
		return steamCommandAdapter{svc: d.SteamSvc}
	}
	return basicCommandAdapter{
		platformKey: strings.TrimSpace(platformKey),
		basicSvc:    d.BasicSvc,
		platformSvc: d.PlatformSvc,
	}
}
