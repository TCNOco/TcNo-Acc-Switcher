//go:build !windows

package winutil

import "strings"

// RegistryKeyPathForAllValuesSpecifier mirrors the Windows implementation (string form only).
func RegistryKeyPathForAllValuesSpecifier(encoded string) (keyPath string, ok bool) {
	s := strings.TrimSpace(encoded)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "REG:"), "reg:")
	idx := strings.LastIndex(s, ":")
	if idx <= 0 || idx >= len(s)-1 {
		return "", false
	}
	if s[idx+1:] != "*" {
		return "", false
	}
	return s[:idx], true
}

// RegistryReadAllValuesInKey is only supported on Windows.
func RegistryReadAllValuesInKey(keyPath string) (map[string]struct {
	Val any
	Typ uint32
}, error) {
	return nil, ErrUnsupported
}

// RegistryClearLoginKey is only supported on Windows.
func RegistryClearLoginKey(encoded string, deleteValues bool) error {
	return ErrUnsupported
}

// RegistryReadValuesMatchingNameGlob is only supported on Windows.
func RegistryReadValuesMatchingNameGlob(keyPath, valueNameGlob string) (map[string]struct {
	Val any
	Typ uint32
}, error) {
	return nil, ErrUnsupported
}

// RegistryReadFirstValueMatchingNameGlob is only supported on Windows.
func RegistryReadFirstValueMatchingNameGlob(keyPath, valueNameGlob string) (valueName string, val any, typ uint32, err error) {
	return "", nil, 0, ErrUnsupported
}

// RegistryClearValuesMatchingNameGlob is only supported on Windows.
func RegistryClearValuesMatchingNameGlob(keyPath, valueNameGlob string, deleteValues bool) error {
	return ErrUnsupported
}
