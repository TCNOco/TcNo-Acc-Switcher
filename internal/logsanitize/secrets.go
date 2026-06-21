package logsanitize

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/Jleagle/steam-go/steamvdf"
	"github.com/tidwall/gjson"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/settingsfile"
)

type idsFile struct {
	IDs map[string]string `json:"ids"`
}

func collectAccountIdentifiers() [][]string {
	var accounts [][]string
	accounts = append(accounts, collectLoginCacheAccounts()...)
	accounts = append(accounts, collectSteamAccounts()...)
	if u := osUsername(); u != "" {
		accounts = append(accounts, []string{u})
	}
	return accounts
}

func collectLoginCacheAccounts() [][]string {
	root, err := paths.DataRoot()
	if err != nil {
		return nil
	}
	loginCache := filepath.Join(root, "LoginCache")
	entries, err := os.ReadDir(loginCache)
	if err != nil {
		return nil
	}
	var accounts [][]string
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(loginCache, ent.Name(), "ids.json"))
		if err != nil {
			continue
		}
		var f idsFile
		if err := json.Unmarshal(data, &f); err != nil || len(f.IDs) == 0 {
			continue
		}
		for id, name := range f.IDs {
			id = strings.TrimSpace(id)
			name = strings.TrimSpace(name)
			var ids []string
			if id != "" {
				ids = append(ids, id)
			}
			if name != "" && !strings.EqualFold(name, id) {
				ids = append(ids, name)
			}
			if len(ids) > 0 {
				accounts = append(accounts, ids)
			}
		}
	}
	return accounts
}

func collectSteamAccounts() [][]string {
	root := resolveSteamRoot()
	if root == "" {
		return nil
	}
	users, err := parseLoginUsers(filepath.Join(root, "config", "loginusers.vdf"))
	if err != nil {
		return nil
	}
	var accounts [][]string
	for _, u := range users {
		var ids []string
		if id := strings.TrimSpace(u.steamID); id != "" {
			ids = append(ids, id)
		}
		if name := strings.TrimSpace(u.accountName); name != "" {
			ids = append(ids, name)
		}
		if persona := strings.TrimSpace(u.personaName); persona != "" {
			ids = append(ids, persona)
		}
		if len(ids) > 0 {
			accounts = append(accounts, ids)
		}
	}
	return accounts
}

func resolveSteamRoot() string {
	if settingsDir, err := paths.SettingsDir(); err == nil {
		if data, err := os.ReadFile(filepath.Join(settingsDir, "SteamSettings.json")); err == nil {
			if fp := strings.TrimSpace(gjson.GetBytes(data, "FolderPath").String()); fp != "" {
				return filepath.Clean(fp)
			}
		}
	}
	dataRoot, err := paths.DataRoot()
	if err != nil {
		return ""
	}
	appSettingsPath := filepath.Join(dataRoot, settingsfile.FileName)
	if data, err := os.ReadFile(appSettingsPath); err == nil {
		if exe := strings.TrimSpace(gjson.GetBytes(data, `PlatformExePaths.Steam`).String()); exe != "" {
			return filepath.Clean(filepath.Dir(exe))
		}
	}
	return ""
}

type steamLoginUser struct {
	steamID     string
	accountName string
	personaName string
}

func parseLoginUsers(path string) ([]steamLoginUser, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		alt := strings.TrimSuffix(path, ".vdf") + ".vdf_last"
		raw, err = os.ReadFile(alt)
		if err != nil {
			return nil, err
		}
	}
	raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	kv, err := steamvdf.ReadBytes(raw)
	if err != nil {
		return nil, err
	}
	usersKV, ok := kv.GetChild("users")
	if !ok {
		for _, ch := range kv.Children {
			if strings.EqualFold(ch.Key, "users") {
				usersKV = ch
				ok = true
				break
			}
		}
	}
	if !ok && len(kv.Children) > 0 && looksLikeSteamID64(kv.Children[0].Key) {
		usersKV = steamvdf.KeyValue{Children: kv.Children}
		ok = true
	}
	if !ok {
		return nil, nil
	}
	var out []steamLoginUser
	for _, u := range usersKV.Children {
		sid := strings.TrimSpace(u.Key)
		if sid == "" {
			continue
		}
		persona := childStringCI(u, "PersonaName")
		acc := childStringCI(u, "AccountName")
		if persona == "" && acc == "" {
			continue
		}
		out = append(out, steamLoginUser{
			steamID:     sid,
			accountName: acc,
			personaName: persona,
		})
	}
	return out, nil
}

func childStringCI(kv steamvdf.KeyValue, key string) string {
	klow := strings.ToLower(key)
	for _, ch := range kv.Children {
		if strings.ToLower(ch.Key) == klow {
			if ch.Value != "" {
				return ch.Value
			}
			if len(ch.Children) > 0 {
				return ch.String()
			}
		}
	}
	return ""
}

func looksLikeSteamID64(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 15 || len(s) > 20 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func osUsername() string {
	if name := strings.TrimSpace(os.Getenv("USERNAME")); name != "" {
		return name
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(filepath.Base(home))
}
