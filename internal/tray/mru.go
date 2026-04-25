package tray

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
)

const trayUsersFile = "Tray_Users.json"

// testTrayUsersPath, when non-empty, overrides the tray JSON path (for unit tests only).
var testTrayUsersPath string

// TrayUser is one remembered account entry (compatible shape with legacy Tray_Users.json).
type TrayUser struct {
	Name string `json:"Name"`
	Arg  string `json:"Arg"`
}

func trayUsersPath() (string, error) {
	if strings.TrimSpace(testTrayUsersPath) != "" {
		return testTrayUsersPath, nil
	}
	root, err := paths.DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, trayUsersFile), nil
}

// LoadUsers reads Tray_Users.json into a map platform -> list (may be empty).
func LoadUsers() (map[string][]TrayUser, error) {
	p, err := trayUsersPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string][]TrayUser{}, nil
		}
		return nil, err
	}
	var raw map[string][]TrayUser
	if err := json.Unmarshal(data, &raw); err != nil {
		return map[string][]TrayUser{}, nil
	}
	if raw == nil {
		raw = map[string][]TrayUser{}
	}
	return raw, nil
}

func saveUsers(m map[string][]TrayUser) error {
	p, err := trayUsersPath()
	if err != nil {
		return err
	}
	if m == nil {
		m = map[string][]TrayUser{}
	}
	data, err := json.MarshalIndent(sortedTrayMap(m), "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(p, data, 0o644)
}

func sortedTrayMap(m map[string][]TrayUser) map[string][]TrayUser {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string][]TrayUser, len(keys))
	for _, k := range keys {
		out[k] = m[k]
	}
	return out
}

// AddUser inserts or moves an account to the front of the platform list and trims to max (max<=0 skips).
func AddUser(platformKey, arg, name string, maxAccounts int) error {
	platformKey = strings.TrimSpace(platformKey)
	arg = strings.TrimSpace(arg)
	name = strings.TrimSpace(name)
	if platformKey == "" || arg == "" || maxAccounts <= 0 {
		return nil
	}

	trayUsers, err := LoadUsers()
	if err != nil {
		return err
	}
	list := trayUsers[platformKey]
	// Remove existing same Arg
	filtered := list[:0]
	for _, u := range list {
		if strings.TrimSpace(u.Arg) != arg {
			filtered = append(filtered, u)
		}
	}
	nu := TrayUser{Name: name, Arg: arg}
	list = append([]TrayUser{nu}, filtered...)
	for len(list) > maxAccounts {
		list = list[:len(list)-1]
	}
	trayUsers[platformKey] = list
	return saveUsers(trayUsers)
}
