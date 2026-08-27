package steamguard

import (
	"errors"
	"strconv"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/enrollmentflow"
	"TcNo-Acc-Switcher/internal/steamguard/registry"
	"TcNo-Acc-Switcher/internal/steamguard/sessionrefresh"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
	"TcNo-Acc-Switcher/internal/steamguard/vaultrecord"
)

// ErrNotLoginOnly refuses a promotion for a record that is not session-only.
// Every other shape either already has an authenticator or cannot be read, and
// enrolling over one destroys secrets held nowhere else.
var ErrNotLoginOnly = errors.New("this Steam account is not a login-only record")

// SteamGuardPromotion is the outcome of promoting a login-only record.
//
// Exactly one of Enrollment and NeedsLogin is meaningful. NeedsLogin is not an
// error: the stored session simply cannot authorize the enrollment call any
// more, so the caller has to collect a password and go through the ordinary
// add-authenticator flow instead.
type SteamGuardPromotion struct {
	NeedsLogin bool `json:"needsLogin"`
	// Reason names why the stored session was refused, in the same vocabulary
	// SteamSessionState uses.
	Reason string `json:"reason,omitempty"`
	// AccountName is the stored Steam login name, so a caller falling back to the
	// password form already has half of it filled in.
	AccountName               string                 `json:"accountName,omitempty"`
	Enrollment                *SteamEnrollmentStatus `json:"enrollment,omitempty"`
	CapabilityRefreshRequired bool                   `json:"capabilityRefreshRequired"`
	RegistryUpdated           bool                   `json:"registryUpdated"`
}

// isLoginOnlyRecord reports whether the vault currently stores a session-only
// record for accountID. A read failure answers false: leaving the enrollment
// flow's own refusal in place is the safe way to be wrong.
func isLoginOnlyRecord(v *vault.Vault, accountID string) bool {
	kind, recordID, err := recordKindForSteamID(v, accountID)
	return err == nil && recordID != "" && kind == vaultrecord.KindLoginOnly
}

// PromoteLoginOnlyAccount turns a session-only record into a real Steam Guard
// authenticator, reusing the session it already holds.
//
// A login-only record holds the same tokens a fresh sign-in would produce, so
// enrollment starts straight from them and the user is only asked for what Steam
// itself insists on - the emailed or texted confirmation code, and the recovery
// code they have to keep. A lapsed session is renewed from its refresh token
// first, exactly as opening the account would have done.
//
// Nothing is destroyed on a refusal: Steam is asked to add the authenticator
// before anything is written, so a rejected enrollment leaves the login-only
// record and its session untouched.
func (s *Service) PromoteLoginOnlyAccount(accountID, token string) (SteamGuardPromotion, error) {
	v, _, steamID, err := s.authorizeSteamFlow(accountID, token)
	if err != nil {
		return SteamGuardPromotion{}, err
	}
	manager := s.enrollmentFlow(v)
	if manager == nil {
		return SteamGuardPromotion{}, ErrSteamAuthenticationState
	}
	wanted := strconv.FormatUint(steamID, 10)
	kind, recordID, err := recordKindForSteamID(v, wanted)
	if err != nil {
		return SteamGuardPromotion{}, err
	}
	switch {
	case recordID == "":
		return SteamGuardPromotion{}, ErrAccountNotFound
	case kind == vaultrecord.KindEnrollmentPending:
		// A promotion already under way. Report where it got to instead of
		// starting a second one: Steam has a pending authenticator either way,
		// and asking for another would only be refused.
		return s.resumedPromotion(accountID, steamID, manager)
	case kind != vaultrecord.KindLoginOnly:
		return SteamGuardPromotion{}, ErrNotLoginOnly
	}

	record, err := recordForSteamID64(v, wanted)
	if err != nil {
		return SteamGuardPromotion{}, err
	}
	accountName := record.AccountName()
	accessToken := record.AccessToken()
	refreshToken := record.RefreshToken()
	record.destroy()

	capabilityRefreshed := false
	needsLogin := func(reason string) SteamGuardPromotion {
		return SteamGuardPromotion{
			NeedsLogin: true, Reason: reason, AccountName: accountName,
			CapabilityRefreshRequired: capabilityRefreshed,
		}
	}
	if accessToken == "" && refreshToken == "" {
		return needsLogin("no_session"), nil
	}
	if accessToken == "" || sessionrefresh.AccessTokenExpired(accessToken, time.Now(), accessTokenSkew) {
		if refreshToken == "" {
			return needsLogin("token_expired"), nil
		}
		renewed, reason, err := s.renewForPromotion(accountID, steamID, v)
		if err != nil {
			return SteamGuardPromotion{}, err
		}
		if reason != "" {
			return needsLogin(reason), nil
		}
		// The renewal wrote to the vault, so the caller's capability is stale
		// whatever happens from here.
		capabilityRefreshed = true
		accessToken = renewed.access
		refreshToken = renewed.refresh
		if accessToken == "" {
			return needsLogin("no_session"), nil
		}
	}

	ctx, finish, err := s.startBoundSteamOperation(accountID)
	if err != nil {
		return SteamGuardPromotion{}, err
	}
	defer finish()
	access := []byte(accessToken)
	refresh := []byte(refreshToken)
	defer wipe(access)
	defer wipe(refresh)
	status, err := manager.Start(ctx, enrollmentflow.StartRequest{
		SteamID: steamID, AccessToken: access, RefreshToken: refresh,
		AuthenticatorTime: uint64(s.authenticatorTime()), ReplaceLoginOnly: true,
	})
	if err != nil {
		authflowLogger().Warn("login-only account could not be promoted",
			"steamId64", accountID, "error", err)
		return SteamGuardPromotion{}, err
	}
	result := enrollmentResult(status)
	promotion := SteamGuardPromotion{CapabilityRefreshRequired: capabilityRefreshed}
	if status.Pending {
		// The pending record replaced the login-only one, so the account is no
		// longer a bare session even though it is not an authenticator yet.
		result.CapabilityRefreshRequired = true
		result.RegistryUpdated = s.upsertRegistry(accountID, registry.StatePending)
		promotion.CapabilityRefreshRequired = true
		promotion.RegistryUpdated = result.RegistryUpdated
	}
	promotion.Enrollment = &result
	authflowLogger().Info("login-only account promoted to a Steam Guard enrollment",
		"steamId64", accountID, "state", result.State, "pending", result.Pending)
	return promotion, nil
}

