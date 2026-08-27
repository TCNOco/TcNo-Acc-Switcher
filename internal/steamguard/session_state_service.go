package steamguard

import (
	"context"
	"errors"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/sessionrefresh"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
	"TcNo-Acc-Switcher/internal/steamguard/vaultrecord"
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
	// Both record shapes carry a session, and a login-only account needs this
	// affordance more than an authenticator one does - re-signing in is the only
	// thing its screen offers.
	record, err := recordForSteamID64(v, accountID)
	if err != nil {
		return SteamSessionState{}, err
	}
	defer record.destroy()
	accessToken := record.AccessToken()
	refreshToken := record.RefreshToken()
	now := time.Now()
	if accessToken != "" && !sessionrefresh.AccessTokenExpired(accessToken, now, accessTokenSkew) {
		return SteamSessionState{}, nil
	}
	// A lapsed access token beside a live refresh token is not a sign-in. Opening
	// the account runs EnsureFreshSession, which mints a new one from the refresh
	// token without the user doing anything - so offering a sign-in here would ask
	// for a password the app does not need.
	if sessionrefresh.RefreshTokenUsable(refreshToken, now, accessTokenSkew) {
		return SteamSessionState{}, nil
	}
	if accessToken == "" {
		return SteamSessionState{NeedsLogin: true, Reason: "no_session"}, nil
	}
	return SteamSessionState{NeedsLogin: true, Reason: "token_expired"}, nil
}

// localSessionStatus is SteamSessionLocalState's verdict for a record already in
// hand, for the listing path: same skew, so a row and the screen it opens cannot
// contradict each other. A token this build cannot read stays unknown rather than
// being reported as lapsed, and a half-finished enrollment has no session to judge.
//
// The refresh token is what keeps a row from claiming a sign-in is needed every
// time an access token ages out overnight.
func localSessionStatus(kind vaultrecord.Kind, accessToken, refreshToken string, now time.Time) SessionStatus {
	if kind == vaultrecord.KindEnrollmentPending {
		return SessionStatusUnknown
	}
	if accessToken != "" {
		if _, readable := sessionrefresh.AccessTokenExpiry(accessToken); !readable {
			return SessionStatusUnknown
		}
		if !sessionrefresh.AccessTokenExpired(accessToken, now, accessTokenSkew) {
			return SessionStatusValid
		}
	}
	if sessionrefresh.RefreshTokenUsable(refreshToken, now, accessTokenSkew) {
		return SessionStatusValid
	}
	return SessionStatusNeedsLogin
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

// accountForSteamID64 loads the authenticator for a SteamID64. Callers must already
// hold an authorized, unlocked vault. A login-only account yields
// ErrNotAuthenticator, since every caller here needs a shared or identity secret.
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

// recordForSteamID64 loads whichever record shape the vault holds for a
// SteamID64, for the few callers that handle more than one.
func recordForSteamID64(v *vault.Vault, steamID64 string) (loadedRecord, error) {
	records, err := v.List()
	if err != nil {
		return loadedRecord{}, err
	}
	for _, record := range records {
		if record.SteamID64 == steamID64 {
			return recordFromVault(v, record.ID)
		}
	}
	return loadedRecord{}, ErrAccountNotFound
}
