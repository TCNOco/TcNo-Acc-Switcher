package steamguard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
	"TcNo-Acc-Switcher/internal/steamguard/sessionrefresh"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

// TestOwnedAppsSpike answers whether a vault access token can list an account's
// owned apps, and which of the two candidate routes actually returns them.
//
// Both are unverified against this app's tokens. GetOwnedGames is documented as
// key-authenticated and only exempts the privacy rule for "your own" account -
// which an access token should satisfy, but nothing here proves Steam agrees for
// a mobile-audience token. dynamicstore/userdata has the better coverage (every
// appid, DLC included) but is an undocumented store-session endpoint, and this
// app's tokens have only ever been exercised against steamcommunity.com.
//
// Neither request widens the host policy: api.steampowered.com is already on the
// RouteRequest allowlist, and the store request reuses RouteTransfer exactly as
// FetchCS2StorePage does.
//
// It touches a real vault and makes real requests, so it is opt-in, and it
// probes exactly one account - a stored token is short-lived, so a sweep over
// the whole vault mostly measures how long ago each record was last used:
//
//	TCNO_OWNED_SPIKE=1 TCNO_VAULT_PASSWORD='...' TCNO_SPIKE_LIST=1 go test ./internal/steamguard/ -run TestOwnedAppsSpike -v
//	TCNO_OWNED_SPIKE=1 TCNO_VAULT_PASSWORD='...' TCNO_SPIKE_ACCOUNT=<login name> go test ./internal/steamguard/ -run TestOwnedAppsSpike -v
//
// Set TCNO_DATA_ROOT when the vault is not under the default data root, and
// TCNO_SPIKE_DUMP_DIR to keep the response bodies.
func TestOwnedAppsSpike(t *testing.T) {
	if os.Getenv("TCNO_OWNED_SPIKE") != "1" {
		t.Skip("opt-in: set TCNO_OWNED_SPIKE=1 to run the live owned-apps probe")
	}
	password := os.Getenv("TCNO_VAULT_PASSWORD")
	if password == "" {
		t.Fatal("TCNO_VAULT_PASSWORD is required to unlock the vault")
	}
	initSpikeDataRoot(t)

	vaultRoot, err := VaultFolderPath()
	if err != nil {
		t.Fatalf("vault folder: %v", err)
	}
	v, err := vault.Open(vaultRoot)
	if err != nil {
		t.Fatalf("open vault at %s: %v", vaultRoot, err)
	}
	if err := v.Unlock(password, vault.FixedLease); err != nil {
		t.Fatalf("unlock vault: %v", err)
	}
	defer func() { _ = v.Lock() }()

	records, err := v.List()
	if err != nil {
		t.Fatalf("list records: %v", err)
	}

	// Listing mode: no network at all. It reports whether each record can be
	// refreshed, because that - not the stale access token - decides whether an
	// account is probeable.
	if os.Getenv("TCNO_SPIKE_LIST") == "1" {
		for _, record := range records {
			loaded, err := recordFromVault(v, record.ID)
			if err != nil {
				t.Logf("%s  (unreadable: %v)", record.SteamID64, err)
				continue
			}
			t.Logf("%s  %-24s  kind=%-13v refreshable=%v  access %s",
				record.SteamID64, loaded.AccountName(), loaded.Kind,
				loaded.RefreshToken() != "", describeExpiry(loaded.AccessToken()))
			loaded.destroy()
		}
		t.Skip("listing only: re-run with TCNO_SPIKE_ACCOUNT or TCNO_SPIKE_STEAMID set to one of the rows above")
	}

	wantedID := strings.TrimSpace(os.Getenv("TCNO_SPIKE_STEAMID"))
	wantedName := strings.TrimSpace(os.Getenv("TCNO_SPIKE_ACCOUNT"))
	if wantedID == "" && wantedName == "" {
		t.Fatal("set TCNO_SPIKE_ACCOUNT (login name) or TCNO_SPIKE_STEAMID so the probe cannot pick an " +
			"account you did not choose. Run once with TCNO_SPIKE_LIST=1 to see what is in your vault.")
	}
	dumpDir := strings.TrimSpace(os.Getenv("TCNO_SPIKE_DUMP_DIR"))

	var (
		steamID64    string
		accountName  string
		accessToken  string
		refreshToken string
		sessionID    string
	)
	for _, record := range records {
		if wantedID != "" && record.SteamID64 != wantedID {
			continue
		}
		loaded, err := recordFromVault(v, record.ID)
		if err != nil {
			t.Logf("record %s unreadable: %v", record.SteamID64, err)
			continue
		}
		name := loaded.AccountName()
		if wantedName != "" && !strings.EqualFold(strings.TrimSpace(name), wantedName) {
			loaded.destroy()
			continue
		}
		steamID64, accountName = record.SteamID64, name
		accessToken, refreshToken, sessionID = loaded.AccessToken(), loaded.RefreshToken(), loaded.SessionID()
		t.Logf("=== %s / %s (kind=%v)", record.SteamID64, name, loaded.Kind)
		loaded.destroy()
		break
	}
	if steamID64 == "" {
		t.Fatalf("no vault record matched %q%q", wantedName, wantedID)
	}
	if sessionID == "" {
		if sessionID, err = newSessionID(); err != nil {
			t.Fatalf("mint session id: %v", err)
		}
	}

	t.Logf("    stored access token: audience=%v %s", tokenAudience(accessToken), describeExpiry(accessToken))
	accessToken = freshAccessToken(t, steamID64, accessToken, refreshToken)

	client := protocol.NewClient(protocol.Options{})
	probeGetOwnedGames(t, client, steamID64, accessToken, dumpDir, accountName)
	probeDynamicStore(t, client, steamID64, accessToken, sessionID, dumpDir, accountName)
}

