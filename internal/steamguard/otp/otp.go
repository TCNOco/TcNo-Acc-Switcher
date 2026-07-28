// Package otp generates Steam Guard codes from shared secrets.
package otp

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"time"
)

const (
	codeLength      = 5
	intervalSeconds = int64(30)
)

const steamAlphabet = "23456789BCDFGHJKMNPQRTVWXY"

var (
	// ErrInvalidSharedSecret reports malformed or unsupported shared-secret data.
	ErrInvalidSharedSecret = errors.New("invalid Steam Guard shared secret")
	// ErrInvalidTime reports a time before the Unix epoch.
	ErrInvalidTime = errors.New("invalid Steam Guard time")
)

// Code is a Steam Guard code and the interval in which it is valid.
type Code struct {
	Value         string
	IntervalStart time.Time
	ExpiresAt     time.Time
}

// Generate creates a five-character Steam Guard code for now.
func Generate(sharedSecret string, now time.Time) (Code, error) {
	if now.Unix() < 0 {
		return Code{}, ErrInvalidTime
	}
	secret, err := base64.StdEncoding.Strict().DecodeString(sharedSecret)
	if err != nil || len(secret) != sha1.Size {
		return Code{}, ErrInvalidSharedSecret
	}

	counter := uint64(now.Unix() / intervalSeconds)
	var message [8]byte
	binary.BigEndian.PutUint64(message[:], counter)

	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(message[:])
	digest := mac.Sum(nil)
	offset := int(digest[len(digest)-1] & 0x0f)
	value := binary.BigEndian.Uint32(digest[offset:offset+4]) & 0x7fffffff

	chars := make([]byte, codeLength)
	for i := range chars {
		chars[i] = steamAlphabet[value%uint32(len(steamAlphabet))]
		value /= uint32(len(steamAlphabet))
	}

	startUnix := (now.Unix() / intervalSeconds) * intervalSeconds
	start := time.Unix(startUnix, 0).UTC()
	return Code{
		Value:         string(chars),
		IntervalStart: start,
		ExpiresAt:     start.Add(time.Duration(intervalSeconds) * time.Second),
	}, nil
}
