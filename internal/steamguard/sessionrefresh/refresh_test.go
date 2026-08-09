package sessionrefresh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/loginrecord"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
	"TcNo-Acc-Switcher/internal/steamguard/vaultrecord"
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
	mu            sync.Mutex
	records       []vault.RecordInfo
	data          map[string][]byte
	listErr       error
	getErr        error
	putErr        error
	putRecordsErr error
	puts          int
	batches       int
	lastBatch     []vault.RecordUpdate
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

// PutRecords mirrors the production vault: it rejects a malformed batch whole
// and copies each plaintext, because the caller wipes its own buffers as soon
// as this returns.
func (f *fakeVault) PutRecords(updates []vault.RecordUpdate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.batches++
	seen := map[string]bool{}
	for _, update := range updates {
		if update.SteamID64 == "" || seen[update.SteamID64] {
			return vault.ErrInvalidFormat
		}
		seen[update.SteamID64] = true
	}
	if f.putRecordsErr != nil {
		return f.putRecordsErr
	}
	f.lastBatch = nil
	for _, update := range updates {
		f.lastBatch = append(f.lastBatch, vault.RecordUpdate{
			SteamID64: update.SteamID64,
			Plaintext: append([]byte(nil), update.Plaintext...),
		})
	}
	for _, update := range f.lastBatch {
		stored := false
		for _, record := range f.records {
			if record.SteamID64 == update.SteamID64 {
				f.data[record.ID] = append([]byte(nil), update.Plaintext...)
				stored = true
				break
			}
		}
		if !stored {
			return errors.New("unexpected insert")
		}
	}
	return nil
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

func loginOnlyRecord(t *testing.T, steamID uint64, accessToken, refreshToken string) []byte {
	t.Helper()
	raw, err := loginrecord.Encode(loginrecord.New(steamID, "session-only-account", accessToken, refreshToken))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func batchRecordID(steamID uint64) string { return "record-" + fmt.Sprint(steamID) }

func batchVault(t *testing.T, records map[uint64][]byte) *fakeVault {
	t.Helper()
	storage := &fakeVault{data: map[string][]byte{}}
	for steamID, raw := range records {
		id := batchRecordID(steamID)
		storage.records = append(storage.records, vault.RecordInfo{ID: id, SteamID64: fmt.Sprint(steamID)})
		storage.data[id] = append([]byte(nil), raw...)
	}
	return storage
}

// perAccountClient answers each account differently, which is what separates a
// batch that survives one bad account from one that does not.
func perAccountClient(answer func(steamID uint64) (protocol.TokenResult, error)) *fakeClient {
	return &fakeClient{call: func(_ context.Context, request protocol.GenerateAccessTokenRequest, _ time.Duration) (protocol.TokenResult, error) {
		return answer(request.SteamID)
	}}
}

func issuedFor(steamID uint64) (protocol.TokenResult, error) {
	return protocol.TokenResult{
		State:        protocol.AuthResultTokenIssued,
		AccessToken:  fmt.Sprintf("new-access-%d", steamID),
		RefreshToken: fmt.Sprintf("new-refresh-%d", steamID),
	}, nil
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
		if _, err := refresher.RefreshBatch(context.Background(), []uint64{testSteamID}); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("invalid configuration returned %v from RefreshBatch", err)
		}
	}
	if storage.batches != 0 || storage.puts != 0 {
		t.Fatal("a rejected configuration reached storage")
	}
}

// The count is the behaviour under test: every generation switch invalidates
// every outstanding capability, so a batch has to cost exactly one.
func TestRefreshBatchWritesEveryAccountInOneVaultCommit(t *testing.T) {
	ids := []uint64{testSteamID, testSteamID + 1, testSteamID + 2}
	storage := batchVault(t, map[uint64][]byte{
		ids[0]: activeAccount(t, ids[0], "old-access-0", "old-refresh-0"),
		ids[1]: activeAccount(t, ids[1], "old-access-1", "old-refresh-1"),
		ids[2]: activeAccount(t, ids[2], "old-access-2", "old-refresh-2"),
	})

	// The repeated ID proves the batch collapses it: the vault rejects a batch
	// that names one account twice, which would sink an otherwise good sweep.
	results, err := New(perAccountClient(issuedFor), storage).
		RefreshBatch(context.Background(), append(append([]uint64(nil), ids...), ids[1]))
	if err != nil {
		t.Fatal(err)
	}
	if storage.batches != 1 || len(storage.lastBatch) != len(ids) {
		t.Fatalf("batch cost %d commits carrying %d updates, want 1 and %d",
			storage.batches, len(storage.lastBatch), len(ids))
	}
	if storage.puts != 0 {
		t.Fatal("batch fell back to per-account writes")
	}
	if len(results) != len(ids) {
		t.Fatalf("got %d results, want %d", len(results), len(ids))
	}
	for i, steamID := range ids {
		if results[i].SteamID != steamID || !results[i].RefreshTokenRenewed {
			t.Fatalf("result %d = %+v", i, results[i])
		}
		parsed, parseErr := mafile.ParsePlaintext(storage.data[batchRecordID(steamID)])
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		if parsed.Account.Session.AccessToken != fmt.Sprintf("new-access-%d", steamID) ||
			parsed.Account.Session.RefreshToken != fmt.Sprintf("new-refresh-%d", steamID) {
			t.Fatalf("account %d kept its old credentials", steamID)
		}
	}
}

