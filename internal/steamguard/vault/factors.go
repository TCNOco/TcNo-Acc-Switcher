package vault

import (
	"bufio"
	"bytes"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const (
	keyfileMagic  = "TcNo Account Switcher Steam Guard keyfile"
	keyfileSecret = 32
	// 16 bytes is 26 base32 characters, which groups evenly into 5s.
	recoveryCodeBytes = 16
	maxKeyfileBytes   = 8 << 10
)

var (
	ErrInvalidKeyfile      = errors.New("not a Steam Guard keyfile")
	ErrInvalidRecoveryCode = errors.New("invalid Steam Guard backup key")
)

// recoveryCodeEncoding omits padding and uses the standard alphabet without
// digits that read ambiguously in print.
var recoveryCodeEncoding = base32.NewEncoding("ABCDEFGHJKLMNPQRSTUVWXYZ23456789").WithPadding(base32.NoPadding)

// Keyfile is generated material the user keeps outside the vault. The ID is not
// secret: it lets the app say "this is the wrong keyfile" instead of failing as
// though the password were wrong.
type Keyfile struct {
	ID     string
	Secret []byte
}

func NewKeyfile() (Keyfile, error) {
	id, err := randomID()
	if err != nil {
		return Keyfile{}, err
	}
	secret, err := randomBytes(keyfileSecret)
	if err != nil {
		return Keyfile{}, err
	}
	return Keyfile{ID: id, Secret: secret}, nil
}

// Encode renders the file the user saves. The warning is part of the file
// because the file will outlive any explanation shown when it was created.
func (k Keyfile) Encode() []byte {
	var b strings.Builder
	b.WriteString(keyfileMagic + "\n")
	b.WriteString("\n")
	b.WriteString("Anyone holding this file, together with the other factors it was\n")
	b.WriteString("enrolled with, can open the Steam Guard vault it belongs to.\n")
	b.WriteString("Losing it may make that vault permanently unreadable.\n")
	b.WriteString("\n")
	fmt.Fprintf(&b, "id: %s\n", k.ID)
	fmt.Fprintf(&b, "key: %s\n", base64.RawStdEncoding.EncodeToString(k.Secret))
	return []byte(b.String())
}

func ParseKeyfile(data []byte) (Keyfile, error) {
	if len(data) == 0 || len(data) > maxKeyfileBytes {
		return Keyfile{}, ErrInvalidKeyfile
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var parsed Keyfile
	sawMagic := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == keyfileMagic:
			sawMagic = true
		case strings.HasPrefix(line, "id:"):
			parsed.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		case strings.HasPrefix(line, "key:"):
			secret, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(strings.TrimPrefix(line, "key:")))
			if err != nil {
				return Keyfile{}, ErrInvalidKeyfile
			}
			parsed.Secret = secret
		}
	}
	if err := scanner.Err(); err != nil {
		return Keyfile{}, ErrInvalidKeyfile
	}
	if !sawMagic || !validID(parsed.ID) || len(parsed.Secret) != keyfileSecret {
		return Keyfile{}, ErrInvalidKeyfile
	}
	return parsed, nil
}

// NewRecoveryCode returns a backup key as the user sees it: groups of five,
// separated by hyphens. The returned string is the only copy.
func NewRecoveryCode() (string, error) {
	raw, err := randomBytes(recoveryCodeBytes)
	if err != nil {
		return "", err
	}
	defer wipe(raw)
	encoded := recoveryCodeEncoding.EncodeToString(raw)
	var b strings.Builder
	for i, r := range encoded {
		if i > 0 && i%5 == 0 {
			b.WriteByte('-')
		}
		b.WriteRune(r)
	}
	return b.String(), nil
}

// ParseRecoveryCode accepts the code however the user typed it: any casing,
// with or without the grouping hyphens or stray whitespace.
func ParseRecoveryCode(code string) ([]byte, error) {
	var cleaned strings.Builder
	for _, r := range strings.ToUpper(code) {
		if r == '-' || r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		cleaned.WriteRune(r)
	}
	raw, err := recoveryCodeEncoding.DecodeString(cleaned.String())
	if err != nil || len(raw) != recoveryCodeBytes {
		return nil, ErrInvalidRecoveryCode
	}
	return raw, nil
}

