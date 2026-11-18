package basic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSwitchFilesAreRemovedOnlyBySwitchCleanup(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	staged := filepath.Join(env.instDir, "games", "Game", "multiexe", "Game.exe")
	missing := filepath.Join(env.instDir, "games", "Other", "multiexe", "Game.exe")
	fc.Descriptor.SwitchFilesToClear = []string{
		filepath.Join("%Platform_Folder%", "games", "Game", "multiexe", "Game.exe"),
		filepath.Join("%Platform_Folder%", "games", "Other", "multiexe", "Game.exe"),
	}
	mustWrite(t, staged, "previous account build")
	kept := filepath.Join(env.instDir, "games", "Game", "Game.exe")
	mustWrite(t, kept, "installed build")

	if err := ClearCurrentLogin(FlowDeps{}, fc); err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, staged); got != "previous account build" {
		t.Fatal("session clear removed a switch cleanup file")
	}

	files, err := resolveSwitchFilesToClear(fc)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0] != staged || files[1] != missing {
		t.Fatalf("resolved switch files = %v", files)
	}
	clearSwitchFiles(files)
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Fatalf("staged exe was not removed: %v", err)
	}
	if got := mustRead(t, kept); got != "installed build" {
		t.Fatal("switch cleanup touched the installed exe")
	}
	// Already cleared files and missing installs are not errors.
	clearSwitchFiles(files)
}

func TestSwitchCleanupKeepsDirectories(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	dir := filepath.Join(env.instDir, "multiexe")
	inside := filepath.Join(dir, "Game.exe")
	mustWrite(t, inside, "build")
	fc.Descriptor.SwitchFilesToClear = []string{dir}

	files, err := resolveSwitchFilesToClear(fc)
	if err != nil {
		t.Fatal(err)
	}
	clearSwitchFiles(files)
	if got := mustRead(t, inside); got != "build" {
		t.Fatal("switch cleanup removed a directory")
	}
}

func fakeSwitchRegistry(t *testing.T, values map[string]any) {
	t.Helper()
	old := readSwitchFileRegistryValue
	readSwitchFileRegistryValue = func(key string) (any, uint32, error) {
		if v, ok := values[key]; ok {
			return v, 1, nil
		}
		return nil, 0, os.ErrNotExist
	}
	t.Cleanup(func() { readSwitchFileRegistryValue = old })
}

func TestSwitchCleanupFindsGameOutsideDefaultLibrary(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	library := t.TempDir()
	staged := filepath.Join(library, "Siege", "multiexe", "RainbowSix.exe")
	mustWrite(t, staged, "previous account build")
	fakeSwitchRegistry(t, map[string]any{
		`HKLM\Installs\635:InstallDir`:  filepath.ToSlash(filepath.Join(library, "Siege")) + "/",
		`HKLM\Installs\999:InstallDir`:  "relative/folder",
		`HKLM\Installs\1000:InstallDir`: uint32(1),
	})
	fc.Descriptor.SwitchFilesToClear = []string{
		`REG_DIR::HKLM\Installs\635:InstallDir::multiexe\RainbowSix.exe`,
		`REG_DIR::HKLM\Installs\1842:InstallDir::multiexe\RainbowSix.exe`,
		`REG_DIR::HKLM\Installs\999:InstallDir::multiexe\RainbowSix.exe`,
		`REG_DIR::HKLM\Installs\1000:InstallDir::multiexe\RainbowSix.exe`,
	}

	files, err := resolveSwitchFilesToClear(fc)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(library, "Siege", "multiexe", "RainbowSix.exe")
	if len(files) != 1 || files[0] != want {
		t.Fatalf("resolved switch files = %v, want only %s", files, want)
	}
	clearSwitchFiles(files)
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Fatalf("staged exe in a custom library was not removed: %v", err)
	}
}

func TestSwitchCleanupRejectsMalformedRegistryEntries(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	fakeSwitchRegistry(t, map[string]any{`HKLM\Installs\635:InstallDir`: t.TempDir()})
	for _, invalid := range []string{
		`REG_DIR::HKLM\Installs\635:InstallDir`,
		`REG_DIR::HKLM\Installs\635:InstallDir::`,
		`REG_DIR::::multiexe\RainbowSix.exe`,
		`REG_DIR::HKLM\Installs\635:InstallDir::..\RainbowSix.exe`,
		`REG_DIR::HKLM\Installs\635:InstallDir::\multiexe\RainbowSix.exe`,
		`REG_DIR::HKLM\Installs\635:InstallDir::C:\multiexe\RainbowSix.exe`,
		`REG_DIR::HKLM\Installs\635:InstallDir::multiexe\*.exe`,
	} {
		fc.Descriptor.SwitchFilesToClear = []string{invalid}
		if _, err := resolveSwitchFilesToClear(fc); err == nil {
			t.Fatalf("accepted malformed registry entry: %q", invalid)
		}
	}
}

func TestSwitchCleanupRejectsUnsafeEntries(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	for _, invalid := range []string{
		"",
		"Game.exe",
		filepath.VolumeName(env.instDir) + string(filepath.Separator),
		filepath.Join(env.instDir, "multiexe", "*.exe"),
		env.instDir + string(filepath.Separator) + ".." + string(filepath.Separator) + "Game.exe",
	} {
		fc.Descriptor.SwitchFilesToClear = []string{invalid}
		if _, err := resolveSwitchFilesToClear(fc); err == nil {
			t.Fatalf("accepted unsafe switch cleanup entry: %q", invalid)
		}
	}
}