// A login-only record has no authenticator secrets, so writing it back as a
// maFile would fail validation and lose the session.
func TestRefreshBatchWritesEachRecordBackInItsStoredShape(t *testing.T) {
	authenticator, session := testSteamID, testSteamID+1
	storage := batchVault(t, map[uint64][]byte{
		authenticator: activeAccount(t, authenticator, "old-access", "old-refresh"),
		session:       loginOnlyRecord(t, session, "old-access", "old-refresh"),
	})

	results, err := New(perAccountClient(issuedFor), storage).
		RefreshBatch(context.Background(), []uint64{authenticator, session})
	if err != nil || len(results) != 2 {
		t.Fatalf("results = %+v, err = %v", results, err)
	}

	storedAuthenticator := storage.data[batchRecordID(authenticator)]
	if kind := vaultrecord.Sniff(storedAuthenticator); kind != vaultrecord.KindMaFile {
		t.Fatalf("authenticator was rewritten as %s", kind)
	}
	parsed, err := mafile.ParsePlaintext(storedAuthenticator)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Account.SharedSecret != "AQEBAQEBAQEBAQEBAQEBAQEBAQE=" || parsed.Account.IdentitySecret == "" {
		t.Fatal("authenticator secrets did not survive the batch")
	}
	if parsed.Account.Session.AccessToken != fmt.Sprintf("new-access-%d", authenticator) ||
		parsed.Account.Session.RefreshToken != fmt.Sprintf("new-refresh-%d", authenticator) {
		t.Fatal("authenticator session was not renewed")
	}

	storedSession := storage.data[batchRecordID(session)]
	if kind := vaultrecord.Sniff(storedSession); kind != vaultrecord.KindLoginOnly {
		t.Fatalf("login-only record was rewritten as %s", kind)
	}
	record, err := loginrecord.Decode(storedSession)
	if err != nil {
		t.Fatal(err)
	}
	if record.AccessToken != fmt.Sprintf("new-access-%d", session) ||
		record.RefreshToken != fmt.Sprintf("new-refresh-%d", session) ||
		record.AccountName != "session-only-account" {
		t.Fatalf("login-only record did not round-trip: %+v", record.AccountName)
	}
}

