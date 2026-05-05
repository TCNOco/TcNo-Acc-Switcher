//go:build windows

package winutil

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// RegistryRead reads a registry value. encoded is like HKCU\Software\Foo:ValueName (optional REG: prefix;
// optional trailing :REG_DWORD / :DWORD / :REG_SZ etc. after the value name).
// Returns value and registry value type (REG_*).
func RegistryRead(encoded string) (any, uint32, error) {
	k, sub, val, _, err := parseRegistryPath(encoded)
	if err != nil {
		return nil, 0, err
	}
	key, err := registry.OpenKey(k, sub, registry.QUERY_VALUE)
	if err != nil {
		return nil, 0, err
	}
	defer key.Close()

	_, typ, err := key.GetValue(val, nil)
	if err != nil {
		return nil, 0, err
	}
	switch typ {
	case registry.SZ, registry.EXPAND_SZ:
		s, _, err := key.GetStringValue(val)
		return s, typ, err
	case registry.DWORD, registry.QWORD:
		n, _, err := key.GetIntegerValue(val)
		if typ == registry.DWORD {
			return uint32(n), typ, err
		}
		return n, typ, err
	default:
		b, _, err := key.GetBinaryValue(val)
		return b, typ, err
	}
}

// RegistryWrite writes a registry value. value may be string, uint32, []byte, or int.
// Strings may be "(hex) aa bb cc" for binary.
// Type is inferred from value unless encoded ends with an explicit :REG_* / :DWORD / :REG_SZ suffix.
func RegistryWrite(encoded string, value any) error {
	return RegistryWriteHint(encoded, value, 0)
}

// RegistryWriteHint writes a registry value like RegistryWrite, but uses savedType when the encoded path
// does not specify a trailing explicit type. If both are absent, type is inferred from value.
// savedType should be a registry value type constant (e.g. registry.DWORD); 0 means unused.
func RegistryWriteHint(encoded string, value any, savedType uint32) error {
	k, sub, val, pathTyp, err := parseRegistryPath(encoded)
	if err != nil {
		return err
	}
	key, _, err := registry.CreateKey(k, sub, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	eff := pathTyp
	if eff == 0 {
		eff = savedType
	}
	if eff == 0 {
		return registryWriteInferred(key, val, value)
	}
	return registryWriteTyped(key, val, eff, value)
}

func registryWriteInferred(key registry.Key, val string, value any) error {
	switch v := value.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return key.DeleteValue(val)
		}
		if strings.HasPrefix(strings.ToLower(s), "(hex)") {
			raw, err := parseHexString(s)
			if err != nil {
				return err
			}
			return key.SetBinaryValue(val, raw)
		}
		return key.SetStringValue(val, s)
	case uint32:
		return key.SetDWordValue(val, v)
	case uint64:
		return key.SetQWordValue(val, v)
	case int:
		return key.SetDWordValue(val, uint32(v))
	case int64:
		return key.SetQWordValue(val, uint64(v))
	case []byte:
		return key.SetBinaryValue(val, v)
	default:
		return fmt.Errorf("unsupported registry value type %T", value)
	}
}

func registryWriteTyped(key registry.Key, val string, typ uint32, value any) error {
	switch typ {
	case registry.DWORD:
		n, err := coerceToUint32(value)
		if err != nil {
			return fmt.Errorf("registry DWORD %s: %w", val, err)
		}
		return key.SetDWordValue(val, n)
	case registry.QWORD:
		n, err := coerceToUint64(value)
		if err != nil {
			return fmt.Errorf("registry QWORD %s: %w", val, err)
		}
		return key.SetQWordValue(val, n)
	case registry.SZ:
		s, err := coerceToString(value)
		if err != nil {
			return fmt.Errorf("registry SZ %s: %w", val, err)
		}
		if s == "" {
			return key.DeleteValue(val)
		}
		return key.SetStringValue(val, s)
	case registry.EXPAND_SZ:
		s, err := coerceToString(value)
		if err != nil {
			return fmt.Errorf("registry EXPAND_SZ %s: %w", val, err)
		}
		if s == "" {
			return key.DeleteValue(val)
		}
		return key.SetExpandStringValue(val, s)
	case registry.BINARY:
		b, err := coerceToBinary(value)
		if err != nil {
			return fmt.Errorf("registry BINARY %s: %w", val, err)
		}
		if len(b) == 0 {
			return key.DeleteValue(val)
		}
		return key.SetBinaryValue(val, b)
	case registry.MULTI_SZ:
		s, err := coerceToString(value)
		if err != nil {
			return fmt.Errorf("registry MULTI_SZ %s: %w", val, err)
		}
		if s == "" {
			return key.DeleteValue(val)
		}
		parts := strings.Split(s, "\x00")
		return key.SetStringsValue(val, parts)
	default:
		return registryWriteInferred(key, val, value)
	}
}

func coerceToString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	case uint32, uint64, int, int64:
		return fmt.Sprint(v), nil
	default:
		return "", fmt.Errorf("expected string-like value, got %T", value)
	}
}

