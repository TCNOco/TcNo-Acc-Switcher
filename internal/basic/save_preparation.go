package basic

import (
	"errors"
	"fmt"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/winutil"
)

// A missing Electron account name can mean the launcher has not flushed its
// local storage yet. Keep it closed between preparation and the save dialog's
// result so saving does not need to close and restart it a second time.
type preparedSaveCurrent struct {
	flow    FlowContext
	restart func() error
}

func (p *preparedSaveCurrent) save(deps FlowDeps, accountName string) (err error) {
	defer finishActionBarStatus(&err)
	defer closeSharedLevelDBHandles("PreparedSaveCurrent.end")
	platform.EmitActionBarStatusI18n("Status_ActionBar_SavingSession")
	return runSaveWithRelaunch(true, true,
		func() error { return saveCurrentAfterKill(deps, accountName, p.flow) },
		p.restart,
		func(err error) { reportFlowWarning("Toast_LaunchFailed", err.Error()) })
}

// PrepareSaveCurrent reads the suggested name without interrupting a launcher
// when it is already available. LevelDB launchers with a missing name are
// closed first and remain closed until SaveCurrent or CancelPreparedSave.
func (b *BasicService) PrepareSaveCurrent(platformKey string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return "", err
	}
	platformKey = strings.TrimSpace(platformKey)
	if b.preparedSave != nil {
		return "", fmt.Errorf("a save dialog is already being prepared")
	}
	name, readErr := b.suggestedSaveAccountName(platformKey)
	if strings.TrimSpace(name) != "" || platformKey == "" {
		return name, readErr
	}
	d, _, err := readDescriptor(platformKey)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(strings.TrimSpace(d.UniqueIdMethod), "LEVELDB") ||
		(!d.ExitBeforeInteract && !d.ExitBeforeSave) {
		return name, readErr
	}
	fc, err := PrepareFlow(b.deps(), platformKey)
	if err != nil {
		return "", err
	}
	readName := func() (string, error) { return b.suggestedSaveAccountName(platformKey) }
	restart := func() error {
		closeSharedLevelDBHandles("PrepareSaveCurrent.beforeRestart")
		return launchBasicNoStatus(b.deps(), platformKey, nil)
	}
	name, stopped, err := prepareMissingSaveName(
		func() (bool, error) { return winutil.ProcessImageRunning(d.ExesToEnd) },
		func() error {
			closeSharedLevelDBHandles("PrepareSaveCurrent.beforeClose")
			return killPlatformExes(b.deps(), fc)
		}, readName, restart)
	if stopped {
		b.preparedSave = &preparedSaveCurrent{flow: fc, restart: restart}
	}
	return name, err
}

// prepareMissingSaveName restores a launcher on preparation failure. A successful
// close transfers responsibility for restarting it to the save/cancel operation.
func prepareMissingSaveName(running func() (bool, error), closeLauncher func() error, readName func() (string, error), restart func() error) (string, bool, error) {
	wasRunning, err := running()
	if err != nil || !wasRunning {
		return "", false, err
	}
	if err := closeLauncher(); err != nil {
		return "", false, errors.Join(err, restart())
	}
	name, err := readName()
	if err != nil {
		return "", false, errors.Join(err, restart())
	}
	return strings.TrimSpace(name), true, nil
}

// CancelPreparedSave restores only a launcher closed by preparation. It does
// not save or clear any account data, and is a no-op after SaveCurrent.
func (b *BasicService) CancelPreparedSave(platformKey string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.preparedSave == nil || b.preparedSave.flow.PlatformKey != strings.TrimSpace(platformKey) {
		return nil
	}
	prepared := b.preparedSave
	b.preparedSave = nil
	closeSharedLevelDBHandles("CancelPreparedSave.beforeRestart")
	return prepared.restart()
}
