package legacyinstall

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
)

const removeRetryWindow = 3 * time.Second

// Failure is one entry that could not be deleted.
type Failure struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// Result summarises a removal pass.
type Result struct {
	Removed []string  `json:"removed"`
	Failed  []Failure `json:"failed"`
	Bytes   int64     `json:"bytes"`
	// Preserved holds the new paths of entries renamed rather than deleted.
	Preserved []string `json:"preserved"`
}

// BackupSuffix is appended to a leftover that is set aside instead of deleted.
const BackupSuffix = ".old"

// Ok reports whether every entry was removed.
func (r Result) Ok() bool { return len(r.Failed) == 0 }

func removeLog() *slog.Logger {
	return slog.Default().With("component", "legacy-install")
}

// Remove deletes every entry in rep. Entries are re-validated against the
// release manifest first, so a Report that has been tampered with or built
// against a different directory cannot widen what gets deleted.
func Remove(rep Report) Result {
	var res Result
	if !rep.Found() {
		return res
	}
	for _, e := range rep.Entries {
		if !safeToRemove(rep.ExeDir, e) {
			removeLog().Warn("skipping entry outside the legacy manifest", "path", e.Path)
			continue
		}
		if e.Preserve {
			backup, err := setAside(e.Path)
			if err != nil {
				res.Failed = append(res.Failed, Failure{Path: e.Path, Error: err.Error()})
				continue
			}
			res.Preserved = append(res.Preserved, backup)
			removeLog().Info("kept a copy of a file the old version let you edit", "path", backup)
			continue
		}
		if err := fsutil.RemoveAllWithRetry(e.Path, removeRetryWindow, os.RemoveAll); err != nil {
			res.Failed = append(res.Failed, Failure{Path: e.Path, Error: err.Error()})
			continue
		}
		res.Removed = append(res.Removed, e.Path)
		res.Bytes += e.Bytes
	}
	removeLog().Info("legacy install cleanup",
		"removed", len(res.Removed), "kept", len(res.Preserved),
		"failed", len(res.Failed), "freed", res.Bytes)
	return res
}

// setAside renames path to path+[BackupSuffix], replacing an older backup.
func setAside(path string) (string, error) {
	backup := path + BackupSuffix
	if err := fsutil.RemoveAllWithRetry(backup, removeRetryWindow, os.RemoveAll); err != nil {
		return "", err
	}
	if err := os.Rename(path, backup); err != nil {
		return "", err
	}
	return backup, nil
}

// safeToRemove re-checks an entry against the manifest: same directory, a name
// the C# release actually shipped, and not something the Go build owns.
func safeToRemove(exeDir string, e Entry) bool {
	if exeDir == "" || e.Path == "" {
		return false
	}
	if filepath.Dir(e.Path) != filepath.Clean(exeDir) {
		return false
	}
	name := filepath.Base(e.Path)
	if !strings.EqualFold(name, e.Name) || isKept(name) {
		return false
	}
	if e.IsDir {
		for _, d := range legacyDirs {
			if strings.EqualFold(d.name, name) {
				return true
			}
		}
		return false
	}
	for _, f := range legacyFiles {
		if strings.EqualFold(f, name) {
			return true
		}
	}
	return false
}
