package steam

import (
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/steam/shortcutsvdf"
)

func linuxTarget() shortcutTarget {
	return shortcutTarget{
		Exe:      `"/usr/local/bin/tcno-acc-switcher"`,
		StartDir: `"/usr/local/bin/"`,
		Icon:     "/usr/share/icons/hicolor/256x256/apps/tcno-acc-switcher.png",
		Binary:   "/usr/local/bin/tcno-acc-switcher",
	}
}

// theirShortcut is a shortcut the user made, which must come through every
// operation untouched.
func theirShortcut() *shortcutsvdf.Node {
	n := &shortcutsvdf.Node{}
	n.SetInt32("appid", -12345)
	n.SetString("AppName", "Some Other Game")
	n.SetString("Exe", `"/opt/games/other"`)
	n.SetString("StartDir", `"/opt/games/"`)
	n.SetInt32("LastPlayTime", 1700000000)
	return n
}

func TestUpsertAddsToAnEmptyList(t *testing.T) {
	list, changed := upsertSelfShortcut(nil, linuxTarget())
	if !changed || len(list) != 1 {
		t.Fatalf("changed=%v, %d entries; want a single added entry", changed, len(list))
	}
	if got := list[0].GetString("AppName"); got != SelfShortcutAppName {
		t.Fatalf("AppName = %q, want %q", got, SelfShortcutAppName)
	}
	if got := list[0].GetInt32("appid"); got >= 0 {
		t.Fatalf("appid = %d, want the derived negative id", got)
	}
}

func TestUpsertIsIdempotent(t *testing.T) {
	list, _ := upsertSelfShortcut(nil, linuxTarget())
	again, changed := upsertSelfShortcut(list, linuxTarget())
	if changed {
		t.Fatal("second upsert reported a change, want none")
	}
	if len(again) != 1 {
		t.Fatalf("%d entries after two upserts, want 1", len(again))
	}
}

func TestUpsertRepairsAMovedAppWithoutLosingWhatSteamPutThere(t *testing.T) {
	list, _ := upsertSelfShortcut(nil, linuxTarget())
	ours := list[0]
	appID := ours.GetInt32("appid")
	ours.SetInt32("LastPlayTime", 1700000000)

	moved := linuxTarget()
	moved.Exe = `"/home/u/Applications/tcno-acc-switcher-x86_64.AppImage"`
	moved.StartDir = `"/home/u/Applications/"`
	moved.Binary = "/home/u/Applications/tcno-acc-switcher-x86_64.AppImage"

	list, changed := upsertSelfShortcut(list, moved)
	if !changed || len(list) != 1 {
		t.Fatalf("changed=%v, %d entries; want the one entry repaired in place", changed, len(list))
	}
	if got := list[0].GetString("Exe"); got != moved.Exe {
		t.Fatalf("Exe = %s, want %s", got, moved.Exe)
	}
	// Steam files a user's artwork under the appid, so recomputing it orphans it.
	if got := list[0].GetInt32("appid"); got != appID {
		t.Fatalf("appid = %d, want the original %d", got, appID)
	}
	if got := list[0].GetInt32("LastPlayTime"); got != 1700000000 {
		t.Fatalf("LastPlayTime = %d, want it left alone", got)
	}
}

func TestUpsertReusesAndKeepsAUserRename(t *testing.T) {
	list, _ := upsertSelfShortcut(nil, linuxTarget())
	list[0].SetString("AppName", "Account Switcher")

	moved := linuxTarget()
	moved.Exe = `"/home/u/Applications/tcno-acc-switcher-x86_64.AppImage"`
	moved.StartDir = `"/home/u/Applications/"`

	list, changed := upsertSelfShortcut(list, moved)
	if !changed || len(list) != 1 {
		t.Fatalf("changed=%v, %d entries; want the renamed entry reused, not duplicated", changed, len(list))
	}
	if got := list[0].GetString("AppName"); got != "Account Switcher" {
		t.Fatalf("AppName = %q, want the user's own name kept", got)
	}
}

