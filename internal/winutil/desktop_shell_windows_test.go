//go:build windows

package winutil

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/actionlog"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

func TestLaunchAsDesktopUserFallback(t *testing.T) {
	shellFailure := errors.New("desktop automation unavailable")
	tokenFailure := errors.New("desktop token denied")
	for _, tc := range []struct {
		name     string
		shellErr error
		tokenErr error
		want     []string
	}{
		{"Explorer accepted", nil, nil, []string{"explorer"}},
		{"token fallback accepted", shellFailure, nil, []string{"explorer", "desktop_token"}},
		{"both failed", shellFailure, tokenFailure, []string{"explorer", "desktop_token"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls, recorded []string
			err := launchAsDesktopUser(func() error {
				calls = append(calls, "explorer")
				return tc.shellErr
			}, func() error {
				calls = append(calls, "desktop_token")
				return tc.tokenErr
			}, func(mode string, err error) {
				recorded = append(recorded, mode)
				want := tc.shellErr
				if mode == "desktop_token" {
					want = tc.tokenErr
				}
				if err != want {
					t.Fatalf("%s recorded %v, want %v", mode, err, want)
				}
			})
			if !reflect.DeepEqual(calls, tc.want) || !reflect.DeepEqual(recorded, tc.want) {
				t.Fatalf("launches = %v, recorded = %v, want %v", calls, recorded, tc.want)
			}
			if tc.shellErr != nil && tc.tokenErr != nil {
				if !errors.Is(err, shellFailure) || !errors.Is(err, tokenFailure) {
					t.Fatalf("lost launch failures: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDesktopShellAutomation(t *testing.T) {
	// Read-only integration check: resolve the actual desktop COM interfaces and
	// method name without launching a program or changing the user's session.
	if os.Getenv("TCNO_TEST_DESKTOP_SHELL") != "1" {
		t.Skip("set TCNO_TEST_DESKTOP_SHELL=1 to check the interactive desktop COM server")
	}
	if windows.GetShellWindow() == 0 {
		t.Skip("no interactive desktop shell")
	}
	if err := withDesktopShell(func(shell *ole.IDispatch) error {
		_, err := shell.GetSingleIDOfName("ShellExecute")
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestShellExecuteParametersRoundTrip(t *testing.T) {
	want := []string{"-login", "two words", "", `quote"inside`, `C:\with spaces\`, "日本語", "&|<>^%PATH%"}
	got, err := windows.DecomposeCommandLine("launcher.exe " + shellExecuteParameters(want))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got[1:], want) {
		t.Fatalf("arguments = %#v, want %#v", got[1:], want)
	}
}

func TestStartFailureLogsNoArguments(t *testing.T) {
	actionlog.Init()
	defer actionlog.Init()
	if err := Start("", []string{"-login", "user-sentinel", "password-sentinel"}, StartOpts{}); err == nil {
		t.Fatal("empty launch succeeded")
	}
	log := actionlog.SnapshotPruned(100, 100)
	for _, want := range []string{"process:start", "outcome=fail", "argument_count=3", "empty executable",
		fmt.Sprintf("app_elevated=%t", IsProcessElevated()), "run_as_admin=false", "as_desktop_user=false", "launch_mode=detached"} {
		if !strings.Contains(log, want) {
			t.Fatalf("launch log is missing %q: %s", want, log)
		}
	}
	for _, secret := range []string{"-login", "user-sentinel", "password-sentinel"} {
		if strings.Contains(log, secret) {
			t.Fatalf("launch log contains argument %q", secret)
		}
	}
}

func TestDesktopLaunchRejectsEmptyExecutable(t *testing.T) {
	if err := startAsDesktopUser(" \t", nil, StartOpts{}); err == nil || !strings.Contains(err.Error(), "empty executable") {
		t.Fatalf("empty launch returned %v", err)
	}
}
