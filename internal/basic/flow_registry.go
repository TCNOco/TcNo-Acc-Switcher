package basic

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"TcNo-Acc-Switcher/internal/winutil"
)

type regDumpEntry struct {
	V      string                  `json:"v,omitempty"`
	T      uint32                  `json:"t,omitempty"`
	Values map[string]regDumpEntry `json:"values,omitempty"`
}

func registryValueStringForDump(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return winutil.HexEncodeBinary(x)
	case uint32:
		return fmt.Sprintf("%d", x)
	case uint64:
		return fmt.Sprintf("%d", x)
	default:
		return fmt.Sprint(x)
	}
}

func regDumpFromJSON(data []byte) (map[string]regDumpEntry, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]regDumpEntry, len(raw))
	for k, rm := range raw {
		var s string
		if err := json.Unmarshal(rm, &s); err == nil {
			out[k] = regDumpEntry{V: s, T: 0}
			continue
		}
		var e regDumpEntry
		if err := json.Unmarshal(rm, &e); err != nil {
			return nil, fmt.Errorf("reg.json key %q: %w", k, err)
		}
		out[k] = e
	}
	return out, nil
}

func splitRegistryPathValue(enc string) (keyPath, valueName string, ok bool) {
	enc = strings.TrimSpace(enc)
	idx := strings.LastIndex(enc, ":")
	if idx <= 0 || idx >= len(enc)-1 {
		return "", "", false
	}
	keyPath = enc[:idx]
	valueName = enc[idx+1:]
	if !strings.Contains(keyPath, `\`) {
		return "", "", false
	}
	return keyPath, valueName, true
}

func splitRegistryKeyPathAndValueGlob(enc string) (keyPath, glob string, ok bool) {
	kp, v, ok := splitRegistryPathValue(enc)
	if !ok || v == "*" || !hasGlobPattern(v) {
		return "", "", false
	}
	return kp, v, true
}

func regDumpLookup(m map[string]regDumpEntry, descriptorKey string) (regDumpEntry, bool) {
	k := strings.TrimSpace(descriptorKey)
	if e, ok := m[k]; ok {
		return e, true
	}
	base := stripREG(k)
	if !isREG(k) {
		if e, ok := m["REG:"+base]; ok {
			return e, true
		}
	}
	if isREG(k) {
		if e, ok := m[base]; ok {
			return e, true
		}
	}
	keyPath, valName, ok := splitRegistryPathValue(base)
	if ok && valName != "" && valName != "*" {
		for wildKey, bundle := range m {
			if len(bundle.Values) == 0 {
				continue
			}
			wb := strings.TrimSpace(stripREG(wildKey))
			wkPath, wValPart, wok := splitRegistryPathValue(wb)
			if !wok || !strings.EqualFold(wkPath, keyPath) {
				continue
			}
			switch {
			case wValPart == "*":
				if ve, ok := bundle.Values[valName]; ok {
					return ve, true
				}
			case hasGlobPattern(wValPart):
				matched, err := filepath.Match(wValPart, valName)
				if err != nil || !matched {
					continue
				}
				if ve, ok := bundle.Values[valName]; ok {
					return ve, true
				}
			}
		}
	}
	return regDumpEntry{}, false
}

func firstValueNameMatchingGlob(values map[string]regDumpEntry, valueNameGlob string) string {
	var names []string
	for vn := range values {
		ok, err := filepath.Match(valueNameGlob, vn)
		if err != nil || !ok {
			continue
		}
		names = append(names, vn)
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return names[0]
}

func writeRegistryFromRegDump(liveKey string, e regDumpEntry) error {
	if len(e.Values) > 0 {
		enc := stripREG(liveKey)
		kp, valPart, ok := splitRegistryPathValue(enc)
		if !ok {
			return fmt.Errorf("registry dump has values map but key %q is not a valid registry path", liveKey)
		}
		if valPart != "*" && !hasGlobPattern(valPart) {
			return fmt.Errorf("registry dump has values map but key %q must use value :* or a glob (* ? [)", liveKey)
		}
		for valName, ent := range e.Values {
			full := "REG:" + kp + ":" + valName
			if err := writeRegistryFromRegDump(full, ent); err != nil {
				return err
			}
		}
		return nil
	}
	enc := stripREG(liveKey)
	v := strings.TrimSpace(e.V)
	if v == "" {
		if e.T != 0 {
			return winutil.RegistryWriteHint(enc, "", e.T)
		}
		return nil
	}
	if strings.HasPrefix(strings.ToLower(v), "(hex)") {
		switch e.T {
		case winutil.RegValueTypeDWORD, winutil.RegValueTypeQWORD:
			return winutil.RegistryWriteHint(enc, v, e.T)
		default:
			raw, err := parseHexReg(v)
			if err != nil {
				return err
			}
			return winutil.RegistryWriteHint(enc, raw, e.T)
		}
	}
	return winutil.RegistryWriteHint(enc, v, e.T)
}

func parseHexReg(s string) ([]byte, error) {
	return winutil.ParseHexString(s)
}
