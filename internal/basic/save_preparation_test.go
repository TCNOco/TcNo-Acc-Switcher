package basic

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPrepareMissingSaveName(t *testing.T) {
	probeErr := errors.New("process inspection failed")
	closeErr := errors.New("close failed")
	readErr := errors.New("read failed")
	for _, tc := range []struct {
		name                        string
		running                     bool
		probeErr, closeErr, readErr error
		wantStopped                 bool
		wantCalls                   []string
		wantErr                     error
	}{
		{name: "running launcher", running: true, wantStopped: true, wantCalls: []string{"running", "close", "read"}},
		{name: "already closed", wantCalls: []string{"running"}},
		{name: "inspection failed", probeErr: probeErr, wantCalls: []string{"running"}, wantErr: probeErr},
		{name: "close failed", running: true, closeErr: closeErr, wantCalls: []string{"running", "close", "restart"}, wantErr: closeErr},
		{name: "read failed", running: true, readErr: readErr, wantCalls: []string{"running", "close", "read", "restart"}, wantErr: readErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			name, stopped, err := prepareMissingSaveName(
				func() (bool, error) { calls = append(calls, "running"); return tc.running, tc.probeErr },
				func() error { calls = append(calls, "close"); return tc.closeErr },
				func() (string, error) { calls = append(calls, "read"); return " Discord user ", tc.readErr },
				func() error { calls = append(calls, "restart"); return nil })
			if !errors.Is(err, tc.wantErr) || stopped != tc.wantStopped || !reflect.DeepEqual(calls, tc.wantCalls) {
				t.Fatalf("name=%q stopped=%t err=%v calls=%v", name, stopped, err, calls)
			}
			if stopped && name != "Discord user" {
				t.Fatalf("unexpected suggested name: %q", name)
			}
		})
	}
}

func TestCancelPreparedSaveRestartsOnlyItsPlatformOnce(t *testing.T) {
	restarts := 0
	b := &BasicService{preparedSave: &preparedSaveCurrent{
		flow:    FlowContext{PlatformKey: "Discord PTB"},
		restart: func() error { restarts++; return nil },
	}}
	if err := b.CancelPreparedSave("Discord"); err != nil || restarts != 0 || b.preparedSave == nil {
		t.Fatalf("different platform: err=%v restarts=%d", err, restarts)
	}
	for i := 0; i < 2; i++ {
		if err := b.CancelPreparedSave("Discord PTB"); err != nil {
			t.Fatal(err)
		}
	}
	if restarts != 1 || b.preparedSave != nil {
		t.Fatalf("restarts=%d pending=%t", restarts, b.preparedSave != nil)
	}
}

func TestPreparedSaveFailureRestartsLauncher(t *testing.T) {
	newFlowTestEnv(t)
	restarts := 0
	b := &BasicService{preparedSave: &preparedSaveCurrent{
		flow:    FlowContext{PlatformKey: "Discord PTB"},
		restart: func() error { restarts++; return nil },
	}}
	// Invalid input fails before touching the filesystem, but the launcher
	// closed to prepare the dialog must still be restored.
	if err := b.SaveCurrent("Discord PTB", ""); err == nil || restarts != 1 || b.preparedSave != nil {
		t.Fatalf("err=%v restarts=%d pending=%t", err, restarts, b.preparedSave != nil)
	}
}

func TestPreparedSaveCompletesAndRestartsLauncherOnce(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	fc.Settings.AutoStart = false
	mustWrite(t, filepath.Join(env.instDir, "config.cfg"), "saved session")
	restarts := 0
	b := &BasicService{preparedSave: &preparedSaveCurrent{
		flow:    fc,
		restart: func() error { restarts++; return nil },
	}}
	if err := b.SaveCurrent(fc.PlatformKey, " Player "); err != nil {
		t.Fatal(err)
	}
	if b.preparedSave != nil || restarts != 1 {
		t.Fatalf("pending=%t restarts=%d", b.preparedSave != nil, restarts)
	}
	if got := mustRead(t, filepath.Join(env.cacheDir("Player"), "Saved", "config.cfg")); got != "saved session" {
		t.Fatalf("saved config = %q", got)
	}
	if err := b.CancelPreparedSave(fc.PlatformKey); err != nil || restarts != 1 {
		t.Fatalf("cancel after save: err=%v restarts=%d", err, restarts)
	}
}
