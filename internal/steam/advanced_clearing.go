package steam

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"TcNo-Acc-Switcher/internal/winutil"
)

// AdvancedClearResult is returned to the UI for each clearing action.
type AdvancedClearResult struct {
	Lines []string `json:"lines"`
}

// AdvancedClearingItem describes an action the UI may offer (e.g. registry-only on Windows).
type AdvancedClearingItem struct {
	ID          string `json:"id"`
	Category    string `json:"category"` // "general" | "login"
	WindowsOnly bool   `json:"windowsOnly"`
}

const (
	acCloseSteam       = "close_steam"
	acClearLogs        = "clear_logs"
	acClearDumps       = "clear_dumps"
	acClearHTMLCache   = "clear_htmlcache"
	acClearUILogs      = "clear_ui_logs"
	acClearAppCache    = "clear_appcache"
	acClearHTTPCache   = "clear_httpcache"
	acClearDepotCache  = "clear_depotcache"
	acDeleteConfigVDF  = "delete_config_vdf"
	acDeleteLoginUsers = "delete_loginusers_vdf"
	acClearSSFN        = "clear_ssfn"
	acRegAutoLogin     = "reg_autologinuser"
	acRegLastGame      = "reg_lastgamenameused"
	acRegPseudoUUID    = "reg_pseudouuid"
	acRegRememberPass  = "reg_rememberpassword"
)

// AdvancedClearingRegistrySupported is true when HKCU registry edits for Steam are supported.
func (s *SteamService) AdvancedClearingRegistrySupported() bool {
	return runtime.GOOS == "windows"
}

// AdvancedClearingItems lists available actions for building the UI.
func (s *SteamService) AdvancedClearingItems() ([]AdvancedClearingItem, error) {
	items := []AdvancedClearingItem{
		{ID: acCloseSteam, Category: "general", WindowsOnly: false},
		{ID: acClearLogs, Category: "general", WindowsOnly: false},
		{ID: acClearDumps, Category: "general", WindowsOnly: false},
		{ID: acClearHTMLCache, Category: "general", WindowsOnly: false},
		{ID: acClearUILogs, Category: "general", WindowsOnly: false},
		{ID: acClearAppCache, Category: "general", WindowsOnly: false},
		{ID: acClearHTTPCache, Category: "general", WindowsOnly: false},
		{ID: acClearDepotCache, Category: "general", WindowsOnly: false},
		{ID: acDeleteLoginUsers, Category: "login", WindowsOnly: false},
		{ID: acClearSSFN, Category: "login", WindowsOnly: false},
		{ID: acDeleteConfigVDF, Category: "login", WindowsOnly: false},
		{ID: acRegAutoLogin, Category: "login", WindowsOnly: true},
		{ID: acRegLastGame, Category: "login", WindowsOnly: true},
		{ID: acRegPseudoUUID, Category: "login", WindowsOnly: true},
		{ID: acRegRememberPass, Category: "login", WindowsOnly: true},
	}
	return items, nil
}

