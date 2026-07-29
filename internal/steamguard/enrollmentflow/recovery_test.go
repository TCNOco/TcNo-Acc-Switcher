package enrollmentflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/enrollmentapi"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
)

var errFinalizeRefused = errors.New("finalize refused by Steam")

// acknowledgedPending drives a manager to the point where finalize is the only
// step left, which is where an already-activated authenticator gets stranded.
func acknowledgedPending(t *testing.T, api *fakeAPI) (*Manager, *memoryVault) {
	t.Helper()
	store := newMemoryVault()
	manager := newManager(api, store, time.Second)
	if _, err := manager.Start(context.Background(), startRequest()); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.AcknowledgeRevocationCode(testSteamID, []byte("R12345")); err != nil {
		t.Fatal(err)
	}
	return manager, store
}

func finalizeAfterRefusal(t *testing.T, manager *Manager) (Status, error) {
	t.Helper()
	return manager.Finalize(context.Background(), FinalizeRequest{
		SteamID: testSteamID, ConfirmationCode: []byte("8Y36B"), AuthenticatorTime: 1_700_000_030,
	})
}

// Adding an authenticator activates it on Steam; finalize only confirms the user
// saw the code. So a refused finalize can arrive when the authenticator is
// already live and its secrets are already on disk. Steam reporting the same
// token GID proves those secrets are the live authenticator, which is stronger
// than any finalize result, so the enrollment is committed.
func TestRefusedFinalizeCommitsWhenSteamReportsTheSameAuthenticator(t *testing.T) {
	api := &fakeAPI{
		addResult:    enrollmentapi.AddResult{State: enrollmentapi.StateAwaitingEmail, Pending: testPending()},
		finalizeErr:  errFinalizeRefused,
		statusResult: enrollmentapi.StatusResult{AuthenticatorType: 1, TokenGID: "token-gid"},
	}
	manager, store := acknowledgedPending(t, api)

	status, err := finalizeAfterRefusal(t, manager)
	if err != nil {
		t.Fatalf("refused finalize was not recovered: %v", err)
	}
	if status.State != enrollmentapi.StateComplete || status.Pending {
		t.Fatalf("status = %#v, want a completed enrollment", status)
	}

	parsed, err := mafile.ParsePlaintext(store.raw(testSteamID))
	if err != nil {
		t.Fatalf("stored record is not an active maFile: %v", err)
	}
	if !parsed.Account.FullyEnrolled || parsed.Account.RevocationCode != "R12345" {
		t.Fatalf("account was not committed as active: %#v", parsed.Account)
	}
	// The pending state is gone, so the account is usable rather than resumable.
	if _, err := manager.Resume(testSteamID); !errors.Is(err, ErrNoPendingEnrollment) {
		t.Fatalf("resume after recovery = %v, want %v", err, ErrNoPendingEnrollment)
	}
}

// A different authenticator on the account means these secrets are not the live
// one. Committing them would leave the user holding codes Steam rejects while
// the app claimed success, so the refusal stands.
func TestRefusedFinalizeKeepsPendingWhenAuthenticatorDiffers(t *testing.T) {
	for name, status := range map[string]enrollmentapi.StatusResult{
		"different authenticator": {AuthenticatorType: 1, TokenGID: "a-different-gid"},
		"no authenticator":        {},
	} {
		t.Run(name, func(t *testing.T) {
			api := &fakeAPI{
				addResult:    enrollmentapi.AddResult{State: enrollmentapi.StateAwaitingEmail, Pending: testPending()},
				finalizeErr:  errFinalizeRefused,
				statusResult: status,
			}
			manager, store := acknowledgedPending(t, api)

			if _, err := finalizeAfterRefusal(t, manager); !errors.Is(err, errFinalizeRefused) {
				t.Fatalf("err = %v, want the original refusal", err)
			}
			if _, err := mafile.ParsePlaintext(store.raw(testSteamID)); err == nil {
				t.Fatal("secrets were committed as active despite a mismatch")
			}
			// The enrollment is still resumable, so nothing was lost.
			resumed, err := manager.Resume(testSteamID)
			if err != nil || !resumed.Pending {
				t.Fatalf("resume = %#v, %v; want a still-pending enrollment", resumed, err)
			}
		})
	}
}

// If Steam cannot be reached the answer is unknown, so nothing is committed and
// both failures are reported rather than one hiding the other.
func TestRefusedFinalizeReportsBothFailuresWhenStatusIsUnavailable(t *testing.T) {
	statusErr := errors.New("status unavailable")
	api := &fakeAPI{
		addResult:   enrollmentapi.AddResult{State: enrollmentapi.StateAwaitingEmail, Pending: testPending()},
		finalizeErr: errFinalizeRefused,
		statusErr:   statusErr,
	}
	manager, store := acknowledgedPending(t, api)

	_, err := finalizeAfterRefusal(t, manager)
	if !errors.Is(err, errFinalizeRefused) || !errors.Is(err, statusErr) {
		t.Fatalf("err = %v, want both the refusal and the status failure", err)
	}
	if _, err := mafile.ParsePlaintext(store.raw(testSteamID)); err == nil {
		t.Fatal("secrets were committed without confirming the authenticator")
	}
}

// A successful finalize must not consult Steam's status at all: the normal path
// is unchanged by the recovery path existing.
func TestSuccessfulFinalizeDoesNotQueryStatus(t *testing.T) {
	api := &fakeAPI{
		addResult:      enrollmentapi.AddResult{State: enrollmentapi.StateAwaitingEmail, Pending: testPending()},
		finalizeResult: enrollmentapi.FinalizeResult{State: enrollmentapi.StateComplete, ServerTime: 1_700_000_030},
	}
	manager, _ := acknowledgedPending(t, api)

	if _, err := finalizeAfterRefusal(t, manager); err != nil {
		t.Fatal(err)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.statusCalls != 0 {
		t.Fatalf("status was queried %d times on the success path", api.statusCalls)
	}
}