func TestUpsertFindsOurEntryBehindFlatpakSpawn(t *testing.T) {
	flatpak := linuxTarget()
	flatpak.Exe = `"/usr/bin/flatpak-spawn"`
	flatpak.StartDir = `"/usr/bin/"`
	flatpak.LaunchOptions = `--host "/usr/local/bin/tcno-acc-switcher"`
	flatpak.Flatpak = true

	list, _ := upsertSelfShortcut(nil, flatpak)
	list[0].SetString("AppName", "renamed by the user")

	list, _ = upsertSelfShortcut(list, flatpak)
	if len(list) != 1 {
		t.Fatalf("%d entries, want the flatpak-spawn entry recognised as ours", len(list))
	}
}

func TestUpsertLeavesOtherShortcutsAlone(t *testing.T) {
	theirs := theirShortcut()
	list, _ := upsertSelfShortcut([]*shortcutsvdf.Node{theirs}, linuxTarget())
	if len(list) != 2 {
		t.Fatalf("%d entries, want theirs plus ours", len(list))
	}
	if got := list[0].GetString("AppName"); got != "Some Other Game" {
		t.Fatalf("first entry = %q, want the user's own untouched", got)
	}
	if got := list[0].GetInt32("LastPlayTime"); got != 1700000000 {
		t.Fatalf("their LastPlayTime = %d, want it untouched", got)
	}
}

func TestRemoveDropsOnlyOurs(t *testing.T) {
	list, _ := upsertSelfShortcut([]*shortcutsvdf.Node{theirShortcut()}, linuxTarget())
	list, changed := removeSelfShortcut(list)
	if !changed {
		t.Fatal("remove reported no change, want ours dropped")
	}
	if len(list) != 1 || list[0].GetString("AppName") != "Some Other Game" {
		t.Fatalf("after remove: %d entries, want only the user's own", len(list))
	}
}

func TestRemoveWhenAbsentReportsNoChange(t *testing.T) {
	// So a file we did not modify is never rewritten, and never backed up over.
	list := []*shortcutsvdf.Node{theirShortcut()}
	if _, changed := removeSelfShortcut(list); changed {
		t.Fatal("remove reported a change with nothing of ours present")
	}
}

func TestSelfShortcutRootsCountsOneInstallOnce(t *testing.T) {
	// A Linux Steam answers to three paths: ~/.local/share/Steam, with
	// ~/.steam/root and ~/.steam/steam symlinked to it.
	real := filepath.Join(t.TempDir(), "Steam")
	if err := os.MkdirAll(filepath.Join(real, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "config", "loginusers.vdf"), []byte(`"users"{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "root")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	prev := steamRootCandidatesFn
	steamRootCandidatesFn = func() []string { return []string{real, link, real} }
	t.Cleanup(func() { steamRootCandidatesFn = prev })

	got := selfShortcutRoots()
	if len(got) != 1 {
		t.Fatalf("roots = %v, want the one install once", got)
	}
}

func TestSelfShortcutRootsSkipsAnInstallNeverSignedInto(t *testing.T) {
	// No loginusers.vdf means no userdata folder to write the entry into.
	empty := t.TempDir()
	prev := steamRootCandidatesFn
	steamRootCandidatesFn = func() []string { return []string{empty} }
	t.Cleanup(func() { steamRootCandidatesFn = prev })

	if got := selfShortcutRoots(); len(got) != 0 {
		t.Fatalf("roots = %v, want none", got)
	}
}

func TestUpsertKeepsWhatTheUserSetInSteamsProperties(t *testing.T) {
	// The startup repair runs on every launch, so a customisation it overwrites
	// here is one the user can never make stick.
	list, _ := upsertSelfShortcut(nil, linuxTarget())
	list[0].SetString("LaunchOptions", "mangohud %command%")
	list[0].SetString("icon", "/home/u/Pictures/my-own-icon.png")

	moved := linuxTarget()
	moved.Exe = `"/home/u/Applications/tcno-acc-switcher-x86_64.AppImage"`
	moved.StartDir = `"/home/u/Applications/"`

	list, changed := upsertSelfShortcut(list, moved)
	if !changed {
		t.Fatal("upsert reported no change, want the moved path repaired")
	}
	if got := list[0].GetString("Exe"); got != moved.Exe {
		t.Fatalf("Exe = %s, want the repaired %s", got, moved.Exe)
	}
	if got := list[0].GetString("LaunchOptions"); got != "mangohud %command%" {
		t.Fatalf("LaunchOptions = %q, want the user's own kept", got)
	}
	if got := list[0].GetString("icon"); got != "/home/u/Pictures/my-own-icon.png" {
		t.Fatalf("icon = %q, want the user's own kept", got)
	}
}

func TestUpsertRepairsAnIconItWroteItself(t *testing.T) {
	// Windows stores the exe in the icon field, so a moved app leaves a dead path
	// there - one we put there, so one we may fix.
	list, _ := upsertSelfShortcut(nil, shortcutTarget{
		Exe:      `"C:\Old\TcNo-Acc-Switcher.exe"`,
		StartDir: `"C:\Old\"`,
		Icon:     `C:\Old\TcNo-Acc-Switcher.exe`,
		Binary:   `C:\Old\TcNo-Acc-Switcher.exe`,
	})

	moved := shortcutTarget{
		Exe:      `"C:\New\TcNo-Acc-Switcher.exe"`,
		StartDir: `"C:\New\"`,
		Icon:     `C:\New\TcNo-Acc-Switcher.exe`,
		Binary:   `C:\New\TcNo-Acc-Switcher.exe`,
	}
	list, _ = upsertSelfShortcut(list, moved)
	if got := list[0].GetString("icon"); got != moved.Icon {
		t.Fatalf("icon = %q, want the repaired %q", got, moved.Icon)
	}
}

