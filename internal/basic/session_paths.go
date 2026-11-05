package basic

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/steamguard/securefile"
)

type sessionPathBackup struct {
	Path   string `json:"path"`
	Backup string `json:"backup"`
}

type sessionPathOps struct {
	lstat     func(string) (os.FileInfo, error)
	reparse   func(string) (bool, error)
	mkdirTemp func(string, string) (string, error)
	rename    func(string, string) error
	readDir   func(string) ([]os.DirEntry, error)
	removeAll func(string) error
}

func defaultSessionPathOps() sessionPathOps {
	return sessionPathOps{
		lstat: os.Lstat, reparse: securefile.IsReparsePoint, mkdirTemp: os.MkdirTemp,
		rename: os.Rename, readDir: os.ReadDir, removeAll: os.RemoveAll,
	}
}

func prepareSessionPaths(fc FlowContext, protected []string) ([]sessionPathBackup, error) {
	return prepareSessionPathsWithOps(fc, protected, defaultSessionPathOps())
}

func prepareSessionPathsWithOps(fc FlowContext, protected []string, ops sessionPathOps) ([]sessionPathBackup, error) {
	var selected []string
	resolved, err := configuredSessionPaths(fc, protected)
	if err != nil {
		return nil, err
	}
	for _, path := range resolved {
		exists, err := validateSessionDirectory(path, ops)
		if err != nil {
			return nil, err
		}
		if exists {
			selected = append(selected, path)
		}
	}
	var backups []sessionPathBackup
	for _, path := range selected {
		holder, err := ops.mkdirTemp(filepath.Dir(path), ".tcno-session-reset-")
		if err != nil {
			return nil, errors.Join(fmt.Errorf("prepare session reset backup: %w", err), discardSessionPathsWithOps(backups, ops))
		}
		backups = append(backups, sessionPathBackup{Path: path, Backup: filepath.Join(holder, "data")})
	}
	return backups, nil
}

func configuredSessionPaths(fc FlowContext, protected []string) ([]string, error) {
	var resolved []string
	ctx := fc.PathCtx
	ctx.PlatformFolder = fc.Folder
	for _, raw := range fc.Descriptor.PathListToClear {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == '/' || r == '\\' }) {
			if part == ".." {
				return nil, fmt.Errorf("session reset path contains traversal: %s", raw)
			}
		}
		upper := strings.ToUpper(raw)
		if strings.ContainsAny(raw, "*?[]") || strings.HasPrefix(upper, "REG:") || strings.HasPrefix(upper, "JSON_") || strings.HasPrefix(upper, "LEVELDB:") {
			return nil, fmt.Errorf("session reset requires an exact directory path: %s", raw)
		}
		path, err := platform.ResolveSafeDeletePattern(raw, ctx)
		if err != nil {
			return nil, err
		}
		if err := validateSessionDirectoryName(path); err != nil {
			return nil, err
		}
		skip := false
		for _, safe := range protected {
			if sameSessionPath(path, safe) {
				skip = true
				break
			}
			if sessionPathsOverlap(path, safe) {
				return nil, fmt.Errorf("session reset path overlaps protected session data: %s", path)
			}
		}
		if skip {
			continue
		}
		declaredCache := false
		for _, rawCache := range fc.Descriptor.Extras.CachePaths {
			if strings.TrimSpace(rawCache) == "" || strings.ContainsAny(rawCache, "*?[]") {
				continue
			}
			cache, err := platform.ResolveSafeDeletePattern(rawCache, ctx)
			if err == nil && sameSessionPath(path, cache) {
				declaredCache = true
				break
			}
		}
		if !declaredCache {
			return nil, fmt.Errorf("session reset directory is not declared as a platform cache: %s", path)
		}
		for _, other := range resolved {
			if sessionPathsOverlap(path, other) {
				return nil, fmt.Errorf("session reset directories overlap: %s and %s", path, other)
			}
		}
		resolved = append(resolved, path)
	}
	return resolved, nil
}

func validateConfiguredSessionBackups(fc FlowContext, protected []string, backups []sessionPathBackup) error {
	return validateConfiguredSessionBackupsWithOps(fc, protected, backups, defaultSessionPathOps())
}

