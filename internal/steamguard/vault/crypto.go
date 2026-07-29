package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

const (
	keyBytes  = 32
	saltBytes = 16
	// Read bounds, deliberately wider than anything this app writes: a header
	// may come from an older install or a newer one. The ceiling is what caps
	// the allocation an untrusted vault folder can force, and it has to exist
	// because argon2.IDKey aborts the process when its single memory block
	// cannot be allocated rather than returning an error.
	maxMemoryKiB   = 1024 * 1024
	minMemoryKiB   = 8 * 1024
	maxPasses      = 10
	maxLanes       = 16
	maxPlainBytes  = 16 << 20
	maxCipherBytes = 32 << 20
)

// argon2.IDKey holds the full memory cost resident for the whole derivation.
// Serialising keeps concurrent unlock attempts from multiplying that into an
// allocation failure, which argon2 answers by killing the process.
var deriveMu sync.Mutex

type envelope struct {
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func validateKDF(p KDFParams) error {
	if p.Algorithm != "argon2id" || p.MemoryKiB < minMemoryKiB || p.MemoryKiB > maxMemoryKiB ||
		p.Passes < 1 || p.Passes > maxPasses || p.Lanes < 1 || p.Lanes > maxLanes || p.KeyBytes != keyBytes {
		return ErrKDFBounds
	}
	// Argon2 requires at least 8 blocks per lane; below that the parameters are
	// not merely weak, they do not describe a valid derivation.
	if uint64(p.MemoryKiB) < 8*uint64(p.Lanes) {
		return ErrKDFBounds
	}
	return nil
}

func derive(password string, salt []byte, p KDFParams) ([]byte, error) {
	if err := validateKDF(p); err != nil {
		return nil, err
	}
	if len(salt) != saltBytes {
		return nil, errors.Join(ErrInvalidFormat, fmt.Errorf("salt length %d", len(salt)))
	}
	deriveMu.Lock()
	defer deriveMu.Unlock()
	return argon2.IDKey([]byte(password), salt, p.Passes, p.MemoryKiB, p.Lanes, p.KeyBytes), nil
}

func deriveRecovery(password string, salt []byte, p KDFParams, version int) ([]byte, error) {
	if version != RecoveryVersion {
		return nil, ErrInvalidFormat
	}
	material := make([]byte, 0, len(password)+48)
	material = append(material, "tcno-steamguard-recovery-kek"...)
	material = append(material, 0, byte(version), 0)
	material = append(material, password...)
	defer wipe(material)
	if err := validateKDF(p); err != nil {
		return nil, err
	}
	if len(salt) != saltBytes {
		return nil, errors.Join(ErrInvalidFormat, fmt.Errorf("recovery salt length %d", len(salt)))
	}
	deriveMu.Lock()
	defer deriveMu.Unlock()
	return argon2.IDKey(material, salt, p.Passes, p.MemoryKiB, p.Lanes, p.KeyBytes), nil
}

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, err
	}
	return b, nil
}

func randomID() (string, error) {
	b, err := randomBytes(16)
	if err != nil {
		return "", err
	}
	// UUID v4 shape makes identifiers easy to validate without exposing data.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(b[0:4]), binary.BigEndian.Uint16(b[4:6]),
		binary.BigEndian.Uint16(b[6:8]), binary.BigEndian.Uint16(b[8:10]), b[10:16]), nil
}

