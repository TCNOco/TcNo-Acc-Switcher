package qr

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
)

func TestParseChallenge(t *testing.T) {
	got, err := ParseChallenge("https://s.team/q/1/1234567890123456789")
	if err != nil {
		t.Fatal(err)
	}
	if got != (Challenge{Version: 1, ClientID: 1234567890123456789}) {
		t.Fatalf("challenge = %#v", got)
	}
}

func TestParseChallengeRejectsNonCanonicalOrUnexpectedURLs(t *testing.T) {
	tests := []string{
		"http://s.team/q/1/1",
		"https://S.TEAM/q/1/1",
		"https://s.team:443/q/1/1",
		"https://user@s.team/q/1/1",
		"https://s.team.evil.example/q/1/1",
		"https://s.team/q/1/1?next=1",
		"https://s.team/q/1/1#fragment",
		"https://s.team/q/1/1/extra",
		"https://s.team/%71/1/1",
		"https://s.team/q/01/1",
		"https://s.team/q/1/01",
		"https://s.team/q/1/0",
		" https://s.team/q/1/1",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseChallenge(input); !errors.Is(err, ErrInvalidChallenge) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestParseChallengeRejectsUnsupportedVersion(t *testing.T) {
	if _, err := ParseChallenge("https://s.team/q/2/1"); !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("error = %v", err)
	}
}

func TestSignChallengeVector(t *testing.T) {
	secret := []byte("0123456789abcdefghij")
	got, err := SignChallenge(Challenge{Version: 1, ClientID: 0x0102030405060708}, 76561198000000000, secret)
	if err != nil {
		t.Fatal(err)
	}
	want, err := hex.DecodeString("d914058a7e70ce327c0e9ca094d6eda2862a67a85bb33f0f75e06ec0f2d0546e")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("signature = %x, want %x", got, want)
	}
}

func TestSignChallengeValidatesInputs(t *testing.T) {
	valid := Challenge{Version: 1, ClientID: 1}
	secret := make([]byte, sharedSecretBytes)
	if _, err := SignChallenge(Challenge{Version: 2, ClientID: 1}, 1, secret); !errors.Is(err, ErrInvalidChallenge) {
		t.Fatalf("version error = %v", err)
	}
	if _, err := SignChallenge(valid, 0, secret); !errors.Is(err, ErrInvalidSteamID) {
		t.Fatalf("SteamID error = %v", err)
	}
	if _, err := SignChallenge(valid, 1, secret[:19]); !errors.Is(err, ErrInvalidSharedSecret) {
		t.Fatalf("secret error = %v", err)
	}
}

func FuzzParseChallenge(f *testing.F) {
	f.Add("https://s.team/q/1/123456789")
	f.Add("https://evil.example/q/1/1")
	f.Add("\x00https://s.team/q/1/1")
	f.Fuzz(func(t *testing.T, input string) {
		challenge, err := ParseChallenge(input)
		if err == nil && (challenge.Version != supportedVersion || challenge.ClientID == 0) {
			t.Fatalf("accepted invalid challenge: %#v", challenge)
		}
	})
}
