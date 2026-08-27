package steamguard

import (
	"errors"
	"strings"

	"TcNo-Acc-Switcher/internal/steam/accountstore"
	"TcNo-Acc-Switcher/internal/steamguard/authflow"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
	"TcNo-Acc-Switcher/internal/steamguard/qrrender"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

// qrFlowAuthorizer is the account authorizer with the expected account name
// added, which is what a QR session is checked against when somebody scans it.
//
// The name must be resolved on every call rather than captured once: authflow
// compares the whole binding on each step, so a binding that differed between
// beginning and polling would be refused as somebody else's.
//
// It stays in this wrapper and out of authorizeSteamFlow: a password sign-in
// already proves which account it is, and a name in that shared binding would
// fail every live password session the moment a vault record was renamed.
func (s *Service) qrFlowAuthorizer(accountID, token string) steamFlowAuthorizer {
	return func() (*vault.Vault, authflow.Binding, uint64, error) {
		v, binding, steamID, err := s.authorizeSteamFlow(accountID, token)
		if err != nil {
			return nil, authflow.Binding{}, 0, err
		}
		// Resolved outside authorizeSteamFlow, not inside it: both take the
		// service lock, and nesting them deadlocks.
		accountName := s.expectedQRAccountName(binding.AccountID)
		if accountName == "" {
			return nil, authflow.Binding{}, 0, ErrAccountNotFound
		}
		binding.ExpectedAccountName = accountName
		return v, binding, steamID, nil
	}
}

// expectedQRAccountName resolves the login name a scan is checked against.
//
// The vault only answers for an account it already holds, and a login-only
// setup runs before there is any such record, so the account store answers when
// the vault cannot.
//
// Never taken from the caller: it is the one thing standing between a scan and
// somebody else's tokens being stored under this account, so it is derived from
// what is on disk.
func (s *Service) expectedQRAccountName(accountID string) string {
	if name, known := s.accountNameForSteamID(accountID); known {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			return trimmed
		}
	}
	record, found, err := accountstore.Get(accountID)
	if err != nil || !found {
		return ""
	}
	return strings.TrimSpace(record.AccountName)
}

// BeginQRLogin opens a sign-in the user completes by scanning, for the account
// accountID names. It runs alongside the password sign-in rather than replacing
// it - the screen offers both at once.
func (s *Service) BeginQRLogin(accountID, token string, purpose SteamAuthPurpose) (SteamCredentialResult, error) {
	if !validSteamAuthPurpose(purpose) {
		authflowLogger().Warn("Steam QR login request rejected",
			"steamId64", strings.TrimSpace(accountID), "purpose", string(purpose))
		return SteamCredentialResult{}, ErrSteamAuthenticationPurpose
	}
	authorize := s.qrFlowAuthorizer(accountID, token)
	_, binding, _, err := authorize()
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			// Not a failure. Without a login name there is nothing to check a
			// scan against, so there is no code worth offering - and an empty
			// result says exactly that.
			authflowLogger().Debug("Steam QR login not offered: no login name for the account",
				"steamId64", strings.TrimSpace(accountID))
			return SteamCredentialResult{}, nil
		}
		return SteamCredentialResult{}, err
	}
	manager, epoch, err := s.authenticationManager()
	if err != nil {
		return SteamCredentialResult{}, err
	}
	key := binding.AccountID
	ctx, finish, err := s.startBoundSteamOperation(key)
	if err != nil {
		return SteamCredentialResult{}, err
	}
	defer finish()
	authflowLogger().Debug("beginning Steam QR login", "steamId64", key, "purpose", string(purpose))
	status, err := manager.BeginQR(ctx, binding, qrLoginRequest())
	if err != nil {
		logAuthflowFailure("begin-qr", key, err)
		return SteamCredentialResult{}, err
	}
	if _, currentBinding, _, authorizeErr := authorize(); authorizeErr != nil || currentBinding != binding {
		s.cancelAuthflowSession(manager, binding, status.Handle, "begin-qr-reauthorize")
		if authorizeErr != nil {
			return SteamCredentialResult{}, authorizeErr
		}
		return SteamCredentialResult{}, ErrSteamAuthenticationState
	}
	s.authStateMu.Lock()
	if s.authShutdown || s.authManager != manager || s.authManagerEpoch != epoch {
		s.authStateMu.Unlock()
		s.cancelAuthflowSession(manager, binding, status.Handle, "begin-qr-manager-replaced")
		return SteamCredentialResult{}, ErrSteamAuthenticationState
	}
	if s.authOperations == nil {
		s.authOperations = make(map[string]steamAuthOperation)
	}
	s.authOperations[status.Handle] = steamAuthOperation{binding: binding, purpose: purpose}
	s.authStateMu.Unlock()
	authflowLogger().Info("Steam QR login started",
		"steamId64", key, "purpose", string(purpose), "state", string(status.State))
	return qrCredentialResult(status), nil
}

// PollQRLogin asks whether the code has been scanned yet. A code Steam has
// rotated comes back as a new image on a session that is still waiting.
func (s *Service) PollQRLogin(accountID, token, handle string) (SteamCredentialResult, error) {
	return s.pollLogin(s.qrFlowAuthorizer(accountID, token), strings.TrimSpace(accountID), handle, true)
}

func (s *Service) CancelQRLogin(accountID, token, handle string) error {
	return s.cancelLogin(s.qrFlowAuthorizer(accountID, token), strings.TrimSpace(accountID), handle)
}

// qrLoginRequest describes this client to Steam. It matches the password
// sign-in's description, so the phone shows the user the same requester
// whichever way they choose on the screen.
func qrLoginRequest() protocol.BeginQRRequest {
	password := passwordAuthRequest("")
	return protocol.BeginQRRequest{
		DeviceFriendlyName: password.DeviceFriendlyName,
		Platform:           password.Platform,
		Device:             password.Device,
		WebsiteID:          password.WebsiteID,
	}
}

// qrCredentialResult adds the parts only a QR sign-in has: the challenge URL and
// the code drawn from it. A failed render is not fatal - the URL is still there,
// and the screen can offer it as a link rather than nothing at all.
func qrCredentialResult(status authflow.Status) SteamCredentialResult {
	result := credentialResult(status)
	result.ChallengeURL = status.ChallengeURL
	if status.ChallengeURL == "" {
		return result
	}
	image, err := qrrender.SVGDataURI(status.ChallengeURL)
	if err != nil {
		authflowLogger().Warn("Steam QR code could not be drawn", "error", err)
		return result
	}
	result.QRImage = image
	return result
}
