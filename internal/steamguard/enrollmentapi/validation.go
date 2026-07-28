package enrollmentapi

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidRequest  = errors.New("invalid Steam authenticator enrollment request")
	ErrInvalidResponse = errors.New("invalid Steam authenticator enrollment response")
	ErrEntropy         = errors.New("Steam authenticator enrollment random source failed")
	ErrSteamRejected   = errors.New("Steam rejected the authenticator enrollment request")
)

const (
	maxAccessTokenBytes = 8192
	requestIDBytes      = 16
	maxResponseBytes    = 32 << 10
	requestTimeout      = 20 * time.Second
	maxClockDifference  = 24 * time.Hour
	minUnixTime         = uint64(1230768000) // 2009-01-01
	maxUnixTime         = uint64(4102444800) // 2100-01-01
	maxAccountNameBytes = 64
	maxTokenGIDBytes    = 64
	maxURIBytes         = 1024
	maxPhoneHintBytes   = 32
)

type clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func validSteamID(value uint64) bool {
	return value >= 76561197960265728 && value <= math.MaxInt64
}

func validToken(value []byte) bool {
	if len(value) == 0 || len(value) > maxAccessTokenBytes {
		return false
	}
	for _, b := range value {
		if b < 0x21 || b > 0x7e {
			return false
		}
	}
	return true
}

func validTimestamp(value uint64, now time.Time) bool {
	if value < minUnixTime || value > maxUnixTime || now.IsZero() {
		return false
	}
	delta := time.Unix(int64(value), 0).Sub(now)
	return delta >= -maxClockDifference && delta <= maxClockDifference
}

func validConfirmationCode(value []byte) bool {
	if len(value) < 4 || len(value) > 10 {
		return false
	}
	for _, b := range value {
		if !((b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')) {
			return false
		}
	}
	return true
}

func validText(value []byte, max int, allowEmpty bool) bool {
	if len(value) > max || (!allowEmpty && len(value) == 0) || !utf8.Valid(value) {
		return false
	}
	for _, b := range value {
		if b < 0x20 || b == 0x7f {
			return false
		}
	}
	return true
}

func validPending(p *PendingEnrollment, requestID []byte, now time.Time) bool {
	return p != nil && len(p.RequestID) == requestIDBytes && bytes.Equal(p.RequestID, requestID) &&
		validSteamID(p.SteamID) && validToken(p.AccessToken) && validDeviceID(p.DeviceID) &&
		len(p.SharedSecret) == 20 && len(p.IdentitySecret) == 20 && len(p.Secret1) == 20 &&
		validRevocationCode(p.RevocationCode) && bytes.HasPrefix(p.URI, []byte("otpauth://totp/Steam:")) &&
		p.SerialNumber != 0 && validTimestamp(p.ServerTime, now) &&
		(p.Confirmation == ConfirmationSMS || p.Confirmation == ConfirmationEmail || p.Confirmation == ConfirmationUnknown)
}

func validDeviceID(value string) bool {
	if len(value) != len("android:00000000-0000-0000-0000-000000000000") || !strings.HasPrefix(value, "android:") {
		return false
	}
	for i := len("android:"); i < len(value); i++ {
		if i == 16 || i == 21 || i == 26 || i == 31 {
			if value[i] != '-' {
				return false
			}
			continue
		}
		b := value[i]
		if !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f')) {
			return false
		}
	}
	return true
}
