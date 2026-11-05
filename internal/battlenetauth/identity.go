package battlenetauth

import (
	"errors"
	"net/mail"
	"strings"
	"unicode"
)

var ErrAccountIdentity = errors.New("Battle.net did not return the account name; no authenticator was added")

type EnrollmentIdentity struct {
	BattleTag string `json:"battleTag"`
	Email     string `json:"email,omitempty"`
}

func (EnrollmentIdentity) String() string   { return "Battle.net account identity (redacted)" }
func (EnrollmentIdentity) GoString() string { return "Battle.net account identity (redacted)" }

func NormalizeAccountEmail(raw string) string {
	email := strings.TrimSpace(raw)
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 320 || strings.ContainsAny(email, "*\r\n") {
		return ""
	}
	return email
}

func (i EnrollmentIdentity) Validate() error {
	name, digits, ok := strings.Cut(i.BattleTag, "#")
	if !ok || len(name) == 0 || len(i.BattleTag) > 200 || len(digits) < 3 || len(digits) > 12 {
		return ErrAccountIdentity
	}
	for _, c := range name {
		if unicode.IsControl(c) || unicode.IsSpace(c) || unicode.Is(unicode.Cf, c) {
			return ErrAccountIdentity
		}
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return ErrAccountIdentity
		}
	}
	if i.Email != "" && NormalizeAccountEmail(i.Email) != i.Email {
		return ErrAccountIdentity
	}
	return nil
}
