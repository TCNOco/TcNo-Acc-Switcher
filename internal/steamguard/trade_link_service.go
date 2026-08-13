package steamguard

import (
	"context"
	"time"

	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
	"TcNo-Acc-Switcher/internal/steamguard/tradelink"
)

// tradeLinkTimeout bounds the single request this makes. Shorter than the
// sweeps' budgets: someone is watching a toast, not a background job.
const tradeLinkTimeout = 20 * time.Second

// SteamTradeLinkResult carries one account's current trade URL, or why it could
// not be read.
//
// State mirrors the confirmations flow's vocabulary so a stale session is
// classified the same way everywhere, with one addition: "unavailable" is a page
// that was read but carried no trade URL, which is a different thing from a
// failure and needs different words on screen.
type SteamTradeLinkResult struct {
	URL          string `json:"url,omitempty"`
	State        string `json:"state"`
	RetryAfterMS int64  `json:"retryAfterMs,omitempty"`
	// NeedsLogin says the stored session could not be renewed, so this account
	// needs the user to sign in again before it can be read.
	NeedsLogin bool `json:"needsLogin"`
	// CapabilityRefreshRequired reports that renewing the session rotated the
	// vault generation, so the capability the caller used is spent.
	CapabilityRefreshRequired bool `json:"capabilityRefreshRequired"`
}

// GetSteamTradeLink reads an account's current trade URL from Steam.
//
// Fetched on every call and never stored. The token in a trade URL is rotated
// whenever the user presses "Create New URL" on Steam's own settings page, and
// nothing tells this app when that happened - so a cached link is a link that
// silently stops working, which is worse than no link at all.
//
// Works for every record shape that holds a session: a login-only account has
// the same access token a full authenticator does, and reading this page needs
// no authenticator secrets.
func (s *Service) GetSteamTradeLink(accountID, token string) (SteamTradeLinkResult, error) {
	// Authorized once, up front. The renewal below writes to the vault and
	// rotates the generation this very capability is bound to, so checking it
	// again afterwards would reject the caller for a side effect of its own call.
	v, _, steamID, err := s.authorizeSteamFlow(accountID, token)
	if err != nil {
		return SteamTradeLinkResult{}, err
	}
	log := serviceLogger()

	refreshed, err := s.ensureFreshSessionAuthorized(v, accountID, steamID)
	if err != nil {
		return SteamTradeLinkResult{}, err
	}
	result := SteamTradeLinkResult{CapabilityRefreshRequired: refreshed.CapabilityRefreshRequired}
	if refreshed.NeedsLogin {
		result.State, result.NeedsLogin = "reauth", true
		return result, nil
	}

	record, err := recordForSteamID64(v, accountID)
	if err != nil {
		return SteamTradeLinkResult{}, err
	}
	credentials := confirmationapi.Credentials{SteamID: accountID, AccessToken: record.AccessToken()}
	sessionID := record.SessionID()
	record.destroy()

	if credentials.AccessToken == "" {
		result.State, result.NeedsLogin = "reauth", true
		return result, nil
	}
	if sessionID == "" {
		// Imported and login-only records routinely carry none. Steam accepts any
		// client-chosen value, so mint one for this request rather than writing it
		// back, which would rotate the generation and drop the live capability.
		if sessionID, err = newSessionID(); err != nil {
			return SteamTradeLinkResult{}, err
		}
	}
	credentials.SessionID = sessionID

	ctx, cancel := context.WithTimeout(context.Background(), tradeLinkTimeout)
	defer cancel()
	body, err := s.confirmationClient.FetchTradeOfferPrivacyPage(ctx, credentials)
	if err != nil {
		failure := confirmationFailure(err)
		logConfirmationFailure(log, "tradelink", accountID, failure.State, err)
		result.State, result.RetryAfterMS = failure.State, failure.RetryAfterMS
		result.NeedsLogin = failure.State == "reauth"
		return result, nil
	}

	parsed := tradelink.Parse(body)
	switch parsed.Outcome {
	case tradelink.OutcomeNotSignedIn:
		// Steam serves the login page with a 200 in some flows, so this is the
		// only place that reading is available.
		log.Info("Steam trade link needs re-authentication", "steamId64", accountID)
		result.State, result.NeedsLogin = "reauth", true
	case tradelink.OutcomeParsed:
		// The page belongs to one account, so the only link on it is that
		// account's - but a wrong trade URL is indistinguishable from a right one
		// once it is on the clipboard, so it is worth proving rather than assuming.
		if !linkBelongsTo(parsed.Partner, accountID) {
			log.Warn("Steam trade link did not match the account it was read for", "steamId64", accountID)
			result.State = "unavailable"
			break
		}
		log.Info("Steam trade link read", "steamId64", accountID)
		result.State, result.URL = "ok", parsed.URL
	default:
		log.Warn("Steam trade link page was not understood", "steamId64", accountID, "bodyBytes", len(body))
		result.State = "unavailable"
	}
	return result, nil
}

// linkBelongsTo reports whether a trade URL's partner id is the account id of
// steamID64. partner is the 32-bit account id, which is exactly what the shared
// SteamID formatter already derives.
func linkBelongsTo(partner, steamID64 string) bool {
	formats, err := steam.FormatsFromID64(steamID64)
	return err == nil && partner != "" && partner == formats.ID32
}
