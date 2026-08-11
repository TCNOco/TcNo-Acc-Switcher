package steambrowser

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidSession rejects credentials that could not produce a working window.
var ErrInvalidSession = errors.New("steambrowser: invalid Steam session")

// Cookie is one cookie to plant before the first navigation. It is deliberately
// free of any webview type so the host backends can each translate it.
type Cookie struct {
	Name   string
	Value  string
	Domain string
	Path   string
}

// sessionDomains are the hosts a Steam web session has to be planted on.
// steamcommunity.com and steampowered.com are separate registrable domains, so
// neither inherits the other's cookies and each needs its own copy.
var sessionDomains = []string{
	"steamcommunity.com",
	"store.steampowered.com",
	"help.steampowered.com",
}

const (
	maxAccessTokenLen = 4096
	minAccessTokenLen = 16
)

// SessionCookies builds the cookies that make a window browse as this account.
//
// The shape mirrors what the app already sends on its own Steam requests, in
// internal/steamguard/confirmationapi: steamLoginSecure is the SteamID and the
// access token joined by a percent-encoded pair of pipes, which is how Steam
// itself stores the value.
//
// Deliberately absent is mobileClient. Setting it asks Steam for the mobile
// shell, and these windows are meant to look like the desktop site.
func SessionCookies(steamID64, accessToken, sessionID string) ([]Cookie, error) {
	if err := validateSession(steamID64, accessToken, sessionID); err != nil {
		return nil, err
	}

	login := steamID64 + "%7C%7C" + accessToken
	cookies := make([]Cookie, 0, len(sessionDomains)*5)
	for _, domain := range sessionDomains {
		cookies = append(cookies,
			Cookie{Name: "steamLoginSecure", Value: login, Domain: domain, Path: "/"},
			// sessionid pairs with the CSRF token in Steam's own forms. Steam accepts
			// any well-formed value, and every request from this window carries the
			// same one.
			Cookie{Name: "sessionid", Value: sessionID, Domain: domain, Path: "/"},
			Cookie{Name: "Steam_Language", Value: "english", Domain: domain, Path: "/"},
			// Without a birth date the store interposes an age gate on mature-rated
			// apps and serves none of the page.
			Cookie{Name: "birthtime", Value: "283996801", Domain: domain, Path: "/"},
			Cookie{Name: "lastagecheckage", Value: "1-January-1979", Domain: domain, Path: "/"},
		)
	}
	return cookies, nil
}

// NewSessionID returns 32 uppercase hex characters, the shape Steam's own clients
// use for the sessionid cookie.
func NewSessionID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("steambrowser: generate session id: %w", err)
	}
	return strings.ToUpper(hex.EncodeToString(raw)), nil
}

// ProfileName is the WebView2 profile an account's storage lives under. Deriving
// it from the SteamID keeps one account's cookies out of every other account's
// window, and keeps the name inside the character set a profile directory allows.
func ProfileName(steamID64 string) (string, error) {
	if !validSteamID64(steamID64) {
		return "", fmt.Errorf("%w: bad Steam ID", ErrInvalidSession)
	}
	return "acct-" + steamID64, nil
}

func validateSession(steamID64, accessToken, sessionID string) error {
	if !validSteamID64(steamID64) {
		return fmt.Errorf("%w: bad Steam ID", ErrInvalidSession)
	}
	if len(accessToken) < minAccessTokenLen || len(accessToken) > maxAccessTokenLen {
		return fmt.Errorf("%w: access token length %d", ErrInvalidSession, len(accessToken))
	}
	if !safeCookieValue(accessToken) {
		return fmt.Errorf("%w: access token is not a safe cookie value", ErrInvalidSession)
	}
	if len(sessionID) < 16 || len(sessionID) > 64 || !isHex(sessionID) {
		return fmt.Errorf("%w: session id must be 16-64 hex characters", ErrInvalidSession)
	}
	return nil
}

func validSteamID64(value string) bool {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return false
	}
	// Reject anything that does not round-trip, which rules out leading zeroes and
	// signs that would otherwise reach the cookie value.
	return strconv.FormatUint(parsed, 10) == value
}

// safeCookieValue rejects anything that could terminate the cookie or start a new
// attribute. A JWT never contains these, so a value that does is not a token this
// package should be planting.
func safeCookieValue(value string) bool {
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c <= 0x20 || c >= 0x7f || c == ';' || c == ',' || c == '"' || c == '\\' {
			return false
		}
	}
	return true
}

func isHex(value string) bool {
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return len(value) > 0
}
