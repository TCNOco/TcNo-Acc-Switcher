//go:build windows

package winutil

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestBuildDetachedCommandLineQuotesArgv0AndArgs(t *testing.T) {
	got := buildDetachedCommandLine(`C:\Program Files (x86)\Steam\steam.exe`, []string{"-silent", `-login`, `a b`})
	want := `"C:\Program Files (x86)\Steam\steam.exe" -silent -login "a b"`
	if got != want {
		t.Fatalf("command line\n got: %s\nwant: %s", got, want)
	}
}

// The empty title placeholder in `cmd /c start "" <exe>` must survive as an empty quoted arg,
// otherwise start treats the executable path as the window title and launches nothing.
func TestBuildDetachedCommandLineKeepsEmptyArg(t *testing.T) {
	got := buildDetachedCommandLine(`C:\Windows\System32\cmd.exe`, []string{"/c", "start", "", `C:\game.exe`})
	want := `C:\Windows\System32\cmd.exe /c start "" C:\game.exe`
	if got != want {
		t.Fatalf("command line\n got: %s\nwant: %s", got, want)
	}
}

func TestDetachedCreationFlagsAlwaysDecouple(t *testing.T) {
	for _, hide := range []bool{false, true} {
		flags := detachedCreationFlags(hide)
		if flags&windows.CREATE_NEW_PROCESS_GROUP == 0 {
			t.Errorf("hideWindow=%v: missing CREATE_NEW_PROCESS_GROUP", hide)
		}
		// DETACHED_PROCESS is mutually exclusive with the console flags below; setting both
		// makes CreateProcess fail outright with ERROR_INVALID_PARAMETER.
		if flags&windows.DETACHED_PROCESS != 0 {
			t.Errorf("hideWindow=%v: DETACHED_PROCESS must not be combined with a new console", hide)
		}
		wantConsole := uint32(windows.CREATE_NEW_CONSOLE)
		if hide {
			wantConsole = windows.CREATE_NO_WINDOW
		}
		if flags&wantConsole == 0 {
			t.Errorf("hideWindow=%v: child would inherit our console (flags=%#x)", hide, flags)
		}
	}
}
