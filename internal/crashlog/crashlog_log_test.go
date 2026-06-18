package crashlog

import (
	"encoding/json"
	"os"
	"path/filepath"
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
