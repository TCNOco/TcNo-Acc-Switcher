package basic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
)

func loginCacheRoot(platformKey string) (string, error) {
	return paths.LoginCacheDir(platformKey)
}

func idsPath(platformKey string) (string, error) {
	base, err := loginCacheRoot(platformKey)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "ids.json"), nil
}

func orderPath(platformKey string) (string, error) {
	base, err := loginCacheRoot(platformKey)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "order.json"), nil
}

type idsFile struct {
	IDs                map[string]string            `json:"ids"`
	LastUsed           map[string]string            `json:"lastused"`
	Tags               map[string]tagFileEntry      `json:"tags,omitempty"`
	AccountTags        map[string][]string          `json:"accountTags,omitempty"`
	AccountTagExpiries map[string]map[string]string `json:"accountTagExpiries,omitempty"`
}

func readIdsFile(platformKey string) (idsFile, error) {
	p, err := idsPath(platformKey)
	if err != nil {
		return idsFile{}, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			f := idsFile{IDs: map[string]string{}, LastUsed: map[string]string{}}
			normalizeTagMaps(&f)
			return f, nil
		}
		return idsFile{}, fmt.Errorf("read %s: %w", p, err)
	}
	var f idsFile
	if err := json.Unmarshal(data, &f); err != nil {
		f2 := idsFile{IDs: map[string]string{}, LastUsed: map[string]string{}}
		normalizeTagMaps(&f2)
		return f2, nil
	}
	if f.IDs == nil {
		f.IDs = map[string]string{}
	}
	if f.LastUsed == nil {
		f.LastUsed = map[string]string{}
	}
	normalizeTagMaps(&f)
	return f, nil
}

func writeIdsFile(platformKey string, f idsFile) error {
	p, err := idsPath(platformKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(p), err)
	}
	if f.IDs == nil {
		f.IDs = map[string]string{}
	}
	if f.LastUsed == nil {
		f.LastUsed = map[string]string{}
	}
	normalizeTagMaps(&f)
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(p, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", p, err)
	}
	return nil
}

func readIDs(platformKey string) (map[string]string, error) {
	f, err := readIdsFile(platformKey)
	if err != nil {
		return nil, err
	}
	return f.IDs, nil
}

func writeIDs(platformKey string, m map[string]string) error {
	f, err := readIdsFile(platformKey)
	if err != nil {
		return err
	}
	f.IDs = m
	return writeIdsFile(platformKey, f)
}

// touchLastUsed records RFC3339 UTC for the account switched to (ids unique id key).
func touchLastUsed(platformKey, uniqueID string) error {
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	if platformKey == "" || uniqueID == "" {
		return nil
	}
	f, err := readIdsFile(platformKey)
	if err != nil {
		return err
	}
	if f.LastUsed == nil {
		f.LastUsed = map[string]string{}
	}
	f.LastUsed[uniqueID] = time.Now().UTC().Format(time.RFC3339)
	return writeIdsFile(platformKey, f)
}

func readOrder(platformKey string) ([]string, error) {
	p, err := orderPath(platformKey)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var o []string
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, nil
	}
	return o, nil
}

func writeOrder(platformKey string, order []string) error {
	p, err := orderPath(platformKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(order, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(p, data, 0o644)
}
