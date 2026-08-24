package steam

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
)

const settingsFileName = "SteamSettings.json"

// Settings adds Steam-only fields; shared options are embedded from platform.PlatformSettings.
type Settings struct {
	platform.PlatformSettings

	FolderPath string `json:"FolderPath"`

	SteamShowSteamID     bool `json:"Steam_ShowSteamID"`
	SteamShowVAC         bool `json:"Steam_ShowVAC"`
	SteamShowLimited     bool `json:"Steam_ShowLimited"`
	SteamShowLastLogin   bool `json:"Steam_ShowLastLogin"`
	SteamShowAccUsername bool `json:"Steam_ShowAccUsername"`
	SteamTrayAccountName bool `json:"Steam_TrayAccountName"`

	SteamImageExpiryTime int `json:"Steam_ImageExpiryTime"`
	SteamOverrideState   int `json:"Steam_OverrideState"`

	ShortcutsJSON map[string]string `json:"ShortcutsJson,omitempty"`

	SteamWebAPIKey string `json:"SteamWebApiKey"`

	ShowSteamSwitcher bool `json:"ShowSteamSwitcher"`
	CollectInfo       bool `json:"CollectInfo"`

	// HiddenBanStatus lists SteamID64s whose VAC or limited status is not shown,
	// however the global toggles are set. A ban on one account is not something
	// the owner necessarily wants on screen every time they open the switcher.
	HiddenBanStatus []string `json:"Steam_HiddenBanStatus,omitempty"`

	SteamShowMiniProfile    bool `json:"Steam_ShowMiniProfile"`
	SteamShowAvatarFrame    bool `json:"Steam_ShowAvatarFrame"`
	SteamShowSteamGuardLock bool `json:"Steam_ShowSteamGuardLock"`

	// SteamCollectCS2Cooldowns gates reading each vault account's CS2
	// competitive cooldown while the Steam Guard vault happens to be unlocked,
	// and showing it on the account tile.
	SteamCollectCS2Cooldowns bool `json:"Steam_CollectCS2Cooldowns"`

	// SteamShowCS2Rank shows the Premier and Competitive ranks the cooldown sweep
	// already read, for accounts with no CS2 game stats configured. Costs nothing
	// extra: the ranks ride along in the same response as the cooldown.
	SteamShowCS2Rank bool `json:"Steam_ShowCS2Rank"`

	// SteamShowCS2PrimeTag shows a Prime pill on vault accounts.
	//
	// Off by default, unlike its neighbours: Prime is not in the GCPD response the
	// sweep already fetches, so each account costs one additional request to the
	// store's licenses page.
	SteamShowCS2PrimeTag bool `json:"Steam_ShowCS2PrimeTag"`
}

func defaultSettings() Settings {
	ps := platform.DefaultPlatformSettings()
	return Settings{
		PlatformSettings:     ps,
		FolderPath:           defaultFolderPath(),
		SteamShowVAC:         true,
		SteamShowLimited:     true,
		SteamShowLastLogin:   true,
		SteamShowAccUsername: true,
		SteamShowSteamID:     false,
		SteamImageExpiryTime: 7,
		SteamOverrideState:   -1,
		ShortcutsJSON:        nil,
		CollectInfo:          true,
		SteamShowMiniProfile: true,
		SteamShowAvatarFrame: true,
		// On by default: the lock is the only place the account list says which
		// accounts the app holds an authenticator for.
		SteamShowSteamGuardLock:  true,
		SteamCollectCS2Cooldowns: true,
		SteamShowCS2Rank:         true,
	}
}

func settingsPath() (string, error) {
	dir, err := paths.SettingsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, settingsFileName), nil
}

// ResetToDefaults overwrites SteamSettings.json with defaults (wired from [platform.SetSteamReset]).
func ResetToDefaults() error {
	return SaveSettings(defaultSettings())
}

