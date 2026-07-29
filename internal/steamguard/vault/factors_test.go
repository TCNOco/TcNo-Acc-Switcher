package vault

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestKeyfileRoundTrip(t *testing.T) {
	k, err := NewKeyfile()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseKeyfile(k.Encode())
	if err != nil {
		t.Fatalf("ParseKeyfile(Encode()) error = %v", err)
	}
	if parsed.ID != k.ID || !bytes.Equal(parsed.Secret, k.Secret) {
		t.Fatal("keyfile did not survive a round trip")
	}
	// Two keyfiles never collide.
	other, err := NewKeyfile()
	if err != nil {
		t.Fatal(err)
	}
	if other.ID == k.ID || bytes.Equal(other.Secret, k.Secret) {
		t.Fatal("NewKeyfile repeated itself")
	}
}

func TestParseKeyfileRejectsOtherFiles(t *testing.T) {
	valid := string(mustKeyfile(t).Encode())
	for name, data := range map[string]string{
		"empty":          "",
		"unrelated text": "just some notes about my steam account",
		"missing magic":  strings.TrimPrefix(valid, keyfileMagic),
		"truncated key":  strings.Replace(valid, "key: ", "key: AAAA", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseKeyfile([]byte(data)); !errors.Is(err, ErrInvalidKeyfile) {
				t.Fatalf("ParseKeyfile(%s) error = %v, want %v", name, err, ErrInvalidKeyfile)
			}
		})
	}
}

func mustKeyfile(t *testing.T) Keyfile {
	t.Helper()
	k, err := NewKeyfile()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// The user retypes a backup key off paper, so parsing has to tolerate how
// people actually type: any casing, and the grouping hyphens or not.
func TestRecoveryCodeParsesHoweverItIsTyped(t *testing.T) {
	code, err := NewRecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	want, err := ParseRecoveryCode(code)
	if err != nil {
		t.Fatalf("ParseRecoveryCode(generated) error = %v", err)
	}
	if len(want) != recoveryCodeBytes {
		t.Fatalf("decoded %d bytes, want %d", len(want), recoveryCodeBytes)
	}
	for name, variant := range map[string]string{
		"lower case": strings.ToLower(code),
		"no hyphens": strings.ReplaceAll(code, "-", ""),
		"spaced":     strings.ReplaceAll(code, "-", " "),
		"padded":     "  " + code + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ParseRecoveryCode(variant)
			if err != nil {
				t.Fatalf("ParseRecoveryCode(%s) error = %v", name, err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("ParseRecoveryCode(%s) decoded differently", name)
			}
		})
	}
	if _, err := ParseRecoveryCode("not a real backup key"); !errors.Is(err, ErrInvalidRecoveryCode) {
		t.Fatal("garbage was accepted as a backup key")
	}
}

// Enrolling a second factor alongside the password means either one opens the
// vault. Enrolling one that replaces the password slot means both are needed.
// The difference is the whole feature, so it is pinned here.
func TestAddSlotReplacingThePasswordSlotRequiresBoth(t *testing.T) {
	const password = "correct horse battery staple"
	v := newUnlocked(t)
	keyfile := mustKeyfile(t)

	slots := v.ListSlots()
	if len(slots) != 1 {
		t.Fatalf("new vault has %d slots, want 1", len(slots))
	}
	combined := Credentials{Password: password, Keyfile: keyfile.Secret}
	if _, err := v.AddSlot(SlotSpec{
		Label:     "Password and keyfile",
		Kinds:     []string{FactorPassword, FactorKeyfile},
		Creds:     combined,
		KeyfileID: keyfile.ID,
	}, []string{slots[0].ID}); err != nil {
		t.Fatal(err)
	}
	if got := v.ListSlots(); len(got) != 1 {
		t.Fatalf("after replacing, vault has %d slots, want 1", len(got))
	}

	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := v.Unlock(password, FixedLease); !errors.Is(err, ErrFactorRequired) {
		t.Fatalf("password alone still opens the vault: %v", err)
	}
	if err := v.UnlockWith(combined, FixedLease); err != nil {
		t.Fatalf("password and keyfile did not open the vault: %v", err)
	}
}

