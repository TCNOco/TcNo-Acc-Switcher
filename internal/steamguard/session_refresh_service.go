package steamguard

import (
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/sessionrefresh"
)

// SteamSessionRefreshState is SteamSessionState plus what a renewal did to the
// caller's capability. Renewing writes to the vault, which rotates its generation
// and invalidates every capability issued against the old one, so a caller holding
// one must take a fresh capability before its next call.
type SteamSessionRefreshState struct {
	SteamSessionState
	CapabilityRefreshRequired bool `json:"capabilityRefreshRequired"`
}

// EnsureFreshSession renews a lapsed session from the stored refresh token before
// reporting on it, so an account left alone for longer than its access token lives
// does not ask for a password it does not need.
//
// The two tokens run on very different clocks: Steam's access token lasts about a
// day, the refresh token beside it months. Judging a session on the access token
// alone - which is all SteamSessionLocalState can do without writing - makes the
// shorter clock decide how often the user signs in, and that is the whole reason
// re-logins were showing up every few days.
//
// Unlike SteamSessionLocalState this can write to the vault, so it belongs only on
// paths that open a single account. The listing path must keep using
// localSessionStatus: one renewal per row would rotate the vault generation once
// per row and invalidate the window's capability under it every time.
func (s *Service) EnsureFreshSession(accountID, token string) (SteamSessionRefreshState, error) {
	v, _, steamID, err := s.authorizeSteamFlow(accountID, token)
	if err != nil {
		return SteamSessionRefreshState{}, err
	}
	record, err := recordForSteamID64(v, accountID)
	if err != nil {
		return SteamSessionRefreshState{}, err
	}
	// Only the verdicts are kept, never the tokens: Refresh reads what it needs
	// from the vault itself, so nothing secret has to outlive this block.
	accessToken := record.AccessToken()
	hasAccessToken := accessToken != ""
	accessTokenLive := hasAccessToken &&
		!sessionrefresh.AccessTokenExpired(accessToken, time.Now(), accessTokenSkew)
	hasRefreshToken := record.RefreshToken() != ""
	record.destroy()

	if accessTokenLive {
		// Renewing a working session would rotate the vault generation, and so
		// invalidate the caller's capability, for no gain.
		return SteamSessionRefreshState{}, nil
	}
	if !hasRefreshToken {
		reason := "token_expired"
		if !hasAccessToken {
			reason = "no_session"
		}
		return SteamSessionRefreshState{SteamSessionState: SteamSessionState{NeedsLogin: true, Reason: reason}}, nil
	}
	if s.newSessionRefresher == nil {
		return SteamSessionRefreshState{}, ErrSteamAuthenticationState
	}

	ctx, finish, err := s.startBoundSteamOperation(accountID)
	if err != nil {
		return SteamSessionRefreshState{}, err
	}
	defer finish()

	if _, refreshErr := s.newSessionRefresher(v).Refresh(ctx, steamID); refreshErr != nil {
		if reason, ok := refreshFailureReason(refreshErr); ok {
			// The stored session genuinely cannot be renewed, so the user does have
			// to sign in. Every one of these classes aborts before the vault write,
			// so the caller's capability is untouched.
			serviceLogger().Info("Steam session could not be renewed",
				"steamId64", accountID, "reason", reason)
			return SteamSessionRefreshState{SteamSessionState: SteamSessionState{NeedsLogin: true, Reason: reason}}, nil
		}
		// Infrastructure, not a verdict. A network blip or a timeout says nothing
		// about the session, and answering it with a password form would be the
		// same false alarm this function exists to remove.
		serviceLogger().Warn("Steam session renewal failed", "steamId64", accountID, "error", refreshErr)
		return SteamSessionRefreshState{}, refreshErr
	}
	serviceLogger().Info("Steam session renewed from the stored refresh token", "steamId64", accountID)
	return SteamSessionRefreshState{CapabilityRefreshRequired: true}, nil
}
