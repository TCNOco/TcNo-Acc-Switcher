package steam

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Jleagle/steam-go/steamvdf"
)

// LoginUser is one row from loginusers.vdf.
type LoginUser struct {
	SteamID64    string
	PersonaName  string
	AccountName  string
	Timestamp    string
	WantsOffline string
	// MostRecent is "1" when Steam marks this row as the active session (when present).
	MostRecent string
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

// ParseLoginUsers reads loginusers.vdf and returns users. Tries path, then path with .vdf_last.
func ParseLoginUsers(path string) ([]LoginUser, error) {
	try := func(p string) ([]LoginUser, error) {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
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
			usersKV = steamvdf.KeyValue{Key: "users", Children: kv.Children}
			ok = true
		}
		if !ok {
			return nil, nil
		}
		var out []LoginUser
		for _, u := range usersKV.Children {
			sid := strings.TrimSpace(u.Key)
			if sid == "" {
				continue
			}
			persona := childStringCI(u, "PersonaName")
			if persona == "" {
				persona = childStringCI(u, "personaname")
			}
			acc := childStringCI(u, "AccountName")
			if acc == "" {
				acc = childStringCI(u, "accountname")
			}
			if persona == "" && acc == "" {
				continue
			}
			ts := childStringCI(u, "Timestamp")
			off := childStringCI(u, "WantsOfflineMode")
			mr := childStringCI(u, "MostRecent")
			if mr == "" {
				mr = childStringCI(u, "mostrecent")
			}
			out = append(out, LoginUser{
				SteamID64:    sid,
				PersonaName:  persona,
				AccountName:  acc,
				Timestamp:    ts,
				WantsOffline: off,
				MostRecent:   mr,
			})
		}
		return out, nil
	}

	out, err := try(path)
	if err == nil && len(out) > 0 {
		return out, nil
	}
	alt := strings.TrimSuffix(path, ".vdf") + ".vdf_last"
	if st, e := os.Stat(alt); e == nil && !st.IsDir() {
		out2, err2 := try(alt)
		if err2 == nil && len(out2) > 0 {
			return out2, nil
		}
		if err == nil {
			err = err2
		}
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ActiveSessionSteamID64 picks the session Steam treats as current: MostRecent=="1" if
// exactly one row has it; otherwise the user with the highest Timestamp (last login).
func ActiveSessionSteamID64(users []LoginUser) string {
	var mostRecentID string
	nMost := 0
	for _, u := range users {
		if strings.TrimSpace(u.MostRecent) == "1" {
			nMost++
			mostRecentID = u.SteamID64
		}
	}
	if nMost == 1 && mostRecentID != "" {
		return mostRecentID
	}

	var bestID string
	var bestTS int64 = -1
	for _, u := range users {
		ts, err := strconv.ParseInt(strings.TrimSpace(u.Timestamp), 10, 64)
		if err != nil || ts <= 0 {
			continue
		}
		if ts > bestTS {
			bestTS = ts
			bestID = u.SteamID64
		}
	}
	return bestID
}

func looksLikeSteamID64(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 15 || len(s) > 20 {
		return false
	}
	_, err := strconv.ParseUint(s, 10, 64)
	return err == nil
}

// LoginUsersFileExists reports whether config/loginusers.vdf exists under steamRoot.
func LoginUsersFileExists(steamRoot string) bool {
	p := filepath.Join(steamRoot, "config", "loginusers.vdf")
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
