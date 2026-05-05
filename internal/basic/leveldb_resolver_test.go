package basic

import (
	"path/filepath"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/platform"

	"github.com/syndtr/goleveldb/leveldb"
)

func TestParseLevelDBReference(t *testing.T) {
	ref, err := parseLevelDBReference(`leveldb:C:\tmp\leveldb:_https://discord.comStore:_state.users.0.id`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if ref.Path != `C:\tmp\leveldb` {
		t.Fatalf("unexpected path: %q", ref.Path)
	}
	if ref.Key != `_https://discord.comStore` {
		t.Fatalf("unexpected key: %q", ref.Key)
	}
	if ref.JSONPath != `_state.users.0.id` {
		t.Fatalf("unexpected json path: %q", ref.JSONPath)
	}
}

func TestResolveLevelDBReference_WithAndWithoutJSONPath(t *testing.T) {
	defer sharedLevelDBStore.closeAll()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ldb")
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		t.Fatalf("open leveldb: %v", err)
	}
	raw := `{"_state":{"users":[{"id":"123","avatar":"abc"}]}}`
	if err := db.Put([]byte("_https://ptb.discord.comMultiAccountStore"), []byte(raw), nil); err != nil {
		t.Fatalf("put: %v", err)
	}
	_ = db.Close()

	full, err := resolveLevelDBReference("leveldb:"+dbPath+":_https://ptb.discord.comMultiAccountStore", platform.PathTokenContext{})
	if err != nil {
		t.Fatalf("resolve full value: %v", err)
	}
	if full != raw {
		t.Fatalf("unexpected full value: %q", full)
	}

	uid, err := resolveLevelDBReference("leveldb:"+dbPath+":_https://ptb.discord.comMultiAccountStore:_state.users.0.id", platform.PathTokenContext{})
	if err != nil {
		t.Fatalf("resolve path value: %v", err)
	}
	if uid != "123" {
		t.Fatalf("unexpected id: %q", uid)
	}
}

func TestExpandDescriptorVariables(t *testing.T) {
	got := expandDescriptorVariables("https://cdn.discordapp.com/avatars/%userid%/%useravatar%.webp", map[string]string{
		"userid":     "123",
		"useravatar": "abc",
	})
	want := "https://cdn.discordapp.com/avatars/123/abc.webp"
	if got != want {
		t.Fatalf("unexpected expanded value: got %q want %q", got, want)
	}
}

func TestLevelDBStoreCloseIdle(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ldb")
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		t.Fatalf("open leveldb: %v", err)
	}
	if err := db.Put([]byte("k"), []byte("v"), nil); err != nil {
		t.Fatalf("put: %v", err)
	}
	_ = db.Close()

	s := &levelDBStore{handles: map[string]*levelDBHandle{}}
	if _, err := s.readValue(dbPath, "k", ""); err != nil {
		t.Fatalf("read value: %v", err)
	}
	if len(s.handles) != 1 {
		t.Fatalf("expected 1 open handle, got %d", len(s.handles))
	}

	s.mu.Lock()
	for _, h := range s.handles {
		h.lastAccess = time.Now().Add(-10 * time.Minute)
	}
	s.mu.Unlock()

	s.closeIdle(2 * time.Minute)
	if len(s.handles) != 0 {
		t.Fatalf("expected handles to close, got %d", len(s.handles))
	}
}

func TestReadUniqueID_LEVELDB(t *testing.T) {
	defer sharedLevelDBStore.closeAll()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ldb")
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		t.Fatalf("open leveldb: %v", err)
	}
	if err := db.Put([]byte("mykey"), []byte(`{"id":"u-42"}`), nil); err != nil {
		t.Fatalf("put: %v", err)
	}
	_ = db.Close()

	d := platform.Descriptor{
		UniqueIdMethod: "LEVELDB",
		UniqueIdFile:   "leveldb:" + dbPath + ":mykey:id",
	}
	got, err := ReadUniqueID("Discord PTB", d, "")
	if err != nil {
		t.Fatalf("ReadUniqueID: %v", err)
	}
	if got != "u-42" {
		t.Fatalf("unexpected unique id: %q", got)
	}
}
