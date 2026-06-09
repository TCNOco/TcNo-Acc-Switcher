package steam

import (
	"bytes"
	"os"
	"path/filepath"
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
