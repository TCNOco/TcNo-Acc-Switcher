package mafile

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/pbkdf2"
)

const canarySteamID = uint64(76561198000000000)

func canaryAccount() Account {
	return Account{
		SharedSecret:   base64.StdEncoding.EncodeToString(bytesOf(0x31, authenticatorSecretBytes)),
		IdentitySecret: base64.StdEncoding.EncodeToString(bytesOf(0x32, authenticatorSecretBytes)),
		DeviceID:       "android:12345678-1234-1234-1234-123456789abc",
		AccountName:    "codec-canary",
		FullyEnrolled:  true,
		Session: &SessionData{
			SteamID:          canarySteamID,
			SessionID:        "canary-session",
			SteamLogin:       "canary-login",
			SteamLoginSecure: "canary-login-secure",
			WebCookie:        "canary-web-cookie",
			OAuthToken:       "canary-oauth",
			AccessToken:      "canary-access",
			RefreshToken:     "canary-refresh",
		},
	}
}

func canaryPlaintext(t *testing.T) []byte {
	t.Helper()
	data, err := ExportPlaintext(canaryAccount(), ExportOptions{IncludeTokens: true})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParsePlaintextPascalCaseSessionAndDiscardedNames(t *testing.T) {
	data := append([]byte(`{"unused_field":"discarded",`), canaryPlaintext(t)[1:]...)
	result, err := ParsePlaintext(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Account.Session == nil || result.Account.Session.SteamID != canarySteamID {
		t.Fatalf("PascalCase Session was not decoded: %#v", result.Account.Session)
	}
	if len(result.DiscardedFields) != 1 || result.DiscardedFields[0] != "unused_field" {
		t.Fatalf("discarded fields = %#v", result.DiscardedFields)
	}
}

func TestParsePlaintextAcceptsSDACompatibilityVariants(t *testing.T) {
	valid := string(canaryPlaintext(t))
	for _, testCase := range []struct {
		name   string
		input  string
		verify func(t *testing.T, result ParseResult)
	}{
		{
			name:  "null optional session field",
			input: strings.Replace(valid, `"SessionID":"canary-session"`, `"SessionID":null`, 1),
			verify: func(t *testing.T, result ParseResult) {
				if result.Account.Session == nil || result.Account.Session.SessionID != "" {
					t.Fatalf("session = %#v", result.Account.Session)
				}
			},
		},
		{
			name:  "percent-encoded device ID prefix",
			input: strings.Replace(valid, `"device_id":"android:`, `"device_id":"android%3A`, 1),
			verify: func(t *testing.T, result ParseResult) {
				if !deviceIDPattern.MatchString(result.Account.DeviceID) {
					t.Fatal("device ID was not normalized")
				}
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := ParsePlaintext([]byte(testCase.input))
			if err != nil {
				t.Fatal(err)
			}
			testCase.verify(t, result)
		})
	}
}

func TestExportOmitsTokensByDefault(t *testing.T) {
	data, err := ExportPlaintext(canaryAccount(), ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"SessionID", "SteamLogin", "SteamLoginSecure", "WebCookie", "OAuthToken", "AccessToken", "RefreshToken",
		"canary-session", "canary-login", "canary-login-secure", "canary-web-cookie", "canary-oauth", "canary-access", "canary-refresh",
	} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("default export retained session material %q: %s", forbidden, data)
		}
	}
	if !strings.Contains(string(data), `"SteamID":76561198000000000`) {
		t.Fatalf("default export did not retain SteamID: %s", data)
	}
	withTokens, err := ExportPlaintext(canaryAccount(), ExportOptions{IncludeTokens: true})
	if err != nil || !strings.Contains(string(withTokens), "canary-access") {
		t.Fatalf("opt-in token export failed: %v", err)
	}
}

func TestImportLegacyEncrypted(t *testing.T) {
	plain := canaryPlaintext(t)
	salt := []byte("salt-123")
	iv := []byte("0123456789abcdef")
	filename := "76561198000000000.maFile"
	manifest := legacyManifest(t, filename, canarySteamID, salt, iv)
	for _, testCase := range []struct {
		name     string
		password string
	}{
		{name: "with password", password: "canary-password"},
		{name: "without password", password: ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ciphertext := encryptLegacy(t, plain, testCase.password, salt, iv)
			result, err := ImportLegacyEncrypted(ciphertext, manifest, filename, testCase.password)
			if err != nil {
				t.Fatal(err)
			}
			if result.Account.AccountName != "codec-canary" {
				t.Fatalf("account = %#v", result.Account)
			}
		})
	}
}

