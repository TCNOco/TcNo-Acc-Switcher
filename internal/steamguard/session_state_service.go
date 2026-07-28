package steamguard

import (
	"context"
	"errors"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/sessionrefresh"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

// accessTokenSkew treats a token about to lapse as lapsed, so the answer does not
// flip between the local check and the probe that follows it.
const accessTokenSkew = 2 * time.Minute

// SteamSessionState reports whether an account's stored Steam session still works.
// NeedsLogin drives an affordance only — the sign-in flow is always available — so
// an inconclusive answer leaves it false rather than sending the user to a sign-in
// that may not be needed.
type SteamSessionState struct {
	NeedsLogin bool   `json:"needsLogin"`
	Reason     string `json:"reason,omitempty"`
}

// SteamSessionLocalState answers from the stored session alone: no Steam request,
// no vault write. It catches the ordinary case of an expired session, which is what
// a stale account looks like, without spending a request to be told so.
func (s *Service) SteamSessionLocalState(accountID, token string) (SteamSessionState, error) {
	v, _, _, err := s.authorizeSteamFlow(accountID, token)
	if err != nil {
		return SteamSessionState{}, err
	}
	account, err := accountForSteamID64(v, accountID)
	if err != nil {
		return SteamSessionState{}, err
	}
	if account.Session == nil || account.Session.AccessToken == "" {
		return SteamSessionState{NeedsLogin: true, Reason: "no_session"}, nil
	}
	if sessionrefresh.AccessTokenExpired(account.Session.AccessToken, time.Now(), accessTokenSkew) {
		return SteamSessionState{NeedsLogin: true, Reason: "token_expired"}, nil
	}
	return SteamSessionState{}, nil
}

// ProbeSteamSession asks Steam the same question the confirmations page asks, and
// reports only whether Steam refused the session. A transport or rate-limit failure
// says nothing about the session, so it is not reported as needing a sign-in.
//
// Read-only: it lists confirmations and discards them, so nothing is written to the
// vault and no capability is invalidated.
func (s *Service) ProbeSteamSession(accountID, token string) (SteamSessionState, error) {
	v, _, _, err := s.authorizeSteamFlow(accountID, token)
	if err != nil {
		return SteamSessionState{}, err
	}
	if s.confirmationClient == nil {
		return SteamSessionState{}, ErrSteamMobileSession
	}
	account, err := accountForSteamID64(v, accountID)
	if err != nil {
		return SteamSessionState{}, err
	}
	credentials, err := confirmationCredentials(account, accountID)
	if err != nil {
		return SteamSessionState{NeedsLogin: true, Reason: "session_unusable"}, nil
	}

	// A private context: hijacking the confirmations window's operation would
	// cancel a refresh that window has in flight.
	ctx, cancel := context.WithTimeout(context.Background(), confirmationOperationTimeout)
	defer cancel()
	if _, listErr := s.confirmationClient.List(ctx, credentials); listErr != nil {
		if confirmationFailure(listErr).State == "reauth" {
			confirmationsLogger().Debug("Steam refused the stored session",
				"operation", "probe", "steamId64", accountID)
			return SteamSessionState{NeedsLogin: true, Reason: "remote_rejected"}, nil
		}
		// success:false without needauth is still Steam declining the signed
		// request — a session verdict, seen on accounts whose session lapsed
		// in ways getlist does not name. Transport failures stay inconclusive.
		var apiErr *confirmationapi.Error
		if errors.As(listErr, &apiErr) && apiErr.Kind == confirmationapi.FailureRefused {
			confirmationsLogger().Debug("Steam declined the confirmations probe",
				"operation", "probe", "steamId64", accountID)
			return SteamSessionState{NeedsLogin: true, Reason: "remote_refused"}, nil
		}
	}
	return SteamSessionState{}, nil
}

// accountForSteamID64 loads the vault record for a SteamID64. Callers must already
// hold an authorized, unlocked vault.
func accountForSteamID64(v *vault.Vault, steamID64 string) (mafile.Account, error) {
	records, err := v.List()
	if err != nil {
		return mafile.Account{}, err
	}
	for _, record := range records {
		if record.SteamID64 == steamID64 {
			return accountFromRecord(v, record.ID)
		}
	}
	return mafile.Account{}, ErrAccountNotFound
}
