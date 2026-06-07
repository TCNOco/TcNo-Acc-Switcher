package basic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
)

// flowTestEnv scaffolds a temp exeDir + install dir for integration tests.
// Do NOT call t.Parallel() in tests using this — it sets global path singletons.
type flowTestEnv struct {
	t       *testing.T
	exeDir  string
	instDir string
}

func newFlowTestEnv(t *testing.T) *flowTestEnv {
	t.Helper()
	exeDir := t.TempDir()
	instDir := t.TempDir()
	platform.ResetPathSingletonsForTest(exeDir)
	paths.ResetForTest(filepath.Join(exeDir, "TcNo Account Switcher"))
	return &flowTestEnv{t: t, exeDir: exeDir, instDir: instDir}
}

func (e *flowTestEnv) flowContext() FlowContext {
	e.t.Helper()
	return FlowContext{
		PlatformKey: "TestPlatform",
		Descriptor: platform.Descriptor{
			LoginFiles:       map[string]string{"%Platform_Folder%\\config.cfg": "Saved/config.cfg"},
			AllFilesRequired: false,
			ExesToEnd:        []string{},
			UniqueIdMethod:   "CREATE_ID_FILE",
			UniqueIdFile:     filepath.Join(e.instDir, ".account_id"),
		},
		Settings: platform.PlatformSettings{ClosingMethod: "none"},
		Folder:   e.instDir,
		PathCtx:  platform.PathTokenContext{PlatformFolder: e.instDir},
	}
}