func TestRefreshBatchDropsAFailedAccountAndWritesTheRest(t *testing.T) {
	ids := []uint64{testSteamID, testSteamID + 1, testSteamID + 2}
	failing := ids[1]
	original := activeAccount(t, failing, "old-access-1", "old-refresh-1")
	storage := batchVault(t, map[uint64][]byte{
		ids[0]:  activeAccount(t, ids[0], "old-access-0", "old-refresh-0"),
		failing: original,
		ids[2]:  activeAccount(t, ids[2], "old-access-2", "old-refresh-2"),
	})
	client := perAccountClient(func(steamID uint64) (protocol.TokenResult, error) {
		if steamID == failing {
			return protocol.TokenResult{}, errors.New("Steam rejected old-refresh-1")
		}
		return issuedFor(steamID)
	})

	results, err := New(client, storage).RefreshBatch(context.Background(), ids)
	if err != nil {
		t.Fatalf("one account's failure sank the batch: %v", err)
	}
	if storage.batches != 1 || len(storage.lastBatch) != 2 {
		t.Fatalf("batch cost %d commits carrying %d updates, want 1 and 2",
			storage.batches, len(storage.lastBatch))
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	for _, result := range results {
		if result.SteamID == failing {
			t.Fatal("failed account was reported as refreshed")
		}
	}
	if !bytes.Equal(storage.data[batchRecordID(failing)], original) {
		t.Fatal("failed account's record was rewritten")
	}
	for _, steamID := range []uint64{ids[0], ids[2]} {
		parsed, parseErr := mafile.ParsePlaintext(storage.data[batchRecordID(steamID)])
		if parseErr != nil || parsed.Account.Session.AccessToken != fmt.Sprintf("new-access-%d", steamID) {
			t.Fatalf("account %d was not written alongside the failure", steamID)
		}
	}
}

func TestRefreshBatchSkipsUnrefreshableRecordsWithoutFailing(t *testing.T) {
	good, pending, noRefreshToken := testSteamID, testSteamID+1, testSteamID+2
	pendingRaw := []byte(`{"kind":"steamguard-enrollment-pending","version":1,"accessToken":"secret"}`)
	storage := batchVault(t, map[uint64][]byte{
		good:           activeAccount(t, good, "old-access", "old-refresh"),
		pending:        pendingRaw,
		noRefreshToken: activeAccount(t, noRefreshToken, "old-access", ""),
	})

	results, err := New(perAccountClient(issuedFor), storage).
		RefreshBatch(context.Background(), []uint64{good, pending, noRefreshToken})
	if err != nil {
		t.Fatalf("a skipped record failed the batch: %v", err)
	}
	if len(results) != 1 || results[0].SteamID != good {
		t.Fatalf("results = %+v, want only %d", results, good)
	}
	if storage.batches != 1 || len(storage.lastBatch) != 1 {
		t.Fatalf("batch cost %d commits carrying %d updates, want 1 and 1",
			storage.batches, len(storage.lastBatch))
	}
	if !bytes.Equal(storage.data[batchRecordID(pending)], pendingRaw) {
		t.Fatal("pending enrollment was overwritten")
	}
}

func TestRefreshBatchCommitFailureLeavesEveryRecordUnchanged(t *testing.T) {
	for _, test := range []struct {
		name   string
		putErr error
		want   error
	}{
		{name: "transaction", putErr: errors.New("simulated batch failure"), want: ErrPersist},
		{name: "locked", putErr: vault.ErrLocked, want: ErrVaultLocked},
	} {
		t.Run(test.name, func(t *testing.T) {
			ids := []uint64{testSteamID, testSteamID + 1}
			originals := map[uint64][]byte{
				ids[0]: activeAccount(t, ids[0], "old-access-0", "old-refresh-0"),
				ids[1]: activeAccount(t, ids[1], "old-access-1", "old-refresh-1"),
			}
			storage := batchVault(t, originals)
			storage.putRecordsErr = test.putErr

			results, err := New(perAccountClient(issuedFor), storage).RefreshBatch(context.Background(), ids)
			if !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
			if results != nil {
				t.Fatalf("a failed commit returned %d results", len(results))
			}
			for _, steamID := range ids {
				if !bytes.Equal(storage.data[batchRecordID(steamID)], originals[steamID]) {
					t.Fatalf("failed commit changed account %d", steamID)
				}
			}
			for _, secret := range []string{"old-refresh-0", "new-refresh-0", "new-access-0"} {
				if bytes.Contains([]byte(err.Error()), []byte(secret)) {
					t.Fatal("typed error disclosed a bearer credential")
				}
			}
		})
	}
}

// Result is what crosses back out of the package, so it must not be able to
// carry a token even if a future field is added carelessly.
func TestRefreshBatchResultsCarryNoBearerCredentials(t *testing.T) {
	ids := []uint64{testSteamID, testSteamID + 1}
	storage := batchVault(t, map[uint64][]byte{
		ids[0]: activeAccount(t, ids[0], "old-access-0", "old-refresh-0"),
		ids[1]: loginOnlyRecord(t, ids[1], "old-access-1", "old-refresh-1"),
	})

	results, err := New(perAccountClient(issuedFor), storage).RefreshBatch(context.Background(), ids)
	if err != nil || len(results) != 2 {
		t.Fatalf("results = %+v, err = %v", results, err)
	}
	rendered := fmt.Sprintf("%#v", results)
	secrets := []string{"old-access-0", "old-refresh-0", "old-access-1", "old-refresh-1"}
	for _, steamID := range ids {
		secrets = append(secrets, fmt.Sprintf("new-access-%d", steamID), fmt.Sprintf("new-refresh-%d", steamID))
	}
	for _, secret := range secrets {
		if strings.Contains(rendered, secret) {
			t.Fatalf("returned results disclosed %q", secret)
		}
	}
	resultType := reflect.TypeOf(Result{})
	for i := 0; i < resultType.NumField(); i++ {
		switch resultType.Field(i).Type.Kind() {
		case reflect.Uint64, reflect.Bool:
		default:
			t.Fatalf("Result.%s can hold a credential", resultType.Field(i).Name)
		}
	}
}
