package steam

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
)

const platformName = "Steam"

// ResolveInstallFolder returns the Steam installation root (folder containing steam.exe).
// Order: SteamSettings.FolderPath → PlatformExePaths["Steam"] (exe dir) → ExeLocationDefault from Platforms.json.
func ResolveInstallFolder(exeDir string, s Settings, app platform.AppSettings, platformsJSON []byte) (string, error) {
	if fp := strings.TrimSpace(s.FolderPath); fp != "" {
		if st, err := os.Stat(filepath.Join(fp, "config", "loginusers.vdf")); err == nil && !st.IsDir() {
			return filepath.Clean(fp), nil
		}
		// path set but loginusers missing — still return folder for user to fix
		return filepath.Clean(fp), nil
	}

	if exe := strings.TrimSpace(app.PlatformExePaths[platformName]); exe != "" {
		dir := filepath.Dir(exe)
		if dir != "" && dir != "." {
			return filepath.Clean(dir), nil
		}
	}

	entry, err := platform.ParsePlatformEntry(platformsJSON, platformName)
	if err != nil {
		return "", err
	}
	if found := entry.ExeLocationDefault.FirstExistingExe(); found != "" {
		return filepath.Clean(filepath.Dir(found)), nil
	}
	if exp := entry.ExeLocationDefault.FirstExpanded(); exp != "" {
		return filepath.Clean(filepath.Dir(exp)), nil
	}
	return "", nil
}

// installRoot resolves the Steam install root for callers that have no
// SteamService to hand. Each call re-reads settings from disk.
func installRoot() (string, error) {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return "", err
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return "", err
	}
	st, err := LoadSettings()
	if err != nil {
		return "", err
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return "", err
	}
	return ResolveInstallFolder(exeDir, st, app, raw)
}

// LoginUsersPath returns .../config/loginusers.vdf under the Steam root.
func LoginUsersPath(steamRoot string) string {
	return filepath.Join(steamRoot, "config", "loginusers.vdf")
}

// steamRootCandidates lists every place Steam might be installed, in the order
// ResolveInstallFolder trusts them and without duplicates.
//
// ResolveInstallFolder has to commit to one answer, and it returns
// SteamSettings.FolderPath whether or not anything is there - the user needs to
// see the path they configured in order to correct it. That default ships as
// C:\Program Files (x86)\Steam, so on a machine with Steam elsewhere the
// configured value is a dead path that outranks the two sources that would have
// found the real one. Callers that can test a root for themselves walk this list
// instead and skip the ones that do not hold what they need.
func steamRootCandidates() []string {
	var out []string
	add := func(p string) {
		if p = strings.TrimSpace(p); p == "" {
			return
		}
		p = filepath.Clean(p)
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}

	if st, err := LoadSettings(); err == nil {
		add(st.FolderPath)
	}
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return out
	}
	if app, err := platform.LoadAppSettings(exeDir); err == nil {
		if exe := strings.TrimSpace(app.PlatformExePaths[platformName]); exe != "" {
			if dir := filepath.Dir(exe); dir != "" && dir != "." {
				add(dir)
			}
		}
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return out
	}
	entry, err := platform.ParsePlatformEntry(raw, platformName)
	if err != nil {
		return out
	}
	if found := entry.ExeLocationDefault.FirstExistingExe(); found != "" {
		add(filepath.Dir(found))
	}
	if exp := entry.ExeLocationDefault.FirstExpanded(); exp != "" {
		add(filepath.Dir(exp))
	}
	return out
}

// A var because resolving the candidates reads the app's settings and
// Platforms.json off disk, which a unit test has no business doing.
var steamRootCandidatesFn = steamRootCandidates

// ErrSteamNotFound reports that no candidate root held a loginusers.vdf. A
// sentinel because "no accounts" and "Steam is somewhere the switcher cannot
// see" look identical on screen, and only the second is worth telling the user
// about.
var ErrSteamNotFound = errors.New("could not find Steam's loginusers.vdf")

// accountsRoot narrows a resolved install folder to one that actually holds
// config/loginusers.vdf, walking the rest of the candidate chain when it does
// not. Returns resolved unchanged when nothing better exists, so an install
// that has simply never been logged into keeps the folder the user configured.
//
// Every path that reads or writes account state has to agree on this. The list
// and the order validation count the same union, so two roots would make every
// reorder look like it named accounts that do not exist.
func accountsRoot(resolved string) string {
	if resolved != "" && LoginUsersFileExists(resolved) {
		return resolved
	}
	for _, root := range steamRootCandidatesFn() {
		if LoginUsersFileExists(root) {
			return root
		}
	}
	return resolved
}
