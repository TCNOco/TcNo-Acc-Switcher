package sessionrefresh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

const testSteamID = uint64(76561198000000013)

type fakeClient struct {
	mu      sync.Mutex
	calls   int
	request protocol.GenerateAccessTokenRequest
	timeout time.Duration
	result  protocol.TokenResult
	err     error
	call    func(context.Context, protocol.GenerateAccessTokenRequest, time.Duration) (protocol.TokenResult, error)
}

func (f *fakeClient) GenerateAccessTokenForApp(ctx context.Context, request protocol.GenerateAccessTokenRequest, timeout time.Duration) (protocol.TokenResult, error) {
	f.mu.Lock()
	f.calls++
	f.request = request
	f.timeout = timeout
	f.mu.Unlock()
	if f.call != nil {
		return f.call(ctx, request, timeout)
	}
	return f.result, f.err
}

type fakeVault struct {
	mu      sync.Mutex
	records []vault.RecordInfo
	data    map[string][]byte
	listErr error
	getErr  error
	putErr  error
	puts    int
}

func (f *fakeVault) ListRecords() ([]vault.RecordInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]vault.RecordInfo(nil), f.records...), f.listErr
}

func (f *fakeVault) GetRecord(id string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	return append([]byte(nil), f.data[id]...), nil
}

func (f *fakeVault) PutRecord(steamID64 string, plaintext []byte) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.puts++
	if f.putErr != nil {
		return "", f.putErr
	}
	for _, record := range f.records {
		if record.SteamID64 == steamID64 {
			f.data[record.ID] = append([]byte(nil), plaintext...)
			return record.ID, nil
		}
	}
	return "", errors.New("unexpected insert")
}

func activeAccount(t *testing.T, steamID uint64, accessToken, refreshToken string) []byte {
	t.Helper()
	account := mafile.Account{
		SharedSecret:   "AQEBAQEBAQEBAQEBAQEBAQEBAQE=",
		IdentitySecret: "AgICAgICAgICAgICAgICAgICAgI=",
		DeviceID:       "android:00112233-4455-6677-8899-aabbccddeeff",
		FullyEnrolled:  true,
		Session: &mafile.SessionData{
			SteamID:      steamID,
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		},
	}
	raw, err := mafile.ExportPlaintext(account, mafile.ExportOptions{IncludeTokens: true})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func oneRecordVault(t *testing.T, raw []byte) *fakeVault {
	t.Helper()
	return &fakeVault{
		records: []vault.RecordInfo{{ID: "record-1", SteamID64: fmt.Sprint(testSteamID)}},
		data:    map[string][]byte{"record-1": append([]byte(nil), raw...)},
	}
}

func TestRefreshRenewsAndCanonicallyPersistsTokens(t *testing.T) {
	storage := oneRecordVault(t, activeAccount(t, testSteamID, "old-access", "old-refresh"))
	client := &fakeClient{result: protocol.TokenResult{
		State: protocol.AuthResultTokenIssued, AccessToken: "new-access", RefreshToken: "new-refresh",
	}}

	result, err := New(client, storage).Refresh(context.Background(), testSteamID)
	if err != nil {
		t.Fatal(err)
	}
	if result.SteamID != testSteamID || !result.RefreshTokenRenewed {
		t.Fatalf("unexpected public result: %+v", result)
	}
	if client.calls != 1 || client.request.SteamID != testSteamID || client.request.RefreshToken != "old-refresh" ||
		client.request.Renewal != protocol.RenewalAllow || client.timeout != DefaultRequestTimeout {
		t.Fatal("refresh client did not receive the exact bounded renewal request")
	}
	parsed, err := mafile.ParsePlaintext(storage.data["record-1"])
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Account.Session.AccessToken != "new-access" || parsed.Account.Session.RefreshToken != "new-refresh" {
		t.Fatal("refreshed credentials were not saved in canonical maFile data")
	}
	canonical, err := mafile.ExportPlaintext(parsed.Account, mafile.ExportOptions{IncludeTokens: true})
	if err != nil || !bytes.Equal(canonical, storage.data["record-1"]) {
		t.Fatal("persisted account is not canonical")
	}
}

func TestRefreshRetainsRefreshTokenWhenSteamDoesNotRenewIt(t *testing.T) {
	storage := oneRecordVault(t, activeAccount(t, testSteamID, "old-access", "old-refresh"))
	client := &fakeClient{result: protocol.TokenResult{State: protocol.AuthResultTokenIssued, AccessToken: "new-access"}}
	result, err := New(client, storage).Refresh(context.Background(), testSteamID)
	if err != nil {
		t.Fatal(err)
	}
	if result.RefreshTokenRenewed {
		t.Fatal("empty optional response token reported as renewed")
	}
	parsed, _ := mafile.ParsePlaintext(storage.data["record-1"])
	if parsed.Account.Session.RefreshToken != "old-refresh" {
		t.Fatal("existing refresh token was erased")
	}
}

func TestRefreshRejectsNonActiveAndAmbiguousRecordsBeforeTransport(t *testing.T) {
	tests := []struct {
		name    string
		storage *fakeVault
		want    error
	}{
		{name: "missing", storage: &fakeVault{data: map[string][]byte{}}, want: ErrAccountNotFound},
		{name: "duplicate", storage: &fakeVault{
			records: []vault.RecordInfo{{ID: "a", SteamID64: fmt.Sprint(testSteamID)}, {ID: "b", SteamID64: fmt.Sprint(testSteamID)}},
			data:    map[string][]byte{"a": activeAccount(t, testSteamID, "a", "r"), "b": activeAccount(t, testSteamID, "a", "r")},
		}, want: ErrDuplicate},
		{name: "pending", storage: oneRecordVault(t, []byte(`{"kind":"steamguard-enrollment-pending","version":1,"accessToken":"secret"}`)), want: ErrPending},
		{name: "corrupt", storage: oneRecordVault(t, []byte(`{"not":"a maFile"}`)), want: ErrCorruptRecord},
		{name: "wrong account", storage: oneRecordVault(t, activeAccount(t, testSteamID+1, "a", "r")), want: ErrWrongAccount},
		{name: "missing refresh token", storage: oneRecordVault(t, activeAccount(t, testSteamID, "a", "")), want: ErrNoRefreshToken},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &fakeClient{result: protocol.TokenResult{State: protocol.AuthResultTokenIssued, AccessToken: "unused"}}
			_, err := New(client, test.storage).Refresh(context.Background(), testSteamID)
			if !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
			if client.calls != 0 {
				t.Fatal("transport called for rejected vault record")
			}
		})
	}
}

