package shortcuts

import (
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/winutil"
)

// findRealLnk returns a .lnk from the Start Menu, so the benchmark measures the
// shell resolve the cache avoids rather than a stand-in.
func findRealLnk(tb testing.TB) string {
	tb.Helper()
	for _, root := range []string{os.Getenv("APPDATA"), os.Getenv("ProgramData")} {
		if root == "" {
			continue
		}
		dir := filepath.Join(root, "Microsoft", "Windows", "Start Menu", "Programs")
		var found string
		_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil || found != "" {
				return nil
			}
			if !d.IsDir() && filepath.Ext(p) == ".lnk" {
				found = p
			}
			return nil
		})
		if found != "" {
			return found
		}
	}
	tb.Skip("no .lnk found in the Start Menu")
	return ""
}

// BenchmarkShortcutTargetResolve is what buildDTOs pays per shortcut. The cache
// replaces the shell resolve with the stat that validates it.
func BenchmarkShortcutTargetResolve(b *testing.B) {
	lnk := findRealLnk(b)

	b.Run("ShellResolve", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, _, _, err := winutil.ReadLnkShortcut(lnk); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("StatOnly", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := os.Stat(lnk); err != nil {
				b.Fatal(err)
			}
		}
	})
}
