package steamguard

import (
	"context"
	"encoding/json"
	"time"

	"TcNo-Acc-Switcher/internal/battlenetauth"
	"TcNo-Acc-Switcher/internal/steambrowser"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

func (s *Service) RefreshBattleNetAccountName(serial, token string) (bool, error) {
	s.mu.Lock()
	v, err := s.battleNetVaultLocked(token)
	if err != nil {
		s.mu.Unlock()
		return false, err
	}
	a, err := battleNetRecord(v, serial)
	if err != nil {
		s.mu.Unlock()
		return false, err
	}
	if s.battleNet == nil || (a.Label != "" && a.Label != a.Serial) {
		s.mu.Unlock()
		return false, nil
	}
	profile, previousName := battleNetBrowserProfile(a), a.AccountName
	a = battlenetauth.Account{}
	browser, read := s.battleNet.browser, s.battleNet.readAccountName
	if read == nil {
		read = steambrowser.ReadBattleNetAccountName
	}
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	name, err := read(ctx, browser, profile)
	if err != nil || name == "" || name == previousName {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	currentVault, err := s.battleNetVaultLocked(token)
	if err != nil {
		return false, err
	}
	if currentVault != v {
		return false, ErrSensitiveView
	}
	a, err = battleNetRecord(v, serial)
	if err != nil {
		return false, err
	}
	if (a.Label != "" && a.Label != a.Serial) || a.AccountName != previousName || battleNetBrowserProfile(a) != profile {
		return false, nil
	}
	a.AccountName = name
	if err := a.Validate(); err != nil {
		return false, err
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return false, err
	}
	defer wipe(raw)
	if _, err := v.PutProviderRecord(vault.ProviderBattleNet, a.Serial, raw); err != nil {
		return false, err
	}
	return true, nil
}
