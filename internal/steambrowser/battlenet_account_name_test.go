package steambrowser

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBattleNetAccountNameSource(t *testing.T) {
	for _, source := range []string{"https://account.battle.net/security", "https://account.battle.net/security/#authenticator", "https://eu.account.battle.net/security", "https://us.account.battle.net/security", "https://kr.account.battle.net/security", "https://tw.account.battle.net/security", "https://cn.account.battle.net/security"} {
		raw, _ := json.Marshal(map[string]string{"source": source, "name": "Example#1234"})
		if name, err := parseBattleNetAccountName(string(raw)); err != nil || name != "Example#1234" {
			t.Fatal("security page identity was rejected")
		}
	}
	for _, source := range []string{"https://account.battle.net/login/en/", "http://account.battle.net/security", "https://account.battle.net.evil.test/security", "https://eu.account.battle.net.evil.test/security", "https://unknown.account.battle.net/security", "https://account.battle.net:1234/security", "https://evil.test/security", "https://user@account.battle.net/security", "https://account.battle.net/%73ecurity"} {
		raw, _ := json.Marshal(map[string]string{"source": source, "name": "Example#1234"})
		if _, err := parseBattleNetAccountName(string(raw)); err == nil {
			t.Fatal("untrusted identity source accepted")
		}
	}
	for _, name := range []string{"Example\nName", "Example\u202eName", strings.Repeat("x", 321)} {
		raw, _ := json.Marshal(map[string]string{"source": "https://account.battle.net/security", "name": name})
		if _, err := parseBattleNetAccountName(string(raw)); err == nil {
			t.Fatal("malformed account name accepted")
		}
	}
	if name, err := parseBattleNetAccountName("null"); err != nil || name != "" {
		t.Fatal("unsigned page did not produce an empty identity")
	}
}

func TestBattleNetIdentityRequiresUnmaskedAccountEmail(t *testing.T) {
	for _, email := range []string{"test@example.invalid", "t***@example.invalid", "Example <test@example.invalid>", "not-an-email"} {
		raw, _ := json.Marshal(map[string]string{"source": "https://eu.account.battle.net/security", "name": "Example#1234", "email": email})
		identity, err := parseBattleNetAccountIdentity(string(raw))
		if err != nil || identity.Name != "Example#1234" {
			t.Fatal("email formatting discarded the BattleTag")
		}
		if email == "test@example.invalid" && identity.Email != email {
			t.Fatal("full email was discarded")
		}
		if email != "test@example.invalid" && identity.Email != "" {
			t.Fatal("ambiguous email can link the wrong account")
		}
	}
}
