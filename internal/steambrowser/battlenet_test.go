package steambrowser

import "testing"

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
