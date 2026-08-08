package steam

import (
	"encoding/json"
	"os"
	"path/filepath"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
)

func orderPath() (string, error) {
	r, err := paths.LoginCacheDir("Steam")
	if err != nil {
		return "", err
	}
	return filepath.Join(r, "order.json"), nil
}

// LoadOrder returns saved SteamID64 order (may be empty).
func LoadOrder() ([]string, error) {
	p, err := orderPath()
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
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// SaveOrder writes order.json.
func SaveOrder(ids []string) error {
	p, err := orderPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ids, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(p, data, 0o644)
}

// removeFromOrder drops one id from the saved order. MergeOrder merely ignores
// ids it does not recognise, so without this a forgotten account would sit in
// order.json forever and reclaim its old position if it were ever re-added.
func removeFromOrder(steamID64 string) error {
	saved, err := LoadOrder()
	if err != nil {
		return err
	}
	next := make([]string, 0, len(saved))
	found := false
	for _, id := range saved {
		if id == steamID64 {
			found = true
			continue
		}
		next = append(next, id)
	}
	if !found {
		return nil
	}
	return SaveOrder(next)
}

// MergeOrder applies saved order: listed ids first (that exist), then remaining in input order.
func MergeOrder(saved []string, users []LoginUser) []LoginUser {
	byID := make(map[string]LoginUser, len(users))
	for _, u := range users {
		byID[u.SteamID64] = u
	}
	seen := make(map[string]struct{})
	var out []LoginUser
	for _, id := range saved {
		if u, ok := byID[id]; ok {
			out = append(out, u)
			seen[id] = struct{}{}
		}
	}
	for _, u := range users {
		if _, ok := seen[u.SteamID64]; !ok {
			out = append(out, u)
			seen[u.SteamID64] = struct{}{}
		}
	}
	return out
}
