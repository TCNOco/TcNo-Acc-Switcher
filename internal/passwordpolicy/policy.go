package passwordpolicy

import (
	"errors"
	"unicode/utf8"
)

var (
	ErrInvalidUTF8 = errors.New("password must be valid UTF-8")
	ErrEmpty       = errors.New("password must not be empty")
)

// ValidateNew preserves the exact user-selected value for the KDF.
func ValidateNew(password string) error {
	if !utf8.ValidString(password) {
		return ErrInvalidUTF8
	}
	if password == "" {
		return ErrEmpty
	}
	return nil
}