// RunAdvancedClearingAction performs one advanced-clearing step and returns log lines.
func (s *SteamService) RunAdvancedClearingAction(action string) (AdvancedClearResult, error) {
	action = strings.TrimSpace(strings.ToLower(action))
	var lines []string
	appendLine := func(s string) { lines = append(lines, s) }

	root, err := s.steamInstallRoot()
	if err != nil {
		return AdvancedClearResult{}, err
	}
	root = strings.TrimSpace(root)
	if root == "" && action != acCloseSteam && !strings.HasPrefix(action, "reg_") {
		return AdvancedClearResult{}, fmt.Errorf("steam install folder not found")
	}

	switch action {
	case acCloseSteam:
		st, err := LoadSettings()
		if err != nil {
			return AdvancedClearResult{}, err
		}
		method := winutil.ClosingMethod(st.ClosingMethod)
		if err := winutil.ErrIfCannotKill(steamKillNames, method); err != nil {
			return AdvancedClearResult{}, err
		}
		if err := winutil.KillByName(steamKillNames, method, nil); err != nil {
			appendLine("Warning: " + err.Error())
		}
		appendLine("Closed Steam (or Steam was not running).")

	case acClearLogs:
		clearDirectoryContents(filepath.Join(root, "logs"), appendLine, "logs")

	case acClearDumps:
		clearDirectoryContents(filepath.Join(root, "dumps"), appendLine, "dumps")

	case acClearHTMLCache:
		p, err := steamLocalHTMLCachePath()
		if err != nil {
			return AdvancedClearResult{}, err
		}
		clearDirectoryContents(p, appendLine, "htmlcache")

	case acClearUILogs:
		clearTopLevelGlob(root, []string{"*.log", "*.last"}, appendLine, "UI logs (*.log, *.last)")

	case acClearAppCache:
		clearTopLevelAllFiles(filepath.Join(root, "appcache"), appendLine, "appcache (top-level files only)")

	case acClearHTTPCache:
		clearAllFilesRecursive(filepath.Join(root, "appcache", "httpcache"), appendLine, "appcache\\httpcache")

	case acClearDepotCache:
		clearTopLevelAllFiles(filepath.Join(root, "depotcache"), appendLine, "depotcache")

	case acDeleteConfigVDF:
		tryRemoveFile(filepath.Join(root, "config", "config.vdf"), appendLine, "config\\config.vdf")

	case acDeleteLoginUsers:
		tryRemoveFile(filepath.Join(root, "config", "loginusers.vdf"), appendLine, "config\\loginusers.vdf")

	case acClearSSFN:
		clearSSFNFiles(root, appendLine)

	case acRegAutoLogin:
		tryDeleteSteamRegValue("AutoLoginuser", appendLine)
	case acRegLastGame:
		tryDeleteSteamRegValue("LastGameNameUsed", appendLine)
	case acRegPseudoUUID:
		tryDeleteSteamRegValue("PseudoUUID", appendLine)
	case acRegRememberPass:
		tryDeleteSteamRegValue("RememberPassword", appendLine)

	default:
		return AdvancedClearResult{}, fmt.Errorf("unknown advanced clearing action: %q", action)
	}

	return AdvancedClearResult{Lines: lines}, nil
}

func steamLocalHTMLCachePath() (string, error) {
	switch runtime.GOOS {
	case "windows":
		la := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		if la == "" {
			return "", fmt.Errorf("LOCALAPPDATA is not set")
		}
		return filepath.Join(la, "Steam", "htmlcache"), nil
	case "darwin":
		hd, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(hd, "Library", "Application Support", "Steam", "htmlcache"), nil
	default:
		hd, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(hd, ".local", "share", "Steam", "htmlcache"), nil
	}
}

func clearDirectoryContents(dir string, appendLine func(string), label string) {
	st, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			appendLine(fmt.Sprintf("Skipped %s: folder does not exist (%s).", label, dir))
			return
		}
		appendLine(fmt.Sprintf("Error opening %s: %v", label, err))
		return
	}
	if !st.IsDir() {
		appendLine(fmt.Sprintf("Skipped %s: not a directory (%s).", label, dir))
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		appendLine(fmt.Sprintf("Error reading %s: %v", label, err))
		return
	}
	var removed int
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() {
			if err := os.RemoveAll(p); err != nil {
				appendLine(fmt.Sprintf("Could not remove %s: %v", p, err))
				continue
			}
			removed++
			continue
		}
		if err := os.Remove(p); err != nil {
			appendLine(fmt.Sprintf("Could not remove %s: %v", p, err))
			continue
		}
		removed++
	}
	if removed == 0 {
		appendLine(fmt.Sprintf("Cleared %s: nothing to remove (already empty).", label))
	} else {
		appendLine(fmt.Sprintf("Cleared %s: removed %d item(s).", label, removed))
	}
}

