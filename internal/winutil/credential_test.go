package winutil

import (
	"reflect"
	"testing"
)

func TestCredentialTargetCandidates(t *testing.T) {
	target := "credentials.com.epicgames.eglv2.prod"
	want := []string{"LegacyGeneric:target=" + target, target}
	if got := CredentialTargetCandidates("LegacyGeneric:target=" + target); !reflect.DeepEqual(got, want) {
		t.Fatalf("CredentialTargetCandidates(prefixed) = %#v, want %#v", got, want)
	}
	want = []string{target, "LegacyGeneric:target=" + target}
	if got := CredentialTargetCandidates(target); !reflect.DeepEqual(got, want) {
		t.Fatalf("CredentialTargetCandidates(raw) = %#v, want %#v", got, want)
	}
	if got := CredentialTargetCandidates(" "); len(got) != 0 {
		t.Fatalf("empty target candidates = %#v, want none", got)
	}
}

func TestCredentialTargetMatches(t *testing.T) {
	for _, tc := range []struct {
		expected string
		actual   string
		want     bool
	}{
		{"LegacyGeneric:target=credentials.com.epicgames.eglv2.prod", "credentials.com.epicgames.eglv2.prod", true},
		{"credentials.com.epicgames.eglv2.prod", "LegacyGeneric:target=credentials.com.epicgames.eglv2.prod", true},
		{"credentials.com.epicgames.eglv2.prod", "credentials.com.other", false},
		{"", "credentials.com.epicgames.eglv2.prod", false},
	} {
		if got := CredentialTargetMatches(tc.expected, tc.actual); got != tc.want {
			t.Errorf("CredentialTargetMatches(%q, %q) = %v, want %v", tc.expected, tc.actual, got, tc.want)
		}
	}
}
