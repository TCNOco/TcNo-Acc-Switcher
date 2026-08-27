package steam

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"

	"github.com/ulikunitz/xz"
)

func validSteamAppArrayJSON() []byte {
	return []byte(`{"730":"Counter-Strike 2","440":"Team Fortress 2"}`)
}

func TestSteamAppNameMapCacheExpired(t *testing.T) {
	dir := t.TempDir()
	paths.ResetForTest(dir)

	cachePath, err := appIdsUserPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsutil.WriteFileAtomic(cachePath, validSteamAppArrayJSON(), 0o644); err != nil {
		t.Fatal(err)
	}

	if steamAppNameMapCacheExpired() {
		t.Fatal("expected fresh cache not to be expired")
	}

	old := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(cachePath, old, old); err != nil {
		t.Fatal(err)
	}
	if !steamAppNameMapCacheExpired() {
		t.Fatal("expected old cache to be expired")
	}
}

func compressXZForTest(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := xz.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDecompressXZSteamAppNameMap(t *testing.T) {
	raw := validSteamAppArrayJSON()
	compressed := compressXZForTest(t, raw)

	got, err := decompressXZSteamAppNameMap(compressed)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatalf("decompressed payload mismatch")
	}
	m, err := parseAppNameMapJSON(got)
	if err != nil {
		t.Fatal(err)
	}
	if m["730"] != "Counter-Strike 2" {
		t.Fatalf("unexpected parsed name: %q", m["730"])
	}
}

func TestGetSteamAppNameMapCachedLoadsMemory(t *testing.T) {
	dir := t.TempDir()
	paths.ResetForTest(dir)

	steamAppNameMapMu.Lock()
	steamAppNameMapMem = nil
	steamAppNameMapMu.Unlock()

	cachePath, err := appIdsUserPath()
	if err != nil {
		t.Fatal(err)
	}
	raw := validSteamAppArrayJSON()
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsutil.WriteFileAtomic(cachePath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := getSteamAppNameMapCached()
	if err != nil {
		t.Fatal(err)
	}
	if got["730"] != "Counter-Strike 2" {
		t.Fatalf("cached map mismatch: %q", got["730"])
	}

	steamAppNameMapMu.RLock()
	mem := steamAppNameMapMem
	steamAppNameMapMu.RUnlock()
	if mem["730"] != "Counter-Strike 2" {
		t.Fatalf("memory cache was not populated")
	}
}

func writeManifest(t *testing.T, dir, appID, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "\"AppState\"\n{\n\t\"appid\"\t\t\"" + appID + "\"\n"
	if name != "" {
		body += "\t\"name\"\t\t\"" + name + "\"\n"
	}
	body += "\t\"installdir\"\t\t\"whatever\"\n}\n"
	path := filepath.Join(dir, "appmanifest_"+appID+".acf")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInstalledAppIDsReadsManifestNames(t *testing.T) {
	root := t.TempDir()
	apps := filepath.Join(root, "steamapps")
	writeManifest(t, apps, "228980", "Steamworks Common Redistributables")
	writeManifest(t, apps, "730", "Counter-Strike 2")
	writeManifest(t, apps, "999999", "")

	got, err := installedAppIDs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 installed apps, got %d (%v)", len(got), got)
	}
	if got["228980"] != "Steamworks Common Redistributables" {
		t.Fatalf("manifest name not read: %q", got["228980"])
	}
	if got["999999"] != "" {
		t.Fatalf("nameless manifest should yield an empty name, got %q", got["999999"])
	}
}

// The manifest is preferred over the catalogue map because the map is built from
// store listings and goes stale the moment an app stops being sellable.
func TestNamedInstalledGamesPrefersManifestName(t *testing.T) {
	installed := map[string]string{
		"271590": "Grand Theft Auto V Legacy", // renamed after the map was built
		"730":    "",                          // no manifest name; map answers
		"584210": "",                          // neither source knows it
	}
	names := map[string]string{
		"271590": "Grand Theft Auto V",
		"730":    "Counter-Strike 2",
	}

	got := map[string]string{}
	for _, g := range namedInstalledGames(installed, names, nil) {
		got[g.AppID] = g.Name
	}
	if got["271590"] != "Grand Theft Auto V Legacy" {
		t.Fatalf("manifest name should win over the map: %q", got["271590"])
	}
	if got["730"] != "Counter-Strike 2" {
		t.Fatalf("map should fill in when the manifest has no name: %q", got["730"])
	}
	if got["584210"] != "App 584210" {
		t.Fatalf("unknown app should fall back to the id: %q", got["584210"])
	}
}

// A game installed in a second library folder must not blank the name read from
// the first.
func TestInstalledAppIDsKeepsNameAcrossLibraryFolders(t *testing.T) {
	root := t.TempDir()
	second := filepath.Join(t.TempDir(), "Library")
	apps := filepath.Join(root, "steamapps")
	if err := os.MkdirAll(apps, 0o755); err != nil {
		t.Fatal(err)
	}
	vdf := "\"libraryfolders\"\n{\n\t\"1\"\n\t{\n\t\t\"path\"\t\t\"" +
		strings.ReplaceAll(second, `\`, `\`) + "\"\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(apps, "libraryfolders.vdf"), []byte(vdf), 0o644); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, apps, "440", "Team Fortress 2")
	writeManifest(t, filepath.Join(second, "steamapps"), "440", "")

	got, err := installedAppIDs(root)
	if err != nil {
		t.Fatal(err)
	}
	if got["440"] != "Team Fortress 2" {
		t.Fatalf("name lost to the nameless duplicate: %q", got["440"])
	}
}
