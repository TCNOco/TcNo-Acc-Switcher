package platform

import (
	"errors"
	"testing"
)

func preserveCredentialSessionBackupHandlers(t *testing.T) {
	t.Helper()
	credentialSessionBackups.RLock()
	backup, restore := credentialSessionBackups.backup, credentialSessionBackups.restore
	credentialSessionBackups.RUnlock()
	t.Cleanup(func() { SetCredentialSessionBackupHandlers(backup, restore) })
}

func TestCredentialSessionBackupHandlersRouteRequests(t *testing.T) {
	preserveCredentialSessionBackupHandlers(t)
	wantBackup := BackupResult{ArchivePath: "session.dpapi", Files: 3, Bytes: 123}
	wantRestore := BackupResult{ArchivePath: "session.dpapi", Files: 3, Bytes: 123, RestoredFrom: "session.dpapi"}
	var backedUp, restored string
	SetCredentialSessionBackupHandlers(
		func(platformKey string) (BackupResult, error) {
			backedUp = platformKey
			return wantBackup, nil
		},
		func(platformKey string) (BackupResult, error) {
			restored = platformKey
			return wantRestore, nil
		},
	)
	backup, err := runCredentialSessionBackup("Example", false)
	if err != nil || backup != wantBackup || backedUp != "Example" || restored != "" {
		t.Fatalf("backup = %+v, %v; backup target %q, restore target %q", backup, err, backedUp, restored)
	}
	restore, err := runCredentialSessionBackup("Example", true)
	if err != nil || restore != wantRestore || restored != "Example" {
		t.Fatalf("restore = %+v, %v; target %q", restore, err, restored)
	}
}

func TestCredentialSessionBackupUnavailableFailsClosed(t *testing.T) {
	preserveCredentialSessionBackupHandlers(t)
	SetCredentialSessionBackupHandlers(nil, nil)
	for _, restore := range []bool{false, true} {
		result, err := runCredentialSessionBackup("Example", restore)
		if err == nil || result != (BackupResult{}) {
			t.Fatalf("restore=%v: result=%+v, error=%v", restore, result, err)
		}
	}
}

func TestCredentialSessionBackupPropagatesFailure(t *testing.T) {
	preserveCredentialSessionBackupHandlers(t)
	failure := errors.New("encrypted backup unavailable")
	handler := func(string) (BackupResult, error) { return BackupResult{}, failure }
	SetCredentialSessionBackupHandlers(handler, handler)
	for _, restore := range []bool{false, true} {
		result, err := runCredentialSessionBackup("Example", restore)
		if !errors.Is(err, failure) || result != (BackupResult{}) {
			t.Fatalf("restore=%v: result=%+v, error=%v", restore, result, err)
		}
	}
}

func TestCredentialSessionBackupCallbacksRunWithoutServiceLocks(t *testing.T) {
	preserveCredentialSessionBackupHandlers(t)
	for _, restore := range []bool{false, true} {
		service := &PlatformService{}
		var serviceUnlocked, handlersUnlocked bool
		handler := func(string) (BackupResult, error) {
			serviceUnlocked = service.mu.TryLock()
			if serviceUnlocked {
				service.mu.Unlock()
			}
			handlersUnlocked = credentialSessionBackups.TryLock()
			if handlersUnlocked {
				credentialSessionBackups.Unlock()
			}
			return BackupResult{}, nil
		}
		SetCredentialSessionBackupHandlers(handler, handler)
		service.mu.Lock()
		if _, err := service.unlockAndBackupCredentialSession("Example", restore); err != nil {
			t.Fatal(err)
		}
		if !serviceUnlocked || !handlersUnlocked {
			t.Fatalf("restore=%v: service unlocked=%v, handlers unlocked=%v", restore, serviceUnlocked, handlersUnlocked)
		}
	}
}
