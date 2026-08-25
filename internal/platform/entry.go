package platform

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
)

// PlatformEntry is a subset of a platform definition from Platforms.json.
type PlatformEntry struct {
	ExeLocationDefault       ExeLocationDefaultList `json:"ExeLocationDefault"`
	GetPathFromShortcutNamed string                 `json:"GetPathFromShortcutNamed"`
	ExesToEnd                []string               `json:"ExesToEnd"`
}

// ParsePlatformEntry returns the platform entry for platformKey.
func ParsePlatformEntry(raw []byte, platformKey string) (PlatformEntry, error) {
	return parsePlatformEntry(raw, platformKey)
}

func parsePlatformEntry(raw []byte, platformKey string) (PlatformEntry, error) {
	entries, _, err := indexCatalog(raw)
	if err != nil {
		return PlatformEntry{}, err
	}
	if entries == nil {
		return PlatformEntry{}, errors.New("missing Platforms")
	}
	blob, ok := entries[platformKey]
	if !ok {
		return PlatformEntry{}, errors.New("unknown platform: " + platformKey)
	}
	var e PlatformEntry
	if err := json.Unmarshal(blob, &e); err != nil {
		return PlatformEntry{}, err
	}
	return e, nil
}

func primaryExeName(e PlatformEntry) string {
	for _, raw := range e.ExeLocationDefault {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		p := ExpandWindowsPath(raw)
		if p != "" {
			return filepath.Base(p)
		}
	}
	return ""
}
