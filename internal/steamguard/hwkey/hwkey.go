// Package hwkey isolates hardware security keys behind one interface. Nothing
// else imports a FIDO library: callers receive already-evaluated bytes, so
// enrolment, unlock, backup and restore all stay testable without a device.
//
// The FIDO dependencies are confined to this package on purpose. They are
// pre-v1 and single-maintainer, so containing them here means an upgrade can
// only break this one file, and pinned_versions_test.go makes such an upgrade
// deliberate. They are not vendored: go mod vendor cannot complete in this
// repository because Wails embeds a WebView2 DLL that is not present in the
// module, and it would add 3358 files for a guarantee the immutable Go module
// proxy already provides alongside go.sum.
package hwkey

import (
	"context"
	"crypto/sha256"
	"errors"
)

var (
	ErrUnavailable = errors.New("no security key support on this system")
	ErrNoDevice    = errors.New("no security key found")
	ErrCancelled   = errors.New("security key request cancelled")
	// ErrNotEnrolled means the key present is not the one this slot expects.
	ErrNotEnrolled = errors.New("this security key is not enrolled")
)

// SecretLength is the size of an evaluated secret, fixed by the hmac-secret and
// PRF extensions.
const SecretLength = 32

// Credential is everything needed to ask an authenticator for the same secret
// again. None of it is secret; it lives in the vault header so an enrolled key
// still opens a backup restored on another machine.
type Credential struct {
	// ID is the credential handle, base64url without padding - the encoding
	// WebAuthn uses. Anything reading it back has to use the same alphabet.
	ID string
	// RPID is the relying-party identifier. It never changes: changing it
	// orphans every key already enrolled.
	RPID string
	// UV records whether user verification was required at enrolment.
	// Authenticators derive a different secret with and without it, so
	// asserting the other way silently returns the wrong bytes.
	UV bool
}

// Authenticator talks to a physical security key.
type Authenticator interface {
	// Available reports whether this system can use security keys at all, and
	// why not when it cannot, so the UI can hide the option with a reason
	// rather than offering something that always fails.
	Available(ctx context.Context) (bool, string)

	// Enroll creates a credential. Implementations must request a discoverable
	// credential: Windows drops the hmac-secret extension for non-discoverable
	// ones, and a credential that works on one platform has to work on all.
	Enroll(ctx context.Context, rpID string, userName string) (Credential, error)

	// Evaluate asks for the secret of whichever of creds the attached key
	// actually holds, and reports which one answered. It is deterministic: the
	// same credential always yields the same bytes, which is what makes it
	// usable as key material rather than as an assertion.
	//
	// Every candidate goes in one request rather than one request per
	// credential. Several keys can be enrolled and the user is holding one of
	// them; asking about them one at a time both prompts repeatedly and cannot
	// tell "this key is not that credential" from "no key is attached", which
	// made every key but the first-enrolled one unusable.
	Evaluate(ctx context.Context, creds []Credential) (Credential, []byte, error)
}

// RPID is this application's relying-party identifier. It must never change.
const RPID = "steamguard.tcno.co"

// SaltFor is the value handed to the authenticator to derive a credential's
// secret. Derived from the credential handle rather than stored separately, so
// enrolment and unlock cannot disagree about it: the same input has to be used
// both times or the key returns different bytes and the slot never opens.
// Distinct per credential, so two keys enrolled on one vault derive different
// secrets.
func SaltFor(cred Credential) []byte {
	sum := sha256.Sum256([]byte("tcno-steamguard-prf/v1\x00" + cred.RPID + "\x00" + cred.ID))
	return sum[:]
}

// Unsupported is the Authenticator used where no driver is built in. It fails
// every call rather than pretending a key is absent, so the UI reports "not
// supported here" instead of "plug in your key".
type Unsupported struct{ Reason string }

func (u Unsupported) Available(context.Context) (bool, string) {
	reason := u.Reason
	if reason == "" {
		reason = "security keys are not supported in this build"
	}
	return false, reason
}

func (u Unsupported) Enroll(context.Context, string, string) (Credential, error) {
	return Credential{}, ErrUnavailable
}

func (u Unsupported) Evaluate(context.Context, []Credential) (Credential, []byte, error) {
	return Credential{}, nil, ErrUnavailable
}