func (e *flowTestEnv) cacheDir(accountName string) string {
	e.t.Helper()
	return filepath.Join(e.exeDir, "TcNo Account Switcher", "LoginCache", "TestPlatform", accountName)
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func pathExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

func TestFlow_FileRoundtrip(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	mustWrite(t, filepath.Join(env.instDir, "config.cfg"), "width=1920\nheight=1080\n")

	if err := saveCurrentAfterKill(FlowDeps{}, "TestAccount", fc); err != nil {
		t.Fatalf("saveCurrentAfterKill: %v", err)
	}

	cache := env.cacheDir("TestAccount")
	saved := filepath.Join(cache, "Saved", "config.cfg")
	if !pathExists(t, saved) {
		t.Fatalf("expected cached file at %s", saved)
	}
	if got := mustRead(t, saved); got != "width=1920\nheight=1080\n" {
		t.Errorf("cached config.cfg = %q, want original content", got)
	}

	mustWrite(t, filepath.Join(env.instDir, "config.cfg"), "corrupted")

	if err := Login(FlowDeps{}, fc, "TestAccount"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	restored := filepath.Join(env.instDir, "config.cfg")
	if got := mustRead(t, restored); got != "width=1920\nheight=1080\n" {
		t.Errorf("restored config.cfg = %q, want original content", got)
	}
}

func TestFlow_SwapTo_Chain(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()

	mustWrite(t, filepath.Join(env.instDir, "config.cfg"), "account=A")
	if err := saveCurrentAfterKill(FlowDeps{}, "AccountA", fc); err != nil {
		t.Fatalf("save AccountA: %v", err)
	}

	cacheA := filepath.Join(env.cacheDir("AccountA"), "Saved", "config.cfg")
	if !pathExists(t, cacheA) {
		t.Fatalf("expected AccountA cache at %s", cacheA)
	}

	mustWrite(t, filepath.Join(env.instDir, "config.cfg"), "account=B")
	if err := saveCurrentAfterKill(FlowDeps{}, "AccountB", fc); err != nil {
		t.Fatalf("save AccountB: %v", err)
	}

	cacheB := filepath.Join(env.cacheDir("AccountB"), "Saved", "config.cfg")
	if !pathExists(t, cacheB) || !pathExists(t, cacheA) {
		t.Fatal("both caches should exist")
	}

	if err := Login(FlowDeps{}, fc, "AccountA"); err != nil {
		t.Fatalf("Login AccountA: %v", err)
	}
	if got := mustRead(t, filepath.Join(env.instDir, "config.cfg")); got != "account=A" {
		t.Errorf("after Login AccountA: got %q, want account=A", got)
	}

	if err := Login(FlowDeps{}, fc, "AccountB"); err != nil {
		t.Fatalf("Login AccountB: %v", err)
	}
	if got := mustRead(t, filepath.Join(env.instDir, "config.cfg")); got != "account=B" {
		t.Errorf("after Login AccountB: got %q, want account=B", got)
	}
}

func TestFlow_GlobFiles(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	fc.Descriptor.LoginFiles = map[string]string{"%Platform_Folder%\\Saves\\*.sav": "Saves_glob/"}

	mustMkdir(t, filepath.Join(env.instDir, "Saves"))
	mustWrite(t, filepath.Join(env.instDir, "Saves", "slot1.sav"), "s1data")
	mustWrite(t, filepath.Join(env.instDir, "Saves", "slot2.sav"), "s2data")

	if err := saveCurrentAfterKill(FlowDeps{}, "GlobAccount", fc); err != nil {
		t.Fatalf("save: %v", err)
	}

	cacheGlobDir := filepath.Join(env.cacheDir("GlobAccount"), "Saves_glob")
	if !pathExists(t, filepath.Join(cacheGlobDir, "slot1.sav")) || !pathExists(t, filepath.Join(cacheGlobDir, "slot2.sav")) {
		t.Fatal("sav files not in cache")
	}

	os.RemoveAll(filepath.Join(env.instDir, "Saves"))
	mustMkdir(t, filepath.Join(env.instDir, "Saves"))

	if err := Login(FlowDeps{}, fc, "GlobAccount"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !pathExists(t, filepath.Join(env.instDir, "Saves", "slot1.sav")) || !pathExists(t, filepath.Join(env.instDir, "Saves", "slot2.sav")) {
		t.Fatal("sav files not restored")
	}
}

func TestFlow_AllFilesRequired_Missing(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	fc.Descriptor.LoginFiles = map[string]string{"%Platform_Folder%\\missing_file.bin": "Saved/missing.bin"}
	fc.Descriptor.AllFilesRequired = true
	if err := saveCurrentAfterKill(FlowDeps{}, "FailAccount", fc); err == nil {
		t.Fatal("expected error for missing required file, got nil")
	}
}

func TestFlow_AllFilesRequired_Optional(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	fc.Descriptor.LoginFiles = map[string]string{"%Platform_Folder%\\missing_file.bin": "Saved/missing.bin"}
	fc.Descriptor.AllFilesRequired = false
	if err := saveCurrentAfterKill(FlowDeps{}, "OkAccount", fc); err != nil {
		t.Fatalf("unexpected error for missing optional file: %v", err)
	}
}

func TestFlow_NameCollision_OldCachePruned(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	mustWrite(t, filepath.Join(env.instDir, "config.cfg"), "v1")
	if err := saveCurrentAfterKill(FlowDeps{}, "SameName", fc); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if !pathExists(t, env.cacheDir("SameName")) {
		t.Fatal("expected cache dir")
	}
	mustWrite(t, filepath.Join(env.instDir, "config.cfg"), "v2")
	if err := saveCurrentAfterKill(FlowDeps{}, "SameName", fc); err != nil {
		t.Fatalf("second save: %v", err)
	}
	cached := filepath.Join(env.cacheDir("SameName"), "Saved", "config.cfg")
	if got := mustRead(t, cached); got != "v2" {
		t.Errorf("cached config = %q, want v2", got)
	}
}

func TestFlow_IdsPersisted(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	mustWrite(t, filepath.Join(env.instDir, "config.cfg"), "hello")
	if err := saveCurrentAfterKill(FlowDeps{}, "MyAccount", fc); err != nil {
		t.Fatalf("save: %v", err)
	}
	ids, _ := readIDs("TestPlatform")
	for _, name := range ids {
		if name == "MyAccount" {
			return
		}
	}
	t.Errorf("expected 'MyAccount' in ids.json, got %v", ids)
}

func TestFlow_DirectoryTree(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	fc.Descriptor.LoginFiles = map[string]string{"%Platform_Folder%\\UserData": "UserData_backup"}

	mustMkdir(t, filepath.Join(env.instDir, "UserData", "Screenshots"))
	mustMkdir(t, filepath.Join(env.instDir, "UserData", "Config"))
	mustWrite(t, filepath.Join(env.instDir, "UserData", "prefs.ini"), "theme=dark")
	mustWrite(t, filepath.Join(env.instDir, "UserData", "Screenshots", "shot1.png"), "PNGDATA")
	mustWrite(t, filepath.Join(env.instDir, "UserData", "Config", "bindings.cfg"), "jump=space")

	if err := saveCurrentAfterKill(FlowDeps{}, "TreeAccount", fc); err != nil {
		t.Fatalf("save: %v", err)
	}

	cacheBase := filepath.Join(env.cacheDir("TreeAccount"), "UserData_backup")
	if !pathExists(t, filepath.Join(cacheBase, "prefs.ini")) || !pathExists(t, filepath.Join(cacheBase, "Screenshots", "shot1.png")) || !pathExists(t, filepath.Join(cacheBase, "Config", "bindings.cfg")) {
		t.Fatal("files not in cache")
	}

	os.RemoveAll(filepath.Join(env.instDir, "UserData"))
	if err := Login(FlowDeps{}, fc, "TreeAccount"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !pathExists(t, filepath.Join(env.instDir, "UserData", "prefs.ini")) || !pathExists(t, filepath.Join(env.instDir, "UserData", "Screenshots", "shot1.png")) || !pathExists(t, filepath.Join(env.instDir, "UserData", "Config", "bindings.cfg")) {
		t.Fatal("directory tree not restored")
	}
}

func TestFlow_BinaryFile(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	fc.Descriptor.LoginFiles = map[string]string{"%Platform_Folder%\\data.bin": "Saved/data.bin"}

	binaryData := []byte{0x00, 0xFF, 0xAB, 0xCD, 0xEF, 0x01, 0x02, 0x03}
	os.WriteFile(filepath.Join(env.instDir, "data.bin"), binaryData, 0o644)

	if err := saveCurrentAfterKill(FlowDeps{}, "BinAccount", fc); err != nil {
		t.Fatalf("save: %v", err)
	}

	cached, _ := os.ReadFile(filepath.Join(env.cacheDir("BinAccount"), "Saved", "data.bin"))
	if string(cached) != string(binaryData) {
		t.Errorf("binary corrupted")
	}

	os.Remove(filepath.Join(env.instDir, "data.bin"))
	if err := Login(FlowDeps{}, fc, "BinAccount"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	restored, _ := os.ReadFile(filepath.Join(env.instDir, "data.bin"))
	if string(restored) != string(binaryData) {
		t.Errorf("restored binary corrupted")
	}
}

func TestFlow_CreateIDFile(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	fc.Descriptor.UniqueIdMethod = "CREATE_ID_FILE"
	fc.Descriptor.UniqueIdFile = filepath.Join(env.instDir, "generated.id")

	id, err := ensureUniqueIDOnSave("TestPlatform", fc.Descriptor, fc.PathCtx)
	if err != nil || id == "" {
		t.Fatalf("ensureUniqueIDOnSave: %v", err)
	}
	id2, _ := ensureUniqueIDOnSave("TestPlatform", fc.Descriptor, fc.PathCtx)
	if id != id2 {
		t.Errorf("id changed: %q -> %q", id, id2)
	}
}

func TestFlow_RegDumpFromJSON(t *testing.T) {
	t.Parallel()
	t.Run("legacy_strings", func(t *testing.T) {
		m, err := regDumpFromJSON([]byte(`{"REG:HKLM\\SOFTWARE\\Game":"value1","REG:HKCU\\Settings":"value2"}`))
		if err != nil {
			t.Fatalf("regDumpFromJSON: %v", err)
		}
		if m["REG:HKLM\\SOFTWARE\\Game"].V != "value1" || m["REG:HKCU\\Settings"].V != "value2" {
			t.Error("legacy strings mismatch")
		}
	})
	t.Run("typed_structs", func(t *testing.T) {
		m, err := regDumpFromJSON([]byte(`{"REG:HKLM\\Val":{"v":"42","t":4}}`))
		if err != nil {
			t.Fatalf("regDumpFromJSON: %v", err)
		}
		e := m["REG:HKLM\\Val"]
		if e.V != "42" || e.T != 4 {
			t.Errorf("typed struct mismatch")
		}
	})
	t.Run("bundle", func(t *testing.T) {
		m, err := regDumpFromJSON([]byte(`{"REG:HKLM\\Key:*":{"values":{"Val1":{"v":"a"},"Val2":{"v":"b"}}}}`))
		if err != nil {
			t.Fatalf("regDumpFromJSON: %v", err)
		}
		e := m["REG:HKLM\\Key:*"]
		if len(e.Values) != 2 || e.Values["Val1"].V != "a" || e.Values["Val2"].V != "b" {
			t.Error("bundle mismatch")
		}
	})
}

func TestFlow_RegDumpLookup(t *testing.T) {
	t.Parallel()
	dump := map[string]regDumpEntry{"REG:HKLM\\SOFTWARE\\Game\\Version": {V: "1.0", T: 1}}
	if e, ok := regDumpLookup(dump, "REG:HKLM\\SOFTWARE\\Game\\Version"); !ok || e.V != "1.0" {
		t.Error("exact match failed")
	}
	if e, ok := regDumpLookup(dump, "HKLM\\SOFTWARE\\Game\\Version"); !ok || e.V != "1.0" {
		t.Error("without prefix match failed")
	}
}

func TestFlow_RegDumpLookup_WildcardBundle(t *testing.T) {
	t.Parallel()
	dump := map[string]regDumpEntry{"REG:HKLM\\SOFTWARE\\Game:*": {Values: map[string]regDumpEntry{"Version": {V: "2.0", T: 1}, "Build": {V: "1234", T: 1}}}}
	if e, ok := regDumpLookup(dump, "REG:HKLM\\SOFTWARE\\Game:Version"); !ok || e.V != "2.0" {
		t.Fatal("wildcard bundle lookup failed")
	}
}

func TestFlow_RegDumpLookup_GlobValue(t *testing.T) {
	t.Parallel()
	dump := map[string]regDumpEntry{"REG:HKLM\\SOFTWARE\\Game:S*": {Values: map[string]regDumpEntry{"Settings": {V: "enabled", T: 1}}}}
	if e, ok := regDumpLookup(dump, "REG:HKLM\\SOFTWARE\\Game:Settings"); !ok || e.V != "enabled" {
		t.Fatal("glob value lookup failed")
	}
}

func TestFlow_SplitRegistryPathValue(t *testing.T) {
	t.Parallel()
	kp, vn, ok := splitRegistryPathValue("HKLM\\SOFTWARE\\Game:Version")
	if !ok || kp != "HKLM\\SOFTWARE\\Game" || vn != "Version" {
		t.Errorf("got %q:%q:%v", kp, vn, ok)
	}
	_, _, ok = splitRegistryPathValue("no_colon_here")
	if ok {
		t.Error("expected false for no-colon input")
	}
	_, _, ok = splitRegistryPathValue("colon_at_end:")
	if ok {
		t.Error("expected false for colon at end")
	}
	kp, vn, ok = splitRegistryPathValue(":value")
	if ok {
		t.Errorf("expected false for colon at start, got %q:%q:%v", kp, vn, ok)
	}
}

func TestFlow_HasGlobPattern(t *testing.T) {
	t.Parallel()
	if hasGlobPattern("plain.txt") {
		t.Error("plain.txt should not match")
	}
	if !hasGlobPattern("*.txt") || !hasGlobPattern("file?.txt") || !hasGlobPattern("[abc].txt") {
		t.Error("glob patterns not detected")
	}
}

func TestFlow_SaveClearsOldCache(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	mustWrite(t, filepath.Join(env.instDir, "config.cfg"), "new-data")

	cacheDir := env.cacheDir("StaleAccount")
	mustMkdir(t, cacheDir)
	mustWrite(t, filepath.Join(cacheDir, "stale.txt"), "garbage")

	if err := saveCurrentAfterKill(FlowDeps{}, "StaleAccount", fc); err != nil {
		t.Fatalf("save: %v", err)
	}
	if pathExists(t, filepath.Join(cacheDir, "stale.txt")) {
		t.Error("stale.txt should have been removed")
	}
	if !pathExists(t, filepath.Join(cacheDir, "Saved", "config.cfg")) {
		t.Error("fresh config.cfg not in cache")
	}
}

func TestFlow_PrimaryExeImageForKill(t *testing.T) {
	t.Parallel()
	tests := []struct {
		exes []string
		want string
	}{
		{[]string{"game.exe"}, "game.exe"},
		{[]string{"SERVICE:svc.exe", "game.exe"}, "game.exe"},
		{[]string{"SERVICE:svc.exe"}, ""},
		{[]string{""}, ""},
		{[]string{"game"}, "game.exe"},
	}
	for _, tt := range tests {
		if got := primaryExeImageForKill(tt.exes); got != tt.want {
			t.Errorf("primaryExeImageForKill(%v) = %q, want %q", tt.exes, got, tt.want)
		}
	}
}

func TestFlow_WrapNeedsAdmin(t *testing.T) {
	t.Parallel()
	if err := wrapNeedsAdminIfPermission(nil); err != nil {
		t.Errorf("nil: got %v", err)
	}
	original := os.ErrNotExist
	if err := wrapNeedsAdminIfPermission(original); err != original {
		t.Errorf("original: got %v, want %v", err, original)
	}
}

func TestFlow_PathTokens_UniqueId(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	fc.PathCtx.UniqueID = "test-uid-123"
	fc.Descriptor.LoginFiles = map[string]string{"%Platform_Folder%\\Profiles\\%UniqueId%\\settings.ini": "Saved/prof_settings.ini"}
	mustMkdir(t, filepath.Join(env.instDir, "Profiles", "test-uid-123"))
	mustWrite(t, filepath.Join(env.instDir, "Profiles", "test-uid-123", "settings.ini"), "uid-data")
	if err := saveCurrentAfterKill(FlowDeps{}, "TokenAcct", fc); err != nil {
		t.Fatalf("save: %v", err)
	}
	cached := filepath.Join(env.cacheDir("TokenAcct"), "Saved", "prof_settings.ini")
	if got := mustRead(t, cached); got != "uid-data" {
		t.Errorf("cached = %q, want uid-data", got)
	}
	os.RemoveAll(filepath.Join(env.instDir, "Profiles"))
	if err := Login(FlowDeps{}, fc, "TokenAcct"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if got := mustRead(t, filepath.Join(env.instDir, "Profiles", "test-uid-123", "settings.ini")); got != "uid-data" {
		t.Errorf("restored = %q, want uid-data", got)
	}
}

func TestFlow_PathTokens_FileName(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	fc.PathCtx.FileName = "PlayerProfile.dat"
	fc.Descriptor.LoginFiles = map[string]string{"%Platform_Folder%\\%FileName%": "Saved/profile.dat"}
	mustWrite(t, filepath.Join(env.instDir, "PlayerProfile.dat"), "player-stats")
	if err := saveCurrentAfterKill(FlowDeps{}, "FileTokAcct", fc); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := mustRead(t, filepath.Join(env.cacheDir("FileTokAcct"), "Saved", "profile.dat")); got != "player-stats" {
		t.Errorf("cached = %q", got)
	}
	os.Remove(filepath.Join(env.instDir, "PlayerProfile.dat"))
	if err := Login(FlowDeps{}, fc, "FileTokAcct"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if got := mustRead(t, filepath.Join(env.instDir, "PlayerProfile.dat")); got != "player-stats" {
		t.Errorf("restored = %q", got)
	}
}

func TestFlow_PathTokens_Largest(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	fc.PathCtx.LargestPath = "GameData"
	fc.Descriptor.LoginFiles = map[string]string{"%Platform_Folder%\\Saves\\%LARGEST%.sav": "Saved/largest.sav"}
	mustMkdir(t, filepath.Join(env.instDir, "Saves"))
	mustWrite(t, filepath.Join(env.instDir, "Saves", "GameData.sav"), "largest-data")
	if err := saveCurrentAfterKill(FlowDeps{}, "LargestAcct", fc); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := mustRead(t, filepath.Join(env.cacheDir("LargestAcct"), "Saved", "largest.sav")); got != "largest-data" {
		t.Errorf("cached = %q", got)
	}
	os.Remove(filepath.Join(env.instDir, "Saves", "GameData.sav"))
	if err := Login(FlowDeps{}, fc, "LargestAcct"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if got := mustRead(t, filepath.Join(env.instDir, "Saves", "GameData.sav")); got != "largest-data" {
		t.Errorf("restored = %q", got)
	}
}

func TestFlow_PathTokens_MultipleTokens(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	fc.PathCtx.UniqueID = "uid-99"
	fc.PathCtx.FileName = "data.bin"
	fc.Descriptor.LoginFiles = map[string]string{
		"%Platform_Folder%\\%FileName%":                    "Saved/data.bin",
		"%Platform_Folder%\\Users\\%UniqueId%\\prefs.cfg": "Saved/prefs.cfg",
	}
	mustWrite(t, filepath.Join(env.instDir, "data.bin"), "binary")
	mustMkdir(t, filepath.Join(env.instDir, "Users", "uid-99"))
	mustWrite(t, filepath.Join(env.instDir, "Users", "uid-99", "prefs.cfg"), "prefs-99")
	if err := saveCurrentAfterKill(FlowDeps{}, "MultiTok", fc); err != nil {
		t.Fatalf("save: %v", err)
	}
	cacheDir := env.cacheDir("MultiTok")
	if got := mustRead(t, filepath.Join(cacheDir, "Saved", "data.bin")); got != "binary" {
		t.Errorf("data.bin = %q", got)
	}
	if got := mustRead(t, filepath.Join(cacheDir, "Saved", "prefs.cfg")); got != "prefs-99" {
		t.Errorf("prefs.cfg = %q", got)
	}
	os.Remove(filepath.Join(env.instDir, "data.bin"))
	os.RemoveAll(filepath.Join(env.instDir, "Users"))
	if err := Login(FlowDeps{}, fc, "MultiTok"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if got := mustRead(t, filepath.Join(env.instDir, "data.bin")); got != "binary" {
		t.Errorf("restored data.bin = %q", got)
	}
	if got := mustRead(t, filepath.Join(env.instDir, "Users", "uid-99", "prefs.cfg")); got != "prefs-99" {
		t.Errorf("restored prefs.cfg = %q", got)
	}
}

func TestFlow_FirstValueNameMatchingGlob(t *testing.T) {
	t.Parallel()
	values := map[string]regDumpEntry{"Version": {V: "1.0"}, "Settings": {V: "enabled"}, "Build": {V: "1234"}}
	if got := firstValueNameMatchingGlob(values, "Version"); got != "Version" {
		t.Errorf("got %q", got)
	}
	if got := firstValueNameMatchingGlob(values, "S*"); got != "Settings" {
		t.Errorf("S*: got %q", got)
	}
	if got := firstValueNameMatchingGlob(values, "Nonexistent"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestFlow_ExpandPlatformPath_EnvVars(t *testing.T) {
	t.Parallel()
	ctx := platform.PathTokenContext{PlatformFolder: "/fake/install"}
	got := platform.ExpandPathTokens(platform.ExpandWindowsPath("%Platform_Folder%\\config.cfg"), ctx)
	got = filepath.Clean(got)
	want := filepath.Join("/fake/install", "config.cfg")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	ctx2 := platform.PathTokenContext{}
	got2 := platform.ExpandPathTokens(platform.ExpandWindowsPath("%Platform_Folder%\\file.txt"), ctx2)
	if !strings.Contains(got2, "Platform_Folder") {
		t.Errorf("expected literal %%Platform_Folder%% in output, got %q", got2)
	}
}

func TestFlow_WriteRegistryFromRegDump_Errors(t *testing.T) {
	t.Parallel()
	err := writeRegistryFromRegDump("not-a-reg-key", regDumpEntry{Values: map[string]regDumpEntry{"a": {V: "1"}}})
	if err == nil {
		t.Error("expected error for non-REG key with values map")
	}
	err = writeRegistryFromRegDump("REG:HKLM\\Key:Value", regDumpEntry{Values: map[string]regDumpEntry{"a": {V: "1"}}})
	if err == nil {
		t.Error("expected error for non-glob value with values map")
	}
	err = writeRegistryFromRegDump("REG:HKLM\\Key:Value", regDumpEntry{V: "", T: 0})
	if err != nil {
		if !strings.Contains(err.Error(), "unsupported") && !strings.Contains(err.Error(), "registry") {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestFlow_JSONEmptyValue_ClearLogin(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()

	jsonPath := filepath.Join(env.instDir, "battlenet.config")
	os.WriteFile(jsonPath, []byte(`{"activeUser":"player99","settings":{"volume":0.8}}`), 0o644)

	fc.Descriptor.PathListToClear = []string{
		"JSON_EMPTY_VALUE::" + jsonPath + "::activeUser",
	}

	if err := ClearCurrentLogin(FlowDeps{}, fc); err != nil {
		t.Fatalf("ClearCurrentLogin: %v", err)
	}

	data, _ := os.ReadFile(jsonPath)
	content := string(data)
	if !strings.Contains(content, `"activeUser":""`) {
		t.Errorf("activeUser should be zeroed, got: %s", content)
	}
	if !strings.Contains(content, `"volume":0.8`) {
		t.Error("other fields should remain intact")
	}
}
