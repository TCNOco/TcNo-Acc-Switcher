package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBGCopyFileRetriesResolvedSourceAndOwnsCopy(t *testing.T) {
	root := t.TempDir()
	actualSource := filepath.Join(root, "network-share", "image.webp")
	if err := os.MkdirAll(filepath.Dir(actualSource), 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte("background image")
	if err := os.WriteFile(actualSource, want, 0o644); err != nil {
		t.Fatal(err)
	}

	unavailableSource := filepath.Join(root, "mapped-drive-no-longer-visible", "image.webp")
	previousResolver := bgResolveSourcePath
	bgResolveSourcePath = func(path string) (string, bool) {
		if path == unavailableSource {
			return actualSource, true
		}
		return "", false
	}
	t.Cleanup(func() { bgResolveSourcePath = previousResolver })

	destination := filepath.Join(root, "app-owned", "app-bg.webp")
	if err := bgCopyFile(unavailableSource, destination); err != nil {
		t.Fatalf("copy background from resolved source: %v", err)
	}
	if err := os.RemoveAll(filepath.Dir(actualSource)); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read app-owned background after source removal: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("saved background = %q, want %q", got, want)
	}
}

func TestBGInstallFileKeepsExistingBackgroundWhenSourceIsUnavailable(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "app-bg.png")
	if err := os.WriteFile(existing, []byte("existing background"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := bgInstallFile(filepath.Join(dir, "missing.webp"), dir, "app-bg", ".webp"); err == nil {
		t.Fatal("bgInstallFile succeeded with an unavailable source")
	}
	got, err := os.ReadFile(existing)
	if err != nil {
		t.Fatalf("read existing background after failed install: %v", err)
	}
	if string(got) != "existing background" {
		t.Fatalf("existing background = %q, want it unchanged", got)
	}
}
