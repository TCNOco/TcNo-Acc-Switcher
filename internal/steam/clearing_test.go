package steam

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClearDirectoryContents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "testdir")
	os.MkdirAll(filepath.Join(target, "sub"), 0o755)
	os.WriteFile(filepath.Join(target, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(target, "sub", "b.txt"), []byte("b"), 0o644)

	var lines []string
	clearDirectoryContents(target, func(s string) { lines = append(lines, s) }, "testdir")

	if _, err := os.Stat(filepath.Join(target, "a.txt")); err == nil {
		t.Error("a.txt should be deleted")
	}
	if _, err := os.Stat(filepath.Join(target, "sub", "b.txt")); err == nil {
		t.Error("sub/b.txt should be deleted")
	}
	if _, err := os.Stat(target); err != nil {
		t.Error("testdir itself should remain")
	}
}

func TestClearDirectoryContents_Missing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var lines []string
	clearDirectoryContents(filepath.Join(dir, "nonexistent"), func(s string) { lines = append(lines, s) }, "nonexistent")
}

func TestClearTopLevelGlob(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "debug.log"), []byte("log1"), 0o644)
	os.WriteFile(filepath.Join(dir, "error.log"), []byte("log2"), 0o644)
	os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("keep"), 0o644)

	var lines []string
	clearTopLevelGlob(dir, []string{"*.log"}, func(s string) { lines = append(lines, s) }, "logs")

	if _, err := os.Stat(filepath.Join(dir, "debug.log")); err == nil {
		t.Error("debug.log should be deleted")
	}
	if _, err := os.Stat(filepath.Join(dir, "error.log")); err == nil {
		t.Error("error.log should be deleted")
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Error("keep.txt should remain")
	}
}

func TestClearTopLevelAllFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "appcache")
	os.MkdirAll(filepath.Join(target, "subdir"), 0o755)
	os.WriteFile(filepath.Join(target, "top.dat"), []byte("top"), 0o644)
	os.WriteFile(filepath.Join(target, "subdir", "nested.dat"), []byte("nested"), 0o644)

	var lines []string
	clearTopLevelAllFiles(target, func(s string) { lines = append(lines, s) }, "appcache")

	if _, err := os.Stat(filepath.Join(target, "top.dat")); err == nil {
		t.Error("top.dat should be deleted")
	}
	if _, err := os.Stat(filepath.Join(target, "subdir", "nested.dat")); err != nil {
		t.Error("nested.dat should remain (non-recursive)")
	}
}

func TestClearAllFilesRecursive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "httpcache")
	os.MkdirAll(filepath.Join(target, "a", "b"), 0o755)
	os.WriteFile(filepath.Join(target, "top.dat"), []byte("top"), 0o644)
	os.WriteFile(filepath.Join(target, "a", "mid.dat"), []byte("mid"), 0o644)
	os.WriteFile(filepath.Join(target, "a", "b", "deep.dat"), []byte("deep"), 0o644)

	var lines []string
	clearAllFilesRecursive(target, func(s string) { lines = append(lines, s) }, "httpcache")

	if _, err := os.Stat(filepath.Join(target, "top.dat")); err == nil {
		t.Error("top.dat should be deleted")
	}
	if _, err := os.Stat(filepath.Join(target, "a", "mid.dat")); err == nil {
		t.Error("mid.dat should be deleted")
	}
	if _, err := os.Stat(filepath.Join(target, "a", "b", "deep.dat")); err == nil {
		t.Error("deep.dat should be deleted")
	}
	if _, err := os.Stat(filepath.Join(target, "a")); err != nil {
		t.Error("dir a should remain")
	}
}

func TestClearAllFilesRecursive_Missing(t *testing.T) {
	t.Parallel()
	var lines []string
	clearAllFilesRecursive(filepath.Join(t.TempDir(), "nonexistent"), func(s string) { lines = append(lines, s) }, "nonexistent")
}

func TestTryRemoveFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "to_remove.vdf")
	os.WriteFile(p, []byte("data"), 0o644)

	var lines []string
	tryRemoveFile(p, func(s string) { lines = append(lines, s) }, "test.vdf")

	if _, err := os.Stat(p); err == nil {
		t.Error("file should be removed")
	}
}

func TestTryRemoveFile_Missing(t *testing.T) {
	t.Parallel()
	var lines []string
	tryRemoveFile(filepath.Join(t.TempDir(), "no_such_file"), func(s string) { lines = append(lines, s) }, "missing")
}

func TestClearSSFNFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ssfn12345"), []byte("token1"), 0o644)
	os.WriteFile(filepath.Join(dir, "ssfn67890"), []byte("token2"), 0o644)
	os.WriteFile(filepath.Join(dir, "regular.txt"), []byte("keep"), 0o644)
	os.WriteFile(filepath.Join(dir, "xssfn"), []byte("keep"), 0o644)

	var lines []string
	clearSSFNFiles(dir, func(s string) { lines = append(lines, s) })

	if _, err := os.Stat(filepath.Join(dir, "ssfn12345")); err == nil {
		t.Error("ssfn12345 should be deleted")
	}
	if _, err := os.Stat(filepath.Join(dir, "ssfn67890")); err == nil {
		t.Error("ssfn67890 should be deleted")
	}
	if _, err := os.Stat(filepath.Join(dir, "regular.txt")); err != nil {
		t.Error("regular.txt should remain")
	}
	if _, err := os.Stat(filepath.Join(dir, "xssfn")); err != nil {
		t.Error("xssfn should remain (prefix match only)")
	}
}

func TestAdvancedClearingItems(t *testing.T) {
	t.Parallel()
	svc := &SteamService{}
	items, err := svc.AdvancedClearingItems()
	if err != nil {
		t.Fatalf("AdvancedClearingItems: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected non-empty items list")
	}

	ids := map[string]bool{}
	for _, item := range items {
		if item.ID == "" {
			t.Error("item with empty ID")
		}
		if ids[item.ID] {
			t.Errorf("duplicate item ID: %q", item.ID)
		}
		ids[item.ID] = true
	}
}
