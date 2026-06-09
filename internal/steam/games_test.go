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

func validSteamAppListJSON() []byte {
	return []byte(`{"applist":{"apps":[{"appid":730,"name":"Counter-Strike 2"}]}}`)
}

func TestSteamAppListCacheExpired(t *testing.T) {
	dir := t.TempDir()
	paths.ResetForTest(dir)

	cachePath, err := appIdsFullCachePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsutil.WriteFileAtomic(cachePath, validSteamAppListJSON(), 0o644); err != nil {
		t.Fatal(err)
	}

	if steamAppListCacheExpired() {
		t.Fatal("expected fresh cache not to be expired")
	}

	old := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(cachePath, old, old); err != nil {
		t.Fatal(err)
	}
	if !steamAppListCacheExpired() {
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

func TestDecompressXZSteamAppList(t *testing.T) {
	raw := validSteamAppListJSON()
	compressed := compressXZForTest(t, raw)

	got, err := decompressXZSteamAppList(compressed)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatalf("decompressed payload mismatch")
	}
	if !steamAppListJSONLooksValid(got) {
		t.Fatal("decompressed payload is not valid app list JSON")
	}
}

func TestGetSteamAppListCachedLoadsMemory(t *testing.T) {
	dir := t.TempDir()
	paths.ResetForTest(dir)

	steamAppListMu.Lock()
	steamAppListData = nil
	steamAppListMu.Unlock()

	cachePath, err := appIdsFullCachePath()
	if err != nil {
		t.Fatal(err)
	}
	raw := validSteamAppListJSON()
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsutil.WriteFileAtomic(cachePath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := getSteamAppListCached()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatalf("cached bytes mismatch")
	}

	steamAppListMu.RLock()
	mem := steamAppListData
	steamAppListMu.RUnlock()
	if string(mem) != string(raw) {
		t.Fatalf("memory cache was not populated")
	}
}