func TestUpsertStillOwnsLaunchOptionsForFlatpakSteam(t *testing.T) {
	// There the option is not a preference, it is how the app gets launched at all.
	flatpak := linuxTarget()
	flatpak.Exe = `"/usr/bin/flatpak-spawn"`
	flatpak.StartDir = `"/usr/bin/"`
	flatpak.LaunchOptions = `--host "/usr/local/bin/tcno-acc-switcher"`
	flatpak.Flatpak = true

	list, _ := upsertSelfShortcut(nil, flatpak)
	list[0].SetString("LaunchOptions", "mangohud %command%")

	list, changed := upsertSelfShortcut(list, flatpak)
	if !changed {
		t.Fatal("upsert reported no change, want the launch path put back")
	}
	if got := list[0].GetString("LaunchOptions"); got != flatpak.LaunchOptions {
		t.Fatalf("LaunchOptions = %q, want %q", got, flatpak.LaunchOptions)
	}
}

func TestUpsertClearsTheFlatpakFormWhenSteamIsNoLongerFlatpak(t *testing.T) {
	flatpak := linuxTarget()
	flatpak.Exe = `"/usr/bin/flatpak-spawn"`
	flatpak.StartDir = `"/usr/bin/"`
	flatpak.LaunchOptions = `--host "/usr/local/bin/tcno-acc-switcher"`
	flatpak.Flatpak = true
	list, _ := upsertSelfShortcut(nil, flatpak)

	list, _ = upsertSelfShortcut(list, linuxTarget())
	if got := list[0].GetString("LaunchOptions"); got != "" {
		t.Fatalf("LaunchOptions = %q, want the flatpak-spawn form cleared", got)
	}
}

func TestUpsertMakesNoChangeWhenOnlyTheNameAndIconDiffer(t *testing.T) {
	// Otherwise every launch rewrites the file - and its backup - for nothing.
	list, _ := upsertSelfShortcut(nil, linuxTarget())
	list[0].SetString("AppName", "Switcher")
	list[0].SetString("icon", "/home/u/Pictures/my-own-icon.png")

	if _, changed := upsertSelfShortcut(list, linuxTarget()); changed {
		t.Fatal("upsert reported a change, want the file left alone")
	}
}

