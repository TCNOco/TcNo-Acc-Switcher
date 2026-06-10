package platform

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/settingsfile"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	UserDataMoveProgressEvent    = "userdata-move-progress"
	userDataMovePendingFileName  = "userdata-move.pending.json"
	userDataMoveRemoveMaxAttempts = 12
)

// UserDataMoveProgressPayload reports copy progress to the frontend overlay.
type UserDataMoveProgressPayload struct {
	Phase string `json:"phase"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

type userDataMovePending struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Top-level user data folders skipped while WebView is running (recreated at the new location).
var userDataSkipTopDirs = []string{
	"WebViewCache",
	"EBWebView",
}

// GetUserDataLocation returns the current resolved user data directory path.
func GetUserDataLocation() (string, error) {
	return EffectiveUserDataDir()
}

// GetPortableUserDataLocation returns the portable user data path next to the executable.
func GetPortableUserDataLocation() (string, error) {
	exeDir, err := ResolveExeDir()
	if err != nil {
		return "", err
	}
	return PortableUserDataDir(exeDir), nil
}

// GetDefaultUserDataLocation returns the default AppData user data path.
func GetDefaultUserDataLocation() (string, error) {
	return DefaultUserDataDir()
}

// MoveUserDataPortable moves user data next to the executable (portable mode).
func MoveUserDataPortable() error {
	dest, err := GetPortableUserDataLocation()
	if err != nil {
		return err
	}
	return moveUserDataToResolved(dest)
}

// MoveUserDataAppData moves user data to the default AppData location.
func MoveUserDataAppData() error {
	dest, err := DefaultUserDataDir()
	if err != nil {
		return err
	}
	return moveUserDataToResolved(dest)
}

// MoveUserDataTo copies user data to the destination selected in the folder picker.
func MoveUserDataTo(picked string) error {
	dest := ResolveDestinationFromPicker(picked)
	if dest == "" {
		return errors.New("empty destination")
	}
	return moveUserDataToResolved(dest)
}

// FinalizeUserDataMove merges any remaining files from the old folder, then removes it.
// Call after the previous process instance has exited (singleton acquired).
func FinalizeUserDataMove(exeDir, from, to string) {
	from = filepath.Clean(strings.TrimSpace(from))
	to = filepath.Clean(strings.TrimSpace(to))
	if from == "" || from == "." || to == "" || to == "." || strings.EqualFold(from, to) {
		return
	}
	if st, err := os.Stat(from); err != nil || !st.IsDir() {
		clearUserDataMovePending(exeDir)
		return
	}
	destIsCustom := !settingsfile.IsDefaultUserDataDir(to, exeDir)
	if err := copyUserDataTree(from, to, destIsCustom, nil); err != nil {
		log.Printf("userdata move finalize copy: %v", err)
	}
	go removeOldUserDataDirAfterExit(exeDir, from)
}

// RunUserDataMoveCleanup resolves pending cleanup from CLI flags or the sidecar file, then finalizes.
func RunUserDataMoveCleanup(exeDir, cliFrom, cliTo string) {
	from, to, ok := resolveUserDataMoveCleanup(exeDir, cliFrom, cliTo)
	if !ok {
		return
	}
	FinalizeUserDataMove(exeDir, from, to)
}

func resolveUserDataMoveCleanup(exeDir, cliFrom, cliTo string) (from, to string, ok bool) {
	cliFrom = strings.TrimSpace(cliFrom)
	cliTo = strings.TrimSpace(cliTo)
	if cliFrom != "" {
		return filepath.Clean(cliFrom), filepath.Clean(cliTo), true
	}
	pending, found := loadUserDataMovePending(exeDir)
	if !found {
		return "", "", false
	}
	return pending.From, pending.To, true
}

func writeUserDataMovePending(exeDir, from, to string) error {
	exeDir = filepath.Clean(exeDir)
	from = filepath.Clean(strings.TrimSpace(from))
	to = filepath.Clean(strings.TrimSpace(to))
	if from == "" || to == "" {
		return errors.New("empty userdata move pending paths")
	}
	payload, err := json.Marshal(userDataMovePending{From: from, To: to})
	if err != nil {
		return err
	}
	path := filepath.Join(exeDir, userDataMovePendingFileName)
	tmp, err := os.CreateTemp(exeDir, "udmove-pending-")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		tmp.Close()
		os.Remove(tmpPath)
	}
	if _, err := tmp.Write(payload); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

func loadUserDataMovePending(exeDir string) (userDataMovePending, bool) {
	path := filepath.Join(filepath.Clean(exeDir), userDataMovePendingFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		return userDataMovePending{}, false
	}
	var p userDataMovePending
	if err := json.Unmarshal(b, &p); err != nil {
		return userDataMovePending{}, false
	}
	p.From = filepath.Clean(strings.TrimSpace(p.From))
	p.To = filepath.Clean(strings.TrimSpace(p.To))
	if p.From == "" {
		return userDataMovePending{}, false
	}
	return p, true
}

func clearUserDataMovePending(exeDir string) {
	path := filepath.Join(filepath.Clean(exeDir), userDataMovePendingFileName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("userdata move pending remove: %v", err)
	}
}

func removeOldUserDataDirAfterExit(exeDir, from string) {
	delays := []time.Duration{
		250 * time.Millisecond,
		500 * time.Millisecond,
		750 * time.Millisecond,
		1 * time.Second,
		1500 * time.Millisecond,
		2 * time.Second,
		3 * time.Second,
		5 * time.Second,
	}
	for attempt := 0; attempt < userDataMoveRemoveMaxAttempts; attempt++ {
		if attempt > 0 {
			delay := delays[min(attempt-1, len(delays)-1)]
			time.Sleep(delay)
		}
		if err := removeUserDataDir(from); err != nil {
			log.Printf("userdata move remove attempt %d: %v", attempt+1, err)
			continue
		}
		clearUserDataMovePending(exeDir)
		return
	}
	log.Printf("userdata move remove: gave up deleting %s; will retry on next launch", from)
}

func removeUserDataDir(path string) error {
	if st, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	} else if !st.IsDir() {
		return os.Remove(path)
	}
	return os.RemoveAll(path)
}

func moveUserDataToResolved(dest string) error {
	current, err := EffectiveUserDataDir()
	if err != nil {
		return err
	}
	dest = filepath.Clean(dest)
	current = filepath.Clean(current)
	if strings.EqualFold(dest, current) {
		return errors.New("destination is the same as the current user data location")
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	total, err := countUserDataCopyFiles(current)
	if err != nil {
		return err
	}
	destIsCustom := !settingsfile.IsDefaultUserDataDir(dest, exeDir)
	emitUserDataMoveProgress("copying", 0, total)
	if err := copyUserDataTree(current, dest, destIsCustom, func(done int) {
		emitUserDataMoveProgress("copying", done, total)
	}); err != nil {
		return err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	settings.UserDataPath = dest
	if err := saveSettingsAtomic(exeDir, settings); err != nil {
		return err
	}
	if destIsCustom {
		_ = os.Remove(filepath.Join(dest, settingsfile.FileName))
	}
	if err := writeUserDataMovePending(exeDir, current, dest); err != nil {
		return err
	}
	emitUserDataMoveProgress("restarting", total, total)
	if err := winutil.RestartSelf([]string{
		"--userdata-move-from=" + current,
		"--userdata-move-to=" + dest,
		"--toast=i18n:Toast_DataLocationSuccess",
	}); err != nil {
		return err
	}
	os.Exit(0)
	return nil
}

func emitUserDataMoveProgress(phase string, done, total int) {
	a := application.Get()
	if a == nil {
		return
	}
	_ = a.Event.Emit(UserDataMoveProgressEvent, UserDataMoveProgressPayload{
		Phase: phase,
		Done:  done,
		Total: total,
	})
}

func countUserDataCopyFiles(src string) (int, error) {
	n := 0
	err := filepath.WalkDir(src, func(path string, de os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if isUnderSkippedUserData(path, src) {
				return nil
			}
			return walkErr
		}
		if isUnderSkippedUserData(path, src) {
			if de.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !de.IsDir() {
			n++
		}
		return nil
	})
	return n, err
}

type userDataCopyProgressFn func(done int)

func copyUserDataTree(src, dst string, skipSettingsFile bool, onProgress userDataCopyProgressFn) error {
	done := 0
	return filepath.WalkDir(src, func(path string, de os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if isUnderSkippedUserData(path, src) {
				return nil
			}
			if isSkippableCopyErr(walkErr) {
				return nil
			}
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if skipSettingsFile && strings.EqualFold(filepath.Base(rel), settingsfile.FileName) {
			return nil
		}
		if isUnderSkippedUserData(path, src) {
			if de.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if de.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := copyUserDataFile(path, target); err != nil {
			if isSkippableCopyErr(err) {
				return nil
			}
			return err
		}
		done++
		if onProgress != nil {
			onProgress(done)
		}
		return nil
	})
}

func isUnderSkippedUserData(path, src string) bool {
	rel, err := filepath.Rel(src, path)
	if err != nil {
		return false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) == 0 {
		return false
	}
	top := parts[0]
	for _, skip := range userDataSkipTopDirs {
		if strings.EqualFold(top, skip) {
			return true
		}
	}
	return false
}

func isSkippableCopyErr(err error) bool {
	if err == nil {
		return false
	}
	if isSkippableCopyErrPlatform(err) {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "being used by another process") ||
		strings.Contains(s, "the process cannot access the file") ||
		strings.Contains(s, "resource busy")
}

func copyUserDataFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), "udcopy-")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		tmp.Close()
		os.Remove(tmpPath)
	}
	if _, err := io.Copy(tmp, in); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		cleanup()
		return err
	}
	return nil
}
