package steambrowser

import (
	"strings"
	"testing"
)

const (
	testSteamID = "76561198000000001"
	testToken   = "eyJhbGciOiJIUzI1NiJ9.payload.signature"
	testSession = "0123456789ABCDEF0123456789ABCDEF"
)

func TestSessionCookiesCoverEveryDomain(t *testing.T) {
	cookies, err := SessionCookies(testSteamID, testToken, testSession)
	if err != nil {
		t.Fatalf("SessionCookies: %v", err)
	}

	byDomain := map[string]map[string]string{}
	for _, c := range cookies {
		if byDomain[c.Domain] == nil {
			byDomain[c.Domain] = map[string]string{}
		}
		byDomain[c.Domain][c.Name] = c.Value
		if c.Path != "/" {
			t.Errorf("cookie %s on %s has path %q, want /", c.Name, c.Domain, c.Path)
		}
	}

	// steamcommunity.com and steampowered.com are separate registrable domains, so
	// a session planted on one is invisible to the other.
	for _, domain := range []string{"steamcommunity.com", "store.steampowered.com", "help.steampowered.com"} {
		set, ok := byDomain[domain]
		if !ok {
			t.Fatalf("no cookies for %s", domain)
		}
		want := testSteamID + "%7C%7C" + testToken
		if set["steamLoginSecure"] != want {
			t.Errorf("%s steamLoginSecure = %q, want %q", domain, set["steamLoginSecure"], want)
		}
		if set["sessionid"] != testSession {
			t.Errorf("%s sessionid = %q, want %q", domain, set["sessionid"], testSession)
		}
		// Without a birth date the store age-gates mature apps and serves nothing.
		if set["birthtime"] == "" || set["lastagecheckage"] == "" {
			t.Errorf("%s is missing the age-check cookies", domain)
		}
	}
}

// The mobile marker asks Steam for its mobile shell; these windows are meant to
// look like the desktop site.
func TestSessionCookiesOmitMobileClient(t *testing.T) {
	cookies, err := SessionCookies(testSteamID, testToken, testSession)
	if err != nil {
		t.Fatalf("SessionCookies: %v", err)
	}
	for _, c := range cookies {
		if strings.EqualFold(c.Name, "mobileClient") || strings.EqualFold(c.Name, "mobileClientVersion") {
			t.Errorf("unexpected mobile cookie %s=%s", c.Name, c.Value)
		}
	}
}

func TestSessionCookiesRejectsBadInput(t *testing.T) {
	tests := []struct {
		name                      string
		steamID, token, sessionID string
	}{
		{"empty steam id", "", testToken, testSession},
		{"zero steam id", "0", testToken, testSession},
		{"non-numeric steam id", "not-a-number", testToken, testSession},
		{"leading zero steam id", "076561198000000001", testToken, testSession},
		{"short token", "76561198000000001", "short", testSession},
		{"token with a semicolon", testSteamID, "abc;Domain=evil.tld", testSession},
		{"token with a comma", testSteamID, "abcdefghijklmnop,x", testSession},
		{"token with whitespace", testSteamID, "abcdefghijklmnop x", testSession},
		{"token with a newline", testSteamID, "abcdefghijklmnop\nx", testSession},
		{"non-hex session id", testSteamID, testToken, "zzzzzzzzzzzzzzzzzzzz"},
		{"short session id", testSteamID, testToken, "0123"},
		{"empty session id", testSteamID, testToken, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := SessionCookies(test.steamID, test.token, test.sessionID); err == nil {
				t.Error("got nil error, want a rejection")
			}
		})
	}
}

func TestProfileNameIsPerAccount(t *testing.T) {
	first, err := ProfileName(testSteamID)
	if err != nil {
		t.Fatalf("ProfileName: %v", err)
	}
	second, err := ProfileName("76561198000000002")
	if err != nil {
		t.Fatalf("ProfileName: %v", err)
	}
	if first == second {
		t.Errorf("two accounts share the profile %q, so they would share a cookie jar", first)
	}
	if _, err := ProfileName("../escape"); err == nil {
		t.Error("a path-traversing Steam ID produced a profile name")
	}
}

func TestNewSessionIDShape(t *testing.T) {
	id, err := NewSessionID()
	if err != nil {
		t.Fatalf("NewSessionID: %v", err)
	}
	if len(id) != 32 || !isHex(id) || id != strings.ToUpper(id) {
		t.Errorf("NewSessionID() = %q, want 32 uppercase hex characters", id)
	}
	other, err := NewSessionID()
	if err != nil {
		t.Fatalf("NewSessionID: %v", err)
	}
	if id == other {
		t.Error("NewSessionID returned the same value twice")
	}
}
