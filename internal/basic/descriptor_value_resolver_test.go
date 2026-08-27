package basic

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/platform"
)

func TestResolveDescriptorValue_LatestModifiedFile(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "battle.net-old.log")
	newPath := filepath.Join(dir, "battle.net-new.log")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("write old log: %v", err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0o644); err != nil {
		t.Fatalf("write new log: %v", err)
	}
	now := time.Now()
	if err := os.Chtimes(oldPath, now.Add(-2*time.Hour), now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("set old file modtime: %v", err)
	}
	if err := os.Chtimes(newPath, now.Add(-1*time.Hour), now.Add(-1*time.Hour)); err != nil {
		t.Fatalf("set new file modtime: %v", err)
	}
	got := resolveDescriptorValue(platform.Descriptor{}, "LATEST_MODIFIED_FILE:"+filepath.Join(dir, "battle.net-*.log"), "", platform.PathTokenContext{}, map[string]string{}, "", false)
	if got != newPath {
		t.Fatalf("unexpected latest modified file: got %q want %q", got, newPath)
	}
}

func TestParseBattleNetAccountIDFromLogData_LastMatch(t *testing.T) {
	data := []byte(`I 2026-05-07 16:47:10.188794 [Main] {Main} Opened database at: C:\Users\tcno\AppData\Local\Battle.net\CachedData.db
I 2026-05-07 16:47:14.311645 [Main] {Main} Opened database at: C:\Users\tcno\AppData\Local\Battle.net\Account\1111185922\account.db
I 2026-05-07 16:50:14.311645 [Main] {Main} Opened database at: C:\Users\tcno\AppData\Local\Battle.net\Account\9999999999\account.db`)
	got := parseBattleNetAccountIDFromLogData(data)
	want := "9999999999"
	if got != want {
		t.Fatalf("unexpected account id: got %q want %q", got, want)
	}
}

func TestResolveDescriptorValue_SQLiteInterpolatesBuiltInUserID(t *testing.T) {
	dbPath, err := filepath.Abs(filepath.Join("testdata", "cacheddata.db"))
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	vars := map[string]string{"BuiltInUserId": "1111185922"}
	raw := "SQLITE:" + dbPath + "|SELECT battle_tag FROM login_cache WHERE account_id_lo = '%BuiltInUserId%'"
	got := resolveDescriptorValue(platform.Descriptor{}, raw, "", platform.PathTokenContext{}, vars, "", false)
	if want := "Player#1234"; got != want {
		t.Fatalf("unexpected sqlite resolved value: got %q want %q", got, want)
	}

	raw = "SQLITE:" + dbPath + "|SELECT * FROM login_cache"
	if got := resolveDescriptorValue(platform.Descriptor{}, raw, "", platform.PathTokenContext{}, vars, "", false); got != "" {
		t.Fatalf("unsupported query should resolve to empty, got %q", got)
	}
}

func TestResolveBuiltInRuntimeVariables_BattleNetReadsAccountIDFromLatestLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "battle.net-1.log")
	logData := []byte(`I 2026-05-07 16:47:10.188794 [Main] {Main} Opened database at: C:\Users\tcno\AppData\Local\Battle.net\CachedData.db
I 2026-05-07 16:47:14.311645 [Main] {Main} Opened database at: C:\Users\tcno\AppData\Local\Battle.net\Account\1111185922\account.db`)
	if err := os.WriteFile(logPath, logData, 0o644); err != nil {
		t.Fatalf("write battlenet log: %v", err)
	}
	d := platform.Descriptor{
		Extras: platform.DescriptorExtras{
			BuiltInUserId: "LATEST_MODIFIED_FILE:" + filepath.Join(dir, "battle.net-*.log"),
		},
	}
	vars := resolveBuiltInRuntimeVariables("BattleNet", d, "", platform.PathTokenContext{}, map[string]string{}, "", false)
	if vars["BuiltInUserId"] != "1111185922" {
		t.Fatalf("unexpected BuiltInUserId var: %q", vars["BuiltInUserId"])
	}
	if vars["builtinuserid"] != "1111185922" {
		t.Fatalf("unexpected builtinuserid var: %q", vars["builtinuserid"])
	}
}
