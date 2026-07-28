package registry

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
)

func useTempRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	paths.ResetForTest(root)
	return root
}

func TestUpsertStatusRemove(t *testing.T) {
	root := useTempRoot(t)
	if err := os.MkdirAll(filepath.Join(root, "SteamGuard"), 0o700); err != nil {
		t.Fatal(err)
	}
	const first = "76561198000000001"
	const second = "76561198000000002"
	if err := Upsert(second, StatePending); err != nil {
		t.Fatal(err)
	}
	if err := Upsert(first, StateActive); err != nil {
		t.Fatal(err)
	}
	if has, pending := Status(first); !has || pending {
		t.Fatalf("active status = %v, %v", has, pending)
	}
	if has, pending := Status(second); has || !pending {
		t.Fatalf("pending status = %v, %v", has, pending)
	}
	entries, err := Load()
	if err != nil || len(entries) != 2 || entries[0].SteamID64 != first {
		t.Fatalf("entries = %#v, err = %v", entries, err)
	}
	if err := Remove(first); err != nil {
		t.Fatal(err)
	}
	if has, pending := Status(first); has || pending {
		t.Fatalf("removed status = %v, %v", has, pending)
	}
	info, err := os.Stat(filepath.Join(root, "SteamGuard", fileName))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("index mode = %o", info.Mode().Perm())
	}
}

func TestLoadRejectsUntrustedHint(t *testing.T) {
	root := useTempRoot(t)
	dir := filepath.Join(root, "SteamGuard")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fileName)
	cases := []string{
		`{"version":1,"entries":[{"steamId64":"76561198000000001","state":"active"},{"steamId64":"76561198000000001","state":"pending"}]}`,
		`{"version":1,"entries":[{"steamId64":"..\\victim","state":"active"}]}`,
		`{"version":1,"entries":[],"extra":true}`,
		`{"version":2,"entries":[]}`,
	}
	for _, data := range cases {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(); !errors.Is(err, ErrInvalidIndex) {
			t.Fatalf("Load(%s) error = %v", data, err)
		}
	}
}

func TestStatusFailsClosed(t *testing.T) {
	root := useTempRoot(t)
	dir := filepath.Join(root, "SteamGuard")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if has, pending := Status("76561198000000001"); has || pending {
		t.Fatalf("invalid hint authorized status: %v, %v", has, pending)
	}
}
