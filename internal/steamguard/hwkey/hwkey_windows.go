//go:build windows

package hwkey

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	webauthn "github.com/go-ctap/ctaphid/pkg/webauthntypes"
	"github.com/go-ctap/winhello"
	"github.com/go-ctap/winhello/hiddenwindow"
	"github.com/ldclabs/cose/iana"
)

// Windows routes security keys through webauthn.dll. Talking to the device
// directly is not an option: Windows has blocked raw FIDO HID for non-elevated
// processes since 10 1903, and prompting for elevation on every vault unlock
// would be worse than not supporting keys at all.
//
// minAPIVersion is where webauthn.dll gained hmac-secret/PRF salts on the
// assertion path. Below it the extension is silently absent rather than
// refused, which would look like a key that returns the wrong secret.
const minAPIVersion = 4

// clientData is fixed. Windows hashes it into the assertion, but the PRF output
// does not depend on it, and a varying value would only make failures harder to
// reason about.
var clientData = []byte(`{"type":"tcno.steamguard"}`)

type windowsAuthenticator struct{}

// New returns the platform's authenticator driver.
func New() Authenticator { return windowsAuthenticator{} }

func (windowsAuthenticator) Available(context.Context) (bool, string) {
	if version := winhello.APIVersionNumber(); version < minAPIVersion {
		return false, fmt.Sprintf("Windows WebAuthn API version %d cannot derive a key from a security key; version %d or later is required", version, minAPIVersion)
	}
	return true, ""
}

func (a windowsAuthenticator) Enroll(ctx context.Context, rpID, userName string) (Credential, error) {
	if ok, reason := a.Available(ctx); !ok {
		return Credential{}, fmt.Errorf("%w: %s", ErrUnavailable, reason)
	}
	window, err := hiddenwindow.New(slog.New(slog.DiscardHandler), "TcNo Account Switcher")
	if err != nil {
		return Credential{}, err
	}
	defer window.Close()

	result, err := winhello.MakeCredential(
		window.WindowHandle(),
		clientData,
		webauthn.PublicKeyCredentialRpEntity{ID: rpID, Name: "TcNo Account Switcher"},
		// The user handle is not an account identifier: one Steam Guard vault
		// holds many Steam accounts, and the key protects the vault.
		webauthn.PublicKeyCredentialUserEntity{
			ID: []byte(userName), Name: userName, DisplayName: userName,
		},
		[]webauthn.PublicKeyCredentialParameters{
			{Type: webauthn.PublicKeyCredentialTypePublicKey, Algorithm: iana.AlgorithmES256},
		},
		nil,
		// No PRF input here. winhello builds a global-eval salt struct from
		// PRFInputs.PRF.Eval whenever PRFInputs is non-nil, and passing it with
		// nothing to evaluate crashes inside the library. PRF is requested at
		// assertion time instead, which is what its own working example does;
		// enrolment proves the key can derive by deriving once, below.
		&webauthn.CreateAuthenticationExtensionsClientInputs{
			CreateCredentialPropertiesInputs: &webauthn.CreateCredentialPropertiesInputs{
				CredentialProperties: true,
			},
		},
		&winhello.AuthenticatorMakeCredentialOptions{
			AuthenticatorAttachment: winhello.WinHelloAuthenticatorAttachmentCrossPlatform,
			// Pinned, not preferred. The authenticator derives a different
			// secret with and without user verification, so anything that could
			// vary between enrolment and unlock silently breaks the slot.
			UserVerificationRequirement: winhello.WinHelloUserVerificationRequirementRequired,
		},
	)
	if err != nil {
		return Credential{}, translateWindowsError(err)
	}
	if len(result.CredentialID) == 0 {
		return Credential{}, errors.New("security key returned no credential handle")
	}
	return Credential{
		ID:   base64.RawURLEncoding.EncodeToString(result.CredentialID),
		RPID: rpID,
		UV:   true,
	}, nil
}

// Evaluate offers every candidate credential in one assertion and lets Windows
// match whichever key is attached. Asking per credential would prompt once per
// enrolled key and, worse, could not distinguish "this key does not hold that
// credential" from "no key is plugged in" - Windows reports both the same way.
func (a windowsAuthenticator) Evaluate(ctx context.Context, creds []Credential) (Credential, []byte, error) {
	if ok, reason := a.Available(ctx); !ok {
		return Credential{}, nil, fmt.Errorf("%w: %s", ErrUnavailable, reason)
	}
	// The user-verification requirement is per request, and an authenticator
	// derives different bytes with and without it, so credentials that disagree
	// cannot share one. In practice enrolment always requires it and this is a
	// single pass.
	var lastErr error = ErrNotEnrolled
	for _, uvRequired := range []bool{true, false} {
		group := make([]Credential, 0, len(creds))
		for _, cred := range creds {
			if cred.UV == uvRequired {
				group = append(group, cred)
			}
		}
		if len(group) == 0 {
			continue
		}
		cred, secret, err := a.assertGroup(group, uvRequired)
		if err == nil {
			return cred, secret, nil
		}
		lastErr = err
		// A cancelled prompt is the user saying no, not a key that did not
		// match; asking again for the other group would just re-prompt.
		if errors.Is(err, ErrCancelled) {
			break
		}
	}
	return Credential{}, nil, lastErr
}

