package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDestinationFromPicker(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{`C:\Users\me\AppData\Roaming`, filepath.Join(`C:\Users\me\AppData\Roaming`, UserDataDirName)},
		{filepath.Join(`D:\`, UserDataDirName), filepath.Join(`D:\`, UserDataDirName)},
		{"", ""},
	}
	for _, tc := range cases {
		got := ResolveDestinationFromPicker(tc.in)
		if got != tc.want {
			t.Fatalf("ResolveDestinationFromPicker(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveUserDataDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	exeDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	custom := filepath.Join(dir, "custom", UserDataDirName)
	s := AppSettings{UserDataPath: custom}
	got, err := ResolveUserDataDir(exeDir, s)
	if err != nil {
		t.Fatal(err)
	}
	if got != custom {
		t.Fatalf("settings path: got %q want %q", got, custom)
	}

	portable := PortableUserDataDir(exeDir)
	if err := os.MkdirAll(portable, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveUserDataDir(exeDir, AppSettings{})
	if err != nil {
		t.Fatal(err)
	}
	if got != portable {
		t.Fatalf("portable detect: got %q want %q", got, portable)
	}
}

func TestInitDataPathsMigratesLegacyStatistics(t *testing.T) {
	dir := t.TempDir()
	exeDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	portable := PortableUserDataDir(exeDir)
	if err := os.MkdirAll(portable, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(exeDir, "Statistics.json")
	if err := os.WriteFile(legacy, []byte(`{"Uuid":"legacy-uuid"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	ResetPathSingletonsForTest(exeDir)
	if err := InitDataPaths(exeDir); err != nil {
		t.Fatal(err)
	}
	migrated := filepath.Join(portable, "Statistics.json")
	if _, err := os.Stat(migrated); err != nil {
		t.Fatalf("expected migrated stats at %s: %v", migrated, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy stats should be removed, err=%v", err)
	}
}

func TestIsUnderSkippedUserData(t *testing.T) {
	src := filepath.Join(`C:\`, "TcNo Account Switcher")
	cases := []struct {
		path string
		want bool
	}{
		{filepath.Join(src, "WebViewCache", "Default"), true},
		{filepath.Join(src, "EBWebView", "Default", "Cache"), true},
		{filepath.Join(src, "Accounts", "Steam.json"), false},
		{filepath.Join(src, "Statistics.json"), false},
	}
	for _, tc := range cases {
		if got := isUnderSkippedUserData(tc.path, src); got != tc.want {
			t.Fatalf("isUnderSkippedUserData(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestUserDataMovePendingRoundTrip(t *testing.T) {
	dir := t.TempDir()
	from := filepath.Join(dir, "old", UserDataDirName)
	to := filepath.Join(dir, "new", UserDataDirName)
	if err := writeUserDataMovePending(dir, from, to); err != nil {
		t.Fatal(err)
	}
	got, ok := loadUserDataMovePending(dir)
	if !ok {
		t.Fatal("expected pending file")
	}
	if got.From != from || got.To != to {
		t.Fatalf("pending = %+v, want from=%q to=%q", got, from, to)
	}
	resFrom, resTo, ok := resolveUserDataMoveCleanup(dir, "", "")
	if !ok || resFrom != from || resTo != to {
		t.Fatalf("resolve pending = %q %q ok=%v", resFrom, resTo, ok)
	}
	clearUserDataMovePending(dir)
	if _, ok := loadUserDataMovePending(dir); ok {
		t.Fatal("pending file should be cleared")
	}
}

func TestResolveUserDataMoveCleanupPrefersCLI(t *testing.T) {
	dir := t.TempDir()
	if err := writeUserDataMovePending(dir, `C:\old`, `C:\new`); err != nil {
		t.Fatal(err)
	}
	from, to, ok := resolveUserDataMoveCleanup(dir, `C:\cli-from`, `C:\cli-to`)
	if !ok || from != `C:\cli-from` || to != `C:\cli-to` {
		t.Fatalf("resolve = %q %q ok=%v", from, to, ok)
	}
}

func TestRemoveUserDataDir(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, UserDataDirName)
	if err := os.MkdirAll(filepath.Join(target, "Accounts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := removeUserDataDir(target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected dir removed, err=%v", err)
	}
}