// resumedPromotion reports an enrollment that is already part-way through.
func (s *Service) resumedPromotion(accountID string, steamID uint64, manager steamEnrollmentManager) (SteamGuardPromotion, error) {
	status, err := manager.Resume(steamID)
	if err != nil {
		if enrollmentflow.IsNoPending(err) {
			// The slot says pending but the flow disagrees, so there is nothing
			// to carry on with and nothing safe to overwrite either.
			return SteamGuardPromotion{}, ErrRecordPending
		}
		return SteamGuardPromotion{}, err
	}
	result := enrollmentResult(status)
	result.RegistryUpdated = s.upsertRegistry(accountID, registry.StatePending)
	return SteamGuardPromotion{Enrollment: &result, RegistryUpdated: result.RegistryUpdated}, nil
}

type renewedSession struct {
	access  string
	refresh string
}

// renewForPromotion renews the stored session and re-reads it. A non-empty
// reason means the session is genuinely unusable and the user must sign in;
// only an infrastructure failure comes back as an error.
func (s *Service) renewForPromotion(accountID string, steamID uint64, v *vault.Vault) (renewedSession, string, error) {
	if s.newSessionRefresher == nil {
		return renewedSession{}, "", ErrSteamAuthenticationState
	}
	ctx, finish, err := s.startBoundSteamOperation(accountID)
	if err != nil {
		return renewedSession{}, "", err
	}
	_, refreshErr := s.newSessionRefresher(v).Refresh(ctx, steamID)
	finish()
	if refreshErr != nil {
		if reason, ok := refreshFailureReason(refreshErr); ok {
			serviceLogger().Info("stored session could not be renewed for a Steam Guard promotion",
				"steamId64", accountID, "reason", reason)
			return renewedSession{}, reason, nil
		}
		return renewedSession{}, "", refreshErr
	}
	record, err := recordForSteamID64(v, strconv.FormatUint(steamID, 10))
	if err != nil {
		return renewedSession{}, "", err
	}
	defer record.destroy()
	return renewedSession{access: record.AccessToken(), refresh: record.RefreshToken()}, "", nil
}