func validateConfiguredSessionBackupsWithOps(fc FlowContext, protected []string, backups []sessionPathBackup, ops sessionPathOps) error {
	configured, err := configuredSessionPaths(fc, protected)
	if err != nil {
		return err
	}
	var seen []string
	for _, backup := range backups {
		found := false
		for _, path := range configured {
			if sameSessionPath(backup.Path, path) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("backup path is not configured for session reset: %s", backup.Path)
		}
		for _, previous := range seen {
			if sessionPathsOverlap(backup.Path, previous) {
				return errors.New("session backup paths overlap")
			}
		}
		if err := validateSessionBackup(backup, ops); err != nil {
			return err
		}
		seen = append(seen, backup.Path)
	}
	return nil
}

func moveSessionPaths(backups []sessionPathBackup) error {
	return moveSessionPathsWithOps(backups, defaultSessionPathOps())
}

func moveSessionPathsWithOps(backups []sessionPathBackup, ops sessionPathOps) error {
	for _, backup := range backups {
		if err := validateSessionBackup(backup, ops); err != nil {
			return err
		}
		if exists, err := validateSessionDirectory(backup.Backup, ops); err != nil {
			return err
		} else if exists {
			return fmt.Errorf("session reset backup already exists: %s", backup.Backup)
		}
	}
	for _, backup := range backups {
		if err := validateSessionBackup(backup, ops); err != nil {
			return err
		}
		if exists, err := validateSessionDirectory(backup.Backup, ops); err != nil {
			return err
		} else if exists {
			return fmt.Errorf("session reset backup already exists: %s", backup.Backup)
		}
		exists, err := validateSessionDirectory(backup.Path, ops)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		if err := ops.rename(backup.Path, backup.Backup); err != nil {
			return fmt.Errorf("back up session reset directory %s: %w", backup.Path, err)
		}
	}
	return nil
}

func restoreSessionPaths(backups []sessionPathBackup) error {
	_, err := restoreSessionPathsTracked(backups)
	return err
}

func restoreSessionPathsWithOps(backups []sessionPathBackup, ops sessionPathOps) error {
	_, err := restoreSessionPathsTrackedWithOps(backups, ops)
	return err
}

func restoreSessionPathsTracked(backups []sessionPathBackup) ([]sessionPathBackup, error) {
	return restoreSessionPathsTrackedWithOps(backups, defaultSessionPathOps())
}

func restoreSessionPathsTrackedWithOps(backups []sessionPathBackup, ops sessionPathOps) ([]sessionPathBackup, error) {
	var restored []sessionPathBackup
	var errs []error
	for i := len(backups) - 1; i >= 0; i-- {
		backup := backups[i]
		if err := validateSessionBackup(backup, ops); err != nil {
			errs = append(errs, err)
			continue
		}
		exists, err := validateSessionDirectory(backup.Backup, ops)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !exists {
			continue
		}
		if exists, err := validateSessionDirectory(backup.Path, ops); err != nil {
			errs = append(errs, err)
			continue
		} else if exists {
			errs = append(errs, fmt.Errorf("current session directory exists; backup retained at %s", backup.Backup))
			continue
		}
		if err := ops.rename(backup.Backup, backup.Path); err != nil {
			errs = append(errs, fmt.Errorf("restore session reset directory %s: %w", backup.Path, err))
			continue
		}
		restored = append(restored, backup)
	}
	return restored, errors.Join(errs...)
}

func revertRestoredSessionPaths(backups []sessionPathBackup) error {
	return revertRestoredSessionPathsWithOps(backups, defaultSessionPathOps())
}

func revertRestoredSessionPathsWithOps(backups []sessionPathBackup, ops sessionPathOps) error {
	for _, backup := range backups {
		if err := validateSessionBackup(backup, ops); err != nil {
			return err
		}
	}
	var errs []error
	for _, backup := range backups {
		if err := validateSessionBackup(backup, ops); err != nil {
			errs = append(errs, err)
			continue
		}
		if exists, err := validateSessionDirectory(backup.Backup, ops); err != nil {
			errs = append(errs, err)
			continue
		} else if exists {
			continue
		}
		exists, err := validateSessionDirectory(backup.Path, ops)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !exists {
			continue
		}
		if err := ops.rename(backup.Path, backup.Backup); err != nil {
			errs = append(errs, fmt.Errorf("retain restored session directory %s: %w", backup.Path, err))
		}
	}
	return errors.Join(errs...)
}

