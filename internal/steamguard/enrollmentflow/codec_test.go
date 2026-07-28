package enrollmentflow

import (
	"bytes"
	"testing"

	"TcNo-Acc-Switcher/internal/steamguard/enrollmentapi"
)

type codecTestHelper interface {
	Helper()
	Fatal(args ...any)
}

func pendingCodecFixture(t codecTestHelper) []byte {
	t.Helper()
	pending := &enrollmentapi.PendingEnrollment{
		RequestID:      bytes.Repeat([]byte{0x11}, 16),
		SteamID:        testSteamID,
		AccessToken:    []byte("generated-access-token"),
		DeviceID:       "android:00112233-4455-4677-8899-aabbccddeeff",
		SharedSecret:   bytes.Repeat([]byte{0x21}, 20),
		IdentitySecret: bytes.Repeat([]byte{0x31}, 20),
		Secret1:        bytes.Repeat([]byte{0x41}, 20),
		RevocationCode: []byte("R12345"),
		URI:            []byte("otpauth://totp/Steam:generated?secret=EXAMPLE"),
		SerialNumber:   123456789,
		ServerTime:     1_700_000_000,
		AccountName:    "generated-account",
		TokenGID:       "generated-token-gid",
		PhoneHint:      "***42",
		Confirmation:   enrollmentapi.ConfirmationSMS,
	}
	record, err := pendingFromAPI(pending, enrollmentapi.StateAwaitingSMS)
	pending.Destroy()
	if err != nil {
		t.Fatal(err)
	}
	defer record.destroy()
	raw, err := encodePending(&record)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestDecodePendingRejectsDuplicateKeysAndUnknownVersions(t *testing.T) {
	valid := pendingCodecFixture(t)
	duplicate := append([]byte(`{"kind":"steamguard-enrollment-pending",`), valid[1:]...)
	unknownVersion := bytes.Replace(valid, []byte(`"version":2`), []byte(`"version":3`), 1)
	for name, input := range map[string][]byte{
		"duplicate key":   duplicate,
		"unknown version": unknownVersion,
		"invalid UTF-8":   append([]byte{0xff}, valid...),
	} {
		t.Run(name, func(t *testing.T) {
			record, err := decodePending(input)
			record.destroy()
			if err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func FuzzDecodePendingBounded(f *testing.F) {
	f.Add(pendingCodecFixture(f))
	f.Add([]byte(`{"kind":"steamguard-enrollment-pending","kind":"other"}`))
	f.Add([]byte{0xff})
	f.Fuzz(func(t *testing.T, input []byte) {
		record, err := decodePending(input)
		if err != nil {
			return
		}
		defer record.destroy()
		canonical, err := encodePending(&record)
		if err != nil {
			t.Fatalf("accepted state could not be encoded: %v", err)
		}
		defer wipe(canonical)
		reparsed, err := decodePending(canonical)
		if err != nil {
			t.Fatalf("canonical state could not be decoded: %v", err)
		}
		reparsed.destroy()
	})
}