// slotAAD binds a slot's wrapped vault key to the vault, to the slot, and to
// the exact factor set the slot declares, so editing the factor list to drop a
// requirement invalidates the envelope rather than weakening it.
func slotAAD(h header, slot keySlot) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "tcno-steamguard-slot\x00%d\x00%s\x00%s\x00%s", h.Version, h.VaultID, h.KeyringID, slot.ID)
	for _, factor := range slot.Factors {
		fmt.Fprintf(&b, "\x00%s\x00%s", factor.Type, factor.Salt)
		if factor.KDF != nil {
			fmt.Fprintf(&b, "\x00%s\x00%d\x00%d\x00%d\x00%d", factor.KDF.Algorithm,
				factor.KDF.MemoryKiB, factor.KDF.Passes, factor.KDF.Lanes, factor.KDF.KeyBytes)
		}
		fmt.Fprintf(&b, "\x00%s\x00%s\x00%s\x00%t", factor.KeyfileID, factor.CredentialID, factor.RPID, factor.UV)
	}
	return []byte(b.String())
}

func slotKEKInfo(h header, slot keySlot) string {
	return fmt.Sprintf("tcno-steamguard-slot-kek\x00%d\x00%s\x00%s", h.Version, h.VaultID, slot.ID)
}

// factorMaterial reduces one factor to keying material. A typed secret pays
// the memory-hard cost here; a factor that already carries full entropy is
// only bound to its salt.
func factorMaterial(factor slotFactor, creds Credentials) ([]byte, error) {
	salt, err := decodeBounded(factor.Salt, saltBytes)
	if err != nil || len(salt) != saltBytes {
		return nil, ErrInvalidFormat
	}
	switch factor.Type {
	case FactorPassword:
		if factor.KDF == nil {
			return nil, ErrInvalidFormat
		}
		if creds.Password == "" {
			return nil, errFactorUnavailable
		}
		return derive(creds.Password, salt, *factor.KDF)
	case FactorKeyfile, FactorRecoveryCode, FactorSecurityKey:
		var secret []byte
		switch factor.Type {
		case FactorKeyfile:
			secret = creds.Keyfile
		case FactorRecoveryCode:
			secret = creds.RecoveryCode
		case FactorSecurityKey:
			secret = creds.SecurityKey
		}
		if len(secret) == 0 {
			return nil, errFactorUnavailable
		}
		mac := hmac.New(sha256.New, secret)
		mac.Write(salt)
		return mac.Sum(nil), nil
	}
	return nil, ErrInvalidFormat
}

