//go:build windows

package winutil

import (
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// RegistryRead reads a registry value. encoded is like HKCU\Software\Foo:ValueName (optional REG: prefix).
// Returns value and registry value type (REG_*).
func RegistryRead(encoded string) (any, uint32, error) {
	k, sub, val, err := parseRegistryPath(encoded)
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
func RegistryWrite(encoded string, value any) error {
	k, sub, val, err := parseRegistryPath(encoded)
	if err != nil {
		return err
	}
	key, _, err := registry.CreateKey(k, sub, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

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

// RegistryDelete removes a value or the whole key if value name is empty (not used here).
func RegistryDelete(encoded string) error {
	k, sub, val, err := parseRegistryPath(encoded)
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

// ParseRegistryPath splits HKCU\Sub\Key:ValueName or REG:HKCU\...:Value.
func parseRegistryPath(encoded string) (registry.Key, string, string, error) {
	s := strings.TrimSpace(encoded)
	s = strings.TrimPrefix(s, "REG:")
	s = strings.TrimPrefix(s, "reg:")
	idx := strings.LastIndex(s, ":")
	if idx <= 0 || idx >= len(s)-1 {
		return 0, "", "", errRegParse
	}
	pathPart := s[:idx]
	valueName := s[idx+1:]
	parts := strings.SplitN(pathPart, `\`, 2)
	if len(parts) < 2 {
		return 0, "", "", errRegParse
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
		return 0, "", "", fmt.Errorf("%w: unknown hive %s", errRegParse, hive)
	}
	return k, sub, valueName, nil
}
