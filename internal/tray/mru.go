package tray

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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
		recovered := recoverMalformedTrayUsers(data)
		if len(recovered) > 0 {
			_ = saveUsers(recovered)
			return recovered, nil
		}
		return map[string][]TrayUser{}, nil
	}
	if raw == nil {
		raw = map[string][]TrayUser{}
	}
	return raw, nil
}

func recoverMalformedTrayUsers(data []byte) map[string][]TrayUser {
	s := string(data)
	keyRe := regexp.MustCompile(`"((?:\\.|[^"\\])*)"\s*:\s*\[`)
	objRe := regexp.MustCompile(`(?s)\{.*?\}`)
	out := map[string][]TrayUser{}

	for _, loc := range keyRe.FindAllStringSubmatchIndex(s, -1) {
		if len(loc) < 4 {
			continue
		}
		nameRaw := s[loc[2]:loc[3]]
		platformKey, err := strconv.Unquote(`"` + nameRaw + `"`)
		if err != nil {
			platformKey = nameRaw
		}
		platformKey = strings.TrimSpace(platformKey)
		if platformKey == "" {
			continue
		}

		openBracket := strings.LastIndex(s[loc[0]:loc[1]], "[")
		if openBracket < 0 {
			continue
		}
		arrayStart := loc[0] + openBracket
		arrayEnd := matchingBracketIndex(s, arrayStart)
		if arrayEnd <= arrayStart {
			continue
		}

		arrayBody := s[arrayStart+1 : arrayEnd]
		for _, objLoc := range objRe.FindAllStringIndex(arrayBody, -1) {
			obj := arrayBody[objLoc[0]:objLoc[1]]
			var u TrayUser
			if err := json.Unmarshal([]byte(obj), &u); err != nil {
				continue
			}
			u.Name = strings.TrimSpace(u.Name)
			u.Arg = strings.TrimSpace(u.Arg)
			if u.Arg == "" {
				continue
			}
			out[platformKey] = append(out[platformKey], u)
		}
	}
	return out
}

func matchingBracketIndex(s string, open int) int {
	if open < 0 || open >= len(s) || s[open] != '[' {
		return -1
	}
	depth := 0
	inString := false
	escaped := false
	for i := open; i < len(s); i++ {
		c := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
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

// SyncPlatformUsers removes stale/duplicate entries for a platform and refreshes
// names for entries that are still valid. It does not add accounts that are not
// already present in the tray MRU.
func SyncPlatformUsers(platformKey string, argNames map[string]string, maxAccounts int) error {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" || maxAccounts <= 0 {
		return nil
	}

	valid := make(map[string]string, len(argNames))
	for arg, name := range argNames {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		valid[arg] = strings.TrimSpace(name)
	}

	trayUsers, err := LoadUsers()
	if err != nil {
		return err
	}
	list := trayUsers[platformKey]
	if len(list) == 0 {
		return nil
	}

	filtered := make([]TrayUser, 0, len(list))
	seen := map[string]struct{}{}
	for _, u := range list {
		arg := strings.TrimSpace(u.Arg)
		name, ok := valid[arg]
		if !ok {
			continue
		}
		if _, ok := seen[arg]; ok {
			continue
		}
		seen[arg] = struct{}{}
		if name == "" {
			name = strings.TrimSpace(u.Name)
		}
		filtered = append(filtered, TrayUser{Name: name, Arg: arg})
	}
	for len(filtered) > maxAccounts {
		filtered = filtered[:len(filtered)-1]
	}
	trayUsers[platformKey] = filtered
	return saveUsers(trayUsers)
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
