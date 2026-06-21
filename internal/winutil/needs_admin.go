package winutil

import (
	"errors"
	"strings"
)

// NeedsAdminPrefix is embedded in errors returned when the switcher must restart elevated.
const NeedsAdminPrefix = "NEEDS_ADMIN:"

// ErrNeedsAdmin is returned when an operation requires the app to run elevated.
var ErrNeedsAdmin = errors.New("NEEDS_ADMIN")

// NeedsAdminError carries the first blocking process/service name for UI.
type NeedsAdminError struct {
	Blocker string
}

func (e *NeedsAdminError) Error() string {
	b := strings.TrimSpace(e.Blocker)
	if b == "" {
		return NeedsAdminPrefix
	}
	return NeedsAdminPrefix + b
}

func (e *NeedsAdminError) Unwrap() error { return ErrNeedsAdmin }

// ErrIfCannotKill returns NewNeedsAdminError when [CanKillProcesses] reports the current token cannot kill all targets.
func ErrIfCannotKill(names []string, method ClosingMethod) error {
	blocker, ok := CanKillProcesses(names, method)
	if ok {
		return nil
	}
	return NewNeedsAdminError(blocker)
}

// NewNeedsAdminError builds an error detectable on the frontend via NeedsAdminPrefix / ErrNeedsAdmin.
func NewNeedsAdminError(blocker string) error {
	return &NeedsAdminError{Blocker: strings.TrimSpace(blocker)}
}

// IsNeedsAdmin reports whether err is a NeedsAdminError or wraps ErrNeedsAdmin.
func IsNeedsAdmin(err error) bool {
	if err == nil {
		return false
	}
	var na *NeedsAdminError
	if errors.As(err, &na) {
		return true
	}
	if errors.Is(err, ErrNeedsAdmin) {
		return true
	}
	return strings.HasPrefix(err.Error(), NeedsAdminPrefix)
}

// BlockerFromNeedsAdmin returns the blocker substring after NeedsAdminPrefix, or empty.
func BlockerFromNeedsAdmin(err error) string {
	var na *NeedsAdminError
	if errors.As(err, &na) {
		return na.Blocker
	}
	s := err.Error()
	if strings.HasPrefix(s, NeedsAdminPrefix) {
		return strings.TrimSpace(s[len(NeedsAdminPrefix):])
	}
	return ""
}
