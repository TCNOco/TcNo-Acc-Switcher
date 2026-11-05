package vault

import "testing"

func TestProviderIdentityCompatibility(t *testing.T) {
	steam, err := ProviderIdentity(ProviderSteam, "76561198000000000")
	if err != nil || steam != "76561198000000000" {
		t.Fatal("legacy Steam identity changed")
	}
	bnet, err := ProviderIdentity(ProviderBattleNet, "US-1234-5678-9012")
	if err != nil || bnet == steam {
		t.Fatal("provider identity collision")
	}
	records := []RecordInfo{{ID: "steam", SteamID64: steam}, {ID: "bnet", SteamID64: bnet}}
	for provider, id := range map[Provider]string{ProviderSteam: "steam", ProviderBattleNet: "bnet"} {
		filtered := filterProvider(records, provider)
		if len(filtered) != 1 || filtered[0].ID != id {
			t.Fatalf("provider %s received another provider's records", provider)
		}
	}
	for _, subject := range []string{"", "battlenet:other", "bad\x00id"} {
		if _, err := ProviderIdentity(ProviderBattleNet, subject); err == nil {
			t.Fatal("invalid subject accepted")
		}
	}
}