func coerceToUint32(value any) (uint32, error) {
	switch v := value.(type) {
	case uint32:
		return v, nil
	case int:
		return uint32(v), nil
	case uint64:
		return uint32(v), nil
	case int64:
		return uint32(v), nil
	case string:
		s := strings.TrimSpace(v)
		if strings.HasPrefix(strings.ToLower(s), "(hex)") {
			b, err := parseHexString(s)
			if err != nil {
				return 0, err
			}
			if len(b) < 4 {
				return 0, fmt.Errorf("DWORD hex: need at least 4 bytes, got %d", len(b))
			}
			return binary.LittleEndian.Uint32(b[:4]), nil
		}
		n, err := strconv.ParseUint(s, 10, 32)
		return uint32(n), err
	default:
		return 0, fmt.Errorf("expected numeric or string DWORD, got %T", value)
	}
}

func coerceToUint64(value any) (uint64, error) {
	switch v := value.(type) {
	case uint64:
		return v, nil
	case uint32:
		return uint64(v), nil
	case int:
		return uint64(v), nil
	case int64:
		return uint64(v), nil
	case string:
		s := strings.TrimSpace(v)
		n, err := strconv.ParseUint(s, 10, 64)
		return n, err
	default:
		return 0, fmt.Errorf("expected numeric or string QWORD, got %T", value)
	}
}

func coerceToBinary(value any) ([]byte, error) {
	switch v := value.(type) {
	case []byte:
		return v, nil
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil, nil
		}
		if strings.HasPrefix(strings.ToLower(s), "(hex)") {
			return parseHexString(s)
		}
		return []byte(s), nil
	default:
		return nil, fmt.Errorf("expected []byte or hex string, got %T", value)
	}
}

// RegistryDelete removes a value or the whole key if value name is empty (not used here).
func RegistryDelete(encoded string) error {
	k, sub, val, _, err := parseRegistryPath(encoded)
	if err != nil {
		return err
	}
	key, err := registry.OpenKey(k, sub, registry.SET_VALUE|registry.WRITE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.DeleteValue(val)
}

// ParseHexString decodes "(hex) aa bb" or "aabbcc" binary strings from reg.json.
func ParseHexString(s string) ([]byte, error) {
	return parseHexString(s)
}

func parseHexString(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToLower(s), "(hex)") {
		s = strings.TrimSpace(s[5:])
	}
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return nil, nil
	}
	return hex.DecodeString(s)
}

// registryTypeFromName maps a trailing path suffix to a registry type constant.
func registryTypeFromName(s string) (uint32, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "REG_SZ", "SZ":
		return registry.SZ, true
	case "REG_EXPAND_SZ", "EXPAND_SZ":
		return registry.EXPAND_SZ, true
	case "REG_MULTI_SZ", "MULTI_SZ":
		return registry.MULTI_SZ, true
	case "REG_DWORD", "DWORD":
		return registry.DWORD, true
	case "REG_QWORD", "QWORD":
		return registry.QWORD, true
	case "REG_BINARY", "BINARY":
		return registry.BINARY, true
	default:
		return 0, false
	}
}

// trimTrailingExplicitType removes a trailing :<known type> from an encoded registry reference when the
// remainder still contains ':' (hive\subkey:value). This avoids treating a value name of "DWORD" as a type
// when the path is HKCU\SubKey:DWORD (only one colon in the path tail).
func trimTrailingExplicitType(s string) (string, uint32) {
	last := strings.LastIndex(s, ":")
	if last < 0 {
		return s, 0
	}
	cand := strings.TrimSpace(s[last+1:])
	t, ok := registryTypeFromName(cand)
	if !ok {
		return s, 0
	}
	prefix := s[:last]
	if !strings.Contains(prefix, ":") {
		return s, 0
	}
	return prefix, t
}

// ParseRegistryPath splits HKCU\Sub\Key:ValueName (optional REG: prefix; optional trailing :REG_DWORD).
func parseRegistryPath(encoded string) (registry.Key, string, string, uint32, error) {
	s := strings.TrimSpace(encoded)
	s = strings.TrimPrefix(s, "REG:")
	s = strings.TrimPrefix(s, "reg:")

	pathTyp := uint32(0)
	s, pathTyp = trimTrailingExplicitType(s)

	idx := strings.LastIndex(s, ":")
	if idx <= 0 || idx >= len(s)-1 {
		return 0, "", "", pathTyp, errRegParse
	}
	pathPart := s[:idx]
	valueName := s[idx+1:]
	parts := strings.SplitN(pathPart, `\`, 2)
	if len(parts) < 2 {
		return 0, "", "", pathTyp, errRegParse
	}
	hive := strings.ToUpper(parts[0])
	sub := parts[1]
	var k registry.Key
	switch hive {
	case "HKCU", "HKEY_CURRENT_USER":
		k = registry.CURRENT_USER
	case "HKLM", "HKEY_LOCAL_MACHINE":
		k = registry.LOCAL_MACHINE
	case "HKCR", "HKEY_CLASSES_ROOT":
		k = registry.CLASSES_ROOT
	case "HKU", "HKEY_USERS":
		k = registry.USERS
	case "HKCC", "HKEY_CURRENT_CONFIG":
		k = registry.CURRENT_CONFIG
	default:
		return 0, "", "", pathTyp, fmt.Errorf("%w: unknown hive %s", errRegParse, hive)
	}
	return k, sub, valueName, pathTyp, nil
}