func discardSessionPaths(backups []sessionPathBackup) error {
	return discardSessionPathsWithOps(backups, defaultSessionPathOps())
}

func discardSessionPathsWithOps(backups []sessionPathBackup, ops sessionPathOps) error {
	var errs []error
	for _, backup := range backups {
		if err := validateSessionBackup(backup, ops); err != nil {
			errs = append(errs, err)
			continue
		}
		holder := filepath.Dir(backup.Backup)
		entries, err := ops.readDir(holder)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if len(entries) > 1 || (len(entries) == 1 && entries[0].Name() != "data") {
			errs = append(errs, fmt.Errorf("unexpected content in session backup holder: %s", holder))
			continue
		}
		if err := validateSessionBackupTree(holder, ops); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := ops.removeAll(holder); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func validateSessionBackup(backup sessionPathBackup, ops sessionPathOps) error {
	path, target := backup.Path, backup.Backup
	holder := filepath.Dir(target)
	if filepath.Clean(path) != path || filepath.Clean(target) != target || filepath.Base(target) != "data" ||
		!strings.HasPrefix(filepath.Base(holder), ".tcno-session-reset-") || filepath.Base(holder) == ".tcno-session-reset-" ||
		!sameSessionPath(filepath.Dir(path), filepath.Dir(holder)) || sessionPathsOverlap(path, holder) {
		return errors.New("invalid session backup paths")
	}
	if _, err := validateSessionDirectory(path, ops); err != nil {
		return err
	}
	if _, err := validateSessionDirectory(target, ops); err != nil {
		return err
	}
	return nil
}

func validateSessionDirectoryName(path string) error {
	if !filepath.IsAbs(path) || strings.ContainsAny(path, "*?[]\x00") {
		return fmt.Errorf("invalid session directory: %s", path)
	}
	if err := platform.ValidateDeleteTargetPath(path); err != nil {
		return err
	}
	for _, name := range []string{"USERPROFILE", "HOME", "APPDATA", "LOCALAPPDATA", "ProgramData", "ProgramFiles", "ProgramFiles(x86)", "SystemRoot"} {
		if base := strings.TrimSpace(os.Getenv(name)); base != "" && sameSessionPath(path, base) {
			return fmt.Errorf("session directory resolves to a system or profile base: %s", path)
		}
	}
	return nil
}

func validateSessionDirectory(path string, ops sessionPathOps) (bool, error) {
	if err := validateSessionDirectoryName(path); err != nil {
		return false, err
	}
	var components []string
	for at := filepath.Clean(path); ; at = filepath.Dir(at) {
		components = append(components, at)
		if filepath.Dir(at) == at {
			break
		}
	}
	for i := len(components) - 1; i >= 0; i-- {
		at := components[i]
		info, err := ops.lstat(at)
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return false, fmt.Errorf("session path is not a regular directory: %s", at)
		}
		reparse, err := ops.reparse(at)
		if err != nil {
			return false, err
		}
		if reparse {
			return false, fmt.Errorf("session path contains a reparse point: %s", at)
		}
	}
	return true, nil
}

func validateSessionBackupTree(path string, ops sessionPathOps) error {
	entries, err := ops.readDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." || filepath.Base(name) != name {
			return errors.New("invalid entry in session backup")
		}
		child := filepath.Join(path, name)
		info, err := ops.lstat(child)
		if err != nil {
			return err
		}
		reparse, err := ops.reparse(child)
		if err != nil {
			return err
		}
		if reparse || info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return fmt.Errorf("nonregular entry in session backup: %s", child)
		}
		if info.IsDir() {
			if err := validateSessionBackupTree(child, ops); err != nil {
				return err
			}
		}
	}
	return nil
}

func sameSessionPath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func sessionPathsOverlap(a, b string) bool {
	a, b = strings.ToLower(filepath.Clean(a)), strings.ToLower(filepath.Clean(b))
	rel, err := filepath.Rel(a, b)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return true
	}
	rel, err = filepath.Rel(b, a)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
