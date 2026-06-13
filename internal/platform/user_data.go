package platform

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"TcNo-Acc-Switcher/internal/paths"
)

// UserDataDirName is the folder that holds per-user app data (%AppData% for installs, or next to exe for portable).
const UserDataDirName = "TcNo Account Switcher"

var (
	resolvedUserDataMu  sync.RWMutex
	resolvedUserDataDir string
	resolvedExeDir      string
	pathsInitialized    bool
)

// DefaultUserDataDir returns the default install location (%AppData%/TcNo Account Switcher on Windows).
func DefaultUserDataDir() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, UserDataDirName), nil
}

// PortableUserDataDir returns {exeDir}/TcNo Account Switcher/.
func PortableUserDataDir(exeDir string) string {
	return filepath.Join(filepath.Clean(exeDir), UserDataDirName)
}

// ResolveUserDataDir picks the effective user data folder from settings, portable auto-detect, or AppData default.
func ResolveUserDataDir(exeDir string, s AppSettings) (string, error) {
	exeDir = filepath.Clean(exeDir)
	if p := strings.TrimSpace(s.UserDataPath); p != "" {
		return filepath.Clean(p), nil
	}

	resolvedUserDataMu.RLock()
	cachedDir, cachedExe, initialized := resolvedUserDataDir, resolvedExeDir, pathsInitialized
	resolvedUserDataMu.RUnlock()
	if initialized && cachedExe == exeDir && cachedDir != "" {
		return cachedDir, nil
	}

	portable := PortableUserDataDir(exeDir)
	if st, err := os.Stat(portable); err == nil && st.IsDir() {
		return portable, nil
	}
	return DefaultUserDataDir()
}

// ResolveDestinationFromPicker maps a folder-picker selection to the final user data directory path.
func ResolveDestinationFromPicker(picked string) string {
	picked = strings.TrimSpace(picked)
	if picked == "" {
		return ""
	}
	picked = filepath.Clean(picked)
	if picked == "." || picked == string(filepath.Separator) {
		return ""
	}
	if strings.EqualFold(filepath.Base(picked), UserDataDirName) {
		return picked
	}
	return filepath.Join(picked, UserDataDirName)
}

// InitDataPaths resolves and caches the user data directory. Call once at startup before other path consumers.
func InitDataPaths(exeDir string) error {
	exeDir = filepath.Clean(exeDir)
	s, err := loadSettingsFromDisk(exeDir)
	if err != nil {
		return err
	}
	dir, err := ResolveUserDataDir(exeDir, s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	persistPortable := strings.TrimSpace(s.UserDataPath) == "" && dir == PortableUserDataDir(exeDir)
	if persistPortable {
		s.UserDataPath = dir
		if err := saveSettingsAtomic(exeDir, s); err != nil {
			return err
		}
	}

	if err := migrateLegacyExeRootFiles(exeDir, dir); err != nil {
		return err
	}

	resolvedUserDataMu.Lock()
	resolvedUserDataDir = dir
	resolvedExeDir = exeDir
	pathsInitialized = true
	resolvedUserDataMu.Unlock()
	paths.InitDataRoot(dir)
	return nil
}

func migrateLegacyExeRootFiles(exeDir, userDataDir string) error {
	for _, name := range []string{"Statistics.json", "Platforms.json"} {
		src := filepath.Join(exeDir, name)
		dst := filepath.Join(userDataDir, name)
		if err := liftLegacyFile(src, dst); err != nil {
			return err
		}
	}
	return nil
}

func liftLegacyFile(src, dst string) error {
	st, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if st.IsDir() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(dst); err == nil {
		_ = os.Remove(src)
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		data, rerr := os.ReadFile(src)
		if rerr != nil {
			return err
		}
		if werr := atomicWriteBytes(dst, data, 0o644); werr != nil {
			return err
		}
		_ = os.Remove(src)
	}
	return nil
}

// EffectiveUserDataDir returns the resolved user data directory set by [InitDataPaths].
func EffectiveUserDataDir() (string, error) {
	if dir, err := paths.DataRoot(); err == nil {
		return dir, nil
	}
	resolvedUserDataMu.RLock()
	defer resolvedUserDataMu.RUnlock()
	if !pathsInitialized || resolvedUserDataDir == "" {
		return "", errors.New("user data paths not initialized")
	}
	return resolvedUserDataDir, nil
}

// UserDataDir returns the resolved user data directory. exeDir is ignored after [InitDataPaths].
func UserDataDir(exeDir string) string {
	dir, err := EffectiveUserDataDir()
	if err == nil {
		return dir
	}
	return PortableUserDataDir(exeDir)
}

// ResetUserDataPathsForTest sets cached paths for tests. Do not use t.Parallel().
func ResetUserDataPathsForTest(exeDir, userDataDir string) {
	resolvedUserDataMu.Lock()
	resolvedExeDir = filepath.Clean(exeDir)
	resolvedUserDataDir = filepath.Clean(userDataDir)
	pathsInitialized = true
	resolvedUserDataMu.Unlock()
	paths.InitDataRoot(userDataDir)
}
