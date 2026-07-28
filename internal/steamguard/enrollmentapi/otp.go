package enrollmentapi

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
)

const steamAlphabet = "23456789BCDFGHJKMNPQRTVWXY"

func authenticatorCode(secret []byte, unixTime uint64) ([]byte, error) {
	if len(secret) != 20 || !validTimestampRange(unixTime) {
		return nil, ErrInvalidRequest
	}
	counter := make([]byte, 8)
	binary.BigEndian.PutUint64(counter, unixTime/30)
	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(counter)
	digest := mac.Sum(nil)
	wipe(counter)
	defer wipe(digest)
	offset := digest[len(digest)-1] & 0x0f
	value := binary.BigEndian.Uint32(digest[offset:offset+4]) & 0x7fffffff
	code := make([]byte, 5)
	for i := range code {
		code[i] = steamAlphabet[value%uint32(len(steamAlphabet))]
		value /= uint32(len(steamAlphabet))
	}
	return code, nil
}