// SlotInfo describes an enrolled way to open the vault. Nothing here is secret.
type SlotInfo struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Factors []string `json:"factors"`
}

// ListSlots reports every enrolled way in, so the UI can state plainly that any
// one of them opens the vault.
func (v *Vault) ListSlots() []SlotInfo {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make([]SlotInfo, 0, len(v.header.Slots))
	for _, slot := range v.header.Slots {
		kinds := make([]string, 0, len(slot.Factors))
		for _, factor := range slot.Factors {
			kinds = append(kinds, factor.Type)
		}
		out = append(out, SlotInfo{ID: slot.ID, Label: slot.Label, Factors: kinds})
	}
	return out
}

// SlotSpec describes a slot to enrol. Kinds lists the factors it will require,
// all of them, and Creds carries the material for each.
type SlotSpec struct {
	Label     string
	Kinds     []string
	Creds     Credentials
	KeyfileID string
	// Security-key descriptors, needed to ask the same authenticator for the
	// same secret again. None of it is secret.
	CredentialID string
	RPID         string
	UV           bool
}

// SecurityKeyRef is what a caller needs to re-derive a slot's security-key
// secret: which slot, and which credential on which authenticator.
type SecurityKeyRef struct {
	SlotID       string
	CredentialID string
	RPID         string
	UV           bool
}

// SecurityKeys lists every enrolled security-key credential. Unlock walks these
// and asks each in turn, because the user may hold any one of several keys.
func (v *Vault) SecurityKeys() []SecurityKeyRef {
	v.mu.Lock()
	defer v.mu.Unlock()
	var out []SecurityKeyRef
	for _, slot := range v.header.Slots {
		for _, factor := range slot.Factors {
			if factor.Type != FactorSecurityKey {
				continue
			}
			out = append(out, SecurityKeyRef{
				SlotID:       slot.ID,
				CredentialID: factor.CredentialID,
				RPID:         factor.RPID,
				UV:           factor.UV,
			})
		}
	}
	return out
}

// AddSlot enrols another way to open the vault. The vault must be unlocked: the
// key is taken from the open vault rather than re-derived, so managing factors
// does not require presenting every factor a second time. Re-deriving was wrong
// - once a slot needed a password and a keyfile, a password alone could no
// longer add or remove anything.
//
// Replaces reports slots to drop in the same commit, which is how "password
// only" becomes "password AND keyfile" rather than "either one".
func (v *Vault) AddSlot(spec SlotSpec, replaces []string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(spec.Kinds) == 0 || len(spec.Kinds) > 4 {
		return "", ErrInvalidFormat
	}
	factors := make([]slotFactor, 0, len(spec.Kinds))
	for _, kind := range spec.Kinds {
		factor, err := v.newFactorLocked(kind, spec)
		if err != nil {
			return "", err
		}
		factors = append(factors, factor)
	}
	kept := make([]keySlot, 0, len(v.header.Slots)+1)
	dropped := make(map[string]bool, len(replaces))
	for _, id := range replaces {
		dropped[id] = true
	}
	for _, existing := range v.header.Slots {
		if !dropped[existing.ID] {
			kept = append(kept, existing)
		}
	}

	var slotID string
	err := v.withKeysLocked(func(vaultKey, outerKey []byte) error {
		nextHeader := v.header
		slot, err := buildSlot(nextHeader, spec.Label, factors, spec.Creds, vaultKey)
		if err != nil {
			return err
		}
		nextHeader.Slots = append(append([]keySlot(nil), kept...), slot)
		if err := validateSlots(nextHeader.Slots); err != nil {
			return err
		}
		if err := v.commitHeaderLocked(nextHeader, vaultKey, outerKey); err != nil {
			return err
		}
		slotID = slot.ID
		return nil
	})
	if err != nil {
		return "", err
	}
	return slotID, nil
}

