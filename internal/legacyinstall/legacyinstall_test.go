package legacyinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newLegacyDir builds a directory that looks like a C# install: one marker plus
// whatever extra names the test needs.
func newLegacyDir(t *testing.T, extra ...string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "TcNo-Acc-Switcher-Server.exe"), 16)
	for _, name := range extra {
		write(t, filepath.Join(dir, filepath.FromSlash(name)), 8)
	}
	return dir
}

func names(rep Report) map[string]bool {
	out := map[string]bool{}
	for _, e := range rep.Entries {
		out[e.Name] = true
	}
	return out
}

func TestDetectWithoutMarkerReportsNothing(t *testing.T) {
	dir := t.TempDir()
	// Names from the manifest, but nothing that identifies a C# install.
	write(t, filepath.Join(dir, "Newtonsoft.Json.dll"), 8)
	write(t, filepath.Join(dir, "appsettings.json"), 8)

	rep := Detect(dir)
	if rep.Found() || rep.Count() != 0 {
		t.Fatalf("expected no report without a marker, got %d entries", rep.Count())
	}
}

func TestDetectListsOnlyManifestNames(t *testing.T) {
	dir := newLegacyDir(t, "SteamKit2.dll", "GameStats.json", "notes.txt", "MyOwnTool.exe")

	rep := Detect(dir)
	if !rep.Found() {
		t.Fatal("expected a report")
	}
	got := names(rep)
	for _, want := range []string{"TcNo-Acc-Switcher-Server.exe", "SteamKit2.dll", "GameStats.json"} {
		if !got[want] {
			t.Errorf("expected %q in report", want)
		}
	}
	for _, unwanted := range []string{"notes.txt", "MyOwnTool.exe"} {
		if got[unwanted] {
			t.Errorf("unrelated file %q must not be listed", unwanted)
		}
	}
}

func TestDetectKeepsFilesTheNewBuildOwns(t *testing.T) {
	dir := newLegacyDir(t,
		"TcNo-Acc-Switcher.exe",
		"OPEN_SOURCE_LICENSES.txt",
		"Uninstall TcNo Account Switcher.exe",
		"TcNo Account Switcher/Settings/settings.json",
	)

	got := names(Detect(dir))
	for _, keep := range keepNames {
		if got[keep] {
			t.Errorf("%q belongs to the current build and must never be listed", keep)
		}
	}
}

func TestDetectRequiresDirectorySignature(t *testing.T) {
	dir := newLegacyDir(t, "themes/MyTheme/theme.json", "updater/notes.txt")

	got := names(Detect(dir))
	if got["themes"] || got["updater"] {
		t.Error("folders without a C# signature must be left alone")
	}

	write(t, filepath.Join(dir, "themes", "Nord", "style.scss"), 4)
	write(t, filepath.Join(dir, "updater", "TcNo-Acc-Switcher-Updater.exe"), 4)

	got = names(Detect(dir))
	if !got["themes"] || !got["updater"] {
		t.Error("folders carrying a C# signature should be listed")
	}
}

func TestDetectSumsSizes(t *testing.T) {
	dir := newLegacyDir(t)
	write(t, filepath.Join(dir, "SteamKit2.dll"), 1000)
	write(t, filepath.Join(dir, "x64", "7z.dll"), 500)

	rep := Detect(dir)
	if rep.Bytes != 16+1000+500 {
		t.Fatalf("total bytes = %d, want %d", rep.Bytes, 16+1000+500)
	}
}

func TestRemoveDeletesEverythingReported(t *testing.T) {
	dir := newLegacyDir(t, "SteamKit2.dll", "x64/7z.dll", "keep-me.txt")

	rep := Detect(dir)
	res := Remove(rep)
	if !res.Ok() {
		t.Fatalf("removal failed: %+v", res.Failed)
	}
	if len(res.Removed) != rep.Count() {
		t.Fatalf("removed %d of %d entries", len(res.Removed), rep.Count())
	}
	for _, e := range rep.Entries {
		if _, err := os.Stat(e.Path); !os.IsNotExist(err) {
			t.Errorf("%s still exists", e.Path)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "keep-me.txt")); err != nil {
		t.Errorf("unrelated file was removed: %v", err)
	}
}

func TestRemoveSetsAsideEditableFilesInsteadOfDeleting(t *testing.T) {
	dir := newLegacyDir(t)
	for _, name := range []string{"Platforms.json", "GameStats.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`{"mine":"`+name+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		// An earlier pass already left one behind; it should be replaced, not block.
		write(t, filepath.Join(dir, name+BackupSuffix), 4)
	}

	res := Remove(Detect(dir))
	if !res.Ok() {
		t.Fatalf("removal failed: %+v", res.Failed)
	}

	for _, name := range []string{"Platforms.json", "GameStats.json"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s should have been renamed away", name)
		}
		data, err := os.ReadFile(path + BackupSuffix)
		if err != nil {
			t.Fatalf("backup for %s missing: %v", name, err)
		}
		if !strings.Contains(string(data), name) {
			t.Errorf("backup for %s holds the wrong content: %s", name, data)
		}
		for _, p := range res.Removed {
			if p == path {
				t.Errorf("%s is preserved and must not be reported as removed", name)
			}
		}
	}
	if len(res.Preserved) != 2 {
		t.Errorf("preserved = %v, want both catalogs", res.Preserved)
	}
}

func TestRemoveRejectsEntriesOutsideTheManifest(t *testing.T) {
	dir := newLegacyDir(t)
	outside := filepath.Join(t.TempDir(), "important.txt")
	write(t, outside, 8)
	sibling := filepath.Join(dir, "important.txt")
	write(t, sibling, 8)

	rep := Detect(dir)
	// A Report that has been tampered with: a path outside the scanned folder,
	// and a name the C# release never shipped.
	rep.Entries = append(rep.Entries,
		Entry{Name: "SteamKit2.dll", Path: outside},
		Entry{Name: "important.txt", Path: sibling},
	)

	Remove(rep)
	for _, path := range []string{outside, sibling} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s was removed but is not in the manifest", path)
		}
	}
}

func TestRemovedBetweenReportsSurvivors(t *testing.T) {
	before := Report{
		ExeDir: `C:\app`,
		Entries: []Entry{
			{Name: "a.dll", Path: `C:\app\a.dll`, Bytes: 10},
			{Name: "b.dll", Path: `C:\app\b.dll`, Bytes: 20},
		},
	}
	after := Report{ExeDir: `C:\app`, Entries: []Entry{{Name: "b.dll", Path: `C:\app\b.dll`, Bytes: 20}}}

	res := removedBetween(before, after)
	if len(res.Removed) != 1 || res.Removed[0] != `C:\app\a.dll` {
		t.Fatalf("removed = %v", res.Removed)
	}
	if res.Bytes != 10 {
		t.Fatalf("freed = %d, want 10", res.Bytes)
	}
	if len(res.Failed) != 1 || res.Failed[0].Path != `C:\app\b.dll` {
		t.Fatalf("failed = %+v", res.Failed)
	}
}
