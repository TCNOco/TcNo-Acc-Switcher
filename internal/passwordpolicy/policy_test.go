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
		{name: "one character", password: "x"},
		{name: "common value", password: "password"},
		{name: "unicode characters", password: "猫"},
		{name: "whitespace", password: " "},
		{name: "long value", password: string(make([]byte, 10_000))},
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
