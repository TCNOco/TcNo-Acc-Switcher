package steam

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/steam/shortcutsvdf"
	"TcNo-Acc-Switcher/internal/winutil"
)

// SteamShortcutState is what the settings screen needs to draw the row.
type SteamShortcutState struct {
	// SteamInstalled decides whether the row is shown at all.
	SteamInstalled bool `json:"steamInstalled"`
	Enabled        bool `json:"enabled"`
	// Present reports that the entry on disk is there and points at where this
	// copy of the app actually lives, which stops being true when it is moved.
	Present bool `json:"present"`
	// Users counts the Steam users there are shortcut lists to write to. Zero
	// with Steam installed means it has never been signed into.
	Users int `json:"users"`
	// SteamRunning means the entry will not show up until Steam is restarted:
	// Steam reads the shortcut list once, at startup.
	SteamRunning bool `json:"steamRunning"`
	// FlatpakSteam means one of the targets is a Flatpak Steam, which needs a
	// permission it does not ship with before it can launch anything on the host.
	FlatpakSteam bool `json:"flatpakSteam"`
}

// SteamShortcutApplyResult reports what one toggle actually did.
type SteamShortcutApplyResult struct {
	Enabled bool `json:"enabled"`
	// Users counts the shortcut files written. Zero means Steam is installed but
	// has never been signed into, so there is nowhere to put the entry yet.
	Users        int  `json:"users"`
	SteamRunning bool `json:"steamRunning"`
	FlatpakSteam bool `json:"flatpakSteam"`
}

// GetSteamShortcutState reports whether the switcher is in Steam's shortcut list.
func (s *SteamService) GetSteamShortcutState() (SteamShortcutState, error) {
	state := SteamShortcutState{SteamRunning: steamIsRunning()}

	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return state, err
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return state, err
	}
	state.Enabled = app.AddToSteam

	roots := selfShortcutRoots()
	state.SteamInstalled = len(roots) > 0
	if !state.SteamInstalled {
		if _, ok := ResolveSteamExePath(); ok {
			state.SteamInstalled = true
		}
		return state, nil
	}

	present := false
	for _, root := range roots {
		target, err := resolveSelfShortcutTarget(root)
		if err != nil {
			return state, err
		}
		if target.Flatpak {
			state.FlatpakSteam = true
		}
		for _, path := range shortcutsVDFPaths(root) {
			state.Users++
			list, err := readShortcuts(path)
			if err != nil {
				continue
			}
			if matchesTarget(list, target) {
				present = true
			}
		}
	}
	state.Present = present
	return state, nil
}

// SetAddToSteam adds the switcher to every local Steam user's shortcut list, or
// takes it back out, and remembers which was asked for.
//
// Steam is not closed first. It only writes shortcuts.vdf when its own list
// changes, so a write underneath a running client survives - measured, against
// the claim that it rewrites the file on exit. What a running Steam does mean is
// that the entry will not appear until it restarts, which is what SteamRunning
// in the result is for.
func (s *SteamService) SetAddToSteam(enabled bool) (SteamShortcutApplyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := SteamShortcutApplyResult{Enabled: enabled, SteamRunning: steamIsRunning()}
	users, flatpak, applyErr := applySelfShortcut(enabled, nil)
	result.Users = users
	result.FlatpakSteam = flatpak

	// The preference is saved even when some users failed. applySelfShortcut
	// carries on past a user it cannot write, so a failure still leaves entries
	// behind - and not recording what was asked for would hide those from the
	// background pass for good, with the toggle drawn off over a Steam library
	// that has the app in it.
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return result, errors.Join(applyErr, err)
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return result, errors.Join(applyErr, err)
	}
	app.AddToSteam = enabled
	// Turning it on is what puts entries in other users' lists, so it is also
	// what signs the background pass up to keep every user in line from here on
	// - including taking the entry back out after the option goes off again.
	if enabled {
		app.AddToSteamManaged = true
	}
	return result, errors.Join(applyErr, platform.SaveAppSettings(exeDir, app))
}

// selfShortcutSyncing keeps the account list, which is reloaded on every window
// focus, from stacking up passes.
var selfShortcutSyncing atomic.Bool

// selfShortcutSynced remembers which shortcut files this process has already
// brought in line with the preference, so the repeat passes cost one directory
// listing per Steam install instead of a read and a parse of every user's list.
//
// It does not record which way round that was, and does not need to: the only
// thing that flips the preference mid-process is the toggle, and that runs a
// full pass, which clears this first.
var selfShortcutSynced sync.Map

