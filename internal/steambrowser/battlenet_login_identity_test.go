package steambrowser

import "testing"

func TestBattleNetLoginIdentityAcceptsOnlyOfficialLoginPages(t *testing.T) {
	for _, source := range []string{"https://account.battle.net/login/en/", "https://eu.account.battle.net/login/en-us/password", "https://account.battle.net/login/"} {
		identity, err := parseBattleNetLoginIdentity(`{"source":"` + source + `","email":"fixture@example.test","name":"ignored"}`)
		if err != nil || identity.Email != "fixture@example.test" || identity.Name != "" {
			t.Fatal("login identity rejected or name taken from login form")
		}
	}
	for _, source := range []string{"http://account.battle.net/login/en/", "https://account.battle.net.evil.test/login/en/", "https://account.battle.net:8443/login/en/", "https://user@account.battle.net/login/en/", "https://account.battle.net/security", "https://account.battle.net/login/en/%70assword", "https://account.battle.net/login/en/localhost"} {
		identity, err := parseBattleNetLoginIdentity(`{"source":"` + source + `","email":"fixture@example.test"}`)
		if err == nil || identity.Email != "" {
			t.Fatal("untrusted login identity accepted")
		}
	}
	identity, err := parseBattleNetLoginIdentity(`{"source":"https://account.battle.net/login/en/password","email":"fi***@example.test"}`)
	if err != nil || identity.Email != "" {
		t.Fatal("masked email accepted")
	}
}
