package platform

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
)

// PlatformEntry is a subset of a platform definition from Platforms.json.
type PlatformEntry struct {
	ExeLocationDefault       string   `json:"ExeLocationDefault"`
	GetPathFromShortcutNamed string   `json:"GetPathFromShortcutNamed"`
	ExesToEnd                []string `json:"ExesToEnd"`
}

// ParsePlatformEntry returns the platform entry for platformKey.
func ParsePlatformEntry(raw []byte, platformKey string) (PlatformEntry, error) {
	return parsePlatformEntry(raw, platformKey)
}

func parsePlatformEntry(raw []byte, platformKey string) (PlatformEntry, error) {
	var top struct {
		Platforms map[string]json.RawMessage `json:"Platforms"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		return PlatformEntry{}, err
	}
	if top.Platforms == nil {
		return PlatformEntry{}, errors.New("missing Platforms")
	}
	blob, ok := top.Platforms[platformKey]
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
	p := ExpandWindowsPath(strings.TrimSpace(e.ExeLocationDefault))
	if p != "" {
		return filepath.Base(p)
	}
	return ""
}
