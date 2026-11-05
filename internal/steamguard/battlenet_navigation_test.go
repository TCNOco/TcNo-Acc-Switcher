package steamguard

import "testing"

func TestBattleNetEnrollmentInterceptsSessionBeforeNavigation(t *testing.T) {
	const token = "EU-012345678901234567890123456789-1111111111"
	for _, callback := range []string{
		"http://localhost/?ST=" + token,
		"https://account.battle.net/login/en/localhost?ST=" + token + "&flowTrackingId=",
		"https://account.battle.net/login/en/localhost/?ST=" + token,
	} {
		calls := 0
		intercept := battleNetEnrollmentNavigation(func(received string) {
			calls++
			if received != token {
				t.Fatal("incorrect session token")
			}
		})
		if intercept("https://account.battle.net/login/en/?ref=localhost") || calls != 0 {
			t.Fatal("normal sign-in was intercepted")
		}
		if !intercept(callback) || calls != 1 {
			t.Fatal("session callback was not consumed before navigation")
		}
		if !intercept(callback) || calls != 1 {
			t.Fatal("duplicate callback repeated enrollment or loaded the error page")
		}
	}
}

func TestBattleNetEnrollmentDoesNotConsumeOtherSessionURLs(t *testing.T) {
	const token = "EU-012345678901234567890123456789-1111111111"
	intercept := battleNetEnrollmentNavigation(func(string) { t.Fatal("unexpected session token accepted") })
	for _, raw := range []string{
		"https://account.battle.net/security?ST=" + token,
		"https://oauth.battle.net/login/en/localhost?ST=" + token,
		"https://account.battle.net/login/en/localhost?ST=" + token + "&ST=" + token,
	} {
		intercept(raw)
	}
	for _, raw := range []string{
		"https://account.battle.net.evil.test/login/en/localhost?ST=" + token,
		"http://account.battle.net/login/en/localhost?ST=" + token,
		"http://localhost/?ST=invalid",
		"file:///C:/example",
	} {
		if !intercept(raw) {
			t.Fatal("unsafe navigation was allowed")
		}
	}
}
