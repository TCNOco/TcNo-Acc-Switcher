package steamguard

import (
	"context"
)

// ConfirmationsSessionRefresh reports what a silent session renewal achieved.
// Refreshed means the confirmations window can carry on; NeedsCredentials means
// Steam wants a password, which only the main window can ask for.
type ConfirmationsSessionRefresh struct {
	Refreshed        bool `json:"refreshed"`
	NeedsCredentials bool `json:"needsCredentials"`
}

// RefreshConfirmationsSession renews the stored Steam session for the account the
// confirmations window is serving, using no user input. Steam usually accepts the
// stored refresh token, so a rejected session is worth retrying silently before
// asking the user to sign in for something that needed no password.
//
// The renewal writes to the vault, which rotates its generation and so invalidates
// the capability this window is holding. The window's recorded generation is
// updated here and its old capability revoked, so the window can request a working
// one instead of failing every later call.
func (s *Service) RefreshConfirmationsSession(token string) (ConfirmationsSessionRefresh, error) {
	binding, _, err := s.confirmationAccount(token)
	if err != nil {
		return ConfirmationsSessionRefresh{}, err
	}
	steamID, err := canonicalSteamID(binding.AccountID)
	if err != nil {
		return ConfirmationsSessionRefresh{}, err
	}
	if s.newSessionRefresher == nil {
		return ConfirmationsSessionRefresh{}, ErrSteamAuthenticationState
	}

	s.mu.Lock()
	v, vaultErr := s.requireVaultLocked()
	s.mu.Unlock()
	if vaultErr != nil {
		return ConfirmationsSessionRefresh{}, vaultErr
	}

	ctx, cancel := context.WithTimeout(context.Background(), confirmationOperationTimeout)
	defer cancel()
	if _, refreshErr := s.newSessionRefresher(v).Refresh(ctx, steamID); refreshErr != nil {
		if reason, ok := refreshFailureReason(refreshErr); ok {
			// Not an error to report: the stored session cannot be renewed, so the
			// user has to sign in. The capability is untouched, because these
			// classes all abort before the vault write.
			confirmationsLogger().Info("Steam session needs re-authentication",
				"steamId64", binding.AccountID, "reason", reason)
			return ConfirmationsSessionRefresh{NeedsCredentials: true}, nil
		}
		confirmationsLogger().Warn("Steam session refresh failed",
			"steamId64", binding.AccountID, "error", refreshErr)
		return ConfirmationsSessionRefresh{}, refreshErr
	}

	s.mu.Lock()
	generation := v.Generation()
	s.mu.Unlock()

	s.confirmationWindowMu.Lock()
	if s.confirmationAccountID == binding.AccountID {
		s.confirmationGeneration = generation
		if s.capabilities != nil {
			s.capabilities.RevokeWindow(confirmationsWindowName)
		}
	}
	s.confirmationWindowMu.Unlock()

	confirmationsLogger().Info("Steam session refreshed for the confirmations window",
		"steamId64", binding.AccountID)
	return ConfirmationsSessionRefresh{Refreshed: true}, nil
}
