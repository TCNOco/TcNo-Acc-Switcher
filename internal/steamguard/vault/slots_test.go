package vault

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

const slotTestPassword = "correct horse battery staple"

func slotTestKDF() KDFParams {
	return KDFParams{Algorithm: "argon2id", MemoryKiB: minMemoryKiB, Passes: 1, Lanes: 1, KeyBytes: keyBytes}
}

func newKeyfileFactor(t *testing.T) slotFactor {
	t.Helper()
	salt, err := randomBytes(saltBytes)
	if err != nil {
		t.Fatal(err)
	}
	id, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	return slotFactor{Type: FactorKeyfile, Salt: base64.RawStdEncoding.EncodeToString(salt), KeyfileID: id}
}

func newSlotTestHeader(t *testing.T) header {
	t.Helper()
	vaultID, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	keyringID, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	return header{Version: FormatVersion, VaultID: vaultID, KeyringID: keyringID}
}

// Every factor listed by a slot is required. This is the property that makes a
// slot mean "password AND keyfile" rather than "either one".
func TestSlotRequiresEveryFactor(t *testing.T) {
	h := newSlotTestHeader(t)
	vaultKey := bytes.Repeat([]byte{0x11}, keyBytes)
	keyfile := bytes.Repeat([]byte{0x22}, 32)
	password, err := newPasswordFactor(slotTestKDF())
	if err != nil {
		t.Fatal(err)
	}
	both := Credentials{Password: slotTestPassword, Keyfile: keyfile}
	slot, err := buildSlot(h, "Password and keyfile", []slotFactor{password, newKeyfileFactor(t)}, both, vaultKey)
	if err != nil {
		t.Fatal(err)
	}
	h.Slots = []keySlot{slot}

	got, err := openVaultKey(h, both)
	if err != nil {
		t.Fatalf("both factors did not open the slot: %v", err)
	}
	if !bytes.Equal(got, vaultKey) {
		t.Fatal("recovered vault key does not match")
	}

	// The password alone cannot open it, and must say so as a missing factor
	// rather than as a wrong password.
	if _, err := openVaultKey(h, PasswordOnly(slotTestPassword)); !errors.Is(err, ErrFactorRequired) {
		t.Fatalf("password alone: err = %v, want %v", err, ErrFactorRequired)
	}
	wrongKeyfile := Credentials{Password: slotTestPassword, Keyfile: bytes.Repeat([]byte{0x33}, 32)}
	if _, err := openVaultKey(h, wrongKeyfile); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("wrong keyfile: err = %v, want %v", err, ErrInvalidPassword)
	}
	wrongPassword := Credentials{Password: "not the password", Keyfile: keyfile}
	if _, err := openVaultKey(h, wrongPassword); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("wrong password: err = %v, want %v", err, ErrInvalidPassword)
	}
}

// Slots are alternatives: any one of them opens the vault.
func TestSlotsAreAlternatives(t *testing.T) {
	h := newSlotTestHeader(t)
	vaultKey := bytes.Repeat([]byte{0x44}, keyBytes)
	keyfile := bytes.Repeat([]byte{0x55}, 32)

	passwordFactor, err := newPasswordFactor(slotTestKDF())
	if err != nil {
		t.Fatal(err)
	}
	passwordSlot, err := buildSlot(h, "Password", []slotFactor{passwordFactor}, PasswordOnly(slotTestPassword), vaultKey)
	if err != nil {
		t.Fatal(err)
	}
	keyfileOnly := Credentials{Keyfile: keyfile}
	keyfileSlot, err := buildSlot(h, "Keyfile", []slotFactor{newKeyfileFactor(t)}, keyfileOnly, vaultKey)
	if err != nil {
		t.Fatal(err)
	}
	h.Slots = []keySlot{passwordSlot, keyfileSlot}

	for name, creds := range map[string]Credentials{
		"password": PasswordOnly(slotTestPassword),
		"keyfile":  keyfileOnly,
	} {
		got, err := openVaultKey(h, creds)
		if err != nil {
			t.Fatalf("%s alone did not open the vault: %v", name, err)
		}
		if !bytes.Equal(got, vaultKey) {
			t.Fatalf("%s recovered the wrong vault key", name)
		}
	}
}

