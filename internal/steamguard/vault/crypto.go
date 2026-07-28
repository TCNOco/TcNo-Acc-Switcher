package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	keyBytes       = 32
	saltBytes      = 16
	maxMemoryKiB   = 256 * 1024
	minMemoryKiB   = 8 * 1024
	maxPasses      = 10
	maxLanes       = 16
	maxPlainBytes  = 16 << 20
	maxCipherBytes = 32 << 20
)

type envelope struct {
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func validateKDF(p KDFParams) error {
	if p.Algorithm != "argon2id" || p.MemoryKiB < minMemoryKiB || p.MemoryKiB > maxMemoryKiB ||
		p.Passes < 1 || p.Passes > maxPasses || p.Lanes < 1 || p.Lanes > maxLanes || p.KeyBytes != keyBytes {
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