func clearTopLevelGlob(root string, patterns []string, appendLine func(string), label string) {
	var n int
	for _, pat := range patterns {
		matches, err := filepath.Glob(filepath.Join(root, pat))
		if err != nil {
			appendLine(fmt.Sprintf("Glob error for %s: %v", pat, err))
			continue
		}
		for _, p := range matches {
			st, err := os.Stat(p)
			if err != nil {
				continue
			}
			if st.IsDir() {
				continue
			}
			if err := os.Remove(p); err != nil {
				appendLine(fmt.Sprintf("Could not remove %s: %v", p, err))
				continue
			}
			n++
		}
	}
	if n == 0 {
		appendLine(fmt.Sprintf("Cleared %s: no matching files found.", label))
	} else {
		appendLine(fmt.Sprintf("Cleared %s: removed %d file(s).", label, n))
	}
}

func clearTopLevelAllFiles(dir string, appendLine func(string), label string) {
	st, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			appendLine(fmt.Sprintf("Skipped %s: folder does not exist (%s).", label, dir))
			return
		}
		appendLine(fmt.Sprintf("Error opening %s: %v", label, err))
		return
	}
	if !st.IsDir() {
		appendLine(fmt.Sprintf("Skipped %s: not a directory.", label))
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		appendLine(fmt.Sprintf("Error reading %s: %v", label, err))
		return
	}
	var n int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if err := os.Remove(p); err != nil {
			appendLine(fmt.Sprintf("Could not remove %s: %v", p, err))
			continue
		}
		n++
	}
	if n == 0 {
		appendLine(fmt.Sprintf("Cleared %s: no files at top level.", label))
	} else {
		appendLine(fmt.Sprintf("Cleared %s: removed %d file(s).", label, n))
	}
}

func clearAllFilesRecursive(dir string, appendLine func(string), label string) {
	st, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			appendLine(fmt.Sprintf("Skipped %s: folder does not exist (%s).", label, dir))
			return
		}
		appendLine(fmt.Sprintf("Error opening %s: %v", label, err))
		return
	}
	if !st.IsDir() {
		appendLine(fmt.Sprintf("Skipped %s: not a directory.", label))
		return
	}
	var n int
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if err := os.Remove(path); err != nil {
			appendLine(fmt.Sprintf("Could not remove %s: %v", path, err))
			return nil
		}
		n++
		return nil
	})
	if err != nil {
		appendLine(fmt.Sprintf("Walk error in %s: %v", label, err))
		return
	}
	if n == 0 {
		appendLine(fmt.Sprintf("Cleared %s: no files found.", label))
	} else {
		appendLine(fmt.Sprintf("Cleared %s: removed %d file(s) (recursive).", label, n))
	}
}

func tryRemoveFile(path string, appendLine func(string), label string) {
	err := os.Remove(path)
	if err != nil {
		if os.IsNotExist(err) {
			appendLine(fmt.Sprintf("%s: file not found (nothing to do).", label))
			return
		}
		appendLine(fmt.Sprintf("Could not remove %s: %v", label, err))
		return
	}
	appendLine(fmt.Sprintf("Removed %s.", label))
}

func clearSSFNFiles(root string, appendLine func(string)) {
	matches, err := filepath.Glob(filepath.Join(root, "ssfn*"))
	if err != nil {
		appendLine(fmt.Sprintf("Glob ssfn*: %v", err))
		return
	}
	var n int
	for _, p := range matches {
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		if err := os.Remove(p); err != nil {
			appendLine(fmt.Sprintf("Could not remove %s: %v", p, err))
			continue
		}
		n++
	}
	if n == 0 {
		appendLine("No SSFN files found.")
	} else {
		appendLine(fmt.Sprintf("Cleared SSFN files: removed %d file(s).", n))
	}
}

func tryDeleteSteamRegValue(valueName string, appendLine func(string)) {
	if runtime.GOOS != "windows" {
		appendLine("Registry actions are only available on Windows.")
		return
	}
	encoded := "HKCU\\Software\\Valve\\Steam:" + valueName
	err := winutil.RegistryDelete(encoded)
	if err != nil {
		if winutil.RegistryDeleteIsNotExist(err) {
			appendLine(fmt.Sprintf("Registry value %s was not set (nothing to do).", valueName))
			return
		}
		appendLine(fmt.Sprintf("Could not delete registry value %s: %v", valueName, err))
		return
	}
	appendLine(fmt.Sprintf("Deleted registry value: %s", valueName))
}