// LoadSettings reads Settings/SteamSettings.json.
func LoadSettings() (Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return Settings{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultSettings(), nil
		}
		return Settings{}, err
	}

	// Migrate legacy JSON keys into embedded PlatformSettings before unmarshalling.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return defaultSettings(), err
	}
	if _, has := raw["RunAsAdmin"]; !has {
		if v, ok := raw["Steam_Admin"]; ok {
			raw["RunAsAdmin"] = v
		}
		delete(raw, "Steam_Admin")
	}
	if _, has := raw["TrayAccNumber"]; !has {
		if v, ok := raw["Steam_TrayAccNumber"]; ok {
			raw["TrayAccNumber"] = v
		}
		delete(raw, "Steam_TrayAccNumber")
	}
	// Accept lowercase shortcuts key (some exports / hand edits); Go tag is "Shortcuts".
	if _, has := raw["Shortcuts"]; !has {
		if v, ok := raw["shortcuts"]; ok {
			raw["Shortcuts"] = v
			delete(raw, "shortcuts")
		}
	}
	// Legacy StartSilent bool → LaunchArguments token (-silent); key dropped on save.
	legacySilent := jsonRawMessageBool(raw["StartSilent"])
	delete(raw, "StartSilent")
	delete(raw, "OldUi")
	data2, err := json.Marshal(raw)
	if err != nil {
		return Settings{}, err
	}

	// Onto the defaults, not onto a zero value. A key absent from the file
	// otherwise decodes to false and is then written back, which is how a
	// settings file that predates a flag silently turns the feature off - it took
	// AutoStart and four Steam_Show* flags with it.
	s := defaultSettings()
	if err := json.Unmarshal(data2, &s); err != nil {
		return defaultSettings(), err
	}
	if legacySilent {
		s.LaunchArguments = platform.EnsureLaunchArg(s.LaunchArguments, "-silent")
	}
	s.FolderPath = NormalizeFolderPath(s.FolderPath)
	if len(s.Shortcuts) == 0 && len(s.ShortcutsJSON) > 0 {
		s.Shortcuts = migrateLegacyShortcutsJSON(s.ShortcutsJSON)
		s.ShortcutsJSON = nil
	}
	if s.Shortcuts == nil {
		s.Shortcuts = []platform.GameShortcutEntry{}
	}
	if s.AccountNotes == nil {
		s.AccountNotes = map[string]string{}
	}
	if s.SteamImageExpiryTime <= 0 {
		s.SteamImageExpiryTime = 7
	}
	if strings.TrimSpace(s.ClosingMethod) == "" {
		s.ClosingMethod = "Combined"
	}
	s.ClosingMethod = platform.NormalizeClosingMethod(s.ClosingMethod)
	defClose, forceClose := platform.DescriptorClosingPolicy("Steam")
	if strings.TrimSpace(s.ClosingMethod) == "" {
		s.ClosingMethod = defClose
	}
	if forceClose {
		s.ClosingMethod = defClose
	}
	s.ClosingMethodForced = forceClose
	return s, nil
}

// UpdateSettings runs a load-mutate-save cycle over SteamSettings.json while
// holding the shared platform settings file lock, so a writer in another lock
// domain - shortcuts, the launch bridge, platform's merge patch - cannot land
// between the read and the write and lose the fields either side changed.
//
// mutate must not call UpdateSettings or platform.SavePlatformSettings.
func UpdateSettings(mutate func(*Settings) error) error {
	defer platform.LockPlatformSettingsFile()()
	s, err := LoadSettings()
	if err != nil {
		return err
	}
	if err := mutate(&s); err != nil {
		return err
	}
	return SaveSettings(s)
}

// SaveSettings writes Settings/SteamSettings.json.
func SaveSettings(s Settings) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if s.Shortcuts == nil {
		s.Shortcuts = []platform.GameShortcutEntry{}
	}
	if s.AccountNotes == nil {
		s.AccountNotes = map[string]string{}
	}
	s.ClosingMethodForced = false
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, data, 0o644)
}

// NormalizeFolderPath strips a trailing steam.exe and ensures directory form.
func NormalizeFolderPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if strings.HasSuffix(strings.ToLower(p), ".exe") {
		p = filepath.Dir(p)
	}
	p = filepath.Clean(p)
	if len(p) >= 2 && p[1] == ':' && !strings.HasSuffix(p, `\`) {
		// keep as clean path
	}
	return p
}

// PlatformKey is the folder name under img/profiles.
const PlatformKey = "Steam"
const ProfileFolderSlug = "steam"

func migrateLegacyShortcutsJSON(m map[string]string) []platform.GameShortcutEntry {
	type kv struct {
		k int
		v string
	}
	var neg, pos []kv
	for ks, v := range m {
		ki, err := strconv.Atoi(strings.TrimSpace(ks))
		if err != nil {
			continue
		}
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if ki < 0 {
			neg = append(neg, kv{ki, v})
		} else {
			pos = append(pos, kv{ki, v})
		}
	}
	sort.Slice(neg, func(i, j int) bool { return neg[i].k < neg[j].k })
	sort.Slice(pos, func(i, j int) bool { return pos[i].k < pos[j].k })
	out := make([]platform.GameShortcutEntry, 0, len(neg)+len(pos))
	for _, e := range neg {
		out = append(out, platform.GameShortcutEntry{FileName: e.v, Pinned: true})
	}
	for _, e := range pos {
		out = append(out, platform.GameShortcutEntry{FileName: e.v, Pinned: false})
	}
	return out
}

func jsonRawMessageBool(m json.RawMessage) bool {
	if len(m) == 0 || string(m) == "null" {
		return false
	}
	var b bool
	if err := json.Unmarshal(m, &b); err != nil {
		return false
	}
	return b
}
