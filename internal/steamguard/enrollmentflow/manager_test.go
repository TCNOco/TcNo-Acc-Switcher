package enrollmentflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/enrollmentapi"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

const testSteamID = uint64(76561198000000001)

var errTestStorage = errors.New("test storage failure")

type fakeAPI struct {
	mu             sync.Mutex
	addResult      enrollmentapi.AddResult
	addErr         error
	finalizeResult enrollmentapi.FinalizeResult
	finalizeErr    error
	addCalls       int
	finalizeCalls  int
}

func (f *fakeAPI) AddAuthenticator(context.Context, enrollmentapi.AddRequest, time.Duration) (enrollmentapi.AddResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.addCalls++
	return f.addResult, f.addErr
}

func (f *fakeAPI) FinalizeAddAuthenticator(context.Context, enrollmentapi.FinalizeRequest, time.Duration) (enrollmentapi.FinalizeResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.finalizeCalls++
	return f.finalizeResult, f.finalizeErr
}

func (f *fakeAPI) counts() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.addCalls, f.finalizeCalls
}

type memoryVault struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
	failPut bool
	deleted int
}

type memoryEntry struct {
	id  string
	raw []byte
}

func newMemoryVault() *memoryVault { return &memoryVault{entries: make(map[string]memoryEntry)} }

func (v *memoryVault) PutRecord(steamID string, plaintext []byte) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.failPut {
		return "", errTestStorage
	}
	id := "record-" + steamID
	previous := v.entries[steamID]
	wipe(previous.raw)
	v.entries[steamID] = memoryEntry{id: id, raw: append([]byte(nil), plaintext...)}
	return id, nil
}

func (v *memoryVault) GetRecord(id string) ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, entry := range v.entries {
		if entry.id == id {
			return append([]byte(nil), entry.raw...), nil
		}
	}
	return nil, vault.ErrNotFound
}

func (v *memoryVault) ListRecords() ([]vault.RecordInfo, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make([]vault.RecordInfo, 0, len(v.entries))
	for steamID, entry := range v.entries {
		out = append(out, vault.RecordInfo{ID: entry.id, SteamID64: steamID})
	}
	return out, nil
}

func (v *memoryVault) DeleteRecord(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	for steamID, entry := range v.entries {
		if entry.id == id {
			wipe(entry.raw)
			delete(v.entries, steamID)
			v.deleted++
			return nil
		}
	}
	return vault.ErrNotFound
}

func (v *memoryVault) raw(steamID uint64) []byte {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]byte(nil), v.entries[steamIDString(steamID)].raw...)
}

func testPending() *enrollmentapi.PendingEnrollment {
	return &enrollmentapi.PendingEnrollment{
		RequestID:      bytes.Repeat([]byte{0x11}, 16),
		SteamID:        testSteamID,
		AccessToken:    []byte("access-token-secret"),
		DeviceID:       "android:00112233-4455-4677-8899-aabbccddeeff",
		SharedSecret:   bytes.Repeat([]byte{0x21}, 20),
		IdentitySecret: bytes.Repeat([]byte{0x31}, 20),
		Secret1:        bytes.Repeat([]byte{0x41}, 20),
		RevocationCode: []byte("R12345"),
		URI:            []byte("otpauth://totp/Steam:test?secret=EXAMPLE"),
		SerialNumber:   123456789,
		ServerTime:     1_700_000_000,
		AccountName:    "test-account",
		TokenGID:       "token-gid",
		PhoneHint:      "  ***42  ",
		Confirmation:   enrollmentapi.ConfirmationSMS,
	}
}

func startRequest() StartRequest {
	return StartRequest{SteamID: testSteamID, AccessToken: []byte("access-token-secret"), AuthenticatorTime: 1_700_000_000}
}

