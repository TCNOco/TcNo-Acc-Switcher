package steamguard

import (
	"errors"
	"strconv"
	"unicode/utf8"

	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/steamguard/loginrecord"
	"TcNo-Acc-Switcher/internal/steamguard/registry"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
	"TcNo-Acc-Switcher/internal/steamguard/vaultrecord"
)

// ErrWouldReplaceAuthenticator refuses to overwrite an authenticator with a
// session-only record.
var ErrWouldReplaceAuthenticator = errors.New("this Steam account already has an authenticator stored")

// putLoginOnlyRecord stores a session-only record for steamID.
//
// The guard is the important part. vault.PutRecord replaces by SteamID64, so
// without a pre-read a user who picks "Login only" on an account that already
// has an authenticator would destroy the shared secret, the identity secret and
// the revocation code - none of which can be recovered. Only an absent record or
// an existing login-only one may be written here; replacing a login-only record
// with a real authenticator is fine and goes through the enrollment flow.
func putLoginOnlyRecord(v *vault.Vault, steamID uint64, accountName, accessToken, refreshToken []byte) error {
	if v == nil || !utf8.Valid(accountName) || !validBearerBytes(accessToken) ||
		(len(refreshToken) != 0 && !validBearerBytes(refreshToken)) {
		return ErrSteamAuthenticationState
	}
	wanted := strconv.FormatUint(steamID, 10)
	existing, _, err := recordKindForSteamID(v, wanted)
	if err != nil {
		return err
	}
	switch existing {
	case vaultrecord.KindUnknown, vaultrecord.KindLoginOnly:
		// Absent, or already the shape we are about to write.
	default:
		return ErrWouldReplaceAuthenticator
	}

	record := loginrecord.New(steamID, string(accountName), string(accessToken), string(refreshToken))
	canonical, err := loginrecord.Encode(record)
	record.Destroy()
	if err != nil {
		wipe(canonical)
		return ErrSteamAuthenticationState
	}
	defer wipe(canonical)
	_, err = v.PutRecord(wanted, canonical)
	return err
}

// registryStateForKind maps a record shape onto its account-list projection.
func registryStateForKind(kind vaultrecord.Kind) registry.State {
	switch kind {
	case vaultrecord.KindLoginOnly:
		return registry.StateLoginOnly
	case vaultrecord.KindEnrollmentPending:
		return registry.StatePending
	default:
		return registry.StateActive
	}
}

// registryStateForRecord reports the registry state matching what the vault
// actually holds for accountID, falling back to fallback when it cannot tell.
//
// Signing in again must not silently change an account's kind: a login-only
// account that re-authenticates is still login-only.
func (s *Service) registryStateForRecord(v *vault.Vault, accountID string, fallback registry.State) registry.State {
	kind, _, err := recordKindForSteamID(v, accountID)
	if err != nil || kind == vaultrecord.KindUnknown {
		return fallback
	}
	return registryStateForKind(kind)
}

// RemoveLoginOnlyAccount deletes a login-only record from the vault.
//
// Only login-only records: an authenticator's secrets exist nowhere else, so
// there is no "remove" for one that would not be a silent, unrecoverable loss.
// The kind is re-checked under the service lock rather than trusted from the
// caller, because the frontend gate is UX and this one is the actual rule.
//
// The account's stored CS2 cooldown is deliberately left behind. It describes
// the account, not the vault record, and signing out of Steam Guard is not a way
// to clear a cooldown.
func (s *Service) RemoveLoginOnlyAccount(accountID, token string) (SteamLoginResult, error) {
	if _, _, _, err := s.authorizeSteamFlow(accountID, token); err != nil {
		return SteamLoginResult{}, err
	}
	steamID, err := canonicalSteamID(accountID)
	if err != nil {
		return SteamLoginResult{}, err
	}
	wanted := strconv.FormatUint(steamID, 10)

	removed, err := s.deleteLoginOnlyRecord(wanted)
	if err != nil {
		return SteamLoginResult{}, err
	}

	registryUpdated := false
	if removed {
		if err := registry.Remove(wanted); err != nil {
			// The vault is the authority; a stale index entry only misdraws an
			// icon and is corrected by the next write.
			serviceLogger().Warn("login-only account removed but the registry entry remains",
				"steamId64", wanted, "error", err)
		} else {
			registryUpdated = true
		}
	}
	return SteamLoginResult{
		State: "removed",
		// DeleteRecord rotated the vault generation, so every outstanding
		// capability - including the modal's own - is now invalid.
		CapabilityRefreshRequired: true,
		RegistryUpdated:           registryUpdated,
	}, nil
}

// deleteLoginOnlyRecord removes the login-only record for wanted and reports
// whether one was actually there to remove.
//
// The kind is re-resolved under the service lock rather than trusted from the
// caller: the vault can change between authorizing and acting, and this is the
// check that protects an authenticator.
func (s *Service) deleteLoginOnlyRecord(wanted string) (bool, error) {
	var removed bool
	err := s.withServiceLock(func() error {
		v, err := s.requireVaultLocked()
		if err != nil {
			return err
		}
		// Named here rather than left to the first read that needs the keys, so
		// a locked vault is one error the caller can act on instead of whichever
		// decryption failed first.
		if v.IsLocked() {
			return vault.ErrLocked
		}
		kind, recordID, err := recordKindForSteamID(v, wanted)
		if err != nil {
			return err
		}
		if recordID == "" {
			return ErrAccountNotFound
		}
		if kind != vaultrecord.KindLoginOnly {
			return ErrNotAuthenticator
		}
		if err := v.DeleteRecord(recordID); err != nil {
			return err
		}
		removed = true
		return nil
	})
	return removed, err
}

// forgetLoginOnlyRecord drops the Steam Guard side of an account the switcher is
// forgetting. Registered with the steam package at startup; unexported so it
// stays an in-process hook rather than becoming a bound frontend call, since it
// carries no capability token.
//
// The kind is still re-checked under the lock, so an authenticator is refused
// here as well as by the menu that offers no Forget for one. The index entry goes
// even when the vault holds no record: a stale entry is enough on its own to
// rebuild the account's row, nameless, which is the whole failure this prevents.
func (s *Service) forgetLoginOnlyRecord(steamID64 string) error {
	steamID, err := canonicalSteamID(steamID64)
	if err != nil {
		return err
	}
	wanted := strconv.FormatUint(steamID, 10)
	if _, err := s.deleteLoginOnlyRecord(wanted); err != nil {
		switch {
		case errors.Is(err, ErrNotAuthenticator):
			return steam.ErrForgetSteamGuardAuthenticator
		case errors.Is(err, vault.ErrLocked):
			return steam.ErrForgetSteamGuardLocked
		case errors.Is(err, ErrAccountNotFound), errors.Is(err, ErrVaultNotReady), errors.Is(err, ErrFeatureDisabled):
			// Nothing to delete. The index entry is all that is left of the
			// account, and clearing it below is what makes the row go away.
		default:
			return err
		}
	}
	return registry.Remove(wanted)
}
