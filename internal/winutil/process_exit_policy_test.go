//go:build windows

package winutil

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestForceKillAndConfirmExitPolicy(t *testing.T) {
	inspectErr := errors.New("process table unavailable")
	killErr := errors.New("termination denied")
	for _, tc := range []struct {
		name       string
		running    bool
		inspectErr error
		killErr    error
		waitErr    error
		wantErr    error
		wantCalls  []string
	}{
		{name: "already exited", wantCalls: []string{"inspect"}},
		{name: "enumeration failed", inspectErr: inspectErr, wantErr: inspectErr, wantCalls: []string{"inspect"}},
		{name: "termination failed", running: true, killErr: killErr, wantErr: killErr, wantCalls: []string{"inspect", "terminate"}},
		{name: "exit timed out", running: true, waitErr: errProcessExitTimeout, wantErr: errProcessExitTimeout, wantCalls: []string{"inspect", "terminate", "wait"}},
		{name: "wait failed", running: true, waitErr: inspectErr, wantErr: inspectErr, wantCalls: []string{"inspect", "terminate", "wait"}},
		{name: "confirmed exit", running: true, wantCalls: []string{"inspect", "terminate", "wait"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			checkName := func(name string) {
				t.Helper()
				if name != "client.exe" {
					t.Fatalf("operation selected %q", name)
				}
			}
			err := forceKillAndConfirm("client.exe", func(name string) (bool, error) {
				checkName(name)
				calls = append(calls, "inspect")
				return tc.running, tc.inspectErr
			}, func(name string) error {
				checkName(name)
				calls = append(calls, "terminate")
				return tc.killErr
			}, func(name string) error {
				checkName(name)
				calls = append(calls, "wait")
				return tc.waitErr
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if !reflect.DeepEqual(calls, tc.wantCalls) {
				t.Fatalf("operations = %v, want %v", calls, tc.wantCalls)
			}
		})
	}
}

func TestConfirmStoppedProcessImagesChecksEveryTarget(t *testing.T) {
	for _, tc := range []struct {
		name    string
		running map[string]struct{}
		want    string
	}{
		{name: "both exited", running: map[string]struct{}{}},
		{name: "helper remains", running: map[string]struct{}{"helper.exe": {}}, want: "Helper.exe"},
		{name: "launcher remains", running: map[string]struct{}{"client.exe": {}}, want: "Client.EXE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			err := confirmStoppedProcessImages([]string{`C:\Launcher\Client.EXE`, "Helper", "SERVICE:Background", " "}, func() (map[string]struct{}, error) {
				calls++
				return tc.running, nil
			})
			if calls != 1 {
				t.Fatalf("process snapshots = %d, want one for all targets", calls)
			}
			if tc.want == "" {
				if err != nil {
					t.Fatalf("confirmed exit returned %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want remaining target %q", err, tc.want)
			}
		})
	}
}

func TestConfirmStoppedProcessImagesRejectsSnapshotFailure(t *testing.T) {
	want := errors.New("process table unavailable")
	err := confirmStoppedProcessImages([]string{"client.exe", "helper.exe"}, func() (map[string]struct{}, error) {
		return nil, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestConfirmStoppedProcessImagesSkipsNonImageTargets(t *testing.T) {
	if err := confirmStoppedProcessImages([]string{"SERVICE:Background", " "}, func() (map[string]struct{}, error) {
		t.Fatal("service-only confirmation inspected process images")
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestProcessImageRunningWithoutTargets(t *testing.T) {
	if running, err := ProcessImageRunning(nil); running || err != nil {
		t.Fatalf("empty process selection = %v, %v", running, err)
	}
}
