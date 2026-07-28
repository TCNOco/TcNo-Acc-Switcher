package confirmationapi

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"strings"
)

const maxConfirmationTagBytes = 32

// GenerateConfirmationHash returns Steam's base64 HMAC-SHA1 confirmation
// signature. URL escaping is deliberately left to url.Values.
func GenerateConfirmationHash(identitySecret string, unixTime int64, tag string) (string, error) {
	if unixTime <= 0 {
		return "", &Error{Kind: FailureInvalid}
	}
	secret, err := base64.StdEncoding.Strict().DecodeString(strings.TrimSpace(identitySecret))
	if err != nil || len(secret) != sha1.Size {
		wipe(secret)
		return "", &Error{Kind: FailureInvalid}
	}
	defer wipe(secret)
	tagBytes := []byte(tag)
	if len(tagBytes) > maxConfirmationTagBytes {
		tagBytes = tagBytes[:maxConfirmationTagBytes]
	}
	payload := make([]byte, 8+len(tagBytes))
	defer wipe(payload)
	binary.BigEndian.PutUint64(payload[:8], uint64(unixTime))
	copy(payload[8:], tagBytes)
	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(payload)
	digest := mac.Sum(nil)
	defer wipe(digest)
	return base64.StdEncoding.EncodeToString(digest), nil
}

func wipe(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
