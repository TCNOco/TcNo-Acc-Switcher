package steamguard

import (
	"errors"
	"testing"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

// EResult 9 is FileNotFound. For an auth-session call it means the sign-in the
// challenge names is gone, which is what every QR code becomes about a minute
// after it is drawn - so it has to read as an expired code rather than as a
// failure, or the user is told to sign in again for no reason.
func TestSteamDroppedSessionRecognisesAnExpiredChallenge(t *testing.T) {
	expired := &protocol.Error{Code: protocol.CodeSteamResult, EResult: 9, HasEResult: true}
	if !steamDroppedSession(expired) {
		t.Fatal("an expired challenge was not recognised")
	}
	if !steamDroppedSession(errors.Join(errors.New("wrapped"), expired)) {
		t.Fatal("a wrapped expired challenge was not recognised")
	}

	for name, err := range map[string]error{
		"other eresult": &protocol.Error{Code: protocol.CodeSteamResult, EResult: 8, HasEResult: true},
		"no eresult":    &protocol.Error{Code: protocol.CodeSteamResult},
		"not protocol":  errors.New("network unreachable"),
		"nil":           nil,
	} {
		if steamDroppedSession(err) {
			t.Fatalf("%s was mistaken for an expired challenge", name)
		}
	}
}
