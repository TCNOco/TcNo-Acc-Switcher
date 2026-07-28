package mafile

import (
	"encoding/base64"
	"testing"
)

// The export is only useful if SDA can read it back, and ImportLegacyEncrypted is
// this repo's implementation of SDA's reader — so a round trip through it is the
// compatibility check that matters.
func TestExportLegacyEncryptedRoundTripsThroughTheImporter(t *testing.T) {
	const filename = "76561198000000000.maFile"
	const password = "canary-password"

	exported, err := ExportLegacyEncrypted(canaryAccount(), ExportOptions{IncludeTokens: true}, password)
	if err != nil {
		t.Fatal(err)
	}
	salt, err := base64.StdEncoding.DecodeString(exported.Salt)
	if err != nil || len(salt) != legacySaltBytes {
		t.Fatalf("salt = %q (%d bytes), err = %v", exported.Salt, len(salt), err)
	}
	iv, err := base64.StdEncoding.DecodeString(exported.IV)
	if err != nil || len(iv) != 16 {
		t.Fatalf("iv = %q (%d bytes), err = %v", exported.IV, len(iv), err)
	}

	manifest := legacyManifest(t, filename, canarySteamID, salt, iv)
	result, err := ImportLegacyEncrypted(exported.Body, manifest, filename, password)
	if err != nil {
		t.Fatal(err)
	}
	if result.Account.AccountName != "codec-canary" {
		t.Fatalf("account = %#v", result.Account)
	}
}

func TestExportLegacyEncryptedRejectsTheWrongPassword(t *testing.T) {
	const filename = "76561198000000000.maFile"

	exported, err := ExportLegacyEncrypted(canaryAccount(), ExportOptions{}, "canary-password")
	if err != nil {
		t.Fatal(err)
	}
	salt, _ := base64.StdEncoding.DecodeString(exported.Salt)
	iv, _ := base64.StdEncoding.DecodeString(exported.IV)
	manifest := legacyManifest(t, filename, canarySteamID, salt, iv)

	if _, err := ImportLegacyEncrypted(exported.Body, manifest, filename, "wrong"); err == nil {
		t.Fatal("the wrong password decrypted the export")
	}
}

// Fresh key material per export: identical output would show that two exports of
// one account share a salt and IV.
func TestExportLegacyEncryptedNeverReusesKeyMaterial(t *testing.T) {
	first, err := ExportLegacyEncrypted(canaryAccount(), ExportOptions{}, "canary-password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := ExportLegacyEncrypted(canaryAccount(), ExportOptions{}, "canary-password")
	if err != nil {
		t.Fatal(err)
	}
	if first.Salt == second.Salt || first.IV == second.IV || string(first.Body) == string(second.Body) {
		t.Fatal("two exports produced the same key material")
	}
}

func TestExportLegacyEncryptedRefusesAnEmptyPassword(t *testing.T) {
	if _, err := ExportLegacyEncrypted(canaryAccount(), ExportOptions{}, ""); err == nil {
		t.Fatal("an empty password produced an encrypted export")
	}
}