func TestRefreshCancellationAndTimeoutAreTyped(t *testing.T) {
	storage := oneRecordVault(t, activeAccount(t, testSteamID, "a", "refresh-secret"))
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	client := &fakeClient{}
	if _, err := New(client, storage).Refresh(canceled, testSteamID); !errors.Is(err, ErrCanceled) {
		t.Fatalf("cancellation was not typed: %v", err)
	}
	if client.calls != 0 {
		t.Fatal("canceled operation reached transport")
	}

	client.call = func(ctx context.Context, _ protocol.GenerateAccessTokenRequest, _ time.Duration) (protocol.TokenResult, error) {
		<-ctx.Done()
		return protocol.TokenResult{}, ctx.Err()
	}
	if _, err := NewWithTimeout(client, storage, 5*time.Millisecond).Refresh(context.Background(), testSteamID); !errors.Is(err, ErrTimedOut) {
		t.Fatalf("timeout was not typed: %v", err)
	}
	if client.timeout != 5*time.Millisecond {
		t.Fatalf("timeout was not bounded at client boundary: %v", client.timeout)
	}
	storage.puts = 0
	client.call = func(ctx context.Context, _ protocol.GenerateAccessTokenRequest, _ time.Duration) (protocol.TokenResult, error) {
		<-ctx.Done()
		return protocol.TokenResult{State: protocol.AuthResultTokenIssued, AccessToken: "late-access"}, nil
	}
	if _, err := NewWithTimeout(client, storage, 5*time.Millisecond).Refresh(context.Background(), testSteamID); !errors.Is(err, ErrTimedOut) {
		t.Fatalf("late success bypassed the deadline: %v", err)
	}
	if storage.puts != 0 {
		t.Fatal("late transport result reached the vault")
	}
}

func TestRefreshRejectsInvalidResponseWithoutWriting(t *testing.T) {
	tests := []protocol.TokenResult{
		{State: protocol.AuthResultWaiting, AccessToken: "access"},
		{State: protocol.AuthResultTokenIssued},
		{State: protocol.AuthResultTokenIssued, AccessToken: "bad token"},
		{State: protocol.AuthResultTokenIssued, AccessToken: "access", RefreshToken: "bad token"},
	}
	for i, result := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			storage := oneRecordVault(t, activeAccount(t, testSteamID, "old", "refresh-secret"))
			_, err := New(&fakeClient{result: result}, storage).Refresh(context.Background(), testSteamID)
			if !errors.Is(err, ErrInvalidResponse) || storage.puts != 0 {
				t.Fatalf("invalid response was accepted or written: err=%v puts=%d", err, storage.puts)
			}
		})
	}
}