// slotSatisfiable reports whether every factor the slot lists has material
// available, without deriving anything.
func slotSatisfiable(slot keySlot, creds Credentials) bool {
	for _, factor := range slot.Factors {
		switch factor.Type {
		case FactorPassword:
			if creds.Password == "" {
				return false
			}
		case FactorKeyfile:
			if len(creds.Keyfile) == 0 {
				return false
			}
		case FactorRecoveryCode:
			if len(creds.RecoveryCode) == 0 {
				return false
			}
		case FactorSecurityKey:
			if len(creds.SecurityKey) == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// deriveSlotKey folds every factor of a slot into one key-encryption key. The
// factors chain through HKDF in declared order, so all of them are required and
// adding one costs no extra memory-hard work.
func deriveSlotKey(h header, slot keySlot, creds Credentials) ([]byte, error) {
	if len(slot.Factors) == 0 {
		return nil, ErrInvalidFormat
	}
	// Checked before deriving anything: a slot needing a keyfile the user has
	// not supplied should cost nothing to skip, not a full memory-hard pass.
	if !slotSatisfiable(slot, creds) {
		return nil, errFactorUnavailable
	}
	prk := []byte(slot.ID)
	for _, factor := range slot.Factors {
		ikm, err := factorMaterial(factor, creds)
		if err != nil {
			wipe(prk)
			return nil, err
		}
		next, err := hkdf.Extract(sha256.New, ikm, prk)
		wipe(ikm)
		wipe(prk)
		if err != nil {
			return nil, err
		}
		prk = next
	}
	defer wipe(prk)
	return hkdf.Expand(sha256.New, prk, slotKEKInfo(h, slot), keyBytes)
}

// openVaultKey tries every slot the supplied credentials can satisfy. Slots are
// alternatives, so the first one that opens wins.
func openVaultKey(h header, creds Credentials) ([]byte, error) {
	attempted := false
	for _, slot := range h.Slots {
		kek, err := deriveSlotKey(h, slot, creds)
		if errors.Is(err, errFactorUnavailable) {
			continue
		}
		if err != nil {
			return nil, err
		}
		attempted = true
		key, err := openEnvelope(kek, slot.VaultKey, slotAAD(h, slot))
		wipe(kek)
		if err != nil {
			continue
		}
		if len(key) != keyBytes {
			wipe(key)
			return nil, ErrInvalidFormat
		}
		return key, nil
	}
	if !attempted {
		return nil, ErrFactorRequired
	}
	return nil, ErrInvalidPassword
}

// newPasswordFactor mints a fresh salt for a password factor.
func newPasswordFactor(params KDFParams) (slotFactor, error) {
	if err := validateKDF(params); err != nil {
		return slotFactor{}, err
	}
	salt, err := randomBytes(saltBytes)
	if err != nil {
		return slotFactor{}, err
	}
	kdf := params
	return slotFactor{
		Type: FactorPassword,
		Salt: base64.RawStdEncoding.EncodeToString(salt),
		KDF:  &kdf,
	}, nil
}

// reissueSlots rebuilds every slot oldCreds can open, re-deriving its password
// factor under params and newCreds. Slots that oldCreds cannot open — other
// enrolled factors — are carried across untouched, so changing a password does
// not silently revoke a security key.
// reissueSlots rebuilds every slot it can open with oldCreds so its password
// factors derive from newCreds instead. Slots it cannot open are carried across
// untouched, and the labels of those that list a password are returned: they
// still answer to the old password, and a caller retiring that password has to
// say so rather than report success.
//
// Only slots skipped for want of material are reported. A slot whose envelope
// simply did not open had every factor present and holds a different password of
// its own, which this change was never about.
func reissueSlots(h header, oldCreds, newCreds Credentials, params KDFParams, vaultKey []byte) ([]keySlot, []string, error) {
	next := make([]keySlot, 0, len(h.Slots))
	var stillOnOldPassword []string
	reissued := 0
	for _, slot := range h.Slots {
		kek, err := deriveSlotKey(h, slot, oldCreds)
		if errors.Is(err, errFactorUnavailable) {
			next = append(next, slot)
			if slotUsesPassword(slot) {
				stillOnOldPassword = append(stillOnOldPassword, slot.Label)
			}
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		existing, openErr := openEnvelope(kek, slot.VaultKey, slotAAD(h, slot))
		wipe(kek)
		if openErr != nil {
			next = append(next, slot)
			continue
		}
		wipe(existing)
		factors := make([]slotFactor, 0, len(slot.Factors))
		for _, factor := range slot.Factors {
			if factor.Type != FactorPassword {
				factors = append(factors, factor)
				continue
			}
			replacement, err := newPasswordFactor(params)
			if err != nil {
				return nil, nil, err
			}
			factors = append(factors, replacement)
		}
		rebuilt, err := buildSlot(h, slot.Label, factors, newCreds, vaultKey)
		if err != nil {
			return nil, nil, err
		}
		next = append(next, rebuilt)
		reissued++
	}
	if reissued == 0 {
		return nil, nil, ErrInvalidPassword
	}
	return next, stillOnOldPassword, nil
}

func slotUsesPassword(slot keySlot) bool {
	for _, factor := range slot.Factors {
		if factor.Type == FactorPassword {
			return true
		}
	}
	return false
}

// buildSlot seals vaultKey under the key derived from the given factors.
func buildSlot(h header, label string, factors []slotFactor, creds Credentials, vaultKey []byte) (keySlot, error) {
	id, err := randomID()
	if err != nil {
		return keySlot{}, err
	}
	slot := keySlot{ID: id, Label: label, Factors: factors}
	kek, err := deriveSlotKey(h, slot, creds)
	if err != nil {
		return keySlot{}, err
	}
	defer wipe(kek)
	slot.VaultKey, err = seal(kek, vaultKey, slotAAD(h, slot))
	if err != nil {
		return keySlot{}, err
	}
	return slot, nil
}

func aad(v int, vaultID, recordID, steamID64, purpose string) []byte {
	return []byte(fmt.Sprintf("tcno-steamguard\x00%d\x00%s\x00%s\x00%s\x00%s", v, vaultID, recordID, steamID64, purpose))
}

func outerAAD(vaultID, generationID, logicalName string) []byte {
	return []byte(fmt.Sprintf("tcno-steamguard-outer\x00%d\x00%s\x00%s\x00%s", OuterLayerVersion, vaultID, generationID, logicalName))
}

func outerProofAAD(h header) []byte {
	return []byte(fmt.Sprintf("tcno-steamguard-outer-proof\x00%d\x00%d\x00%s\x00%s", h.Version, h.OuterVersion, h.VaultID, h.KeyringID))
}

func recoveryAAD(h header, recovery recoveryWrapper) []byte {
	return []byte(fmt.Sprintf(
		"tcno-steamguard-recovery-wrapper\x00%d\x00%d\x00%d\x00%s\x00%s\x00%s\x00%d\x00%d\x00%d\x00%d\x00%s\x00outer-key",
		h.Version, recovery.Version, h.OuterVersion, h.VaultID, h.KeyringID,
		recovery.KDF.Algorithm, recovery.KDF.MemoryKiB, recovery.KDF.Passes,
		recovery.KDF.Lanes, recovery.KDF.KeyBytes, recovery.Salt))
}

func createRecoveryWrapper(password string, outerKey []byte, h header, params KDFParams) (*recoveryWrapper, error) {
	if strings.TrimSpace(password) == "" {
		return nil, ErrInvalidPassword
	}
	if h.OuterVersion != OuterLayerVersion || len(outerKey) != keyBytes {
		return nil, ErrOuterKeyRequired
	}
	if err := validateKDF(params); err != nil {
		return nil, err
	}
	salt, err := randomBytes(saltBytes)
	if err != nil {
		return nil, err
	}
	recovery := recoveryWrapper{
		Version: RecoveryVersion,
		KDF:     params,
		Salt:    base64.RawStdEncoding.EncodeToString(salt),
	}
	kek, err := deriveRecovery(password, salt, recovery.KDF, recovery.Version)
	if err != nil {
		return nil, err
	}
	defer wipe(kek)
	recovery.OuterKey, err = seal(kek, outerKey, recoveryAAD(h, recovery))
	if err != nil {
		return nil, err
	}
	return &recovery, nil
}

func openRecoveryOuterKey(password string, h header) ([]byte, error) {
	if h.Recovery == nil {
		return nil, ErrRecoveryNotConfigured
	}
	recovery := *h.Recovery
	salt, err := decodeBounded(recovery.Salt, saltBytes)
	if err != nil || len(salt) != saltBytes {
		return nil, ErrInvalidFormat
	}
	kek, err := deriveRecovery(password, salt, recovery.KDF, recovery.Version)
	if err != nil {
		return nil, err
	}
	defer wipe(kek)
	outerKey, err := openEnvelope(kek, recovery.OuterKey, recoveryAAD(h, recovery))
	if err != nil {
		return nil, ErrInvalidRecoveryPassword
	}
	if len(outerKey) != keyBytes {
		wipe(outerKey)
		return nil, ErrInvalidFormat
	}
	if err := validateOuterProof(outerKey, h); err != nil {
		wipe(outerKey)
		return nil, errors.Join(ErrInvalidRecoveryPassword, err)
	}
	return outerKey, nil
}

func sealOuterProof(key []byte, h header) (envelope, error) {
	if len(key) != keyBytes || h.OuterVersion != OuterLayerVersion {
		return envelope{}, ErrOuterKeyRequired
	}
	return seal(key, []byte("tcno-steamguard-outer-enabled-v1"), outerProofAAD(h))
}

func validateOuterProof(key []byte, h header) error {
	if h.OuterVersion == 0 {
		if h.OuterProof.Nonce != "" || h.OuterProof.Ciphertext != "" {
			return ErrInvalidFormat
		}
		return nil
	}
	if len(key) != keyBytes {
		return ErrOuterKeyRequired
	}
	plain, err := openEnvelope(key, h.OuterProof, outerProofAAD(h))
	if err != nil {
		return errors.Join(ErrInvalidOuterKey, err)
	}
	defer wipe(plain)
	if string(plain) != "tcno-steamguard-outer-enabled-v1" {
		return ErrInvalidOuterKey
	}
	return nil
}

func sealOuterFile(key, inner []byte, version int, additional []byte) ([]byte, error) {
	if version == 0 {
		return inner, nil
	}
	if version != OuterLayerVersion || len(key) != keyBytes {
		return nil, ErrOuterKeyRequired
	}
	env, err := seal(key, inner, additional)
	if err != nil {
		return nil, err
	}
	return marshalJSON(outerFile{Version: OuterLayerVersion, Ciphertext: env})
}

func openOuterFile(key, raw []byte, version int, additional []byte) ([]byte, error) {
	if version == 0 {
		return append([]byte(nil), raw...), nil
	}
	if version != OuterLayerVersion || len(key) != keyBytes {
		return nil, ErrOuterKeyRequired
	}
	var wrapped outerFile
	if err := unmarshalStrict(raw, &wrapped); err != nil || wrapped.Version != OuterLayerVersion {
		return nil, ErrInvalidFormat
	}
	plain, err := openEnvelope(key, wrapped.Ciphertext, additional)
	if err != nil {
		return nil, errors.Join(ErrInvalidOuterKey, err)
	}
	return plain, nil
}

func seal(key, plain, additional []byte) (envelope, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return envelope{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return envelope{}, err
	}
	nonce, err := randomBytes(aead.NonceSize())
	if err != nil {
		return envelope{}, err
	}
	ciphertext := aead.Seal(nil, nonce, plain, additional)
	return envelope{Nonce: base64.RawStdEncoding.EncodeToString(nonce), Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext)}, nil
}

func openEnvelope(key []byte, env envelope, additional []byte) ([]byte, error) {
	nonce, err := decodeBounded(env.Nonce, 64)
	if err != nil {
		return nil, err
	}
	ciphertext, err := decodeBounded(env.Ciphertext, maxCipherBytes)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != aead.NonceSize() || len(ciphertext) < aead.Overhead() {
		return nil, ErrInvalidFormat
	}
	return aead.Open(nil, nonce, ciphertext, additional)
}

// maxCredentialID bounds a stored security-key handle. Authenticators choose
// their own length; 1024 bytes is far above anything in the wild.
const maxCredentialID = 1024

// decodeCredentialID validates a stored security-key handle.
//
// Handles are base64url, which is what WebAuthn uses and what the platform
// driver writes. Validating them with the standard alphabet only worked for
// handles that happened to contain none of the two characters the alphabets
// disagree on, so roughly a quarter of keys enrolled and the rest were rejected
// as a corrupt vault.
func decodeCredentialID(value string) ([]byte, error) {
	if len(value) > base64.RawURLEncoding.EncodedLen(maxCredentialID)+4 {
		return nil, ErrInvalidFormat
	}
	b, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(b) > maxCredentialID {
		return nil, ErrInvalidFormat
	}
	return b, nil
}

func decodeBounded(value string, max int) ([]byte, error) {
	if len(value) > base64.RawStdEncoding.EncodedLen(max)+4 {
		return nil, ErrInvalidFormat
	}
	b, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil || len(b) > max {
		return nil, ErrInvalidFormat
	}
	return b, nil
}

func wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}