// SyncSelfShortcutsInBackground brings every local Steam user's shortcut list in
// line with the saved preference, off the caller's goroutine.
//
// Both directions run. On, so an account that signed in after the last pass gets
// the entry too - Steam creates userdata/<id32> at first sign-in, so the set of
// users to write to grows underneath a running app, and the entry would
// otherwise exist for every account except the one just added. Off, so an entry
// a removal could not reach at the time - a Steam install on a drive that was
// not plugged in, a second client that was not installed yet, settings carried
// over from another PC - is cleared the next time the app can see it.
//
// It does nothing until the option has been turned on once. Before that nothing
// of ours is in anyone's list, and reading and rewriting shortcut files for a
// feature the user has never touched is not something to do on the way to
// drawing a page.
//
// This is not a poll: callers hang it off things that only happen after a
// sign-in could have. Users already brought in line are skipped without opening
// their file, so a repeat pass is a handful of stats. The memo starts empty, so
// the first pass of a process is a full one - which is also what re-points every
// entry at the app's new path after an update or a move.
func SyncSelfShortcutsInBackground() {
	if !selfShortcutSyncing.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer selfShortcutSyncing.Store(false)
		exeDir, err := platform.ResolveExeDir()
		if err != nil {
			return
		}
		// Cached in-process, so this is not a settings file read per call.
		app, err := platform.LoadAppSettings(exeDir)
		if err != nil || !app.AddToSteamManaged {
			return
		}
		if _, _, err := applySelfShortcut(app.AddToSteam, skipAlreadySynced); err != nil {
			steamLog().Warn("steam shortcut sync", slog.Any("err", err))
		}
	}()
}

func skipAlreadySynced(path string) bool {
	_, done := selfShortcutSynced.Load(path)
	return done
}

// forgetSyncedSelfShortcuts drops what a full pass is about to redo. Every full
// pass runs for a reason the memo cannot see - the preference flipped, or the
// app moved and every entry now points at the old path.
func forgetSyncedSelfShortcuts() {
	selfShortcutSynced.Range(func(k, _ any) bool {
		selfShortcutSynced.Delete(k)
		return true
	})
}

// applySelfShortcut writes or removes the entry across every Steam install and
// every user of each. It reports how many shortcut files it wrote.
//
// A user whose file cannot be parsed is skipped rather than overwritten: that
// file holds shortcuts we merely failed to read, and replacing it would lose
// every one of them. Such a user is also left out of the synced memo, so the
// next pass tries it again.
//
// skip, when set, drops a user's file before it is opened.
func applySelfShortcut(enabled bool, skip func(path string) bool) (users int, flatpak bool, err error) {
	var failures []error
	if skip == nil {
		forgetSyncedSelfShortcuts()
	}
	for _, root := range selfShortcutRoots() {
		target, err := resolveSelfShortcutTarget(root)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if target.Flatpak {
			flatpak = true
		}
		for _, path := range shortcutsVDFPaths(root) {
			if skip != nil && skip(path) {
				users++
				continue
			}
			list, err := readShortcuts(path)
			if err != nil {
				slog.Warn("skipping unreadable Steam shortcut list", "path", path, "err", err)
				failures = append(failures, err)
				continue
			}
			next, changed := applyToList(list, target, enabled)
			if changed {
				if err := writeShortcuts(path, next); err != nil {
					failures = append(failures, err)
					continue
				}
			}
			selfShortcutSynced.Store(path, struct{}{})
			users++
		}
	}
	return users, flatpak, errors.Join(failures...)
}

func applyToList(list []*shortcutsvdf.Node, target shortcutTarget, enabled bool) ([]*shortcutsvdf.Node, bool) {
	if enabled {
		return upsertSelfShortcut(list, target)
	}
	return removeSelfShortcut(list)
}

// upsertSelfShortcut points our entry at the current target, adding it when it is
// not there yet.
//
// An existing entry keeps its appid even though the target may have moved, since
// Steam files the artwork a user has set under that number and recomputing it
// orphans the lot. Everything else Steam or the user put on the entry - play
// time, tags, controller settings - is left alone for the same reason.
func upsertSelfShortcut(list []*shortcutsvdf.Node, target shortcutTarget) ([]*shortcutsvdf.Node, bool) {
	for _, entry := range list {
		if !isSelfShortcut(entry) {
			continue
		}
		return list, pointEntryAtTarget(entry, target)
	}
	return append(list, newSelfShortcut(target)), true
}

