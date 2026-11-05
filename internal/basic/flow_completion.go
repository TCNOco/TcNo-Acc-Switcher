package basic

import "sync"

var (
	flowWarningMu       sync.RWMutex
	flowWarningReporter func(string, string)
)

func SetFlowWarningReporter(reporter func(titleKey, message string)) {
	flowWarningMu.Lock()
	flowWarningReporter = reporter
	flowWarningMu.Unlock()
}

func reportFlowWarning(titleKey, message string) {
	flowWarningMu.RLock()
	reporter := flowWarningReporter
	flowWarningMu.RUnlock()
	if reporter != nil {
		reporter(titleKey, message)
	}
}

func launchAfterCompletedSwitch(platformKey string, launch func() error) bool {
	return launchAfterCompletedOperation(platformKey, "Toast_AccountSwitchedLaunchFailed", launch, reportFlowWarning)
}

func launchAfterCompletedOperation(platformKey, warningKey string, launch func() error, warn func(string, string)) bool {
	if err := launch(); err != nil {
		logFlow().Warn("launcher failed after completed account operation", "platform", platformKey, "err", err)
		if warn != nil {
			warn(warningKey, err.Error())
		}
		return false
	}
	return true
}

func runSaveWithRelaunch(wasRunning, autoStart bool, save func() error, launch func() error, warn func(error)) error {
	saveErr := save()
	if wasRunning && autoStart {
		if err := launch(); err != nil && warn != nil {
			warn(err)
		}
	}
	return saveErr
}
