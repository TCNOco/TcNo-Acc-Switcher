package winutil

import (
	"testing"
)

func TestEmbeddedPathIsUnderPlatformArt(t *testing.T) {
	if !embeddedPathIsUnderPlatformArt(`frontend/dist/img/platform/Steam.svg`) {
		t.Fatal("expected frontend/dist prefix")
	}
	if !embeddedPathIsUnderPlatformArt(`img/platform/Steam.svg`) {
		t.Fatal("expected img/platform prefix")
	}
	if embeddedPathIsUnderPlatformArt(`frontend/dist/index.html`) {
		t.Fatal("index should not match")
	}
}

func TestPlatformArtStemMatchesKey(t *testing.T) {
	cases := []struct {
		stem, key string
		want       bool
	}{
		{"BattleNet", "BattleNet", true},
		{"BattleNet", "Battle.net", true},
		{"BattleNet", "battle.net", true},
		{"discord", "Discord", true},
		{"ubisoft", "Ubisoft", true},
		{"Epic Games", "Epic Games", true},
		{"Steam", "Steam", true},
		{"Steam", "Steamy", false},
		{"GOG Galaxy", "GOG Galaxy", true},
	}
	for _, tc := range cases {
		if got := PlatformArtStemMatchesKey(tc.stem, tc.key); got != tc.want {
			t.Errorf("PlatformArtStemMatchesKey(%q, %q) = %v, want %v", tc.stem, tc.key, got, tc.want)
		}
	}
}
