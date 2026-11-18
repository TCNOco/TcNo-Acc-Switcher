package basic

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/winutil"
)

// REG_DIR::<HIVE\Key:Value>::<relative path> resolves a file inside a folder the launcher
// records in the registry, e.g. a game installed outside the default library.
const switchFileRegistryDirPrefix = "REG_DIR::"

var readSwitchFileRegistryValue = winutil.RegistryRead

// Resolved before anything changes, so a bad catalog entry fails the switch up front.
func resolveSwitchFilesToClear(fc FlowContext) ([]string, error) {
	ctx := fc.PathCtx
	ctx.PlatformFolder = fc.Folder
	var files []string
	for _, raw := range fc.Descriptor.SwitchFilesToClear {
		if hasPathTraversal(raw) {
			return nil, fmt.Errorf("switch cleanup file contains traversal: %s", raw)
		}
		if rest, ok := strings.CutPrefix(strings.TrimSpace(raw), switchFileRegistryDirPrefix); ok {
			path, err := resolveRegistryDirSwitchFile(rest)
			if err != nil {
				return nil, err
			}
			if path != "" {
				files = append(files, path)
			}
			continue
		}
		path, err := platform.ResolveSafeDeletePattern(raw, ctx)
		if err != nil {
			return nil, err
		}
		if err := validateSessionDirectoryName(path); err != nil {
			return nil, err
		}
		files = append(files, filepath.Clean(path))
	}
	return files, nil
}

// Returns "" when the folder is not recorded: the game is simply not installed.
func resolveRegistryDirSwitchFile(spec string) (string, error) {
	key, rel, ok := strings.Cut(spec, "::")
	key, rel = strings.TrimSpace(key), strings.TrimSpace(rel)
	if !ok || key == "" || rel == "" || hasPathTraversal(rel) || strings.ContainsAny(rel, ":*?[]") || strings.HasPrefix(rel, `\`) || strings.HasPrefix(rel, "/") {
		return "", fmt.Errorf("switch cleanup entry must be %s<registry value>::<relative path>: %s", switchFileRegistryDirPrefix, spec)
	}
	value, _, err := readSwitchFileRegistryValue(key)
	if err != nil {
		logFlow().Debug("switch cleanup: install folder not in registry", "key", key, "err", err)
		return "", nil
	}
	dir, _ := value.(string)
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", nil
	}
	path := filepath.Join(filepath.Clean(dir), rel)
	if !filepath.IsAbs(dir) || validateSessionDirectoryName(path) != nil {
		logFlow().Warn("switch cleanup: ignoring unusable install folder from registry", "key", key, "folder", dir)
		return "", nil
	}
	return path, nil
}

func hasPathTraversal(raw string) bool {
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return true
		}
	}
	return false
}

// Best effort: a running game keeps its exe locked, and that must not fail the switch.
func clearSwitchFiles(files []string) {
	for _, path := range files {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			logFlow().Warn("switch cleanup: cannot stat file", "path", path, "err", err)
			continue
		}
		if !info.Mode().IsRegular() {
			logFlow().Warn("switch cleanup: skipping non-regular file", "path", path)
			continue
		}
		if err := validateCredentialSessionPath(path); err != nil {
			logFlow().Warn("switch cleanup: skipping linked path", "path", path, "err", err)
			continue
		}
		emitUpdatingFileStatus(path)
		if err := fsutil.RemoveAllWithRetry(path, 2*time.Second, os.Remove); err != nil {
			logFlow().Warn("switch cleanup: file was not removed", "path", path, "err", err)
		}
	}
}