func (v *Vault) newFactorLocked(kind string, spec SlotSpec) (slotFactor, error) {
	switch kind {
	case FactorPassword:
		return newPasswordFactor(v.opts.kdf)
	case FactorSecurityKey:
		salt, err := randomBytes(saltBytes)
		if err != nil {
			return slotFactor{}, err
		}
		if spec.CredentialID == "" || spec.RPID == "" {
			return slotFactor{}, ErrInvalidFormat
		}
		return slotFactor{
			Type:         kind,
			Salt:         base64.RawStdEncoding.EncodeToString(salt),
			CredentialID: spec.CredentialID,
			RPID:         spec.RPID,
			UV:           spec.UV,
		}, nil
	case FactorKeyfile, FactorRecoveryCode:
		salt, err := randomBytes(saltBytes)
		if err != nil {
			return slotFactor{}, err
		}
		id := spec.KeyfileID
		if kind == FactorRecoveryCode || id == "" {
			if id, err = randomID(); err != nil {
				return slotFactor{}, err
			}
		}
		if !validID(id) {
			return slotFactor{}, ErrInvalidFormat
		}
		return slotFactor{
			Type:      kind,
			Salt:      base64.RawStdEncoding.EncodeToString(salt),
			KeyfileID: id,
		}, nil
	}
	return slotFactor{}, ErrInvalidFormat
}

// RemoveSlot drops one enrolled way in. It refuses to remove the last one:
// there is no such thing as a vault nobody can open, only data nobody can read,
// and that has to be an explicit choice made somewhere else.
func (v *Vault) RemoveSlot(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	found := false
	kept := make([]keySlot, 0, len(v.header.Slots))
	for _, slot := range v.header.Slots {
		if slot.ID == id {
			found = true
			continue
		}
		kept = append(kept, slot)
	}
	// Checked before the last-slot rule so an unknown id says so, rather than
	// reporting whichever rule happened to trip first.
	if !found {
		return ErrSlotNotFound
	}
	if len(kept) == 0 {
		return ErrLastSlot
	}
	return v.withKeysLocked(func(vaultKey, outerKey []byte) error {
		nextHeader := v.header
		nextHeader.Slots = kept
		return v.commitHeaderLocked(nextHeader, vaultKey, outerKey)
	})
}

// maxSlotLabel bounds a slot's display name in bytes, matching what
// validateSlots accepts when reading a header back.
const maxSlotLabel = 64

// RenameSlot changes what a way in is called. Only the label moves: it is not
// covered by slotAAD, so nothing is re-derived and no factor has to be presented
// beyond the open vault. Two security keys enrolled before names existed are
// otherwise indistinguishable, and removing the wrong one is discovered by
// finding out which device stopped working.
func (v *Vault) RenameSlot(id, label string) error {
	if len(label) == 0 || len(label) > maxSlotLabel {
		return ErrInvalidFormat
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	nextHeader := v.header
	nextHeader.Slots = append([]keySlot(nil), v.header.Slots...)
	found := false
	for i := range nextHeader.Slots {
		if nextHeader.Slots[i].ID == id {
			nextHeader.Slots[i].Label = label
			found = true
			break
		}
	}
	if !found {
		return ErrSlotNotFound
	}
	return v.withKeysLocked(func(vaultKey, outerKey []byte) error {
		return v.commitHeaderLocked(nextHeader, vaultKey, outerKey)
	})
}

// commitHeaderLocked writes a header-only change as a new verified generation,
// carrying the keyring and records across untouched.
func (v *Vault) commitHeaderLocked(nextHeader header, vaultKey, outerKey []byte) error {
	ring, err := v.loadKeyring(vaultKey, outerKey)
	if err != nil {
		return v.revokeOnIntegrityLocked(err)
	}
	files, err := v.collectRecordFiles(ring, nil, outerKey)
	if err != nil {
		return v.revokeOnIntegrityLocked(err)
	}
	return v.commitGenerationLocked(vaultKey, outerKey, nextHeader, ring, files)
}