func TestStartPersistsSecretStateAndResumeReturnsOnlyProjection(t *testing.T) {
	pending := testPending()
	api := &fakeAPI{addResult: enrollmentapi.AddResult{State: enrollmentapi.StateAwaitingSMS, Pending: pending}}
	store := newMemoryVault()
	manager := newManager(api, store, time.Second)

	status, err := manager.Start(context.Background(), startRequest())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Pending || status.State != enrollmentapi.StateAwaitingSMS || status.PhoneHint != "***42" || !status.RevocationViewAvailable {
		t.Fatalf("unexpected status: %#v", status)
	}
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"access-token-secret", "R12345", "ISEhISEh"} {
		if bytes.Contains(encoded, []byte(secret)) {
			t.Fatalf("status exposed secret %q", secret)
		}
	}
	if pending.SteamID != 0 || pending.AccessToken != nil || pending.SharedSecret != nil {
		t.Fatal("API pending result was not destroyed after persistence")
	}

	resumed, err := newManager(api, store, time.Second).Resume(testSteamID)
	if err != nil {
		t.Fatal(err)
	}
	if !resumed.Resumed || resumed.State != enrollmentapi.StateAwaitingSMS || !resumed.RevocationViewAvailable {
		t.Fatalf("unexpected resumed status: %#v", resumed)
	}
	addCalls, _ := api.counts()
	if addCalls != 1 {
		t.Fatalf("add calls = %d, want 1", addCalls)
	}
}

func TestStartIsIdempotentForPersistedPendingEnrollment(t *testing.T) {
	api := &fakeAPI{addResult: enrollmentapi.AddResult{State: enrollmentapi.StateAwaitingSMS, Pending: testPending()}}
	store := newMemoryVault()
	manager := newManager(api, store, time.Second)
	if _, err := manager.Start(context.Background(), startRequest()); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Start(context.Background(), startRequest()); err != nil {
		t.Fatal(err)
	}
	addCalls, _ := api.counts()
	if addCalls != 1 {
		t.Fatalf("add calls = %d, want 1", addCalls)
	}
}

