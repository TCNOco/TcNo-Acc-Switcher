package cli

import (
	"encoding/json"
	"sort"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
)

// PlatformIndex maps CLI short tokens to full platform names from Platforms.json.
type PlatformIndex struct {
	// LowerName -> canonical name from JSON (exact key)
	Names map[string]string
	// First identifier (lowercase) -> canonical platform name
	FirstIdentifier map[string]string
	// Any identifier alias (lowercase) -> canonical platform name
	IdentifierAliases map[string]string
	// OrderedNames is canonical platform names sorted for stable CLI output.
	OrderedNames []string
}

// LoadPlatformIndex reads Platforms.json and builds lookup tables.
func LoadPlatformIndex() (*PlatformIndex, error) {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return nil, err
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	var top struct {
		Platforms map[string]json.RawMessage `json:"Platforms"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, err
	}
	idx := &PlatformIndex{
		Names:             make(map[string]string),
		FirstIdentifier:   make(map[string]string),
		IdentifierAliases: make(map[string]string),
	}
	for rawName := range top.Platforms {
		name := strings.TrimSpace(rawName)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		idx.Names[key] = name
		idx.OrderedNames = append(idx.OrderedNames, name)

		var d struct {
			Identifiers []string `json:"Identifiers"`
		}
		_ = json.Unmarshal(top.Platforms[rawName], &d)
		for i, rawID := range d.Identifiers {
			id := strings.ToLower(strings.TrimSpace(rawID))
			if id == "" {
				continue
			}
			idx.IdentifierAliases[id] = name
			if i == 0 {
				idx.FirstIdentifier[id] = name
			}
		}
	}
	sort.Strings(idx.OrderedNames)
	return idx, nil
}

// ShortTokenForPlatform returns Identifiers[0] (lowercase) for the given canonical platform name.
func ShortTokenForPlatform(idx *PlatformIndex, platformName string) string {
	if idx == nil {
		return ""
	}
	want := strings.ToLower(strings.TrimSpace(platformName))
	for short, name := range idx.FirstIdentifier {
		if strings.ToLower(name) == want {
			return short
		}
	}
	return ""
}
