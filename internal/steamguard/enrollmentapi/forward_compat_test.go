package enrollmentapi

import (
	"errors"
	"strings"
	"testing"
)

// Steam adds fields to its responses over time. Rejecting a whole message for
// an unknown field makes enrolment impossible the day Steam adds one.
func TestAddResponseSkipsFieldsAddedLater(t *testing.T) {
	base := validAddResponse(2)

	withUnknown := appendVarint(base, maxKnownAddResponseField+1, 7)
	withUnknown = appendBytes(withUnknown, maxKnownAddResponseField+2, []byte("something new"))
	withUnknown = appendFixed64(withUnknown, maxKnownAddResponseField+9, 1234)
	// A later field may be repeated, which must not read as a duplicate.
	withUnknown = appendBytes(withUnknown, maxKnownAddResponseField+2, []byte("second value"))
	withUnknown = appendBytes(withUnknown, maxKnownAddResponseField+2, []byte("third value"))

	result, err := unmarshalAddResponse(withUnknown)
	if err != nil {
		t.Fatalf("unknown fields rejected the response: %v", err)
	}
	if result.status != 1 || result.pending == nil {
		t.Fatalf("status = %d, pending = %v", result.status, result.pending != nil)
	}
	if len(result.pending.SharedSecret) != 20 {
		t.Fatal("shared secret was not parsed alongside the unknown fields")
	}
	result.pending.Destroy()
}

func TestFinalizeResponseSkipsFieldsAddedLater(t *testing.T) {
	base := appendVarint(nil, 1, 1)
	base = appendVarint(base, 4, 1)

	// A later field of a wire type this parser never expected must also be
	// skipped, not rejected before its field number is read.
	withUnknown := appendBytes(base, maxKnownFinalizeResponseField+1, []byte("added later"))
	withUnknown = appendVarint(withUnknown, maxKnownFinalizeResponseField+2, 3)
	withUnknown = appendBytes(withUnknown, maxKnownFinalizeResponseField+1, []byte("repeated"))

	result, err := unmarshalFinalizeResponse(withUnknown)
	if err != nil {
		t.Fatalf("unknown fields rejected the response: %v", err)
	}
	if !result.success || result.status != 1 {
		t.Fatalf("success = %v, status = %d", result.success, result.status)
	}
}

func TestKnownFieldsStillValidated(t *testing.T) {
	shortSecret := appendBytes(nil, 1, []byte("too short"))
	shortSecret = appendFixed64(shortSecret, 2, 42)
	shortSecret = appendBytes(shortSecret, 3, []byte("R12345"))
	shortSecret = appendBytes(shortSecret, 4, []byte("otpauth://totp/Steam:test?secret=AAAA&issuer=Steam"))
	shortSecret = appendVarint(shortSecret, 5, testUnix)
	shortSecret = appendBytes(shortSecret, 6, []byte("test_account"))
	shortSecret = appendBytes(shortSecret, 7, []byte("123456789"))
	shortSecret = appendBytes(shortSecret, 8, make([]byte, 20))
	shortSecret = appendBytes(shortSecret, 9, make([]byte, 20))
	shortSecret = appendVarint(shortSecret, 10, 1)
	shortSecret = appendVarint(shortSecret, 12, 2)

	if _, err := unmarshalAddResponse(shortSecret); !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("a bad shared secret was accepted: %v", err)
	}
	// The reason is named, so a failure in the field is diagnosable from a log.
	if _, err := unmarshalAddResponse(shortSecret); err.Error() == ErrInvalidResponse.Error() {
		t.Fatal("rejection did not say which check failed")
	}
	// A duplicated known field is still a rejection.
	dup := appendVarint(validAddResponse(2), 10, 1)
	if _, err := unmarshalAddResponse(dup); !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("a duplicate status field was accepted: %v", err)
	}
}

