package security

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type accountSaveTestInfo struct {
	mode os.FileMode
}

func (i accountSaveTestInfo) Name() string       { return "account" }
func (i accountSaveTestInfo) Size() int64        { return 0 }
func (i accountSaveTestInfo) Mode() os.FileMode  { return i.mode }
func (i accountSaveTestInfo) ModTime() time.Time { return time.Time{} }
func (i accountSaveTestInfo) IsDir() bool        { return i.mode.IsDir() }
func (i accountSaveTestInfo) Sys() any           { return nil }

type accountSaveTestFS struct {
	entries   map[string]string
	modes     map[string]os.FileMode
	renameErr map[string]error
	mkdirErr  error
	synced    bool
}

func (f *accountSaveTestFS) ops() accountSaveDirectoryOps {
	return accountSaveDirectoryOps{
		lstat: func(path string) (os.FileInfo, error) {
			if _, ok := f.entries[path]; !ok {
				return nil, os.ErrNotExist
			}
			mode := os.ModeDir | 0o700
			if configured, ok := f.modes[path]; ok {
				mode = configured
			}
			return accountSaveTestInfo{mode: mode}, nil
		},
		mkdirTemp: func(parent, prefix string) (string, error) {
			if f.mkdirErr != nil {
				return "", f.mkdirErr
			}
			path := filepath.Join(parent, prefix+"fixture")
			f.entries[path] = "directory"
			return path, nil
		},
		rename: func(src, dest string) error {
			if err := f.renameErr[src]; err != nil {
				return err
			}
			value, ok := f.entries[src]
			if !ok {
				return os.ErrNotExist
			}
			if _, exists := f.entries[dest]; exists {
				return os.ErrExist
			}
			f.entries[dest] = value
			delete(f.entries, src)
			return nil
		},
		removeAll: func(path string) error {
			for entry := range f.entries {
				if entry == path || strings.HasPrefix(entry, path+string(filepath.Separator)) {
					delete(f.entries, entry)
				}
			}
			return nil
		},
		syncDir: func(string) { f.synced = true },
	}
}

func TestAccountSaveDirectoryCommit(t *testing.T) {
	parent := filepath.Join("fixtures", "accounts")
	staged := filepath.Join(parent, ".account-save-captured")
	destination := filepath.Join(parent, "account")
	backupDir := filepath.Join(parent, ".account-save-backup-fixture")
	backup := filepath.Join(backupDir, "previous")
	installFailure := errors.New("install blocked")

	for _, tc := range []struct {
		name         string
		previous     bool
		configure    func(*accountSaveTestFS)
		wantErr      string
		wantCurrent  string
		wantBackup   bool
		wantCaptured bool
		wantSynced   bool
	}{
		{name: "first save", wantCurrent: "captured", wantSynced: true},
		{name: "replace after capture", previous: true, wantCurrent: "captured", wantSynced: true},
		{
			name: "install failure restores previous", previous: true,
			configure: func(f *accountSaveTestFS) { f.renameErr[staged] = installFailure },
			wantErr:   "install blocked", wantCurrent: "previous", wantCaptured: true, wantSynced: true,
		},
		{
			name: "rollback failure retains recovery copy", previous: true,
			configure: func(f *accountSaveTestFS) {
				f.renameErr[staged] = installFailure
				f.renameErr[backup] = errors.New("rollback blocked")
			},
			wantErr: "previous account kept at " + backup, wantBackup: true, wantCaptured: true,
		},
		{
			name: "backup preparation failure leaves previous", previous: true,
			configure: func(f *accountSaveTestFS) { f.mkdirErr = errors.New("backup unavailable") },
			wantErr:   "backup unavailable", wantCurrent: "previous", wantCaptured: true,
		},
		{
			name: "backup rename failure leaves previous", previous: true,
			configure: func(f *accountSaveTestFS) { f.renameErr[destination] = errors.New("previous in use") },
			wantErr:   "previous in use", wantCurrent: "previous", wantCaptured: true,
		},
		{
			name:      "first save failure remains uncommitted",
			configure: func(f *accountSaveTestFS) { f.renameErr[staged] = installFailure },
			wantErr:   "install blocked", wantCaptured: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs := &accountSaveTestFS{
				entries: map[string]string{staged: "captured"}, modes: map[string]os.FileMode{}, renameErr: map[string]error{},
			}
			if tc.previous {
				fs.entries[destination] = "previous"
			}
			if tc.configure != nil {
				tc.configure(fs)
			}
			err := commitAccountSaveDirectory(staged, destination, fs.ops())
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("commitAccountSaveDirectory() = %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
			if got := fs.entries[destination]; got != tc.wantCurrent {
				t.Errorf("current account = %q, want %q", got, tc.wantCurrent)
			}
			_, hasBackup := fs.entries[backup]
			if hasBackup != tc.wantBackup {
				t.Errorf("recovery copy present = %v, want %v", hasBackup, tc.wantBackup)
			}
			_, hasCaptured := fs.entries[staged]
			if hasCaptured != tc.wantCaptured {
				t.Errorf("captured copy present = %v, want %v", hasCaptured, tc.wantCaptured)
			}
			if fs.synced != tc.wantSynced {
				t.Errorf("parent directory synced = %v, want %v", fs.synced, tc.wantSynced)
			}
		})
	}
}

func TestAccountSaveDirectoryRejectsUnsafePaths(t *testing.T) {
	parent := filepath.Join("fixtures", "accounts")
	destination := filepath.Join(parent, "account")
	staged := filepath.Join(parent, ".account-save-captured")
	for _, tc := range []struct {
		name   string
		staged string
		modes  map[string]os.FileMode
	}{
		{name: "live directory used as staging", staged: destination},
		{name: "staging in another directory", staged: filepath.Join("elsewhere", ".account-save-captured")},
		{name: "unrecognized staging directory", staged: filepath.Join(parent, "other")},
		{name: "staging symlink", staged: staged, modes: map[string]os.FileMode{staged: os.ModeDir | os.ModeSymlink}},
		{name: "destination symlink", staged: staged, modes: map[string]os.FileMode{destination: os.ModeDir | os.ModeSymlink}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs := &accountSaveTestFS{
				entries: map[string]string{staged: "captured", destination: "previous"}, modes: tc.modes, renameErr: map[string]error{},
			}
			if err := commitAccountSaveDirectory(tc.staged, destination, fs.ops()); err == nil {
				t.Fatal("expected invalid commit paths to be rejected")
			}
			if fs.entries[destination] != "previous" || fs.entries[staged] != "captured" {
				t.Fatal("rejected commit changed an account")
			}
		})
	}
}

func TestAccountSaveDestinationRejectsEmptyAndRoot(t *testing.T) {
	root := filepath.VolumeName(filepath.Clean(os.TempDir())) + string(filepath.Separator)
	for _, path := range []string{"", " ", root} {
		if _, err := accountSaveDestination(path); err == nil {
			t.Errorf("accountSaveDestination(%q) succeeded", path)
		}
	}
}