// pointEntryAtTarget updates an existing entry and reports whether anything
// actually changed, so a startup that finds nothing to fix writes nothing.
//
// Only the fields this code owns are touched. AppName, the icon and the launch
// options are all things a user can set from Steam's own properties dialog, and
// the repair pass runs every time the Steam page is opened - rewriting them
// would undo the change every time they made it.
func pointEntryAtTarget(entry *shortcutsvdf.Node, target shortcutTarget) bool {
	changed := false
	set := func(key, want string) {
		if entry.GetString(key) != want {
			entry.SetString(key, want)
			changed = true
		}
	}

	if entry.GetInt32("appid") == 0 {
		entry.SetInt32("appid", shortcutsvdf.ShortcutAppID(target.Exe, SelfShortcutAppName))
		changed = true
	}
	set("Exe", target.Exe)
	set("StartDir", target.StartDir)
	if isManagedIcon(entry.GetString("icon")) {
		set("icon", target.Icon)
	}
	// The Flatpak form keeps the app's real path in LaunchOptions, so there it is
	// ours whatever it currently says.
	if target.Flatpak || isManagedLaunchOptions(entry.GetString("LaunchOptions")) {
		set("LaunchOptions", target.LaunchOptions)
	}
	return changed
}

// isManagedIcon reports an icon this code could have written: blank, one of our
// own binaries (which is what Windows stores), the packaged hicolor PNG, or the
// copy taken out of an AppImage. Anything else is a picture the user chose.
func isManagedIcon(icon string) bool {
	if strings.TrimSpace(icon) == "" {
		return true
	}
	return isSelfBinaryPath(icon) || filepath.Base(icon) == appImageIconName
}

// isManagedLaunchOptions reports launch options this code could have written:
// empty, or the flatpak-spawn form naming one of our binaries.
func isManagedLaunchOptions(opts string) bool {
	opts = strings.TrimSpace(opts)
	if opts == "" {
		return true
	}
	rest := strings.TrimSpace(strings.TrimPrefix(opts, "--host"))
	return rest != opts && isSelfBinaryPath(unquotePath(rest))
}

// removeSelfShortcut drops every entry that launches the switcher.
func removeSelfShortcut(list []*shortcutsvdf.Node) ([]*shortcutsvdf.Node, bool) {
	kept := make([]*shortcutsvdf.Node, 0, len(list))
	for _, entry := range list {
		if isSelfShortcut(entry) {
			continue
		}
		kept = append(kept, entry)
	}
	return kept, len(kept) != len(list)
}

// newSelfShortcut builds an entry in the field order Steam itself writes.
func newSelfShortcut(target shortcutTarget) *shortcutsvdf.Node {
	n := &shortcutsvdf.Node{}
	n.SetInt32("appid", shortcutsvdf.ShortcutAppID(target.Exe, SelfShortcutAppName))
	n.SetString("AppName", SelfShortcutAppName)
	n.SetString("Exe", target.Exe)
	n.SetString("StartDir", target.StartDir)
	n.SetString("icon", target.Icon)
	n.SetString("ShortcutPath", "")
	n.SetString("LaunchOptions", target.LaunchOptions)
	n.SetInt32("IsHidden", 0)
	// Both of these are Steam's own defaults for a shortcut added through its UI,
	// and the overlay is not optional here: in Game Mode it is what puts the
	// on-screen keyboard up, and this app has password fields.
	n.SetInt32("AllowDesktopConfig", 1)
	n.SetInt32("AllowOverlay", 1)
	n.SetInt32("OpenVR", 0)
	n.SetInt32("Devkit", 0)
	n.SetString("DevkitGameID", "")
	n.SetInt32("DevkitOverrideAppID", 0)
	n.SetInt32("LastPlayTime", 0)
	n.SetString("FlatpakAppID", "")
	n.SetMap("tags", &shortcutsvdf.Node{})
	return n
}

// isSelfShortcut recognises our own entry across the two things a user can change
// about it. The appid cannot be used: it is derived from the path, so it changes
// whenever the app moves, which is exactly when we most need to find the entry.
func isSelfShortcut(entry *shortcutsvdf.Node) bool {
	if _, ok := shortcutSelfBinary(entry); ok {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(entry.GetString("AppName")), SelfShortcutAppName)
}

