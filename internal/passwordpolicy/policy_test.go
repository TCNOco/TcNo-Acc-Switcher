package passwordpolicy

import (
	"errors"
	"testing"
)

func TestValidateNew(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     error
	}{
		{name: "exactly the minimum", password: "abcde"},
		{name: "common value", password: "password"},
		{name: "passphrase", password: "correct horse battery staple"},
		{name: "no composition rules", password: "aaaaa"},
		{name: "long value", password: string(make([]byte, 10_000))},
		{name: "one character", password: "x", want: ErrTooShort},
		{name: "one below the minimum", password: "abcd", want: ErrTooShort},
		{name: "whitespace", password: " ", want: ErrTooShort},
		{name: "empty", password: "", want: ErrEmpty},
		{name: "invalid UTF-8", password: string([]byte{0xff, 0xfe}), want: ErrInvalidUTF8},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateNew(test.password)
			if test.want == nil && err != nil {
				t.Fatalf("ValidateNew() error = %v", err)
			}
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("ValidateNew() error = %v, want %v", err, test.want)
			}
		})
	}
}

// Length is counted in runes, not bytes: four multi-byte characters are still
// short, and five are enough however many bytes they occupy.
func TestValidateNewCountsRunes(t *testing.T) {
	if err := ValidateNew("猫猫猫猫"); !errors.Is(err, ErrTooShort) {
		t.Fatalf("ValidateNew(4 runes) error = %v, want %v", err, ErrTooShort)
	}
	if err := ValidateNew("猫猫猫猫猫"); err != nil {
		t.Fatalf("ValidateNew(5 runes) error = %v", err)
	}
}

// A password already in use was chosen under whatever policy applied then.
// Raising the minimum must never reject it, or the user is locked out.
func TestValidateExistingIgnoresLength(t *testing.T) {
	for _, password := range []string{"x", "abcd", " "} {
		if err := ValidateExisting(password); err != nil {
			t.Fatalf("ValidateExisting(%q) error = %v", password, err)
		}
	}
	if err := ValidateExisting(""); !errors.Is(err, ErrEmpty) {
		t.Fatalf("ValidateExisting(empty) error = %v, want %v", err, ErrEmpty)
	}
	if err := ValidateExisting(string([]byte{0xff})); !errors.Is(err, ErrInvalidUTF8) {
		t.Fatalf("ValidateExisting(invalid UTF-8) error = %v, want %v", err, ErrInvalidUTF8)
	}
}
