package vault

import (
	"bytes"
	"encoding/json"
	"testing"
)

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

func TestProviderRecoveryRequiresBothEncryptionLayers(t *testing.T) {
	key, outer := bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)
	plain := []byte("fixture authenticator")
	additional := aad(FormatVersion, "vault-id", "", "", "battlenet-recovery")
	env, err := seal(key, plain, additional)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := sealOuterFile(outer, raw, OuterLayerVersion, outerAAD("vault-id", "", "receipt"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openOuterFile(bytes.Repeat([]byte{3}, 32), wrapped, OuterLayerVersion, outerAAD("vault-id", "", "receipt")); err == nil {
		t.Fatal("wrong outer key accepted")
	}
	inner, err := openOuterFile(outer, wrapped, OuterLayerVersion, outerAAD("vault-id", "", "receipt"))
	if err != nil {
		t.Fatal(err)
	}
	var recovered envelope
	if err := json.Unmarshal(inner, &recovered); err != nil {
		t.Fatal(err)
	}
	if _, err := openEnvelope(bytes.Repeat([]byte{3}, 32), recovered, additional); err == nil {
		t.Fatal("wrong vault key accepted")
	}
	got, err := openEnvelope(key, recovered, additional)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatal("recovery round trip failed", err)
	}
}