// shortcutSelfBinary returns the app path an entry launches, when it is ours.
func shortcutSelfBinary(entry *shortcutsvdf.Node) (string, bool) {
	exe := unquotePath(entry.GetString("Exe"))
	if isSelfBinaryPath(exe) {
		return exe, true
	}
	// The Flatpak form hides the real target behind flatpak-spawn --host.
	if strings.EqualFold(filepath.Base(exe), "flatpak-spawn") {
		opts := strings.TrimSpace(entry.GetString("LaunchOptions"))
		opts = strings.TrimSpace(strings.TrimPrefix(opts, "--host"))
		if candidate := unquotePath(opts); isSelfBinaryPath(candidate) {
			return candidate, true
		}
	}
	return "", false
}

// matchesTarget reports that our entry is present and already says what a fresh
// write would say, so nothing needs rewriting.
func matchesTarget(list []*shortcutsvdf.Node, target shortcutTarget) bool {
	for _, entry := range list {
		if isSelfShortcut(entry) && entryMatchesTarget(entry, target) {
			return true
		}
	}
	return false
}

// entryMatchesTarget asks only whether the entry launches this copy of the app.
// The icon and the name are not part of the question: a user is free to change
// both, and doing so does not make the shortcut stale.
func entryMatchesTarget(entry *shortcutsvdf.Node, target shortcutTarget) bool {
	if entry.GetString("Exe") != target.Exe || entry.GetString("StartDir") != target.StartDir {
		return false
	}
	return !target.Flatpak || entry.GetString("LaunchOptions") == target.LaunchOptions
}

func unquotePath(p string) string {
	return strings.Trim(strings.TrimSpace(p), `"`)
}

// readShortcuts loads one user's list. A file that is not there yet is an empty
// list, not an error: a user who has never added a non-Steam game has no file.
func readShortcuts(path string) ([]*shortcutsvdf.Node, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return shortcutsvdf.ParseShortcuts(raw)
}

// writeShortcuts replaces one user's list, keeping a copy of what was there.
//
// Both halves matter more here than usual. Steam answers a shortcuts.vdf it
// cannot parse by clearing it, so a half-written file loses every shortcut the
// user has - which is also why the previous contents are kept next to it.
func writeShortcuts(path string, list []*shortcutsvdf.Node) error {
	if previous, err := os.ReadFile(path); err == nil {
		if err := fsutil.WriteFileAtomic(path+".tcnobak", previous, 0o644); err != nil {
			return err
		}
	}
	return fsutil.WriteFileAtomic(path, shortcutsvdf.MarshalShortcuts(list), 0o644)
}

// selfShortcutRoots lists every Steam install that has been signed into, which is
// the same as every install with a userdata folder worth writing to.
//
// It walks the whole candidate list rather than taking ResolveInstallFolder's
// single answer: the native, Flatpak and Snap builds are separate clients under
// separate homes, and someone running two of them wants the entry in both.
func selfShortcutRoots() []string {
	var out []string
	add := func(p string) {
		if p = strings.TrimSpace(p); p == "" {
			return
		}
		// One Linux install answers to three of these paths: ~/.steam/root and
		// ~/.steam/steam are both symlinks to whatever ~/.local/share/Steam is.
		// Without resolving them the same shortcut file gets rewritten - and
		// backed up over - once per alias.
		p = filepath.Clean(p)
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			p = resolved
		}
		if slices.Contains(out, p) || !LoginUsersFileExists(p) {
			return
		}
		out = append(out, p)
	}

	for _, root := range steamRootCandidatesFn() {
		add(root)
	}

	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return out
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return out
	}
	entry, err := platform.ParsePlatformEntry(raw, platformName)
	if err != nil {
		return out
	}
	for _, candidate := range entry.ExeLocationDefault {
		if expanded := platform.ExpandWindowsPath(strings.TrimSpace(candidate)); expanded != "" {
			add(filepath.Dir(expanded))
		}
	}
	return out
}

// shortcutsVDFPaths returns the shortcut list of every user of one Steam install.
func shortcutsVDFPaths(root string) []string {
	var out []string
	for _, id32 := range listNumericSubdirNames(filepath.Join(root, "userdata")) {
		out = append(out, filepath.Join(root, "userdata", id32, "config", "shortcuts.vdf"))
	}
	return out
}

func steamIsRunning() bool {
	for _, name := range steamKillNames {
		if strings.HasPrefix(strings.ToUpper(name), "SERVICE:") {
			continue
		}
		if winutil.IsExeRunning(name) {
			return true
		}
	}
	return false
}
