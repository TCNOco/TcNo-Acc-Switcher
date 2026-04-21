package basic

import (
	"encoding/json"
	"os"
	"path/filepath"

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
	IDs map[string]string `json:"ids"`
}

func readIDs(platformKey string) (map[string]string, error) {
	p, err := idsPath(platformKey)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	var f idsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return map[string]string{}, nil
	}
	if f.IDs == nil {
		f.IDs = map[string]string{}
	}
	return f.IDs, nil
}

func writeIDs(platformKey string, m map[string]string) error {
	p, err := idsPath(platformKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f := idsFile{IDs: m}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(p, data, 0o644)
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
