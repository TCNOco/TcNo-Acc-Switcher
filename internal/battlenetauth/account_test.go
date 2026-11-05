package battlenetauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"golang.org/x/crypto/pbkdf2"
	"testing"
	"time"
)

func fixture() Account {
	return Account{Version: 1, Serial: "US-1234-5678-9012", DeviceSecret: hex.EncodeToString([]byte("12345678901234567890"))}
}

func TestLegacyPBKDF2Backups(t *testing.T) {
	for _, iterations := range []int{100000, 600000} {
		salt, nonce := make([]byte, 16), make([]byte, 12)
		key := pbkdf2.Key([]byte("legacy password"), salt, iterations, 32, sha256.New)
		block, err := aes.NewCipher(key)
		if err != nil {
			t.Fatal(err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			t.Fatal(err)
		}
		plain := []byte(`{"serial":"US-1234-5678-9012","deviceSecret":"3132333435363738393031323334353637383930","restoreCode":"0123456789"}`)
		packet := map[string]any{"salt": base64.StdEncoding.EncodeToString(salt), "nonce": base64.StdEncoding.EncodeToString(nonce), "ciphertext": base64.StdEncoding.EncodeToString(aead.Seal(nil, nonce, plain, nil))}
		if iterations == 600000 {
			packet["kdf_iterations"] = iterations
		}
		raw, err := json.Marshal(packet)
		if err != nil {
			t.Fatal(err)
		}
		accounts, err := Import(raw, "legacy password")
		if err != nil || len(accounts) != 1 || accounts[0].Serial != fixture().Serial {
			t.Fatalf("legacy %d import failed: %v", iterations, err)
		}
	}
}

func TestRFC6238Codes(t *testing.T) {
	for _, tc := range []struct {
		timestamp int64
		code      string
	}{{59, "94287082"}, {1111111109, "07081804"}, {1111111111, "14050471"}, {1234567890, "89005924"}, {2000000000, "69279037"}} {
		code, expires, err := fixture().Code(time.Unix(tc.timestamp, 0))
		if err != nil || code != tc.code || expires <= tc.timestamp || expires-tc.timestamp > 30 {
			t.Fatalf("code at %d: %s %d %v", tc.timestamp, code, expires, err)
		}
	}
}

func TestURIAndExternalImport(t *testing.T) {
	a := fixture()
	uri, err := a.URI()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Import([]byte(uri), "")
	if err != nil || len(got) != 1 || got[0].DeviceSecret != a.DeviceSecret {
		t.Fatal("URI round trip failed")
	}
	for _, raw := range []string{`{"serial":"US-1234-5678-9012","serial":"EU-1234-5678-9012"}`, `{"serial":"US-1234-5678-9012","deviceSecret":"bad"}`, `otpauth://totp/Battle.net:US-1234-5678-9012?digits=6&secret=ABC`, `{"entries":{},"version":99}`, `{"version":99,"serial":"US-1234-5678-9012","deviceSecret":"3132333435363738393031323334353637383930"}`} {
		if _, err := Import([]byte(raw), ""); err == nil {
			t.Fatal("invalid import accepted")
		}
	}
}

func TestEncryptedVaultCompatibility(t *testing.T) {
	a := fixture()
	a.RequiresAccountSetup = true
	raw, err := Export([]Account{a}, "test password")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Import(raw, "test password")
	if err != nil || len(got) != 1 || got[0].DeviceSecret != a.DeviceSecret || !got[0].RequiresAccountSetup {
		t.Fatal("encrypted round trip failed", err)
	}
	if _, err := Import(raw, "wrong password"); err == nil {
		t.Fatal("wrong password accepted")
	}
	if _, err := Import([]byte(`{"salt":"AAAAAAAAAAAAAAAAAAAAAA==","nonce":"AAAAAAAAAAAAAAAA","ciphertext":"AAAAAAAAAAAAAAAAAAAAAA==","kdf":"scrypt","scrypt":{"n":1073741824,"r":8,"p":1}}`), "test"); err == nil {
		t.Fatal("unbounded KDF accepted")
	}
}
