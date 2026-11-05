package steamguard

import (
	"context"
	"encoding/json"
	"log/slog"
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
	if s.battleNet == nil {
		s.mu.Unlock()
		return false, nil
	}
	profile, previousName, previousLink := battleNetBrowserProfile(a), a.AccountName, a.AccountLink
	a = battlenetauth.Account{}
	browser, read := s.battleNet.browser, s.battleNet.readAccountName
	if read == nil {
		read = steambrowser.ReadBattleNetAccountIdentity
	}
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	identity, err := read(ctx, browser, profile)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.battleNet != nil {
		status := "waiting-for-account"
		if err != nil {
			status = "read-failed"
		} else if identity.Name != "" {
			status = "available"
		}
		if s.battleNet.identityStatus == nil {
			s.battleNet.identityStatus = map[string]string{}
		}
		if s.battleNet.identityStatus[profile] != status {
			s.battleNet.identityStatus[profile] = status
			slog.Debug("Battle.net account identity lookup", "profile", profile, "status", status)
		}
	}
	if err != nil || identity.Name == "" || (identity.Name == previousName && (previousLink != "" || identity.Email == "")) {
		return false, err
	}
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
	if a.AccountName != previousName || a.AccountLink != previousLink || battleNetBrowserProfile(a) != profile {
		return false, nil
	}
	a.AccountName = identity.Name
	if a.AccountLink == "" {
		a.AccountLink = identity.Email
	}
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
	slog.Debug("Battle.net account identity saved", "profile", profile, "linked", a.AccountLink != "")
	return true, nil
}
