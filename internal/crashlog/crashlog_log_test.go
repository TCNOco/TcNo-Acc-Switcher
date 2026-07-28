package crashlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/actionlog"
)

func TestCaptureDump_includesLogField(t *testing.T) {
	dir := t.TempDir()
	origResolver := crashDumpDirResolver
	crashDumpDirResolver = func() (string, error) { return dir, nil }
	t.Cleanup(func() { crashDumpDirResolver = origResolver })

	actionlog.Init()
	actionlog.Record("file:write", "test.txt", "", nil)

	origExit := osExit
	osExit = func(int) {}
	t.Cleanup(func() { osExit = origExit })

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer Capture()
		panic("test panic for log field")
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Capture() did not return")
	}

	data, err := os.ReadFile(filepath.Join(dir, crashDumpFile))
	if err != nil {
		t.Fatal(err)
	}
	var dump CrashDump
	if err := json.Unmarshal(data, &dump); err != nil {
		t.Fatal(err)
	}
	if dump.Log == "" {
		t.Fatal("expected non-empty Log field in crash dump")
	}
	if dump.OSInfo == "" {
		t.Fatal("expected non-empty OSInfo field in crash dump")
	}
	if dump.Stack == "" || dump.Error == "" {
		t.Fatal("expected stack and error in crash dump")
	}
}

func TestCaptureDump_redactsPanicAndActionLogSecrets(t *testing.T) {
	dir := t.TempDir()
	origResolver := crashDumpDirResolver
	crashDumpDirResolver = func() (string, error) { return dir, nil }
	t.Cleanup(func() { crashDumpDirResolver = origResolver })

	actionlog.Init()
	actionlog.Record(
		"steamguard:confirm",
		"accountID=76561198123456789 username=ordinary_username",
		`{"access_token":"CRASH_LOG_TOKEN_SENTINEL"}`,
		nil,
	)

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer Capture()
		panic(fmt.Errorf("wrapped panic: shared_secret=%s username=ordinary_username", "PANIC_SECRET_SENTINEL"))
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Capture() did not return")
	}

	data, err := os.ReadFile(filepath.Join(dir, crashDumpFile))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, secret := range []string{"CRASH_LOG_TOKEN_SENTINEL", "PANIC_SECRET_SENTINEL"} {
		if strings.Contains(text, secret) {
			t.Fatalf("secret %q leaked in crash dump: %s", secret, text)
		}
	}
	for _, public := range []string{"76561198123456789", "ordinary_username"} {
		if !strings.Contains(text, public) {
			t.Fatalf("public identifier %q was removed from crash dump: %s", public, text)
		}
	}
}