func TestRefreshSanitizesRemoteFailure(t *testing.T) {
	storage := oneRecordVault(t, activeAccount(t, testSteamID, "old", "refresh-secret"))
	_, err := New(&fakeClient{err: errors.New("transport included refresh-secret")}, storage).Refresh(context.Background(), testSteamID)
	if !errors.Is(err, ErrRemote) || bytes.Contains([]byte(err.Error()), []byte("refresh-secret")) {
		t.Fatalf("remote failure was not sanitized: %v", err)
	}
	protocolFailure := &protocol.Error{Code: protocol.CodeInvalidResponse, State: protocol.StateFailed}
	_, err = New(&fakeClient{err: protocolFailure}, storage).Refresh(context.Background(), testSteamID)
	if !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("invalid protocol response was not classified: %v", err)
	}
}

func TestRefreshTransactionFailureLeavesOriginalRecord(t *testing.T) {
	original := activeAccount(t, testSteamID, "old-access", "old-refresh")
	storage := oneRecordVault(t, original)
	storage.putErr = errors.New("simulated transaction failure")
	client := &fakeClient{result: protocol.TokenResult{
		State: protocol.AuthResultTokenIssued, AccessToken: "new-access", RefreshToken: "new-refresh",
	}}
	_, err := New(client, storage).Refresh(context.Background(), testSteamID)
	if !errors.Is(err, ErrPersist) {
		t.Fatalf("got %v, want persist failure", err)
	}
	if !bytes.Equal(storage.data["record-1"], original) {
		t.Fatal("failed transaction changed the original record")
	}
	for _, secret := range []string{"old-refresh", "new-refresh", "new-access"} {
		if bytes.Contains([]byte(err.Error()), []byte(secret)) {
			t.Fatal("typed error disclosed a bearer credential")
		}
	}
}

type permissiveHardener struct{}

func (permissiveHardener) HardenDir(string) error  { return nil }
func (permissiveHardener) HardenFile(string) error { return nil }

func TestProductionVaultAtomicFailureAndCiphertextSentinel(t *testing.T) {
	root := t.TempDir()
	armed := false
	transactionFailure := errors.New("stop after journal")
	options := []vault.Option{
		vault.WithHardener(permissiveHardener{}),
		vault.WithKDFParams(vault.KDFParams{Algorithm: "argon2id", MemoryKiB: 8 * 1024, Passes: 1, Lanes: 1, KeyBytes: 32}),
		vault.WithTransactionHook(func(stage string) error {
			if armed && stage == "after-journal" {
				return transactionFailure
			}
			return nil
		}),
	}
	production, err := vault.Create(root, "test vault password", options...)
	if err != nil {
		t.Fatal(err)
	}
	if err := production.Unlock("test vault password", vault.ProcessLease); err != nil {
		t.Fatal(err)
	}
	original := activeAccount(t, testSteamID, "old-access-sentinel", "old-refresh-sentinel")
	if _, err := production.PutRecord(fmt.Sprint(testSteamID), original); err != nil {
		t.Fatal(err)
	}
	armed = true
	client := &fakeClient{result: protocol.TokenResult{
		State: protocol.AuthResultTokenIssued, AccessToken: "new-access-sentinel", RefreshToken: "new-refresh-sentinel",
	}}
	_, err = New(client, production).Refresh(context.Background(), testSteamID)
	if !errors.Is(err, ErrPersist) {
		t.Fatalf("production transaction failure was not mapped safely: %v", err)
	}
	records, err := production.ListRecords()
	if err != nil || len(records) != 1 {
		t.Fatalf("production vault no longer has exactly one record: %v", err)
	}
	stored, err := production.GetRecord(records[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, original) {
		t.Fatal("failed production generation switch replaced the original record")
	}
	for _, secret := range [][]byte{[]byte("old-refresh-sentinel"), []byte("new-refresh-sentinel"), []byte("new-access-sentinel")} {
		walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if bytes.Contains(raw, secret) {
				return errors.New("plaintext token reached production vault storage")
			}
			return nil
		})
		if walkErr != nil {
			t.Fatal(walkErr)
		}
	}
}

func TestInvalidConfigurationIsRejectedBeforeStorage(t *testing.T) {
	storage := oneRecordVault(t, activeAccount(t, testSteamID, "a", "r"))
	client := &fakeClient{}
	tests := []*Refresher{
		nil,
		New(nil, storage),
		New(client, nil),
		NewWithTimeout(client, storage, 0),
		NewWithTimeout(client, storage, maxRequestTimeout+time.Nanosecond),
	}
	for _, refresher := range tests {
		if _, err := refresher.Refresh(context.Background(), testSteamID); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("invalid configuration returned %v", err)
		}
	}
}
