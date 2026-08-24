package steamguard

// Adding an account the vault has never seen inverts the identity assumption the
// rest of this package is built on. Every other flow starts from a SteamID64 the
// caller already has; an add only learns one when Steam authorises the login.
//
// Rather than loosen canonicalSteamID - it gates ten entry points and is the
// front door to every secret-bearing call - an add runs under a pending id this
// service issues, and hands the real SteamID64 back in the result. The layers
// underneath already allow it: capability bindings take any non-empty account
// id, and authflow reads ExpectedSteamID == 0 as "accept whatever Steam
// returns". This file is the only place a pending id may be minted or accepted;
// nothing else in the package will recognise one.

import (
	"crypto/rand"
	"encoding/hex"
	"runtime"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/authflow"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

const (
	pendingAddPrefix = "pending:"
	// pendingAddDigits is the hex length of the random part. 128 bits, so an id
	// cannot be guessed by a caller that was never handed one.
	pendingAddDigits = 32
	// pendingAddTTL bounds how long an abandoned attempt keeps its slot. Long
	// enough for a user to find their phone and read an email code.
	pendingAddTTL = 15 * time.Minute
	// maxPendingAdds caps concurrent attempts so a caller looping on
	// NewAddAccountAttempt cannot grow the map without bound.
	maxPendingAdds = 8
)

type pendingAdd struct {
	issued time.Time
}

// NewAddAccountAttempt issues the id an add-account login runs under. The
// caller passes it to RequestAddAccountView to obtain a capability, then to the
// Begin/Submit/Poll calls below. It is single-use: the attempt is discarded as
// soon as Steam authorises it or the caller cancels.
func (s *Service) NewAddAccountAttempt() (string, error) {
	raw := make([]byte, pendingAddDigits/2)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	id := pendingAddPrefix + hex.EncodeToString(raw)

	s.pendingAddMu.Lock()
	defer s.pendingAddMu.Unlock()
	s.prunePendingAddsLocked(time.Now())
	if len(s.pendingAdds) >= maxPendingAdds {
		return "", ErrSteamAuthenticationState
	}
	if s.pendingAdds == nil {
		s.pendingAdds = make(map[string]pendingAdd, 1)
	}
	s.pendingAdds[id] = pendingAdd{issued: time.Now()}
	return id, nil
}

// validPendingAddFormat is the shape check. It says nothing about whether this
// service issued the id, which is what validPendingAdd is for.
func validPendingAddFormat(id string) bool {
	if !strings.HasPrefix(id, pendingAddPrefix) {
		return false
	}
	suffix := id[len(pendingAddPrefix):]
	if len(suffix) != pendingAddDigits {
		return false
	}
	// hex.DecodeString accepts upper case; the ids we mint are lower, and
	// accepting both would make two spellings of one id.
	for _, r := range suffix {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// validPendingAdd reports whether this service issued the id and it has neither
// expired nor been used up.
func (s *Service) validPendingAdd(id string) bool {
	if !validPendingAddFormat(id) {
		return false
	}
	s.pendingAddMu.Lock()
	defer s.pendingAddMu.Unlock()
	s.prunePendingAddsLocked(time.Now())
	_, ok := s.pendingAdds[id]
	return ok
}

// consumePendingAdd retires an attempt. Called once Steam has authorised the
// login, or when the caller gives up, so an id can never be replayed.
func (s *Service) consumePendingAdd(id string) {
	s.pendingAddMu.Lock()
	defer s.pendingAddMu.Unlock()
	delete(s.pendingAdds, id)
}

func (s *Service) prunePendingAddsLocked(now time.Time) {
	for id, attempt := range s.pendingAdds {
		if now.Sub(attempt.issued) >= pendingAddTTL {
			delete(s.pendingAdds, id)
		}
	}
}

// RequestAddAccountView is RequestSensitiveView for an attempt that has no
// SteamID64 yet. It exists only because that method gates on ParseUint, which a
// pending id cannot satisfy; everything it does afterwards is identical.
func (s *Service) RequestAddAccountView(pendingID, requestID string) error {
	pendingID = strings.TrimSpace(pendingID)
	if !s.validPendingAdd(pendingID) {
		return ErrSensitiveView
	}
	return s.issueSensitiveView(pendingID, requestID)
}

// authorizePendingFlow is authorizeSteamFlow for an add. Same vault and
// capability checks; the difference is that the binding carries no expected
// SteamID, which is what lets authflow accept the account Steam names.
func (s *Service) authorizePendingFlow(pendingID, token string) (*vault.Vault, authflow.Binding, uint64, error) {
	pendingID = strings.TrimSpace(pendingID)
	if !s.validPendingAdd(pendingID) {
		return nil, authflow.Binding{}, 0, ErrAccountNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.requireVaultLocked()
	if err != nil {
		return nil, authflow.Binding{}, 0, err
	}
	if v.IsLocked() {
		return nil, authflow.Binding{}, 0, vault.ErrLocked
	}
	if err := s.authorizeModalLocked(v, pendingID, token); err != nil {
		return nil, authflow.Binding{}, 0, err
	}
	return v, authflow.Binding{
		AccountID: pendingID, VaultGeneration: v.Generation(), CapabilityID: authCapabilityID(token),
	}, 0, nil
}

func (s *Service) pendingFlowAuthorizer(pendingID, token string) steamFlowAuthorizer {
	return func() (*vault.Vault, authflow.Binding, uint64, error) {
		return s.authorizePendingFlow(pendingID, token)
	}
}

// validAddAccountPurpose excludes login_again: signing in again is something you
// do to an account the vault already holds, which is the opposite of an add.
func validAddAccountPurpose(purpose SteamAuthPurpose) bool {
	return purpose == SteamAuthPurposeLoginOnly || purpose == SteamAuthPurposeAddAuthenticator
}

func (s *Service) BeginAddAccountLogin(pendingID, token, accountName, password string, purpose SteamAuthPurpose) (SteamCredentialResult, error) {
	passwordBytes := []byte(password)
	password = ""
	defer wipe(passwordBytes)
	defer runtime.KeepAlive(password)
	if !validAddAccountPurpose(purpose) || !validAccountName(accountName) || len(passwordBytes) == 0 {
		authflowLogger().Warn("Steam add-account login request rejected",
			"purpose", string(purpose), "validPurpose", validAddAccountPurpose(purpose),
			"validAccountName", validAccountName(accountName), "hasPassword", len(passwordBytes) > 0)
		return SteamCredentialResult{}, ErrSteamAuthenticationPurpose
	}
	return s.beginLogin(s.pendingFlowAuthorizer(pendingID, token), pendingID, accountName, passwordBytes, purpose)
}

func (s *Service) SubmitAddAccountCode(pendingID, token, handle, challenge, code string) (SteamCredentialResult, error) {
	codeBytes := []byte(code)
	code = ""
	defer wipe(codeBytes)
	defer runtime.KeepAlive(code)
	return s.submitLoginCode(s.pendingFlowAuthorizer(pendingID, token), pendingID, handle, challenge, codeBytes)
}

func (s *Service) PollAddAccountLogin(pendingID, token, handle string) (SteamCredentialResult, error) {
	result, err := s.pollLogin(s.pendingFlowAuthorizer(pendingID, token), pendingID, handle, false)
	// The attempt is spent the moment Steam names an account: from here the
	// caller continues under the real SteamID64, through the strict endpoints.
	if err == nil && result.SteamID64 != "" {
		s.consumePendingAdd(strings.TrimSpace(pendingID))
	}
	return result, err
}

func (s *Service) CancelAddAccountLogin(pendingID, token, handle string) error {
	err := s.cancelLogin(s.pendingFlowAuthorizer(pendingID, token), pendingID, handle)
	// Retire the attempt even when the cancel failed, so an abandoned id cannot
	// be reused; the TTL is the backstop for callers that never get here at all.
	s.consumePendingAdd(strings.TrimSpace(pendingID))
	return err
}
