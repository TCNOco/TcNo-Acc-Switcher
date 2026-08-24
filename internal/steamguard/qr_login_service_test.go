package steamguard

import (
	"strings"
	"testing"
)

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
