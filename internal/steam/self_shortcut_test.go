package steam

import (
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
	// Steam files a user's artwork under the appid, so recomputing it would
	// orphan whatever they set.
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
