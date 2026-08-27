package steamguard

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
	"TcNo-Acc-Switcher/internal/steamguard/gcpd"
	"TcNo-Acc-Switcher/internal/steamguard/primestatus"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

// TestGCPDSpike answers the one question the repo cannot answer offline: does
// Steam accept this app's mobile-audience access token on the GCPD page?
//
// Everything else in the CS2 cooldown feature assumes it does. The login flow
// requests platform_type=3 / website_id="Mobile" (passwordAuthRequest), which is
// known to work for mobileconf and for the economy itemclasshover endpoint, but
// GCPD is a different gate. If Steam serves the sign-in page instead, the
// feature needs a web-audience token - and minting one means writing the vault,
// which rotates its generation and breaks the sweep's whole no-write design.
//
// It touches a real vault and makes a real request, so it is opt-in:
//
//	TCNO_GCPD_SPIKE=1 TCNO_VAULT_PASSWORD='...' go test ./internal/steamguard/ -run TestGCPDSpike -v
//
// Optionally set TCNO_DATA_ROOT when the vault is not under the default data
// root, and TCNO_SPIKE_STEAMID to pick one account out of several.
func TestGCPDSpike(t *testing.T) {
	if os.Getenv("TCNO_GCPD_SPIKE") != "1" {
		t.Skip("opt-in: set TCNO_GCPD_SPIKE=1 to run the live GCPD probe")
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

	// Listing mode: no network at all. Picking an account by SteamID64 is
	// otherwise guesswork, and the alternative - letting it default to whichever
	// record comes first - probes an account the user did not choose.
	if os.Getenv("TCNO_SPIKE_LIST") == "1" {
		for _, record := range records {
			loaded, err := recordFromVault(v, record.ID)
			if err != nil {
				t.Logf("%s  (unreadable: %v)", record.SteamID64, err)
				continue
			}
			t.Logf("%s  %-24s  kind=%v  hasToken=%v",
				record.SteamID64, loaded.AccountName(), loaded.Kind, loaded.AccessToken() != "")
			loaded.destroy()
		}
		t.Skip("listing only: re-run with TCNO_SPIKE_ACCOUNT or TCNO_SPIKE_STEAMID set to one of the rows above")
	}

	// Either identifier works. A login name is what a user actually knows; the
	// SteamID64 is there for the case where two records share a name.
	wantedID := strings.TrimSpace(os.Getenv("TCNO_SPIKE_STEAMID"))
	wantedName := strings.TrimSpace(os.Getenv("TCNO_SPIKE_ACCOUNT"))
	if wantedID == "" && wantedName == "" {
		t.Fatal("set TCNO_SPIKE_ACCOUNT (login name) or TCNO_SPIKE_STEAMID so the probe cannot pick an " +
			"account you did not choose. Run once with TCNO_SPIKE_LIST=1 to see what is in your vault.")
	}

	var target cooldownTarget
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
		token := loaded.AccessToken()
		session := loaded.SessionID()
		kind := loaded.Kind
		loaded.destroy()
		if wantedName != "" && !strings.EqualFold(strings.TrimSpace(name), wantedName) {
			continue
		}
		if token == "" {
			t.Logf("record %s (%v) has no access token", record.SteamID64, kind)
			continue
		}
		if session == "" {
			if session, err = newSessionID(); err != nil {
				t.Fatalf("mint session id: %v", err)
			}
		}
		target = cooldownTarget{steamID64: record.SteamID64, accessToken: token, sessionID: session}
		t.Logf("using account %s / %s (record kind: %v)", record.SteamID64, name, kind)
		break
	}
	if target.steamID64 == "" {
		t.Fatalf("no vault record with a usable access token for %q%q", wantedName, wantedID)
	}

	t.Logf("token audience: %v", tokenAudience(target.accessToken))

	client := confirmationapi.NewClient(confirmationapi.Options{
		Protocol: protocol.NewClient(protocol.Options{}),
		Offline:  func() bool { return false },
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	body, err := client.FetchCS2GCPD(ctx, confirmationapi.Credentials{
		SteamID:     target.steamID64,
		AccessToken: target.accessToken,
		SessionID:   target.sessionID,
	})
	if err != nil {
		t.Fatalf("GCPD fetch failed: %v", err)
	}
	t.Logf("response: %d bytes", len(body))

	if dir := strings.TrimSpace(os.Getenv("TCNO_SPIKE_DUMP_DIR")); dir != "" {
		dumpTab(t, dir, confirmationapi.GCPDTabMatchmaking, body)
		// accountmain carries the CS2 profile level and XP the C++ reference
		// infers Prime from; primeaccount is the dedicated page that turned out to
		// exist, which would answer it outright. Neither is read in production -
		// each is a second request per account - so these are dumped to settle
		// what they contain, not to ship.
		for _, tab := range []string{confirmationapi.GCPDTabAccountMain, confirmationapi.GCPDTabPrimeAccount} {
			tabCtx, tabCancel := context.WithTimeout(context.Background(), 30*time.Second)
			tabBody, tabErr := client.FetchCS2GCPDTab(tabCtx, confirmationapi.Credentials{
				SteamID:     target.steamID64,
				AccessToken: target.accessToken,
				SessionID:   target.sessionID,
			}, tab)
			tabCancel()
			if tabErr != nil {
				t.Logf("%s fetch failed: %v", tab, tabErr)
				continue
			}
			dumpTab(t, dir, tab, tabBody)
		}
	}

	result := gcpd.Parse(body, time.Now())
	t.Logf("parse outcome=%s hasCooldown=%v permanent=%v expiresAt=%v",
		result.Outcome, result.HasCooldown, result.Permanent, result.ExpiresAt)
	logSpikeRanks(t, result.Ranks)

	// After the GCPD parse, so the probe can report the verdict the sweep would
	// actually store rather than package ownership alone.
	probePrimeStatus(t, client, target, result, strings.TrimSpace(os.Getenv("TCNO_SPIKE_DUMP_DIR")))

	switch result.Outcome {
	case gcpd.OutcomeNotSignedIn:
		t.Fatal("Steam served the sign-in page: the mobile-audience token is NOT accepted on GCPD. " +
			"The feature needs a web-audience token, which is a different design - see the plan's Step 0.")
	case gcpd.OutcomeUnrecognised:
		if dump := os.Getenv("TCNO_SPIKE_DUMP"); dump != "" {
			if err := os.WriteFile(dump, body, 0o600); err != nil {
				t.Logf("could not write dump: %v", err)
			} else {
				t.Logf("wrote response to %s", dump)
			}
		}
		t.Fatal("response was not a recognisable GCPD page. Set TCNO_SPIKE_DUMP=<path> to save it and " +
			"compare against the parser's expectations (English markers, generic_kv_table, closed document).")
	}
	t.Log("GCPD is readable with the stored mobile-audience token; the feature's premise holds.")
}

// probePrimeStatus answers whether Prime is readable at all.
//
// The store page marks each purchase section the account already owns, and Prime
// is sold as a package. The two alternatives are both dead ends: GCPD's own
// "primeaccount" tab is empty even for a Prime account, and the account licenses
// page serves this client a responsive shell with no licenses table.
func probePrimeStatus(
	t *testing.T,
	client *confirmationapi.Client,
	target cooldownTarget,
	gcpdResult gcpd.Result,
	dumpDir string,
) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	body, err := client.FetchCS2StorePage(ctx, confirmationapi.Credentials{
		SteamID:     target.steamID64,
		AccessToken: target.accessToken,
		SessionID:   target.sessionID,
	})
	if err != nil {
		t.Logf("store page fetch FAILED: %v (Prime has no source if this does not resolve)", err)
		return
	}
	t.Logf("store page response: %d bytes", len(body))
	if dumpDir != "" {
		dumpTab(t, dumpDir, "storepage", body)
	}

	store := primestatus.Parse(body)
	switch store.Outcome {
	case primestatus.OutcomeParsed:
		t.Logf("PRIME: store page parsed, ownsPrimePackage=%v", store.OwnsPrimePackage)
	case primestatus.OutcomeNotSignedIn:
		t.Log("PRIME: store served a signed-out page; the community session is not accepted there")
	default:
		// Deliberately not a verdict: an unreadable page is not evidence that the
		// account lacks Prime.
		t.Log("PRIME: page not recognised (age gate, region, or layout change) - INCONCLUSIVE")
	}

	// The shipped rule, not a restatement of it, so a run verifies what the
	// sweep would actually store.
	state := decidePrimeState(store, gcpdResult.Ranks, gcpdResult.HasGameData)
	if state == PrimeStateUnknown {
		t.Log("PRIME VERDICT: unknown - no pill would be drawn")
		return
	}
	t.Logf("PRIME VERDICT: %s (premierPlayed=%v premierRated=%v hasGameData=%v)",
		state, gcpdResult.Ranks.PremierPlayed, gcpdResult.Ranks.Premier.Found, gcpdResult.HasGameData)
}

// logSpikeRanks reports what the page carried and, from it, whether the rank
// collector would claim the stats tile or hand back to Leetify.
//
// The verdict mirrors collectCS2Ranks' rule rather than re-deriving it: claiming
// the variant stops the chain, so an account missing either gate would end up
// showing less than the public APIs already do.
func logSpikeRanks(t *testing.T, ranks gcpd.Ranks) {
	t.Helper()
	show := func(label string, rank gcpd.Rank) {
		if !rank.Found {
			t.Logf("  %-12s absent", label)
			return
		}
		t.Logf("  %-12s value=%d wins=%d", label, rank.Value, rank.Wins)
	}
	show("premier", ranks.Premier)
	show("wingman", ranks.Wingman)
	show("competitive", ranks.Competitive)

	premier, comp := -1, -1
	if ranks.Premier.Found {
		premier = ranks.Premier.Value
	}
	if ranks.Competitive.Found {
		comp = ranks.Competitive.Value
	}
	if premier <= 0 || comp <= 0 {
		t.Logf("collector WOULD DECLINE (premier=%d comp=%d): stats fall through to Leetify", premier, comp)
		return
	}
	t.Logf("collector WOULD CLAIM (premier=%d comp=%d): stats served from Steam", premier, comp)
}

// initSpikeDataRoot points the probe at the real install's data directory.
//
// The app resolves this during startup; a test process has to do it itself, or
// every path lookup fails with "data root not initialized". Candidates are tried
// in order and the first one that actually holds a vault wins, so a portable
// install is found without being told where it is.
func initSpikeDataRoot(t *testing.T) {
	t.Helper()
	candidates := []string{strings.TrimSpace(os.Getenv("TCNO_DATA_ROOT"))}
	if def, err := platform.DefaultUserDataDir(); err == nil {
		candidates = append(candidates, def)
	}
	// go test runs with the working directory set to the package, so a portable
	// install sits several levels up - and a dev build puts it under bin/ rather
	// than beside the repo root. Walk up and check both shapes at each level.
	if cwd, err := os.Getwd(); err == nil {
		dir := cwd
		for i := 0; i < 8; i++ {
			candidates = append(candidates,
				platform.PortableUserDataDir(dir),
				platform.PortableUserDataDir(filepath.Join(dir, "bin")),
			)
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	var tried []string
	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		tried = append(tried, dir)
		paths.InitDataRoot(dir)
		root, err := VaultFolderPath()
		if err != nil {
			continue
		}
		if info, err := os.Stat(root); err == nil && info.IsDir() {
			t.Logf("using data root %s", dir)
			return
		}
	}
	t.Fatalf("no Steam Guard vault found. Set TCNO_DATA_ROOT to the folder containing SteamGuard\\.\nTried: %v", tried)
}

// dumpTab writes one GCPD response to disk for offline inspection.
//
// These bodies are personal: they name the account and its match history. Write
// them 0600 and delete them once whatever question prompted the dump is settled.
func dumpTab(t *testing.T, dir, tab string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Logf("could not create dump dir: %v", err)
		return
	}
	path := filepath.Join(dir, "gcpd-"+tab+".html")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Logf("could not write %s: %v", path, err)
		return
	}
	t.Logf("wrote %s (%d bytes)", path, len(body))
}

// tokenAudience reports a JWT's aud claim without validating the signature. The
// repo otherwise reads only exp, so this is the cheapest way to see what Steam
// actually issued.
func tokenAudience(token string) []string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var claims struct {
		Aud []string `json:"aud"`
	}
	if json.Unmarshal(raw, &claims) != nil {
		return nil
	}
	return claims.Aud
}