func TestAddSlotAlongsideThePasswordIsAnAlternative(t *testing.T) {
	const password = "correct horse battery staple"
	v := newUnlocked(t)
	code, err := NewRecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := ParseRecoveryCode(code)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.AddSlot(SlotSpec{
		Label: "Backup key",
		Kinds: []string{FactorRecoveryCode},
		Creds: Credentials{RecoveryCode: raw},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := v.Unlock(password, FixedLease); err != nil {
		t.Fatalf("password stopped opening the vault: %v", err)
	}
	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := v.UnlockWith(Credentials{RecoveryCode: raw}, FixedLease); err != nil {
		t.Fatalf("backup key did not open the vault: %v", err)
	}
}

// There is no such thing as a vault nobody can open, only data nobody can read.
func TestRemoveSlotRefusesTheLastWayIn(t *testing.T) {
	const password = "correct horse battery staple"
	v := newUnlocked(t)
	slots := v.ListSlots()
	if err := v.RemoveSlot(slots[0].ID); !errors.Is(err, ErrLastSlot) {
		t.Fatalf("removing the only slot: err = %v, want %v", err, ErrLastSlot)
	}

	code, err := NewRecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := ParseRecoveryCode(code)
	if err != nil {
		t.Fatal(err)
	}
	added, err := v.AddSlot(SlotSpec{
		Label: "Backup key", Kinds: []string{FactorRecoveryCode}, Creds: Credentials{RecoveryCode: raw},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.RemoveSlot(added); err != nil {
		t.Fatalf("removing a spare slot: %v", err)
	}
	if got := v.ListSlots(); len(got) != 1 {
		t.Fatalf("after removal, vault has %d slots, want 1", len(got))
	}
	if err := v.RemoveSlot("not-a-slot"); !errors.Is(err, ErrSlotNotFound) {
		t.Fatalf("removing an unknown slot: err = %v, want %v", err, ErrSlotNotFound)
	}
}

// A security-key slot stores only non-secret descriptors and takes the already
// evaluated secret, so the whole path works without a device attached.
func TestSecurityKeySlotOpensFromEvaluatedBytes(t *testing.T) {
	h := newSlotTestHeader(t)
	vaultKey := bytes.Repeat([]byte{0xa1}, keyBytes)
	evaluated := bytes.Repeat([]byte{0xb2}, 32)

	salt, err := randomBytes(saltBytes)
	if err != nil {
		t.Fatal(err)
	}
	factor := slotFactor{
		Type:         FactorSecurityKey,
		Salt:         base64.RawStdEncoding.EncodeToString(salt),
		CredentialID: base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{0x0c}, 32)),
		RPID:         "steamguard.tcno.co",
		UV:           true,
	}
	creds := Credentials{SecurityKey: evaluated}
	slot, err := buildSlot(h, "Security key", []slotFactor{factor}, creds, vaultKey)
	if err != nil {
		t.Fatal(err)
	}
	h.Slots = []keySlot{slot}
	if err := validateSlots(h.Slots); err != nil {
		t.Fatalf("security key slot failed validation: %v", err)
	}

	got, err := openVaultKey(h, creds)
	if err != nil {
		t.Fatalf("evaluated secret did not open the slot: %v", err)
	}
	if !bytes.Equal(got, vaultKey) {
		t.Fatal("recovered the wrong vault key")
	}
	if _, err := openVaultKey(h, PasswordOnly("anything")); !errors.Is(err, ErrFactorRequired) {
		t.Fatalf("a password opened a security-key slot: %v", err)
	}
	other := Credentials{SecurityKey: bytes.Repeat([]byte{0xc3}, 32)}
	if _, err := openVaultKey(h, other); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("a different key opened the slot: %v", err)
	}
}

// Changing the recorded user-verification flag changes what the authenticator
// would return, so it must invalidate the slot rather than silently mismatch.
func TestSecurityKeyDescriptorsAreBoundToTheSlot(t *testing.T) {
	h := newSlotTestHeader(t)
	vaultKey := bytes.Repeat([]byte{0xd4}, keyBytes)
	evaluated := bytes.Repeat([]byte{0xe5}, 32)
	salt, err := randomBytes(saltBytes)
	if err != nil {
		t.Fatal(err)
	}
	factor := slotFactor{
		Type:         FactorSecurityKey,
		Salt:         base64.RawStdEncoding.EncodeToString(salt),
		CredentialID: base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{0x0f}, 32)),
		RPID:         "steamguard.tcno.co",
		UV:           true,
	}
	creds := Credentials{SecurityKey: evaluated}
	slot, err := buildSlot(h, "Security key", []slotFactor{factor}, creds, vaultKey)
	if err != nil {
		t.Fatal(err)
	}
	slot.Factors[0].UV = false
	h.Slots = []keySlot{slot}
	if _, err := openVaultKey(h, creds); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("edited descriptors still opened the slot: %v", err)
	}
}
