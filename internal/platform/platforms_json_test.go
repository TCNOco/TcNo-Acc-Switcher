package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestMergePlatformsJSON_addAndReplace(t *testing.T) {
	t.Parallel()
	base := []byte(`{"Platforms":{"A":{"x":1},"B":{"y":2}}}`)
	over := []byte(`{"Platforms":{"B":{"z":3},"C":{"w":4}}}`)
	out, err := mergePlatformsJSON(base, over)
	if err != nil {
		t.Fatal(err)
	}
	var got platformsFile
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Platforms) != 3 {
		t.Fatalf("expected 3 platforms, got %d", len(got.Platforms))
	}
	if string(got.Platforms["A"]) != `{"x":1}` {
		t.Fatalf("A: %s", string(got.Platforms["A"]))
	}
	if string(got.Platforms["B"]) != `{"z":3}` {
		t.Fatalf("B should be replaced: %s", string(got.Platforms["B"]))
	}
	if string(got.Platforms["C"]) != `{"w":4}` {
		t.Fatalf("C: %s", string(got.Platforms["C"]))
	}
}

func TestLoadPlatformsJSON_mergesCustom(t *testing.T) {
	dir := t.TempDir()
	prev := append([]byte(nil), embeddedPlatformsJSON...)
	defer SetEmbeddedPlatformsJSON(prev)
	SetEmbeddedPlatformsJSON([]byte(`{"Platforms":{"Steam":{"Identifiers":["s"]}}}`))

	if err := os.MkdirAll(PortableUserDataDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteBytes(filepath.Join(PortableUserDataDir(dir), settingsFileName), []byte(`{"version":1,"language":"en-US"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ud := UserDataDir(dir)
	if err := atomicWriteBytes(filepath.Join(ud, "Platforms.custom.json"), []byte(`{"Platforms":{"Epic Games":{"Identifiers":["e"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	raw, err := LoadPlatformsJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	names, err := parsePlatformNames(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("names: %v", names)
	}
}

// loadCatalogWithEmbedded seeds a user-data catalog and settings file, swaps the
// embedded catalog, and returns the platform names LoadPlatformsJSON resolves.
func loadCatalogWithEmbedded(t *testing.T, localCatalog, embedded string) []string {
	t.Helper()
	setTestAppData(t)
	dir := t.TempDir()
	userDataDir := PortableUserDataDir(dir)
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteBytes(filepath.Join(userDataDir, settingsFileName), []byte(`{"version":1,"language":"en-US"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteBytes(filepath.Join(userDataDir, "Platforms.json"), []byte(localCatalog), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := append([]byte(nil), embeddedPlatformsJSON...)
	t.Cleanup(func() { SetEmbeddedPlatformsJSON(previous) })
	SetEmbeddedPlatformsJSON([]byte(embedded))

	ResetPathSingletonsForTest(dir)
	raw, err := LoadPlatformsJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	names, err := parsePlatformNames(raw)
	if err != nil {
		t.Fatal(err)
	}
	return names
}

// The C# releases wrote a date-tagged catalog to the same path v4 uses. Without
// this, an in-place upgrade keeps a pre-rewrite catalog forever and no
// descriptor fix ever reaches the user.
func TestLoadPlatformsJSON_supersedesLegacyDateTaggedCatalog(t *testing.T) {
	names := loadCatalogWithEmbedded(t,
		`{"Version":"2025-11-09_00","Platforms":{"Epic Games":{"Identifiers":["e"]}}}`,
		`{"Version":"4.0.4","Platforms":{"Steam":{"Identifiers":["s","steam"]}}}`)

	if slices.Contains(names, "Epic Games") {
		t.Errorf("legacy catalog survived instead of being superseded: %v", names)
	}
	if !slices.Contains(names, "Steam") {
		t.Errorf("embedded catalog was not seeded: %v", names)
	}
}

// A catalog with no Version at all is also pre-rewrite and must be replaced.
func TestLoadPlatformsJSON_supersedesVersionlessCatalog(t *testing.T) {
	names := loadCatalogWithEmbedded(t,
		`{"Platforms":{"Epic Games":{"Identifiers":["e"]}}}`,
		`{"Version":"4.0.4","Platforms":{"Steam":{"Identifiers":["s","steam"]}}}`)

	if slices.Contains(names, "Epic Games") {
		t.Errorf("version-less catalog survived instead of being superseded: %v", names)
	}
}

// A catalog that is already current must not be clobbered, or a user who
// applied their own via the UI loses it on every launch.
func TestLoadPlatformsJSON_keepsCurrentCatalog(t *testing.T) {
	names := loadCatalogWithEmbedded(t,
		`{"Version":"4.0.4","Platforms":{"Steam":{"Identifiers":["s"]},"Epic Games":{"Identifiers":["e"]}}}`,
		`{"Version":"4.0.4","Platforms":{"Steam":{"Identifiers":["s","steam"]}}}`)

	if !slices.Contains(names, "Epic Games") {
		t.Errorf("a current catalog was overwritten: %v", names)
	}
}
