package steamguard

import (
	"errors"
	"strconv"

	"TcNo-Acc-Switcher/internal/steambrowser"
)

// ErrBrowserSessionNeedsLogin reports an account whose stored session can no
// longer be renewed, so the user has to sign in before a window can open.
var ErrBrowserSessionNeedsLogin = errors.New("steamguard: sign in again to open a browser window")

// browserSessionSource adapts the vault to what the session browser needs.
//
// It is a separate unexported type rather than a method on Service on purpose.
// Service is registered with Wails, so every exported method on it becomes a
// frontend binding — and this one returns an access token, which must never be
// reachable from JavaScript. Keeping it here means only Go can call it.
type browserSessionSource struct{ service *Service }

// NewBrowserSessionSource lets the session browser draw sessions from the vault.
// A plain function, not a method, so it is not bound either.
func NewBrowserSessionSource(service *Service) steambrowser.SessionSource {
	return browserSessionSource{service: service}
}

// BrowserSession hands the session browser one account's web credentials.
//
// It works for every record shape the vault holds. A login-only record carries
// the same tokens a full authenticator does — the refresh token is proof a Steam
// Guard challenge was already satisfied — so both open a window; only the
// authenticator's secrets, which browsing never needs, are absent from one.
//
// The modal token is checked exactly as any other sensitive view's is, so a
// window cannot be opened without the user having unlocked the vault for this
// account.
func (b browserSessionSource) BrowserSession(accountID, modalToken string) (steambrowser.WebSession, error) {
	// Renew first. A window's cookies are minted once and then outlive the vault
	// lock, so it is worth opening with a fresh token rather than one about to
	// lapse mid-session.
	refreshed, err := b.service.EnsureFreshSession(accountID, modalToken)
	if err != nil {
		return steambrowser.WebSession{}, err
	}
	if refreshed.NeedsLogin {
		return steambrowser.WebSession{}, ErrBrowserSessionNeedsLogin
	}

	v, _, steamID, err := b.service.authorizeSteamFlow(accountID, modalToken)
	if err != nil {
		return steambrowser.WebSession{}, err
	}
	record, err := recordForSteamID64(v, accountID)
	if err != nil {
		return steambrowser.WebSession{}, err
	}
	defer record.destroy()

	accessToken := record.AccessToken()
	if accessToken == "" {
		return steambrowser.WebSession{}, ErrBrowserSessionNeedsLogin
	}
	sessionID := record.SessionID()
	if sessionID == "" {
		// Imported records often carry no sessionid. Steam accepts any
		// client-chosen value, so mint one for this window rather than writing it
		// back, which would rotate the vault generation and drop the live
		// capability.
		if sessionID, err = steambrowser.NewSessionID(); err != nil {
			return steambrowser.WebSession{}, err
		}
	}

	return steambrowser.WebSession{
		SteamID64:   strconv.FormatUint(steamID, 10),
		AccountName: record.AccountName(),
		AccessToken: accessToken,
		SessionID:   sessionID,
	}, nil
}
