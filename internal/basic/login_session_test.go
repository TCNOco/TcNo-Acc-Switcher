package basic

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/platform"
)

func TestResetLoginPreservesFreshTokens(t *testing.T) {
	// A successful login refreshes a token after the initial Save Current.
	// Add New must capture that refreshed token before clearing the session.
	accounts := map[string]string{"a": "Player A", "b": "Player B"}
	snapshots := map[string]string{"Player A": "stale-a", "Player B": "stale-b"}
	for _, id := range []string{"a", "b"} {
		liveToken := "fresh-" + id
		err := resetLoginPreservingKnownSession(id, accounts, func(name string) error {
			snapshots[name] = liveToken
			return nil
		}, func() error {
			liveToken = ""
			return nil
		})
		if err != nil || liveToken != "" || snapshots[accounts[id]] != "fresh-"+id {
			t.Fatalf("reset %s: token=%q snapshots=%v err=%v", id, liveToken, snapshots, err)
		}
	}
	if snapshots["Player A"] != "fresh-a" || snapshots["Player B"] != "fresh-b" {
		t.Fatalf("both accounts must remain restorable: %v", snapshots)
	}
}

func TestResetLoginAbortsOnSaveFailure(t *testing.T) {
	saveErr := errors.New("snapshot failed")
	cleared := false
	err := resetLoginPreservingKnownSession("saved", map[string]string{"saved": "Player"}, func(string) error {
		return saveErr
	}, func() error {
		cleared = true
		return nil
	})
	if !errors.Is(err, saveErr) || cleared {
		t.Fatalf("failed save must preserve live session: err=%v cleared=%t", err, cleared)
	}
}

func TestResetLoginDoesNotSaveUnknownOrLoggedOutAccounts(t *testing.T) {
	for _, currentID := range []string{"", "unknown"} {
		var actions []string
		resetErr := errors.New("reset failed")
		err := resetLoginPreservingKnownSession(currentID, map[string]string{"saved": "Player"}, func(string) error {
			actions = append(actions, "save")
			return nil
		}, func() error {
			actions = append(actions, "reset")
			return resetErr
		})
		if !errors.Is(err, resetErr) || !reflect.DeepEqual(actions, []string{"reset"}) {
			t.Fatalf("currentID=%q: actions=%v err=%v", currentID, actions, err)
		}
	}
}

func TestRequiredLoginFiles(t *testing.T) {
	identity := func(path string) string { return path }
	root := filepath.Join("testdata", "login_session")
	for _, tc := range []struct {
		name    string
		pattern string
		valid   bool
	}{
		{"exact file", filepath.Join(root, "live", "session.cookie"), true},
		{"glob", filepath.Join(root, "live", "*.cookie"), true},
		{"missing file", filepath.Join(root, "missing.cookie"), false},
		{"empty file", filepath.Join(root, "empty.cookie"), false},
		{"directory", filepath.Join(root, "live"), false},
		{"invalid pattern", "[", false},
		{"empty path", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRequiredLoginFiles([]string{tc.pattern}, identity)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%t err=%v", tc.valid, err)
			}
		})
	}
	if err := validateRequiredLoginFiles([]string{filepath.Join(root, "live", "session.cookie"), filepath.Join(root, "missing.cookie")}, identity); err == nil {
		t.Fatal("every required pattern must have login data")
	}
}

func TestLoggedOutSessionCannotUseLeftoverIDOrReplaceSnapshot(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("testdata", "login_session"))
	if err != nil {
		t.Fatal(err)
	}
	d := platform.Descriptor{
		UniqueIdFile:       filepath.Join(root, "current_id.txt"),
		RequiredLoginFiles: []string{filepath.Join(root, "missing.cookie")},
	}
	if id, err := ReadUniqueID("Example App", d, root); err == nil || id != "" {
		t.Fatalf("logged-out state reported an active account: %q, %v", id, err)
	}
	// Validation must fail before any snapshot destination is opened or changed.
	err = saveCurrentAfterKill(FlowDeps{}, "Player", FlowContext{PlatformKey: "Example App", Descriptor: d})
	if err == nil || !strings.Contains(err.Error(), "required login file") {
		t.Fatalf("save accepted a logged-out state: %v", err)
	}
	d.RequiredLoginFiles = []string{filepath.Join(root, "live", "session.cookie")}
	if id, err := ReadUniqueID("Example App", d, root); err != nil || id != "old-correlation-id" {
		t.Fatalf("valid session changed existing account IDs: %q, %v", id, err)
	}
}

func TestRequiredSavedLoginFilesUseSnapshotPaths(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("testdata", "login_session"))
	if err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(root, "live", "session.cookie")
	fc := FlowContext{Descriptor: platform.Descriptor{
		LoginFiles:         map[string]string{live: "session.cookie"},
		RequiredLoginFiles: []string{live},
	}}
	if err := validateSavedLoginFiles(fc, filepath.Join(root, "saved")); err != nil {
		t.Fatalf("valid snapshot: %v", err)
	}
	if err := validateSavedLoginFiles(fc, filepath.Join(root, "missing-snapshot")); err == nil {
		t.Fatal("live login file hid a missing saved login file")
	}
}

func TestEACatalogRequiresRememberMeCookie(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "Platforms.json"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := platform.ParseDescriptor(raw, "EA Desktop")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`%LocalAppData%\Electronic Arts\EA Desktop\cookie.ini`}
	if !reflect.DeepEqual(d.RequiredLoginFiles, want) {
		t.Fatalf("required EA login files = %v; want %v", d.RequiredLoginFiles, want)
	}
}