// token_gid is an opaque Steam identifier, not a number: Steam GIDs commonly
// contain hex letters, and this package never reads it as a value.
func TestAddResponseAcceptsNonNumericTokenGID(t *testing.T) {
	for _, gid := range []string{"c1a2b3d4e5f60718", "0123456789abcdef", "9876543210"} {
		message := appendBytes(nil, 1, make([]byte, 20))
		message = appendFixed64(message, 2, 42)
		message = appendBytes(message, 3, []byte("R12345"))
		message = appendBytes(message, 4, []byte("otpauth://totp/Steam:test?secret=AAAA&issuer=Steam"))
		message = appendVarint(message, 5, testUnix)
		message = appendBytes(message, 6, []byte("test_account"))
		message = appendBytes(message, 7, []byte(gid))
		message = appendBytes(message, 8, make([]byte, 20))
		message = appendBytes(message, 9, make([]byte, 20))
		message = appendVarint(message, 10, 1)
		message = appendVarint(message, 12, 2)

		result, err := unmarshalAddResponse(message)
		if err != nil {
			t.Fatalf("token_gid %q rejected: %v", gid, err)
		}
		if result.pending.TokenGID != gid {
			t.Fatalf("token_gid = %q, want %q", result.pending.TokenGID, gid)
		}
		result.pending.Destroy()
	}
}

// By the time a confirm_type arrives, Steam has already created the
// authenticator and emailed or texted the user their code. A value this build
// does not recognise must not discard the response: that would throw away the
// account's secrets while leaving the enrolment half-finished on Steam's side.
func TestAddResponseKeepsSecretsOnUnknownConfirmType(t *testing.T) {
	for _, confirm := range []uint64{0, 3, 4, 9, 255} {
		result, err := unmarshalAddResponse(validAddResponse(confirm))
		if err != nil {
			t.Fatalf("confirm_type %d rejected the response: %v", confirm, err)
		}
		if result.pending == nil || len(result.pending.SharedSecret) != 20 {
			t.Fatalf("confirm_type %d lost the shared secret", confirm)
		}
		want := ConfirmationUnknown
		if confirm == 1 {
			want = ConfirmationSMS
		}
		if result.pending.Confirmation != want {
			t.Fatalf("confirm_type %d mapped to %v, want %v", confirm, result.pending.Confirmation, want)
		}
		result.pending.Destroy()
	}

	// The two Steam values this build does know still map to their own states.
	for confirm, want := range map[uint64]ConfirmationType{1: ConfirmationSMS, 2: ConfirmationEmail} {
		result, err := unmarshalAddResponse(validAddResponse(confirm))
		if err != nil {
			t.Fatal(err)
		}
		if result.pending.Confirmation != want {
			t.Fatalf("confirm_type %d mapped to %v, want %v", confirm, result.pending.Confirmation, want)
		}
		result.pending.Destroy()
	}
}

// validate_sms_code must reflect how Steam asked the user to confirm. Claiming
// an SMS code when the code arrived by email makes Steam reject an otherwise
// correct finalize.
func TestFinalizeRequestOnlyClaimsSMSWhenSteamUsedSMS(t *testing.T) {
	code := []byte("ABCDE")
	confirmation := []byte("8Y36B")

	sms := marshalFinalizeRequest(76561197960265729, code, uint64(testUnix), confirmation, true)
	email := marshalFinalizeRequest(76561197960265729, code, uint64(testUnix), confirmation, false)

	if !hasField(sms, 6) {
		t.Fatal("SMS finalize did not set validate_sms_code")
	}
	if hasField(email, 6) {
		t.Fatal("email finalize claimed the code was an SMS code")
	}
	// Everything else is identical, so only the claim changed.
	for _, number := range []uint32{1, 2, 3, 4} {
		if !hasField(sms, number) || !hasField(email, number) {
			t.Fatalf("field %d missing from a finalize request", number)
		}
	}
}

func hasField(message []byte, number uint32) bool {
	decoder := wireDecoder{data: message}
	for {
		field, ok := decoder.next()
		if !ok {
			return false
		}
		if field.number == number {
			return true
		}
	}
}

// A rejection has to carry Steam's result code, or every refusal reads the same
// and there is nothing to tell a wrong code from a rate limit.
func TestSteamErrorNamesTheResultCode(t *testing.T) {
	err := &SteamError{ResultCode: 84}
	if !errors.Is(err, ErrSteamRejected) {
		t.Fatal("SteamError no longer unwraps to ErrSteamRejected")
	}
	if !strings.Contains(err.Error(), "84") {
		t.Fatalf("error text %q does not name the result code", err.Error())
	}
}
