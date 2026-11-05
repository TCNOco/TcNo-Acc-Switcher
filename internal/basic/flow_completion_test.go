package basic

import (
	"errors"
	"testing"
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
