package mafile

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"errors"

	"golang.org/x/crypto/pbkdf2"
)

// SDA's encryption parameters, matched exactly so files interoperate in both
// directions. Changing any of these breaks compatibility with SDA.
const (
	legacyPBKDF2Iterations = 50000
	legacyKeyBytes         = 32
	legacySaltBytes        = 8
)

// LegacyEncryptedExport is SDA's encrypted representation of one account. SDA
// stores the ciphertext as the whole file body and keeps the key material in the
// adjacent manifest.json, so both halves are needed to import it again.
type LegacyEncryptedExport struct {
	// Body is the file content: base64 ciphertext, nothing else.
	Body []byte
	// Salt and IV are base64, as they appear in the manifest entry.
	Salt string
	IV   string
}

// ErrExportPasswordEmpty reports an encrypted export with no password, which
// would produce a file that is encrypted in name only.
var ErrExportPasswordEmpty = errors.New("mafile: export password is empty")

// ExportLegacyEncrypted produces the SDA-compatible encrypted form of an account:
// AES-256-CBC under a PBKDF2-HMAC-SHA1 key, the exact inverse of
// ImportLegacyEncrypted, so a file written here reads back there and in SDA.
//
// Salt and IV are fresh for every export; SDA generates them per save too, and
// reusing them across exports of one account would leak that they match.
func ExportLegacyEncrypted(account Account, options ExportOptions, password string) (LegacyEncryptedExport, error) {
	if password == "" {
		return LegacyEncryptedExport{}, ErrExportPasswordEmpty
	}
	plaintext, err := ExportPlaintext(account, options)
	if err != nil {
		return LegacyEncryptedExport{}, err
	}
	defer wipeBytes(plaintext)

	salt := make([]byte, legacySaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return LegacyEncryptedExport{}, err
	}
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return LegacyEncryptedExport{}, err
	}

	key := pbkdf2.Key([]byte(password), salt, legacyPBKDF2Iterations, legacyKeyBytes, sha1.New)
	defer wipeBytes(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return LegacyEncryptedExport{}, err
	}
	padded := padPKCS7(plaintext, aes.BlockSize)
	defer wipeBytes(padded)
	sealed := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(sealed, padded)

	return LegacyEncryptedExport{
		Body: []byte(base64.StdEncoding.EncodeToString(sealed)),
		Salt: base64.StdEncoding.EncodeToString(salt),
		IV:   base64.StdEncoding.EncodeToString(iv),
	}, nil
}

// padPKCS7 appends PKCS#7 padding, always adding at least one byte so the length
// is unambiguous when unpadding.
func padPKCS7(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padded := make([]byte, len(data)+padding)
	copy(padded, data)
	for index := len(data); index < len(padded); index++ {
		padded[index] = byte(padding)
	}
	return padded
}
