package enrollmentflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/enrollmentapi"
	"TcNo-Acc-Switcher/internal/steamguard/loginrecord"
	"TcNo-Acc-Switcher/internal/steamguard/vaultrecord"
)

func loginOnlyRecord(t *testing.T) []byte {
	t.Helper()
	raw, err := loginrecord.Encode(
		loginrecord.New(testSteamID, "tcno", "access-token-secret", "refresh-token-secret"),
	)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func occupiedManager(t *testing.T, occupant []byte, api *fakeAPI) (*Manager, *memoryVault) {
	t.Helper()
	store := newMemoryVault()
	if _, err := store.PutRecord(steamIDString(testSteamID), occupant); err != nil {
		t.Fatal(err)
	}
	return newManager(api, store, time.Second), store
}

func pendingAPI() *fakeAPI {
	return &fakeAPI{addResult: enrollmentapi.AddResult{
		State: enrollmentapi.StateAwaitingSMS, Pending: testPending(),
	}}
}

// ReplaceLoginOnly narrows the already-enrolled refusal to one record shape.
// Widening it by accident would let an enrollment overwrite an authenticator's
// shared secret, identity secret and revocation code, none of which exist
// anywhere else.
func TestStartReplacesOnlyLoginOnlyRecordsAndOnlyWhenAsked(t *testing.T) {
	t.Run("refuses a login-only record without the flag", func(t *testing.T) {
		manager, _ := occupiedManager(t, loginOnlyRecord(t), pendingAPI())
		if _, err := manager.Start(context.Background(), startRequest()); !errors.Is(err, ErrAlreadyEnrolled) {
			t.Fatalf("start over a login-only record = %v, want ErrAlreadyEnrolled", err)
		}
	})

	t.Run("promotes a login-only record when asked", func(t *testing.T) {
		manager, store := occupiedManager(t, loginOnlyRecord(t), pendingAPI())
		request := startRequest()
		request.ReplaceLoginOnly = true
		status, err := manager.Start(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if !status.Pending || status.State != enrollmentapi.StateAwaitingSMS {
			t.Fatalf("unexpected promotion status: %#v", status)
		}
		if kind := vaultrecord.Sniff(store.raw(testSteamID)); kind != vaultrecord.KindEnrollmentPending {
			t.Fatalf("record kind after promotion = %v, want enrollment-pending", kind)
		}
	})

	t.Run("still refuses an authenticator with the flag set", func(t *testing.T) {
		authenticator := []byte(`{"shared_secret":"c2hhcmVk","account_name":"tcno"}`)
		api := pendingAPI()
		manager, store := occupiedManager(t, authenticator, api)
		request := startRequest()
		request.ReplaceLoginOnly = true
		if _, err := manager.Start(context.Background(), request); !errors.Is(err, ErrAlreadyEnrolled) {
			t.Fatalf("start over an authenticator = %v, want ErrAlreadyEnrolled", err)
		}
		if adds, _ := api.counts(); adds != 0 {
			t.Fatalf("Steam was asked to add an authenticator %d times before the refusal", adds)
		}
		if string(store.raw(testSteamID)) != string(authenticator) {
			t.Fatal("the authenticator record was modified by a refused promotion")
		}
	})

	// The whole promotion is only safe because the session survives a refusal:
	// the record is replaced after Steam accepts, never before.
	t.Run("keeps the session when Steam refuses", func(t *testing.T) {
		occupant := loginOnlyRecord(t)
		api := &fakeAPI{addErr: errors.New("steam said no")}
		manager, store := occupiedManager(t, occupant, api)
		request := startRequest()
		request.ReplaceLoginOnly = true
		if _, err := manager.Start(context.Background(), request); err == nil {
			t.Fatal("start reported success despite the API failing")
		}
		if string(store.raw(testSteamID)) != string(occupant) {
			t.Fatal("the login-only session was lost to a failed promotion")
		}
	})
}