func TestImportLegacyEncryptedHidesWrongPasswordAndTampering(t *testing.T) {
	plain, salt, iv := canaryPlaintext(t), []byte("salt-123"), []byte("0123456789abcdef")
	filename := "76561198000000000.maFile"
	ciphertext := encryptLegacy(t, plain, "canary-password", salt, iv)
	manifest := legacyManifest(t, filename, canarySteamID, salt, iv)
	for name, testCase := range map[string]struct {
		input    []byte
		password string
	}{
		"wrong password": {ciphertext, "not-the-canary-password"},
		"CBC tampering":  {tamperBase64(t, ciphertext), "canary-password"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ImportLegacyEncrypted(testCase.input, manifest, filename, testCase.password)
			if !errors.Is(err, ErrWrongPasswordOrCorruptSource) {
				t.Fatalf("error = %v", err)
			}
			if strings.Contains(err.Error(), "canary") || strings.Contains(err.Error(), testCase.password) {
				t.Fatalf("error leaked sensitive input: %v", err)
			}
		})
	}
}

func TestParseRejectsUnsafeInputsWithoutSecretErrors(t *testing.T) {
	valid := string(canaryPlaintext(t))
	secret := "very-secret-canary-value"
	cases := map[string][]byte{
		"duplicate key":      []byte(`{"shared_secret":"` + secret + `","shared_secret":"MTExMTExMTExMTExMTExMTExMTE="}`),
		"invalid UTF-8":      append([]byte{0xff}, []byte(valid)...),
		"unpaired surrogate": []byte(strings.Replace(valid, "codec-canary", `codec-\ud800`, 1)),
		"invalid base64":     []byte(strings.Replace(valid, canaryAccount().SharedSecret, "not-base64", 1)),
		"invalid SteamID":    []byte(strings.Replace(valid, "76561198000000000", "42", 1)),
		"invalid device ID":  []byte(strings.Replace(valid, "android:12345678-1234-1234-1234-123456789abc", "android:not-a-uuid", 1)),
		"over limit":         []byte(strings.Repeat(" ", MaxInputBytes+1)),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParsePlaintext(input)
			if err == nil {
				t.Fatal("expected rejection")
			}
			if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "MTExMTExMTExMTExMTExMTExMTE=") {
				t.Fatalf("error leaked a secret: %v", err)
			}
		})
	}
}

