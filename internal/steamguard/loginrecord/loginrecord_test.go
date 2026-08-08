package loginrecord

import (
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/steamguard/vaultrecord"
)

const testSteamID = uint64(76561198000000000)

func testRecord() Record {
	return New(testSteamID, "someaccount", "access-token-value-long-enough", "refresh-token-value-long-enough")
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()
	raw, err := Encode(testRecord())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got != testRecord() {
		t.Fatalf("round trip = %#v", got)
	}
}

func TestEncodedRecordSniffsAsLoginOnly(t *testing.T) {
	t.Parallel()
	raw, err := Encode(testRecord())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if got := vaultrecord.Sniff(raw); got != vaultrecord.KindLoginOnly {
		t.Fatalf("Sniff = %v, want KindLoginOnly", got)
	}
}

func TestEncodeStampsTheEnvelopeRegardlessOfInput(t *testing.T) {
	t.Parallel()
	// A caller that forgets New() must not be able to persist an unlabelled
	// record, because Sniff would then read it back as a maFile.
	record := testRecord()
	record.Kind = "wrong"
	record.Version = 99
	raw, err := Encode(record)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.Kind != vaultrecord.KindStringLoginOnly || got.Version != Version {
		t.Fatalf("envelope = %q/%d", got.Kind, got.Version)
	}
}

func TestEncodeRejectsRecordsMissingRequiredFields(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*Record){
		"no steam id":       func(r *Record) { r.SteamID = 0 },
		"steam id too low":  func(r *Record) { r.SteamID = 1 },
		"no access token":   func(r *Record) { r.AccessToken = "" },
		"token with space":  func(r *Record) { r.AccessToken = "has a space" },
		"token too long":    func(r *Record) { r.AccessToken = strings.Repeat("a", maxTokenBytes+1) },
		"bad refresh token": func(r *Record) { r.RefreshToken = "has a space" },
		"name too long":     func(r *Record) { r.AccountName = strings.Repeat("a", maxAccountNameBytes+1) },
		"control char name": func(r *Record) { r.AccountName = "bad\x01name" },
	}
	for name, mutate := range cases {
		record := testRecord()
		mutate(&record)
		if _, err := Encode(record); err == nil {
			t.Fatalf("%s: Encode succeeded, want error", name)
		}
	}
}

func TestEncodeAllowsAnAbsentRefreshToken(t *testing.T) {
	t.Parallel()
	// Accounts enrolled through the app's own flow get an access token and no
	// refresh token, so this must round-trip rather than being rejected.
	record := testRecord()
	record.RefreshToken = ""
	raw, err := Encode(record)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if _, err := Decode(raw); err != nil {
		t.Fatalf("Decode: %v", err)
	}
}

func TestDecodeRejectsMalformedInput(t *testing.T) {
	t.Parallel()
	valid := `"kind":"steamguard-login-only","version":1,"steamId":76561198000000000,"accessToken":"tokentokentoken"`
	cases := map[string]string{
		"empty":          "",
		"not json":       "nonsense",
		"unknown field":  `{` + valid + `,"extra":true}`,
		"trailing value": `{` + valid + `} {}`,
		"wrong kind":     `{"kind":"steamguard-enrollment-pending","version":1,"steamId":76561198000000000,"accessToken":"tokentokentoken"}`,
		"wrong version":  `{"kind":"steamguard-login-only","version":2,"steamId":76561198000000000,"accessToken":"tokentokentoken"}`,
		// encoding/json takes the last occurrence of a duplicate key, so without
		// the pre-pass a crafted record could carry a decoy token.
		"duplicate key": `{` + valid + `,"accessToken":"othertokenothertoken"}`,
	}
	for name, raw := range cases {
		if _, err := Decode([]byte(raw)); err == nil {
			t.Fatalf("%s: Decode succeeded, want error", name)
		}
	}
}

func TestDecodeRejectsAnOversizedRecord(t *testing.T) {
	t.Parallel()
	raw := append([]byte(`{"kind":"steamguard-login-only","version":1,"accountName":"`), make([]byte, maxRecordBytes)...)
	for i := range raw[58:] {
		raw[58+i] = 'a'
	}
	raw = append(raw, []byte(`"}`)...)
	if _, err := Decode(raw); err == nil {
		t.Fatal("Decode succeeded, want error")
	}
}
