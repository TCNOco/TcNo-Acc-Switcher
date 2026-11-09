package basic

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/platform"
)

// A launcher's login browser can remember the previous account after its normal
// account selector has been cleared. Reset those transient files only for Add
// New, so switching back to saved accounts keeps their remembered sessions.
func clearLoginForAddNew(deps FlowDeps, fc FlowContext) error {
	var files []string
	ctx := fc.PathCtx
	ctx.PlatformFolder = fc.Folder
	for _, raw := range fc.Descriptor.AddNewFilesToClear {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == '/' || r == '\\' }) {
			if part == ".." {
				return fmt.Errorf("Add New login file contains traversal: %s", raw)
			}
		}
		path, err := platform.ResolveSafeDeletePattern(raw, ctx)
		if err != nil {
			return err
		}
		if err := validateSessionDirectoryName(path); err != nil {
			return err
		}
		if err := validateCredentialSessionPath(path); err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Add New login reset requires a regular file: %s", path)
		}
		files = append(files, filepath.Clean(path))
	}
	// Validate every target before changing the live account selection.
	if err := ClearCurrentLogin(deps, fc); err != nil {
		return err
	}
	for _, path := range files {
		emitUpdatingFileStatus(path)
		if err := fsutil.RemoveAllWithRetry(path, 2*time.Second, os.Remove); err != nil {
			return fmt.Errorf("clear Add New login file %s: %w", path, err)
		}
	}
	return nil
}