// freshAccessToken mints a token for this run rather than trusting the stored
// one, which is the whole reason the first pass of this probe was inconclusive:
// every record answered HTTP 401, which is what an expired token and a rejected
// audience look like alike.
//
// Renewal is deliberately RenewalNone. RenewalAllow can hand back a rotated
// refresh token, and this probe never writes the vault - accepting a rotation it
// cannot persist would leave the vault holding a superseded token.
func freshAccessToken(t *testing.T, steamID64, stored, refresh string) string {
	t.Helper()
	if refresh == "" {
		t.Log("    no refresh token on this record: probing with the stored access token as-is")
		return stored
	}
	id, err := strconv.ParseUint(steamID64, 10, 64)
	if err != nil {
		t.Fatalf("parse steamid %q: %v", steamID64, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	auth := protocol.NewAuthenticationClient(protocol.NewClient(protocol.Options{}))
	result, err := auth.GenerateAccessTokenForApp(ctx, protocol.GenerateAccessTokenRequest{
		SteamID:      id,
		RefreshToken: refresh,
		Renewal:      protocol.RenewalNone,
	}, 30*time.Second)
	if err != nil {
		t.Fatalf("    GenerateAccessTokenForApp FAILED: %v\n"+
			"    (a refused refresh token means this account needs a real login before it can be probed)", err)
	}
	if result.AccessToken == "" {
		t.Fatalf("    GenerateAccessTokenForApp returned no token (state=%v)", result.State)
	}
	t.Logf("    minted access token: audience=%v %s", tokenAudience(result.AccessToken), describeExpiry(result.AccessToken))
	return result.AccessToken
}

// probeGetOwnedGames is the route worth shipping if it works: plain JSON on a
// host already permitted for requests, no cookie session, no store scraping.
//
// An empty response object is the interesting failure. That is what Steam
// returns when the caller is not authorised to see the library, so it means the
// token was ignored rather than rejected - which no status code would reveal.
func probeGetOwnedGames(t *testing.T, client *protocol.Client, steamID, token, dumpDir, name string) {
	t.Helper()

	query := url.Values{
		"access_token":              {token},
		"steamid":                   {steamID},
		"include_appinfo":           {"1"},
		"include_played_free_games": {"1"},
		"include_free_sub":          {"1"},
		"skip_unvetted_apps":        {"0"},
	}
	endpoint := "https://api.steampowered.com/IPlayerService/GetOwnedGames/v1?" + query.Encode()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	response, err := client.Do(ctx, protocol.Request{
		Method: http.MethodGet, Endpoint: endpoint, Route: protocol.RouteRequest,
		Timeout: 60 * time.Second, MaxResponseBytes: protocol.MaxResponseBodyBytes,
	})
	if err != nil {
		t.Logf("    GetOwnedGames  FAILED: %v", err)
		return
	}
	dumpSpikeBody(t, dumpDir, name+"-getownedgames.json", response.Body)

	var owned struct {
		Response struct {
			GameCount int `json:"game_count"`
			Games     []struct {
				AppID int    `json:"appid"`
				Name  string `json:"name"`
			} `json:"games"`
		} `json:"response"`
	}
	if err := json.Unmarshal(response.Body, &owned); err != nil {
		t.Logf("    GetOwnedGames  %d bytes, not JSON: %v", len(response.Body), err)
		return
	}
	if owned.Response.GameCount == 0 && len(owned.Response.Games) == 0 {
		t.Logf("    GetOwnedGames  EMPTY response - the access token was not accepted as this account's own auth")
		return
	}
	t.Logf("    GetOwnedGames  OK: game_count=%d, %d returned. %s",
		owned.Response.GameCount, len(owned.Response.Games), sampleGames(owned.Response.Games))
}

// probeDynamicStore checks the higher-coverage route. rgOwnedApps carries every
// appid on the account including DLC, which GetOwnedGames omits by design.
//
// A signed-out request still answers 200 with the same envelope and empty
// arrays, so zero owned apps is the signal that the cookie was not honoured -
// most likely because the token's audience does not cover the store.
func probeDynamicStore(t *testing.T, client *protocol.Client, steamID, token, session, dumpDir, name string) {
	t.Helper()

	cookie := "steamLoginSecure=" + steamID + "%7C%7C" + token +
		"; sessionid=" + session + "; Steam_Language=english"
	headers := make(http.Header)
	headers.Set("Cookie", cookie)
	headers.Set("Accept", "application/json")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// RouteTransfer, not RouteRequest: store.steampowered.com is only on the
	// transfer allowlist. Same permission FetchCS2StorePage already uses.
	response, err := client.Do(ctx, protocol.Request{
		Method:   http.MethodGet,
		Endpoint: "https://store.steampowered.com/dynamicstore/userdata/",
		Route:    protocol.RouteTransfer,
		Header:   headers, Timeout: 60 * time.Second,
		MaxResponseBytes: protocol.MaxResponseBodyBytes,
		AllowRedirects:   true,
	})
	if err != nil {
		t.Logf("    dynamicstore   FAILED: %v", err)
		return
	}
	dumpSpikeBody(t, dumpDir, name+"-userdata.json", response.Body)

	var userdata struct {
		OwnedApps     []int `json:"rgOwnedApps"`
		OwnedPackages []int `json:"rgOwnedPackages"`
		Wishlist      []int `json:"rgWishlist"`
	}
	if err := json.Unmarshal(response.Body, &userdata); err != nil {
		t.Logf("    dynamicstore   %d bytes, not JSON: %v", len(response.Body), err)
		return
	}
	if len(userdata.OwnedApps) == 0 && len(userdata.OwnedPackages) == 0 {
		t.Logf("    dynamicstore   EMPTY (%d bytes) - the store did not accept this session cookie",
			len(response.Body))
		return
	}
	t.Logf("    dynamicstore   OK: %d owned apps, %d packages, %d wishlist. %s",
		len(userdata.OwnedApps), len(userdata.OwnedPackages), len(userdata.Wishlist),
		sampleAppIDs(userdata.OwnedApps))
}

func sampleGames(games []struct {
	AppID int    `json:"appid"`
	Name  string `json:"name"`
}) string {
	if len(games) == 0 {
		return ""
	}
	parts := make([]string, 0, 3)
	for i, game := range games {
		if i == 3 {
			break
		}
		parts = append(parts, fmt.Sprintf("%d %s", game.AppID, game.Name))
	}
	return "first: " + strings.Join(parts, ", ")
}

func sampleAppIDs(appIDs []int) string {
	if len(appIDs) == 0 {
		return ""
	}
	sorted := append([]int(nil), appIDs...)
	sort.Ints(sorted)
	parts := make([]string, 0, 5)
	for i, appID := range sorted {
		if i == 5 {
			break
		}
		parts = append(parts, fmt.Sprint(appID))
	}
	return "lowest: " + strings.Join(parts, ", ")
}

func describeExpiry(token string) string {
	expiry, ok := sessionrefresh.AccessTokenExpiry(token)
	if !ok {
		return "expiry=unreadable"
	}
	if remaining := time.Until(expiry); remaining <= 0 {
		return fmt.Sprintf("expiry=%s (EXPIRED - refresh before trusting a failure below)",
			expiry.Format(time.RFC3339))
	}
	return fmt.Sprintf("expiry=%s (%s left)", expiry.Format(time.RFC3339),
		time.Until(expiry).Truncate(time.Minute))
}

// dumpSpikeBody writes one response for offline inspection.
//
// These bodies name the account and everything it owns. Write them 0600 and
// delete them once whatever question prompted the dump is settled.
func dumpSpikeBody(t *testing.T, dir, filename string, body []byte) {
	t.Helper()
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Logf("could not create dump dir: %v", err)
		return
	}
	path := filepath.Join(dir, sanitiseDumpName(filename))
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Logf("could not write %s: %v", path, err)
		return
	}
	t.Logf("    wrote %s (%d bytes)", path, len(body))
}

// sanitiseDumpName keeps a login name from steering the dump out of its
// directory. Vault contents are trusted, but a filename built from one is the
// kind of thing that stops being true later.
func sanitiseDumpName(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-', r == '_', r == '.':
			return r
		default:
			return '_'
		}
	}, name)
}
