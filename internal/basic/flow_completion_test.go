package basic

import (
	"errors"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

func TestCompletedSwitchLaunchSuccessDoesNotWarn(t *testing.T) {
	launches, warnings := 0, 0
	started := launchAfterCompletedOperation("Test Platform", "warning-key", func() error {
		launches++
		return nil
	}, func(string, string) {
		warnings++
	})
	if !started || launches != 1 || warnings != 0 {
		t.Fatalf("started=%t launches=%d warnings=%d", started, launches, warnings)
	}
}

func TestCompletedSwitchLaunchFailureReportsWarning(t *testing.T) {
	launches, warnings := 0, 0
	var titleKey, message string
	started := launchAfterCompletedOperation("Test Platform", "warning-key", func() error {
		launches++
		return errors.New("launcher unavailable")
	}, func(key, text string) {
		warnings++
		titleKey, message = key, text
	})
	if started || launches != 1 || warnings != 1 || titleKey != "warning-key" || message != "launcher unavailable" {
		t.Fatalf("started=%t launches=%d warnings=%d title=%q message=%q", started, launches, warnings, titleKey, message)
	}
}

func TestCompletedSwitchLaunchFailureWithoutReporter(t *testing.T) {
	if launchAfterCompletedOperation("Test Platform", "warning-key", func() error {
		return errors.New("launcher unavailable")
	}, nil) {
		t.Fatal("failed launch reported success")
	}
}

func TestCompletedSwitchLaunchUsesRegisteredWarningReporter(t *testing.T) {
	var titleKey, message string
	SetFlowWarningReporter(func(key, text string) {
		titleKey, message = key, text
	})
	defer SetFlowWarningReporter(nil)
	if launchAfterCompletedSwitch("Test Platform", func() error {
		return errors.New("launcher unavailable")
	}) {
		t.Fatal("failed launch reported success")
	}
	if titleKey != "Toast_AccountSwitchedLaunchFailed" || message != "launcher unavailable" {
		t.Fatalf("title=%q message=%q", titleKey, message)
	}
}

func TestSaveRelaunchHonorsRunningStateAndAutoStart(t *testing.T) {
	for _, tc := range []struct {
		name       string
		wasRunning bool
		autoStart  bool
		wantLaunch bool
	}{
		{name: "closed", autoStart: true},
		{name: "disabled", wasRunning: true},
		{name: "closed and disabled"},
		{name: "running", wasRunning: true, autoStart: true, wantLaunch: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saves, launches := 0, 0
			err := runSaveWithRelaunch(tc.wasRunning, tc.autoStart, func() error {
				saves++
				return nil
			}, func() error {
				launches++
				return nil
			}, nil)
			wantLaunches := 0
			if tc.wantLaunch {
				wantLaunches = 1
			}
			if err != nil || saves != 1 || launches != wantLaunches {
				t.Fatalf("err=%v saves=%d launches=%d wantLaunch=%t", err, saves, launches, tc.wantLaunch)
			}
		})
	}
}

func TestSaveRelaunchFailureDoesNotFailSuccessfulSave(t *testing.T) {
	launchErr := errors.New("launcher unavailable")
	var warning error
	err := runSaveWithRelaunch(true, true, func() error {
		return nil
	}, func() error {
		return launchErr
	}, func(err error) {
		warning = err
	})
	if err != nil || warning != launchErr {
		t.Fatalf("save error=%v warning=%v", err, warning)
	}
}

func TestSaveRelaunchPreservesOriginalSaveFailure(t *testing.T) {
	saveErr := errors.New("session capture failed")
	launchErr := errors.New("launcher unavailable")
	var warning error
	launches := 0
	err := runSaveWithRelaunch(true, true, func() error {
		return saveErr
	}, func() error {
		launches++
		return launchErr
	}, func(err error) {
		warning = err
	})
	if err != saveErr || warning != launchErr || launches != 1 {
		t.Fatalf("save error=%v warning=%v launches=%d", err, warning, launches)
	}
}

func TestSaveReleasesLevelDBBeforeRelaunch(t *testing.T) {
	for _, saveErr := range []error{nil, errors.New("session capture failed")} {
		name := "success"
		if saveErr != nil {
			name = "failure"
		}
		t.Run(name, func(t *testing.T) {
			defer closeSharedLevelDBHandles("test.end")
			var db *leveldb.DB
			launched := false
			err := runSaveWithRelaunch(true, true, func() error {
				var err error
				db, err = leveldb.Open(storage.NewMemStorage(), nil)
				if err != nil {
					t.Fatalf("open database: %v", err)
				}
				// Model the handle retained by account ID / avatar resolution.
				// The save callback leaves it open for the flow to release.
				sharedLevelDBStore.mu.Lock()
				sharedLevelDBStore.handles["saved-session"] = &levelDBHandle{db: db}
				sharedLevelDBStore.mu.Unlock()
				return saveErr
			}, func() error {
				launched = true
				if _, err := db.Get([]byte("key"), nil); !errors.Is(err, leveldb.ErrClosed) {
					t.Fatalf("database still open when launcher starts: %v", err)
				}
				return nil
			}, nil)
			if err != saveErr || !launched {
				t.Fatalf("save error=%v want=%v launched=%t", err, saveErr, launched)
			}
		})
	}
}
