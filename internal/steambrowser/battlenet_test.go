package steambrowser

import (
	"errors"
	"testing"
)

func TestBattleNetTrustRemainsSeparateFromSteam(t *testing.T) {
	for _, url := range []string{"https://account.battle.net/security", "https://oauth.battle.net/", "https://www.blizzard.com/"} {
		if !Classify(PlatformBattleNet, url).Trusted || Classify(PlatformSteam, url).Trusted {
			t.Fatal("incorrect provider trust", url)
		}
	}
	for _, url := range []string{"https://battle.net.evil.test/", "https://evilbattle.net/", "http://account.battle.net/", "file:///C:/secret", "https://steamcommunity.com/"} {
		if Classify(PlatformBattleNet, url).Trusted {
			t.Fatal("untrusted origin accepted", url)
		}
	}
}

func TestBrowserRefusesMissingStorage(t *testing.T) {
	if !Supported() {
		t.Skip("native browser unavailable")
	}
	s := NewService(nil, "", false)
	if s.Available() {
		t.Fatal("missing browser storage reported available")
	}
	if _, err := s.OpenBrowser("account", "store", "token"); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatal("missing storage did not stop browser creation", err)
	}
	if err := OpenBattleNetLogin(s, "battlenet-fixture", func(string) bool { return false }); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatal("missing storage did not stop Battle.net browser creation", err)
	}
}
