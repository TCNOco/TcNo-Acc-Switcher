package platforms

import (
	"path/filepath"
	"testing"
)

func TestCacheProviderFallbacks(t *testing.T) {
	// This primary file exists but contains no profile data for either provider.
	primary := filepath.Join("testdata", "ea_desktop", "user_current.ini")
	rockstar := filepath.Join("testdata", "cache_providers", "rockstar.txt")
	geforce := filepath.Join("testdata", "cache_providers", "geforce_now.txt")
	name, err := RockstarSuggestedName(primary, rockstar)
	if err != nil || name != "RockstarPlayer" {
		t.Fatalf("Rockstar name = %q, %v", name, err)
	}
	image, err := RockstarImageSource(primary, rockstar)
	if err != nil || image.RemoteURL != "https://prod-avatars.akamaized.net/avatars/player.png" {
		t.Fatalf("Rockstar avatar = %+v, %v", image, err)
	}
	name, err = GeForceNowSuggestedName(primary, geforce)
	if err != nil || name != "GeForcePlayer" {
		t.Fatalf("GeForce Now name = %q, %v", name, err)
	}
	name, err = GeForceNowSuggestedName(filepath.Join("testdata", "missing", "data_*"), geforce)
	if err != nil || name != "GeForcePlayer" {
		t.Fatalf("missing primary fallback = %q, %v", name, err)
	}
}
