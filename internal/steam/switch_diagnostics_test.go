package steam

import (
	"fmt"
	"strings"
	"testing"
)

func TestSwitchDiagnosticIdentifiersIncludeUserdataID(t *testing.T) {
	ids := steamDiagnosticIDs("76561198679191951", "LoginSentinel", "PersonaSentinel")
	if len(ids) != 4 || ids[3] != "718926223" {
		t.Fatalf("identifiers=%v", ids)
	}
	for _, invalid := range []string{"", "invalid", "76561197960265727", "18446744073709551615"} {
		if len(steamDiagnosticIDs(invalid, "", "")) != 3 {
			t.Fatalf("derived ID for invalid ID %q", invalid)
		}
	}
}

func TestSwitchDiagnosticSelectionContainsNoAccountIdentifiers(t *testing.T) {
	users := []LoginUser{{SteamID64: "76561198679191951", AccountName: "LoginSentinel", PersonaName: "PersonaSentinel", AutoLogin: "1", RememberPassword: "1"}}
	got := steamSwitchSelectionDetails(users, users[0].SteamID64)
	if got["selection_matches_target"] != true || got["target_has_login_name"] != true || got["remember_password_flag"] != true {
		t.Fatalf("details=%v", got)
	}
	text := fmt.Sprint(got)
	for _, secret := range []string{users[0].SteamID64, users[0].AccountName, users[0].PersonaName} {
		if strings.Contains(text, secret) {
			t.Fatalf("identifier leaked in metadata: %s", text)
		}
	}
	if steamSwitchSelectionDetails(users, "other")["selection_matches_target"] != false {
		t.Fatal("wrong account matched")
	}
	if steamSwitchSelectionDetails(nil, "")["selection_matches_target"] != false {
		t.Fatal("empty markers treated as login")
	}
}
