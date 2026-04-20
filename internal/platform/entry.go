package platform

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
)

type platformEntry struct {
	ExeLocationDefault       string   `json:"ExeLocationDefault"`
	GetPathFromShortcutNamed string   `json:"GetPathFromShortcutNamed"`
	ExesToEnd                []string `json:"ExesToEnd"`
}

func parsePlatformEntry(raw []byte, platformKey string) (platformEntry, error) {
	var top struct {
		Platforms map[string]json.RawMessage `json:"Platforms"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		return platformEntry{}, err
	}
	if top.Platforms == nil {
		return platformEntry{}, errors.New("missing Platforms")
	}
	blob, ok := top.Platforms[platformKey]
	if !ok {
		return platformEntry{}, errors.New("unknown platform: " + platformKey)
	}
	var e platformEntry
	if err := json.Unmarshal(blob, &e); err != nil {
		return platformEntry{}, err
	}
	return e, nil
}

func primaryExeName(e platformEntry) string {
	if len(e.ExesToEnd) > 0 && strings.TrimSpace(e.ExesToEnd[0]) != "" {
		return filepath.Base(strings.TrimSpace(e.ExesToEnd[0]))
	}
	p := expandWindowsPath(strings.TrimSpace(e.ExeLocationDefault))
	if p == "" {
		return ""
	}
	return filepath.Base(p)
}
