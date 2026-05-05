//go:build windows

package winutil

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// RegistryKeyPathForAllValuesSpecifier reports whether encoded is a "whole key" reference:
// hive\Sub\Key:* with value name exactly * (after REG: prefix and optional trailing type suffix).
// When true, keyPath is hive\Sub\Key (suitable for appending :ValueName for each value).
func RegistryKeyPathForAllValuesSpecifier(encoded string) (keyPath string, ok bool) {
	s := strings.TrimSpace(encoded)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "REG:"), "reg:")
	s, _ = trimTrailingExplicitType(s)
	idx := strings.LastIndex(s, ":")
	if idx <= 0 || idx >= len(s)-1 {
		return "", false
	}
	if s[idx+1:] != "*" {
		return "", false
	}
	return s[:idx], true
}

func parseHiveSubKeyPath(keyPath string) (hive registry.Key, sub string, err error) {
	parts := strings.SplitN(strings.TrimSpace(keyPath), `\`, 2)
	if len(parts) < 2 {
		return 0, "", fmt.Errorf("%w: need hive\\subkey path", errRegParse)
	}
	h := strings.ToUpper(parts[0])
	sub = parts[1]
	switch h {
	case "HKCU", "HKEY_CURRENT_USER":
		hive = registry.CURRENT_USER
	case "HKLM", "HKEY_LOCAL_MACHINE":
		hive = registry.LOCAL_MACHINE
	case "HKCR", "HKEY_CLASSES_ROOT":
		hive = registry.CLASSES_ROOT
	case "HKU", "HKEY_USERS":
		hive = registry.USERS
	case "HKCC", "HKEY_CURRENT_CONFIG":
		hive = registry.CURRENT_CONFIG
	default:
		return 0, "", fmt.Errorf("%w: unknown hive %s", errRegParse, h)
	}
	return hive, sub, nil
}

func readRegistryValueAt(key registry.Key, name string) (val any, typ uint32, err error) {
	_, typ, err = key.GetValue(name, nil)
	if err != nil {
		return nil, 0, err
	}
	switch typ {
	case registry.SZ, registry.EXPAND_SZ:
		s, _, err := key.GetStringValue(name)
		return s, typ, err
	case registry.DWORD, registry.QWORD:
		n, _, err := key.GetIntegerValue(name)
		if err != nil {
			return nil, typ, err
		}
		if typ == registry.DWORD {
			return uint32(n), typ, nil
		}
		return n, typ, nil
	default:
		b, _, err := key.GetBinaryValue(name)
		return b, typ, err
	}
}

// RegistryReadAllValuesInKey reads every value under keyPath (hive\Sub\Key, no :Value suffix).
func RegistryReadAllValuesInKey(keyPath string) (map[string]struct {
	Val any
	Typ uint32
}, error) {
	hive, sub, err := parseHiveSubKeyPath(keyPath)
	if err != nil {
		return nil, err
	}
	key, err := registry.OpenKey(hive, sub, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer key.Close()

	names, err := key.ReadValueNames(0)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct {
		Val any
		Typ uint32
	}, len(names))
	for _, name := range names {
		v, typ, err := readRegistryValueAt(key, name)
		if err != nil {
			return nil, err
		}
		out[name] = struct {
			Val any
			Typ uint32
		}{Val: v, Typ: typ}
	}
	return out, nil
}

// RegistryReadValuesMatchingNameGlob reads values under keyPath whose names match valueNameGlob
// (path/filepath syntax, e.g. LastLoginDate_*).
func RegistryReadValuesMatchingNameGlob(keyPath, valueNameGlob string) (map[string]struct {
	Val any
	Typ uint32
}, error) {
	hive, sub, err := parseHiveSubKeyPath(keyPath)
	if err != nil {
		return nil, err
	}
	key, err := registry.OpenKey(hive, sub, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer key.Close()

	names, err := key.ReadValueNames(0)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct {
		Val any
		Typ uint32
	})
	for _, name := range names {
		ok, err := filepath.Match(valueNameGlob, name)
		if err != nil {
			return nil, fmt.Errorf("registry value name glob %q: %w", valueNameGlob, err)
		}
		if !ok {
			continue
		}
		v, typ, err := readRegistryValueAt(key, name)
		if err != nil {
			return nil, err
		}
		out[name] = struct {
			Val any
			Typ uint32
		}{Val: v, Typ: typ}
	}
	return out, nil
}

func deleteEntireSubkey(hive registry.Key, sub string) error {
	k, err := registry.OpenKey(hive, sub, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}

	children, err := k.ReadSubKeyNames(0)
	if err != nil {
		k.Close()
		return err
	}
	for _, name := range children {
		if err := deleteEntireSubkey(hive, sub+`\`+name); err != nil {
			k.Close()
			return err
		}
	}
	vals, err := k.ReadValueNames(0)
	if err != nil {
		k.Close()
		return err
	}
	for _, vn := range vals {
		if err := k.DeleteValue(vn); err != nil {
			k.Close()
			return err
		}
	}
	k.Close()
	return registry.DeleteKey(hive, sub)
}

// RegistryClearLoginKey removes all child subkeys (entire subtrees) and clears or deletes
// every value under the key identified by encoded (REG:hive\Sub\Key:*). The key itself remains.
// When deleteValues is true, each value is removed; otherwise each value is written empty via
// RegistryWriteHint (same semantics as clearing a single REG: path in the basic flow).
func RegistryClearLoginKey(encoded string, deleteValues bool) error {
	kp, ok := RegistryKeyPathForAllValuesSpecifier(encoded)
	if !ok {
		return fmt.Errorf("%w: expected path ending with :*", errRegParse)
	}
	hive, sub, err := parseHiveSubKeyPath(kp)
	if err != nil {
		return err
	}

	for {
		k, err := registry.OpenKey(hive, sub, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE|registry.SET_VALUE)
		if err != nil {
			return err
		}
		subs, err := k.ReadSubKeyNames(0)
		k.Close()
		if err != nil {
			return err
		}
		if len(subs) == 0 {
			break
		}
		for _, name := range subs {
			if err := deleteEntireSubkey(hive, sub+`\`+name); err != nil {
				return err
			}
		}
	}

	k, err := registry.OpenKey(hive, sub, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	vals, err := k.ReadValueNames(0)
	if err != nil {
		return err
	}
	for _, vn := range vals {
		full := kp + ":" + vn
		if deleteValues {
			if err := RegistryDelete(full); err != nil {
				return err
			}
		} else {
			if err := RegistryWriteHint(full, "", 0); err != nil {
				return err
			}
		}
	}
	return nil
}

// RegistryClearValuesMatchingNameGlob deletes or clears values under keyPath whose names match
// valueNameGlob. Subkeys of keyPath are not modified.
func RegistryClearValuesMatchingNameGlob(keyPath, valueNameGlob string, deleteValues bool) error {
	hive, sub, err := parseHiveSubKeyPath(keyPath)
	if err != nil {
		return err
	}
	k, err := registry.OpenKey(hive, sub, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	vals, err := k.ReadValueNames(0)
	if err != nil {
		return err
	}
	for _, vn := range vals {
		ok, err := filepath.Match(valueNameGlob, vn)
		if err != nil {
			return fmt.Errorf("registry value name glob %q: %w", valueNameGlob, err)
		}
		if !ok {
			continue
		}
		full := keyPath + ":" + vn
		if deleteValues {
			if err := RegistryDelete(full); err != nil {
				return err
			}
		} else {
			if err := RegistryWriteHint(full, "", 0); err != nil {
				return err
			}
		}
	}
	return nil
}
