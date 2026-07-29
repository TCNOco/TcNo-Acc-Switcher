package passwordpolicy

import (
	"errors"
	"unicode/utf8"
)

// MinLength is counted in runes, so a short passphrase of multi-byte
// characters is measured the way the user typed it. The frontend mirror in
// frontend/src/lib/passwordPolicy.ts must count code points to agree.
const MinLength = 5

var (
	ErrInvalidUTF8 = errors.New("password must be valid UTF-8")
	ErrEmpty       = errors.New("password must not be empty")
	ErrTooShort    = errors.New("password must be at least 5 characters")
)

// ValidateNew rejects a password the user is choosing. Length is the only
// requirement: composition rules push people towards shorter, patterned
// passwords, so none are applied.
func ValidateNew(password string) error {
	if err := ValidateExisting(password); err != nil {
		return err
	}
	if utf8.RuneCountInString(password) < MinLength {
		return ErrTooShort
	}
	return nil
}

// ValidateExisting rejects a password the user is supplying to prove identity
// rather than choosing. Callers on that path must never use ValidateNew:
// tightening the policy would retroactively lock out anyone whose stored
// password predates the current rules.
func ValidateExisting(password string) error {
	if !utf8.ValidString(password) {
		return ErrInvalidUTF8
	}
	if password == "" {
		return ErrEmpty
	}
	return nil
}
