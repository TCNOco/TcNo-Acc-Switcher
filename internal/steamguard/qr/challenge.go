// Package qr parses and signs Steam mobile-login QR challenges.
package qr

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

const (
	challengeHost     = "s.team"
	supportedVersion  = uint16(1)
	sharedSecretBytes = 20
)

var (
	ErrInvalidChallenge    = errors.New("invalid Steam QR challenge")
	ErrUnsupportedVersion  = errors.New("unsupported Steam QR challenge version")
	ErrInvalidSharedSecret = errors.New("invalid Steam Guard shared secret")
	ErrInvalidSteamID      = errors.New("invalid Steam account ID")
)

type Challenge struct {
	Version  uint16
	ClientID uint64
}

func ParseChallenge(raw string) (Challenge, error) {
	if raw == "" || raw != strings.TrimSpace(raw) || len(raw) > 256 {
		return Challenge{}, ErrInvalidChallenge
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Opaque != "" || parsed.User != nil || parsed.Scheme != "https" || parsed.Host != challengeHost {
		return Challenge{}, ErrInvalidChallenge
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || parsed.EscapedPath() != parsed.Path {
		return Challenge{}, ErrInvalidChallenge
	}
	parts := strings.Split(parsed.Path, "/")
	if len(parts) != 4 || parts[0] != "" || parts[1] != "q" {
		return Challenge{}, ErrInvalidChallenge
	}
	version64, err := strconv.ParseUint(parts[2], 10, 16)
	if err != nil || strconv.FormatUint(version64, 10) != parts[2] {
		return Challenge{}, ErrInvalidChallenge
	}
	version := uint16(version64)
	if version != supportedVersion {
		return Challenge{}, ErrUnsupportedVersion
	}
	clientID, err := strconv.ParseUint(parts[3], 10, 64)
	if err != nil || clientID == 0 || strconv.FormatUint(clientID, 10) != parts[3] {
		return Challenge{}, ErrInvalidChallenge
	}
	return Challenge{Version: version, ClientID: clientID}, nil
}

// SignChallenge computes the signature required by
// UpdateAuthSessionWithMobileConfirmation. The returned bytes are suitable for
// the protobuf request and must not be logged or retained.
func SignChallenge(challenge Challenge, steamID uint64, sharedSecret []byte) ([]byte, error) {
	if challenge.Version != supportedVersion || challenge.ClientID == 0 {
		return nil, ErrInvalidChallenge
	}
	if steamID == 0 {
		return nil, ErrInvalidSteamID
	}
	if len(sharedSecret) != sharedSecretBytes {
		return nil, ErrInvalidSharedSecret
	}
	payload := make([]byte, 18)
	binary.LittleEndian.PutUint16(payload[0:2], challenge.Version)
	binary.LittleEndian.PutUint64(payload[2:10], challenge.ClientID)
	binary.LittleEndian.PutUint64(payload[10:18], steamID)
	mac := hmac.New(sha256.New, sharedSecret)
	_, _ = mac.Write(payload)
	signature := mac.Sum(nil)
	clear(payload)
	return signature, nil
}

func clear(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
