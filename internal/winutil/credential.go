package winutil

import (
	"errors"
	"strings"
)

var ErrCredentialNotFound = errors.New("Windows credential not found")

const legacyGenericTargetPrefix = "LegacyGeneric:target="

// Credential is a generic Windows Credential Manager entry. Blob contains
// opaque credential bytes and must never be logged.
type Credential struct {
	TargetName  string
	UserName    string
	TargetAlias string
	Comment     string
	Persist     uint32
	Blob        []byte
}

// CredentialTargetCandidates handles the target form exposed by some legacy
// Credential Manager entries while keeping reads/deletes scoped to one target.
func CredentialTargetCandidates(target string) []string {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	if strings.HasPrefix(strings.ToLower(target), strings.ToLower(legacyGenericTargetPrefix)) {
		raw := strings.TrimSpace(target[len(legacyGenericTargetPrefix):])
		if raw == "" || strings.EqualFold(raw, target) {
			return []string{target}
		}
		return []string{target, raw}
	}
	return []string{target, legacyGenericTargetPrefix + target}
}

// CredentialTargetMatches confirms two target names refer to the same generic
// credential without allowing a snapshot to redirect a write to another target.
func CredentialTargetMatches(expected, actual string) bool {
	normalize := func(s string) string {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(strings.ToLower(s), strings.ToLower(legacyGenericTargetPrefix)) {
			s = strings.TrimSpace(s[len(legacyGenericTargetPrefix):])
		}
		return strings.ToLower(s)
	}
	return normalize(expected) != "" && normalize(expected) == normalize(actual)
}
