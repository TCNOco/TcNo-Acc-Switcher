package basic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddNewResetsLoginCookiesWithoutTouchingSavedAccounts(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	config := filepath.Join(env.instDir, "Battle.net.config")
	fc.Descriptor.PathListToClear = []string{"JSON_EMPTY_VALUE::" + config + "::Client.SavedAccountNames"}
	for _, name := range []string{"Cookies", "Cookies-journal", "Cookies-wal", "Cookies-shm"} {
		path := filepath.Join(env.instDir, "BrowserCaches", "common", "Network", name)
		fc.Descriptor.AddNewFilesToClear = append(fc.Descriptor.AddNewFilesToClear, path)
		mustWrite(t, path, "remembered login")
	}
	kept := []string{
		filepath.Join(env.instDir, "BrowserCaches", "common", "Network", "Network Persistent State"),
		filepath.Join(env.instDir, "BrowserCaches", "123", "Network", "Cookies"),
		filepath.Join(env.instDir, "Account", "123", "account.db"),
		filepath.Join(env.instDir, "CachedData.db"),
		filepath.Join(env.cacheDir("saved-account"), "email.json"),
	}
	for _, path := range kept {
		mustWrite(t, path, "saved data")
	}
	mustWrite(t, config, `{"Client":{"SavedAccountNames":"old@example.invalid","AutoLogin":"true"},"Games":{"installed":true}}`)
	// Ordinary switches must not reset the shared login browser.
	if err := ClearCurrentLogin(FlowDeps{}, fc); err != nil {
		t.Fatal(err)
	}
	for _, path := range fc.Descriptor.AddNewFilesToClear {
		if got := mustRead(t, path); got != "remembered login" {
			t.Fatalf("normal account switch changed %s", path)
		}
	}
	if err := clearLoginForAddNew(FlowDeps{}, fc); err != nil {
		t.Fatal(err)
	}
	for _, path := range fc.Descriptor.AddNewFilesToClear {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("login cookie file was not reset: %s (%v)", path, err)
		}
	}
	for _, path := range kept {
		if got := mustRead(t, path); got != "saved data" {
			t.Fatalf("Add New changed unrelated or saved data: %s", path)
		}
	}
	if got := mustRead(t, config); !strings.Contains(got, `"SavedAccountNames":""`) || !strings.Contains(got, `"installed":true`) {
		t.Fatalf("unexpected config after Add New: %s", got)
	}
	// Cold installs and already-cleared files must also work.
	if err := clearLoginForAddNew(FlowDeps{}, fc); err != nil {
		t.Fatal(err)
	}
}

func TestAddNewValidatesAllLoginFilesBeforeChangingAnything(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	config := filepath.Join(env.instDir, "Battle.net.config")
	before := `{"Client":{"SavedAccountNames":"old@example.invalid"}}`
	mustWrite(t, config, before)
	fc.Descriptor.PathListToClear = []string{"JSON_EMPTY_VALUE::" + config + "::Client.SavedAccountNames"}
	cookies := filepath.Join(env.instDir, "Cookies")
	mustWrite(t, cookies, "remembered login")
	for _, invalid := range []string{"", "Cookies", filepath.VolumeName(env.instDir) + string(filepath.Separator), filepath.Join(env.instDir, "Cookies*"), env.instDir, env.instDir + string(filepath.Separator) + ".." + string(filepath.Separator) + "Cookies"} {
		fc.Descriptor.AddNewFilesToClear = []string{cookies, invalid}
		if err := clearLoginForAddNew(FlowDeps{}, fc); err == nil {
			t.Fatalf("accepted invalid Add New login file: %q", invalid)
		}
		if got := mustRead(t, config); got != before {
			t.Fatalf("invalid target changed the account selection: %s", got)
		}
		if got := mustRead(t, cookies); got != "remembered login" {
			t.Fatal("invalid target caused a partial cookie reset")
		}
	}
}