func TestRevealCanResumeAfterCrashUntilAcknowledged(t *testing.T) {
	api := &fakeAPI{addResult: enrollmentapi.AddResult{State: enrollmentapi.StateAwaitingSMS, Pending: testPending()}}
	store := newMemoryVault()
	manager := newManager(api, store, time.Second)
	if _, err := manager.Start(context.Background(), startRequest()); err != nil {
		t.Fatal(err)
	}
	view, err := manager.RevealRevocationCode(testSteamID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Code != "R12345" || view.String() != "Steam Guard revocation view (secret redacted)" {
		t.Fatalf("unexpected revocation view: %q", view.Code)
	}
	view.Destroy()
	if view.Code != "" {
		t.Fatal("Destroy did not clear DTO field")
	}
	restarted := newManager(api, store, time.Second)
	view, err = restarted.RevealRevocationCode(testSteamID)
	if err != nil || view.Code != "R12345" {
		t.Fatalf("restart reveal=%#v err=%v", view, err)
	}
	if _, err := restarted.AcknowledgeRevocationCode(testSteamID, []byte("R12345")); err != nil {
		t.Fatal(err)
	}
	resumed, err := newManager(api, store, time.Second).Resume(testSteamID)
	if err != nil || resumed.RevocationViewAvailable {
		t.Fatalf("acknowledged resume=%#v err=%v", resumed, err)
	}
	if _, err := newManager(api, store, time.Second).RevealRevocationCode(testSteamID); !errors.Is(err, ErrRevocationCodeAlreadyAcknowledged) {
		t.Fatalf("reveal after acknowledgment error = %v", err)
	}
}

func TestAcknowledgmentFailurePreservesReissuablePendingState(t *testing.T) {
	api := &fakeAPI{addResult: enrollmentapi.AddResult{State: enrollmentapi.StateAwaitingSMS, Pending: testPending()}}
	store := newMemoryVault()
	manager := newManager(api, store, time.Second)
	if _, err := manager.Start(context.Background(), startRequest()); err != nil {
		t.Fatal(err)
	}
	store.failPut = true
	if _, err := manager.AcknowledgeRevocationCode(testSteamID, []byte("R12345")); !errors.Is(err, errTestStorage) {
		t.Fatalf("acknowledgment error=%v", err)
	}
	store.failPut = false
	view, err := newManager(api, store, time.Second).RevealRevocationCode(testSteamID)
	if err != nil || view.Code != "R12345" {
		t.Fatalf("restart view=%#v err=%v", view, err)
	}
}

func TestLegacyViewedPendingMigratesToReissuableUnacknowledgedState(t *testing.T) {
	api := &fakeAPI{addResult: enrollmentapi.AddResult{State: enrollmentapi.StateAwaitingSMS, Pending: testPending()}}
	store := newMemoryVault()
	manager := newManager(api, store, time.Second)
	if _, err := manager.Start(context.Background(), startRequest()); err != nil {
		t.Fatal(err)
	}
	legacy, err := decodePending(store.raw(testSteamID))
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.destroy()
	legacy.Version = legacyPendingVersion
	legacy.RevocationViewed = true
	legacy.RevocationAcknowledged = false
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutRecord(steamIDString(testSteamID), raw); err != nil {
		t.Fatal(err)
	}
	restarted := newManager(api, store, time.Second)
	resumed, err := restarted.Resume(testSteamID)
	if err != nil || !resumed.RevocationViewAvailable {
		t.Fatalf("legacy resume=%#v err=%v", resumed, err)
	}
	view, err := restarted.RevealRevocationCode(testSteamID)
	if err != nil || view.Code != "R12345" {
		t.Fatalf("legacy reveal=%#v err=%v", view, err)
	}
}

func TestFinalizeRequiresPersistedRevocationAcknowledgment(t *testing.T) {
	api := &fakeAPI{
		addResult:      enrollmentapi.AddResult{State: enrollmentapi.StateAwaitingSMS, Pending: testPending()},
		finalizeResult: enrollmentapi.FinalizeResult{State: enrollmentapi.StateComplete},
	}
	store := newMemoryVault()
	manager := newManager(api, store, time.Second)
	if _, err := manager.Start(context.Background(), startRequest()); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Finalize(context.Background(), FinalizeRequest{
		SteamID: testSteamID, ConfirmationCode: []byte("12345"), AuthenticatorTime: 1_700_000_030,
	}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("finalize without acknowledgment error=%v", err)
	}
	_, finalizeCalls := api.counts()
	if finalizeCalls != 0 {
		t.Fatal("unacknowledged enrollment reached remote finalization")
	}
}

func TestFinalizeCommitsCanonicalActiveMaFile(t *testing.T) {
	api := &fakeAPI{
		addResult:      enrollmentapi.AddResult{State: enrollmentapi.StateAwaitingSMS, Pending: testPending()},
		finalizeResult: enrollmentapi.FinalizeResult{State: enrollmentapi.StateComplete, ServerTime: 1_700_000_030},
	}
	store := newMemoryVault()
	manager := newManager(api, store, time.Second)
	if _, err := manager.Start(context.Background(), startRequest()); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.AcknowledgeRevocationCode(testSteamID, []byte("R12345")); err != nil {
		t.Fatal(err)
	}
	manager = newManager(api, store, time.Second)
	status, err := manager.Finalize(context.Background(), FinalizeRequest{
		SteamID: testSteamID, ConfirmationCode: []byte("12345"), AuthenticatorTime: 1_700_000_030,
	})
	if err != nil {
		t.Fatal(err)
	}
	if status.State != enrollmentapi.StateComplete || status.Pending {
		t.Fatalf("unexpected complete status: %#v", status)
	}
	parsed, err := mafile.ParsePlaintext(store.raw(testSteamID))
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.Account.FullyEnrolled || parsed.Account.RevocationCode != "R12345" ||
		parsed.Account.Session == nil || parsed.Account.Session.SteamID != testSteamID ||
		parsed.Account.Session.AccessToken != "access-token-secret" {
		t.Fatalf("unexpected active maFile: %#v", parsed.Account)
	}
	if _, err := manager.Resume(testSteamID); !errors.Is(err, ErrNoPendingEnrollment) {
		t.Fatalf("resume after completion error = %v", err)
	}
	if _, err := manager.Start(context.Background(), startRequest()); !errors.Is(err, ErrAlreadyEnrolled) {
		t.Fatalf("start after completion error = %v", err)
	}
}

func TestFinalizeRetryStateIsPersistedAcrossManagerRestart(t *testing.T) {
	api := &fakeAPI{
		addResult: enrollmentapi.AddResult{State: enrollmentapi.StateAwaitingSMS, Pending: testPending()},
		finalizeResult: enrollmentapi.FinalizeResult{
			State: enrollmentapi.StateConfirmationCodeRejected,
		},
	}
	store := newMemoryVault()
	manager := newManager(api, store, time.Second)
	if _, err := manager.Start(context.Background(), startRequest()); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.AcknowledgeRevocationCode(testSteamID, []byte("R12345")); err != nil {
		t.Fatal(err)
	}
	manager = newManager(api, store, time.Second)
	status, err := manager.Finalize(context.Background(), FinalizeRequest{
		SteamID: testSteamID, ConfirmationCode: []byte("WRONG"), AuthenticatorTime: 1_700_000_030,
	})
	if err != nil {
		t.Fatal(err)
	}
	if status.State != enrollmentapi.StateConfirmationCodeRejected || !status.Pending {
		t.Fatalf("unexpected retry status: %#v", status)
	}
	resumed, err := newManager(api, store, time.Second).Resume(testSteamID)
	if err != nil || resumed.State != enrollmentapi.StateConfirmationCodeRejected {
		t.Fatalf("resumed=%#v err=%v", resumed, err)
	}
}

func TestCancelOnlyDeletesValidatedPendingRecord(t *testing.T) {
	api := &fakeAPI{addResult: enrollmentapi.AddResult{State: enrollmentapi.StateAwaitingSMS, Pending: testPending()}}
	store := newMemoryVault()
	manager := newManager(api, store, time.Second)
	if _, err := manager.Start(context.Background(), startRequest()); err != nil {
		t.Fatal(err)
	}
	if err := manager.Cancel(testSteamID); err != nil {
		t.Fatal(err)
	}
	if store.deleted != 1 {
		t.Fatalf("deleted = %d, want 1", store.deleted)
	}

	active := []byte(`{"shared_secret":"ISEhISEhISEhISEhISEhISEhISE=","identity_secret":"MTExMTExMTExMTExMTExMTExMTE=","device_id":"android:00112233-4455-4677-8899-aabbccddeeff"}`)
	if _, err := store.PutRecord(steamIDString(testSteamID), active); err != nil {
		t.Fatal(err)
	}
	if err := manager.Cancel(testSteamID); !errors.Is(err, ErrNoPendingEnrollment) {
		t.Fatalf("cancel active error = %v", err)
	}
	if store.deleted != 1 {
		t.Fatal("active record was deleted")
	}
}

func TestStartPendingWriteFailureDestroysAPISecrets(t *testing.T) {
	pending := testPending()
	api := &fakeAPI{addResult: enrollmentapi.AddResult{State: enrollmentapi.StateAwaitingSMS, Pending: pending}}
	store := newMemoryVault()
	store.failPut = true
	_, err := newManager(api, store, time.Second).Start(context.Background(), startRequest())
	if !errors.Is(err, errTestStorage) {
		t.Fatalf("error = %v", err)
	}
	if pending.AccessToken != nil || pending.SharedSecret != nil || pending.RevocationCode != nil {
		t.Fatal("API pending secrets survived failed persistence")
	}
}

func TestProductionVaultDoesNotPersistPendingSecretsInPlaintext(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	const password = "valid vault password"
	encrypted, err := vault.Create(root, password)
	if err != nil {
		t.Fatal(err)
	}
	if err := encrypted.Unlock(password, vault.FixedLease); err != nil {
		t.Fatal(err)
	}
	api := &fakeAPI{addResult: enrollmentapi.AddResult{State: enrollmentapi.StateAwaitingSMS, Pending: testPending()}}
	if _, err := New(api, encrypted).Start(context.Background(), startRequest()); err != nil {
		t.Fatal(err)
	}
	if err := encrypted.Lock(); err != nil {
		t.Fatal(err)
	}
	for _, secret := range [][]byte{[]byte("access-token-secret"), []byte("R12345"), bytes.Repeat([]byte{0x21}, 20)} {
		err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil || info.IsDir() {
				return walkErr
			}
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if bytes.Contains(raw, secret) {
				t.Fatalf("secret persisted in plaintext in %s", filepath.Base(path))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