// The slot envelope is bound to the factor set, so stripping a requirement out
// of the header breaks the slot instead of weakening it.
func TestStrippingAFactorInvalidatesTheSlot(t *testing.T) {
	h := newSlotTestHeader(t)
	vaultKey := bytes.Repeat([]byte{0x66}, keyBytes)
	keyfile := bytes.Repeat([]byte{0x77}, 32)
	passwordFactor, err := newPasswordFactor(slotTestKDF())
	if err != nil {
		t.Fatal(err)
	}
	both := Credentials{Password: slotTestPassword, Keyfile: keyfile}
	slot, err := buildSlot(h, "Password and keyfile", []slotFactor{passwordFactor, newKeyfileFactor(t)}, both, vaultKey)
	if err != nil {
		t.Fatal(err)
	}
	slot.Factors = slot.Factors[:1]
	h.Slots = []keySlot{slot}

	if _, err := openVaultKey(h, both); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("downgraded slot: err = %v, want %v", err, ErrInvalidPassword)
	}
}

// Changing the password must not revoke a separately enrolled factor.
func TestReissueSlotsLeavesOtherFactorsIntact(t *testing.T) {
	h := newSlotTestHeader(t)
	vaultKey := bytes.Repeat([]byte{0x88}, keyBytes)
	keyfile := bytes.Repeat([]byte{0x99}, 32)

	passwordFactor, err := newPasswordFactor(slotTestKDF())
	if err != nil {
		t.Fatal(err)
	}
	passwordSlot, err := buildSlot(h, "Password", []slotFactor{passwordFactor}, PasswordOnly(slotTestPassword), vaultKey)
	if err != nil {
		t.Fatal(err)
	}
	keyfileOnly := Credentials{Keyfile: keyfile}
	keyfileSlot, err := buildSlot(h, "Keyfile", []slotFactor{newKeyfileFactor(t)}, keyfileOnly, vaultKey)
	if err != nil {
		t.Fatal(err)
	}
	h.Slots = []keySlot{passwordSlot, keyfileSlot}

	const replacement = "a different vault password"
	var skipped []string
	h.Slots, skipped, err = reissueSlots(h, PasswordOnly(slotTestPassword), PasswordOnly(replacement), slotTestKDF(), vaultKey)
	if err != nil {
		t.Fatal(err)
	}
	// The keyfile slot has no password factor, so nothing is left behind on the
	// old password and the change is safe to commit.
	if len(skipped) != 0 {
		t.Fatalf("reported %v as still using the old password", skipped)
	}
	if _, err := openVaultKey(h, PasswordOnly(replacement)); err != nil {
		t.Fatalf("new password did not open the vault: %v", err)
	}
	if _, err := openVaultKey(h, PasswordOnly(slotTestPassword)); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("old password still opens the vault: %v", err)
	}
	if _, err := openVaultKey(h, keyfileOnly); err != nil {
		t.Fatalf("password change revoked the keyfile factor: %v", err)
	}
}

// Credential handles are base64url, the encoding WebAuthn uses and the one the
// platform driver writes. Validating them with the standard alphabet accepted
// only handles that happened to avoid the two characters the alphabets disagree
// on, so roughly a quarter of security keys enrolled and the rest were rejected
// as a corrupt vault.
func TestSecurityKeySlotAcceptsABase64URLCredential(t *testing.T) {
	// Encodes to "--8_AAEC..." - '-' and '_' where the standard alphabet has
	// '+' and '/'.
	raw := append([]byte{0xfb, 0xef, 0x3f}, make([]byte, 29)...)
	for i := range raw[3:] {
		raw[3+i] = byte(i)
	}
	handle := base64.RawURLEncoding.EncodeToString(raw)
	if !strings.ContainsAny(handle, "-_") {
		t.Fatalf("handle %q does not exercise the alphabets", handle)
	}

	h := newSlotTestHeader(t)
	vaultKey := bytes.Repeat([]byte{0x5a}, keyBytes)
	salt, err := randomBytes(saltBytes)
	if err != nil {
		t.Fatal(err)
	}
	factor := slotFactor{
		Type:         FactorSecurityKey,
		Salt:         base64.RawStdEncoding.EncodeToString(salt),
		CredentialID: handle,
		RPID:         "steamguard.tcno.co",
		UV:           true,
	}
	creds := Credentials{SecurityKey: bytes.Repeat([]byte{0x77}, 32)}
	slot, err := buildSlot(h, "Blue key", []slotFactor{factor}, creds, vaultKey)
	if err != nil {
		t.Fatal(err)
	}
	h.Slots = []keySlot{slot}
	if err := validateSlots(h.Slots); err != nil {
		t.Fatalf("a base64url credential handle was rejected: %v", err)
	}
	if _, err := openVaultKey(h, creds); err != nil {
		t.Fatalf("the slot did not open: %v", err)
	}
}