func TestParseRejectsDepthStringAndCollectionLimits(t *testing.T) {
	deep := strings.Repeat(`{"a":`, maxJSONDepth+2) + `null` + strings.Repeat(`}`, maxJSONDepth+2)
	largeString := `{"shared_secret":"` + strings.Repeat("x", maxStringBytes+1) + `"}`
	entries := make([]string, maxCollectionEntries+1)
	for i := range entries {
		entries[i] = `null`
	}
	largeCollection := `[` + strings.Join(entries, ",") + `]`
	for name, data := range map[string][]byte{
		"deep nesting":     []byte(deep),
		"oversized string": []byte(largeString),
		"large collection": []byte(largeCollection),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParsePlaintext(data); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestParseRejectsWrongAuthenticatorSecretLengths(t *testing.T) {
	valid := string(canaryPlaintext(t))
	original := canaryAccount().SharedSecret
	for _, size := range []int{1, authenticatorSecretBytes - 1, authenticatorSecretBytes + 1, 4096} {
		replacement := base64.StdEncoding.EncodeToString(bytesOf(0x44, size))
		if _, err := ParsePlaintext([]byte(strings.Replace(valid, original, replacement, 1))); err == nil {
			t.Fatalf("accepted %d-byte shared secret", size)
		}
	}
}

func TestParseRejectsUnsafeMetadataAndSessionStructure(t *testing.T) {
	valid := string(canaryPlaintext(t))
	cases := map[string]string{
		"control in account name": strings.Replace(valid, "codec-canary", `codec\u000acanary`, 1),
		"bad revocation code":     strings.Replace(valid, `"fully_enrolled":true`, `"revocation_code":"not-a-code","fully_enrolled":true`, 1),
		"credential without ID":   strings.Replace(valid, strconv.FormatUint(canarySteamID, 10), "0", 1),
		"uppercase device ID":     strings.Replace(valid, "android:12345678-1234-1234-1234-123456789abc", "android:12345678-1234-1234-1234-123456789ABC", 1),
		"space in access token":   strings.Replace(valid, "canary-access", "canary access", 1),
		"oversized access token":  strings.Replace(valid, "canary-access", strings.Repeat("A", maxSessionTokenBytes+1), 1),
		"not fully enrolled":      strings.Replace(valid, `"fully_enrolled":true`, `"fully_enrolled":false`, 1),
		"unsupported version":     strings.Replace(valid, `{"shared_secret"`, `{"version":1,"shared_secret"`, 1),
		"typed pending envelope":  strings.Replace(valid, `{"shared_secret"`, `{"kind":"steamguard-enrollment-pending","shared_secret"`, 1),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParsePlaintext([]byte(input)); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestParseCanonicalRoundTripProperty(t *testing.T) {
	account := canaryAccount()
	for i := 0; i < 64; i++ {
		account.AccountName = strings.Repeat("a", i)
		encoded, err := ExportPlaintext(account, ExportOptions{IncludeTokens: true})
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := ParsePlaintext(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(parsed.Account, account) {
			t.Fatalf("round trip changed account at length %d", i)
		}
	}
}

func FuzzParsePlaintextBounded(f *testing.F) {
	seed, err := ExportPlaintext(canaryAccount(), ExportOptions{IncludeTokens: true})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte(`{"shared_secret":"duplicate","shared_secret":"duplicate"}`))
	f.Add([]byte{0xff, '{', '}'})
	f.Fuzz(func(t *testing.T, input []byte) {
		result, err := ParsePlaintext(input)
		if err != nil {
			return
		}
		canonical, err := ExportPlaintext(result.Account, ExportOptions{IncludeTokens: true})
		if err != nil {
			t.Fatalf("accepted account could not be exported: %v", err)
		}
		reparsed, err := ParsePlaintext(canonical)
		if err != nil {
			t.Fatalf("canonical output could not be parsed: %v", err)
		}
		if !reflect.DeepEqual(reparsed.Account, result.Account) {
			t.Fatal("canonical round trip changed accepted account")
		}
	})
}

func TestLegacyImportRejectsManifestMismatchAndUnsafeFilename(t *testing.T) {
	plain, salt, iv := canaryPlaintext(t), []byte("salt-123"), []byte("0123456789abcdef")
	filename := "76561198000000000.maFile"
	ciphertext := encryptLegacy(t, plain, "canary-password", salt, iv)
	for name, testCase := range map[string]struct {
		manifest []byte
		source   string
	}{
		"steam ID mismatch":     {legacyManifest(t, filename, canarySteamID+1, salt, iv), filename},
		"traversal filename":    {legacyManifest(t, "../76561198000000000.maFile", canarySteamID, salt, iv), "../76561198000000000.maFile"},
		"alternate data stream": {legacyManifest(t, filename+":stream", canarySteamID, salt, iv), filename + ":stream"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ImportLegacyEncrypted(ciphertext, testCase.manifest, testCase.source, "canary-password")
			if !errors.Is(err, ErrWrongPasswordOrCorruptSource) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestNestedUnknownFieldIsReported(t *testing.T) {
	data := strings.Replace(string(canaryPlaintext(t)), `"SteamID":76561198000000000`, `"SteamID":76561198000000000,"OtherSessionThing":"discarded"`, 1)
	result, err := ParsePlaintext([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.DiscardedFields) != 1 || result.DiscardedFields[0] != "Session.OtherSessionThing" {
		t.Fatalf("discarded = %#v", result.DiscardedFields)
	}
}

func encryptLegacy(t *testing.T, plaintext []byte, password string, salt, iv []byte) []byte {
	t.Helper()
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := append(append([]byte{}, plaintext...), bytesOf(byte(padding), padding)...)
	block, err := aes.NewCipher(pbkdf2.Key([]byte(password), salt, 50000, 32, sha1.New))
	if err != nil {
		t.Fatal(err)
	}
	sealed := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(sealed, padded)
	return []byte(base64.StdEncoding.EncodeToString(sealed))
}
func legacyManifest(t *testing.T, filename string, id uint64, salt, iv []byte) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]any{"encrypted": true, "entries": []map[string]any{{"filename": filename, "steamid": id, "encryption_salt": base64.StdEncoding.EncodeToString(salt), "encryption_iv": base64.StdEncoding.EncodeToString(iv)}}})
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func tamperBase64(t *testing.T, data []byte) []byte {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		t.Fatal(err)
	}
	decoded[len(decoded)-1] ^= 1
	return []byte(base64.StdEncoding.EncodeToString(decoded))
}
func bytesOf(value byte, length int) []byte {
	result := make([]byte, length)
	for i := range result {
		result[i] = value
	}
	return result
}