// steamInstallWithUsers builds a signed-into Steam root holding one userdata
// folder per id.
func steamInstallWithUsers(t *testing.T, ids ...string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "Steam")
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "loginusers.vdf"), []byte(`"users"{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		addSteamUser(t, root, id)
	}
	return root
}

func addSteamUser(t *testing.T, root, id32 string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "userdata", id32, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func hasSelfShortcut(t *testing.T, root, id32 string) bool {
	t.Helper()
	list, err := readShortcuts(filepath.Join(root, "userdata", id32, "config", "shortcuts.vdf"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range list {
		if isSelfShortcut(entry) {
			return true
		}
	}
	return false
}

func TestApplyReachesAUserThatSignedInAfterTheFirstPass(t *testing.T) {
	// Steam creates userdata/<id32> at first sign-in, so a user can appear after
	// the first pass has already walked the list.
	root := steamInstallWithUsers(t, "111")
	prev := steamRootCandidatesFn
	steamRootCandidatesFn = func() []string { return []string{root} }
	t.Cleanup(func() { steamRootCandidatesFn = prev })
	forgetSyncedSelfShortcuts()
	t.Cleanup(forgetSyncedSelfShortcuts)

	if _, _, err := applySelfShortcut(true, nil); err != nil {
		t.Fatal(err)
	}
	addSteamUser(t, root, "222")
	if _, _, err := applySelfShortcut(true, skipAlreadySynced); err != nil {
		t.Fatal(err)
	}

	if !hasSelfShortcut(t, root, "222") {
		t.Error("the user who signed in after the first pass has no entry")
	}
	if !hasSelfShortcut(t, root, "111") {
		t.Error("the user who was already there lost the entry")
	}
}

func TestRepeatPassLeavesSyncedUsersUnopened(t *testing.T) {
	// The trigger is every account list load, which is every window focus.
	root := steamInstallWithUsers(t, "111")
	prev := steamRootCandidatesFn
	steamRootCandidatesFn = func() []string { return []string{root} }
	t.Cleanup(func() { steamRootCandidatesFn = prev })
	forgetSyncedSelfShortcuts()
	t.Cleanup(forgetSyncedSelfShortcuts)

	if _, _, err := applySelfShortcut(true, nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "userdata", "111", "config", "shortcuts.vdf")
	sentinel := []byte("not a shortcut list at all")
	if err := os.WriteFile(path, sentinel, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := applySelfShortcut(true, skipAlreadySynced); err != nil {
		t.Fatalf("a skipped user must not even be parsed: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(sentinel) {
		t.Error("a user already synced was read and rewritten")
	}
}

func TestFullPassRedoesUsersAMemoisedPassWouldSkip(t *testing.T) {
	// Toggling the option or moving the app has to reach users an earlier pass
	// recorded as done.
	root := steamInstallWithUsers(t, "111")
	prev := steamRootCandidatesFn
	steamRootCandidatesFn = func() []string { return []string{root} }
	t.Cleanup(func() { steamRootCandidatesFn = prev })
	forgetSyncedSelfShortcuts()
	t.Cleanup(forgetSyncedSelfShortcuts)

	if _, _, err := applySelfShortcut(true, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := applySelfShortcut(false, nil); err != nil {
		t.Fatal(err)
	}

	if hasSelfShortcut(t, root, "111") {
		t.Error("turning the option off left the entry behind")
	}
}

func TestRemovalPassReachesAUserTheMemoNeverSaw(t *testing.T) {
	// An entry outlives the option being turned off whenever the removal could not
	// reach it, so the pass that runs when the Steam page opens starts with an
	// empty memo.
	root := steamInstallWithUsers(t, "111")
	prev := steamRootCandidatesFn
	steamRootCandidatesFn = func() []string { return []string{root} }
	t.Cleanup(func() { steamRootCandidatesFn = prev })
	forgetSyncedSelfShortcuts()
	t.Cleanup(forgetSyncedSelfShortcuts)

	if _, _, err := applySelfShortcut(true, nil); err != nil {
		t.Fatal(err)
	}
	forgetSyncedSelfShortcuts()

	if _, _, err := applySelfShortcut(false, skipAlreadySynced); err != nil {
		t.Fatal(err)
	}
	if hasSelfShortcut(t, root, "111") {
		t.Error("an entry left over from an earlier run survived the pass")
	}
}