func (a windowsAuthenticator) assertGroup(creds []Credential, uvRequired bool) (Credential, []byte, error) {
	descriptors := make([]webauthn.PublicKeyCredentialDescriptor, 0, len(creds))
	// evalByCredential is keyed by the standard base64url form of the handle,
	// with padding, which is not the form stored in the header.
	salts := make(map[string]webauthn.AuthenticationExtensionsPRFValues, len(creds))
	byID := make(map[string]Credential, len(creds))
	for _, cred := range creds {
		id, err := base64.RawURLEncoding.DecodeString(cred.ID)
		if err != nil || len(id) == 0 {
			continue
		}
		descriptors = append(descriptors, webauthn.PublicKeyCredentialDescriptor{
			ID: id, Type: webauthn.PublicKeyCredentialTypePublicKey,
		})
		salts[base64.URLEncoding.EncodeToString(id)] = webauthn.AuthenticationExtensionsPRFValues{
			First: SaltFor(cred),
		}
		byID[cred.ID] = cred
	}
	if len(descriptors) == 0 {
		return Credential{}, nil, ErrNotEnrolled
	}
	window, err := hiddenwindow.New(slog.New(slog.DiscardHandler), "TcNo Account Switcher")
	if err != nil {
		return Credential{}, nil, err
	}
	defer window.Close()

	uv := winhello.WinHelloUserVerificationRequirementRequired
	if !uvRequired {
		uv = winhello.WinHelloUserVerificationRequirementDiscouraged
	}
	assertion, err := winhello.GetAssertion(
		window.WindowHandle(),
		// Every credential shares the relying party; it is a constant.
		creds[0].RPID,
		clientData,
		descriptors,
		&webauthn.GetAuthenticationExtensionsClientInputs{
			PRFInputs: &webauthn.PRFInputs{
				PRF: webauthn.AuthenticationExtensionsPRFInputs{EvalByCredential: salts},
			},
		},
		&winhello.AuthenticatorGetAssertionOptions{
			AuthenticatorAttachment:     winhello.WinHelloAuthenticatorAttachmentCrossPlatform,
			UserVerificationRequirement: uv,
			CredentialHints: []webauthn.PublicKeyCredentialHint{
				webauthn.PublicKeyCredentialHintSecurityKey,
			},
		},
	)
	if err != nil {
		return Credential{}, nil, translateWindowsError(err)
	}
	used, ok := byID[assertionCredentialID(assertion)]
	if !ok {
		// Windows answered with a credential that was not offered. Deriving from
		// it would produce bytes no slot expects.
		return Credential{}, nil, ErrNotEnrolled
	}
	secret := assertionSecret(assertion)
	if len(secret) != SecretLength {
		// A key that authenticated but returned nothing derived the wrong thing
		// or is not the enrolled one. Committing that would produce a slot that
		// never opens.
		return Credential{}, nil, ErrNotEnrolled
	}
	return used, secret, nil
}

// assertionCredentialID reports which offered credential answered, in the same
// encoding the vault header stores.
func assertionCredentialID(assertion *winhello.WinHelloGetAssertionResponse) string {
	if assertion == nil || assertion.AuthenticatorGetAssertionResponse == nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(assertion.Credential.ID)
}

// assertionSecret digs the PRF result out of the response. Every level is
// optional in the type, and an authenticator that ignored the extension returns
// a perfectly valid assertion with nothing in it, so each hop is checked rather
// than assumed.
func assertionSecret(assertion *winhello.WinHelloGetAssertionResponse) []byte {
	if assertion == nil || assertion.AuthenticatorGetAssertionResponse == nil {
		return nil
	}
	outputs := assertion.ExtensionOutputs
	if outputs == nil || outputs.PRFOutputs == nil {
		return nil
	}
	return outputs.PRF.Results.First
}

// translateWindowsError maps the platform's failures onto reasons the caller can
// act on. Windows reports a cancelled prompt and an absent key the same way, so
// both read as "no key" rather than pretending to know which.
func translateWindowsError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "cancel"):
		return errors.Join(ErrCancelled, err)
	case strings.Contains(message, "not found"), strings.Contains(message, "no credential"),
		strings.Contains(message, "no authenticator"), strings.Contains(message, "timeout"),
		strings.Contains(message, "timed out"):
		return errors.Join(ErrNoDevice, err)
	}
	return err
}
