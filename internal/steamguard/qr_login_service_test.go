package steamguard

import (
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/steam/accountstore"
)

// Login Only signs in before the account has a vault record, so the vault
// cannot be the only place the name is read from.
func TestExpectedQRAccountNameFallsBackToTheSwitchersOwnList(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)
	const unheldID = "76561198000000200"

	if name := service.expectedQRAccountName(unheldID); name != "" {
		t.Fatalf("an account nothing knows about resolved to %q", name)
	}
	if _, err := accountstore.Upsert(accountstore.Record{SteamID64: unheldID, AccountName: "listed_account"}); err != nil {
		t.Fatal(err)
	}
	if name := service.expectedQRAccountName(unheldID); name != "listed_account" {
		t.Fatalf("account name = %q, want the switcher's own", name)
	}
}

// A held account keeps answering from the vault, which is the record that
// actually governs the sign-in.
func TestExpectedQRAccountNamePrefersTheVault(t *testing.T) {
	service, accountID, _ := newAuthServiceFixture(t)
	if _, err := accountstore.Upsert(accountstore.Record{SteamID64: accountID, AccountName: "stale_list_name"}); err != nil {
		t.Fatal(err)
	}
	if name := service.expectedQRAccountName(accountID); name != "test_account" {
		t.Fatalf("account name = %q, want the vault's", name)
	}
}

func TestBeginQRLoginDrawsACodeForTheAccountsOwnName(t *testing.T) {
	service, accountID, grant := newAuthServiceFixture(t)
	manager := &authServiceCredentialManager{steamID: authServiceSteamID}
	service.newAuthManager = func() (steamCredentialAuthManager, error) { return manager, nil }

	result, err := service.BeginQRLogin(accountID, grant.Capability, SteamAuthPurposeLoginAgain)
	if err != nil {
		t.Fatal(err)
	}
	if manager.beginQRCalls != 1 || manager.beginCalls != 0 {
		t.Fatalf("calls: beginQR=%d begin=%d", manager.beginQRCalls, manager.beginCalls)
	}
	// The name is what the manager checks a scan against, so the session is
	// useless without it.
	if manager.binding.ExpectedAccountName != "test_account" {
		t.Fatalf("expected account name = %q", manager.binding.ExpectedAccountName)
	}
	if result.ChallengeURL == "" || !strings.HasPrefix(result.QRImage, "data:image/svg+xml;base64,") {
		t.Fatalf("qr result = %#v", result)
	}
}

// The URL is still worth returning when the code cannot be drawn: the screen can
// offer it as a link rather than showing the user nothing.
func TestBeginQRLoginSurvivesAnUndrawableChallengeURL(t *testing.T) {
	service, accountID, grant := newAuthServiceFixture(t)
	manager := &authServiceCredentialManager{steamID: authServiceSteamID, qrURL: strings.Repeat("a", 600)}
	service.newAuthManager = func() (steamCredentialAuthManager, error) { return manager, nil }

	result, err := service.BeginQRLogin(accountID, grant.Capability, SteamAuthPurposeLoginAgain)
	if err != nil {
		t.Fatal(err)
	}
	if result.QRImage != "" {
		t.Fatal("an oversized URL was drawn anyway")
	}
	if result.ChallengeURL == "" {
		t.Fatal("the URL was dropped along with the image")
	}
}

func TestBeginQRLoginRejectsAnUnknownPurpose(t *testing.T) {
	service, accountID, grant := newAuthServiceFixture(t)
	manager := &authServiceCredentialManager{steamID: authServiceSteamID}
	service.newAuthManager = func() (steamCredentialAuthManager, error) { return manager, nil }

	if _, err := service.BeginQRLogin(accountID, grant.Capability, SteamAuthPurpose("nonsense")); err == nil {
		t.Fatal("an unknown purpose started a sign-in")
	}
	if manager.beginQRCalls != 0 {
		t.Fatal("a rejected purpose still reached Steam")
	}
}
