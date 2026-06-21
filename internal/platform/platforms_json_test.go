package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
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
