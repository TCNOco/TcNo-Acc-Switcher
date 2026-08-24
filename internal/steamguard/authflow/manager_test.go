package authflow

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

const testSteamID = uint64(76561198000000000)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type fakeClient struct {
	begin       func(context.Context, protocol.PasswordCredentialsRequest, []byte, time.Duration) (protocol.BeginCredentialsResult, error)
	submit      func(context.Context, protocol.AuthSession, protocol.ChallengeType, []byte, time.Duration) (protocol.ChallengeResult, error)
	poll        func(context.Context, protocol.AuthSession, time.Duration) (protocol.PollResult, error)
	beginQR     func(context.Context, protocol.BeginQRRequest, time.Duration) (protocol.BeginQRResult, error)
	beginCalls  atomic.Int32
	submitCalls atomic.Int32
	pollCalls   atomic.Int32
}

func (client *fakeClient) Begin(ctx context.Context, request protocol.PasswordCredentialsRequest, password []byte, timeout time.Duration) (protocol.BeginCredentialsResult, error) {
	client.beginCalls.Add(1)
	return client.begin(ctx, request, password, timeout)
}

func (client *fakeClient) BeginQR(ctx context.Context, request protocol.BeginQRRequest, timeout time.Duration) (protocol.BeginQRResult, error) {
	if client.beginQR == nil {
		return protocol.BeginQRResult{}, flowError(ErrorInvalid)
	}
	return client.beginQR(ctx, request, timeout)
}

func (client *fakeClient) SubmitCode(ctx context.Context, session protocol.AuthSession, challenge protocol.ChallengeType, code []byte, timeout time.Duration) (protocol.ChallengeResult, error) {
	client.submitCalls.Add(1)
	return client.submit(ctx, session, challenge, code, timeout)
}

func (client *fakeClient) Poll(ctx context.Context, session protocol.AuthSession, timeout time.Duration) (protocol.PollResult, error) {
	client.pollCalls.Add(1)
	return client.poll(ctx, session, timeout)
}

type clockWaiter struct {
	at      time.Time
	channel chan time.Time
}

type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []clockWaiter
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Unix(1_700_000_000, 0)}
}

func (clock *fakeClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *fakeClock) After(delay time.Duration) <-chan time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	channel := make(chan time.Time, 1)
	clock.waiters = append(clock.waiters, clockWaiter{at: clock.now.Add(delay), channel: channel})
	return channel
}

func (clock *fakeClock) Advance(duration time.Duration) {
	clock.mu.Lock()
	clock.now = clock.now.Add(duration)
	now := clock.now
	remaining := clock.waiters[:0]
	for _, waiter := range clock.waiters {
		if waiter.at.After(now) {
			remaining = append(remaining, waiter)
			continue
		}
		waiter.channel <- now
	}
	clock.waiters = remaining
	clock.mu.Unlock()
}

func (clock *fakeClock) WaiterCount() int {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return len(clock.waiters)
}

func TestBeginProjectsOnlySafeStateAndEnforcesBinding(t *testing.T) {
	clock := newFakeClock()
	var returnedSession protocol.AuthSession
	client := defaultFakeClient(t, protocol.ChallengeEmailCode)
	originalBegin := client.begin
	client.begin = func(ctx context.Context, request protocol.PasswordCredentialsRequest, password []byte, timeout time.Duration) (protocol.BeginCredentialsResult, error) {
		if string(password) != "password-secret" {
			t.Fatalf("password = %q", password)
		}
		result, err := originalBegin(ctx, request, password, timeout)
		returnedSession = result.Session
		return result, err
	}
	manager := newTestManager(t, client, clock, bytes.NewReader(bytes.Repeat([]byte{0x41}, handleBytes*4)), 2)
	defer manager.Close()
	binding := testBinding("account-a")
	password := []byte("password-secret")
	status, err := manager.Begin(context.Background(), binding, testPasswordRequest(), password)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateChallengeRequired || !status.CanSubmitEmailCode || status.CanSubmitDeviceCode || len(status.Challenges) != 1 || status.Challenges[0] != ChallengeEmailCode {
		t.Fatalf("status = %#v", status)
	}
	if got, want := status.Handle, base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x41}, handleBytes)); got != want {
		t.Fatalf("handle = %q, want %q", got, want)
	}
	assertNoSecrets(t, status, "password-secret", "private-email@example.test", "request-id-secret")
	wrongBinding := binding
	wrongBinding.VaultGeneration = "019f8acb-7ffd-7120-ad85-96bffa12a273"
	_, err = manager.Status(wrongBinding, status.Handle)
	assertFlowKind(t, err, ErrorBindingMismatch)
	_, err = manager.Begin(context.Background(), binding, testPasswordRequest(), password)
	assertFlowKind(t, err, ErrorConflict)
	if err := manager.Cancel(binding, status.Handle); err != nil {
		t.Fatal(err)
	}
	assertProtocolSessionDestroyed(t, returnedSession)
}

func TestBindingRequiresBoundedVaultGeneration(t *testing.T) {
	valid := testBinding("account-a")
	if !validBinding(valid) {
		t.Fatal("valid string vault generation was rejected")
	}
	for name, generation := range map[string]string{
		"empty":        "",
		"too-long":     strings.Repeat("g", maxBindingBytes+1),
		"invalid-utf8": string([]byte{0xff}),
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			candidate.VaultGeneration = generation
			if validBinding(candidate) {
				t.Fatal("invalid vault generation was accepted")
			}
		})
	}
	different := valid
	different.VaultGeneration = "019f8acb-7ffd-7120-ad85-96bffa12a273"
	if sameBinding(valid, different) {
		t.Fatal("different vault generations compared equal")
	}
}

func TestSubmitOnlyAllowedCodeAndDoesNotLeakAnswer(t *testing.T) {
	clock := newFakeClock()
	client := defaultFakeClient(t, protocol.ChallengeDeviceCode)
	var submitted []byte
	client.submit = func(_ context.Context, _ protocol.AuthSession, challenge protocol.ChallengeType, code []byte, _ time.Duration) (protocol.ChallengeResult, error) {
		if challenge != protocol.ChallengeDeviceCode {
			t.Fatalf("challenge = %d", challenge)
		}
		submitted = append([]byte(nil), code...)
		return protocol.ChallengeResult{State: protocol.AuthResultChallengeAccepted}, nil
	}
	manager := newTestManager(t, client, clock, entropyBlocks(0x11, 4), 2)
	defer manager.Close()
	binding := testBinding("account-a")
	status := beginTestSession(t, manager, binding)
	_, err := manager.SubmitCode(context.Background(), binding, status.Handle, ChallengeEmailCode, []byte("ABCDE"))
	assertFlowKind(t, err, ErrorInvalid)
	if client.submitCalls.Load() != 0 {
		t.Fatal("invalid challenge reached client")
	}
	status, err = manager.SubmitCode(context.Background(), binding, status.Handle, ChallengeDeviceCode, []byte("A1B2C"))
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateCodeAccepted || status.PollAfterMillis != 0 || string(submitted) != "A1B2C" {
		t.Fatalf("submit result = %#v, code = %q", status, submitted)
	}
	assertNoSecrets(t, status, "A1B2C")
	if err := manager.Cancel(binding, status.Handle); err != nil {
		t.Fatal(err)
	}
	_, err = manager.SubmitCode(context.Background(), binding, status.Handle, ChallengeDeviceCode, []byte("A1B2C"))
	assertFlowKind(t, err, ErrorGone)
}

func TestPollTransfersTokensExactlyOnceAndWipesBorrowedSlices(t *testing.T) {
	clock := newFakeClock()
	client := defaultFakeClient(t, protocol.ChallengeDeviceConfirmation)
	client.poll = func(_ context.Context, session protocol.AuthSession, _ time.Duration) (protocol.PollResult, error) {
		return protocol.PollResult{
			State:                protocol.AuthResultAuthorized,
			Session:              session,
			AccessToken:          "access-token-secret",
			RefreshToken:         "refresh-token-secret",
			AccountName:          "account-name-secret",
			GuardData:            "guard-data-secret",
			HadRemoteInteraction: true,
		}, nil
	}
	manager := newTestManager(t, client, clock, entropyBlocks(0x22, 4), 2)
	defer manager.Close()
	binding := testBinding("account-a")
	status := beginTestSession(t, manager, binding)
	_, err := manager.Poll(context.Background(), binding, status.Handle)
	assertFlowKind(t, err, ErrorTooSoon)
	if client.pollCalls.Load() != 0 {
		t.Fatal("early poll reached client")
	}
	clock.Advance(5 * time.Second)
	status, err = manager.Poll(context.Background(), binding, status.Handle)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateAuthorizedReady || status.CanPoll || len(status.Challenges) != 0 {
		t.Fatalf("authorized status = %#v", status)
	}
	assertNoSecrets(t, status, "access-token-secret", "refresh-token-secret", "account-name-secret", "guard-data-secret")
	var borrowed [][]byte
	err = manager.Consume(binding, status.Handle, func(steamID uint64, accountName, accessToken, refreshToken, guardData []byte, remote bool) error {
		if steamID != testSteamID || !remote || string(accountName) != "account-name-secret" || string(accessToken) != "access-token-secret" || string(refreshToken) != "refresh-token-secret" || string(guardData) != "guard-data-secret" {
			t.Fatalf("unexpected credentials supplied")
		}
		borrowed = [][]byte{accountName, accessToken, refreshToken, guardData}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range borrowed {
		if !allZero(value) {
			t.Fatalf("borrowed credentials were not wiped: %x", value)
		}
	}
	err = manager.Consume(binding, status.Handle, func(uint64, []byte, []byte, []byte, []byte, bool) error { return nil })
	assertFlowKind(t, err, ErrorGone)
}

func TestConsumerFailureIsSanitizedAndStillConsumes(t *testing.T) {
	clock := newFakeClock()
	client := defaultFakeClient(t, protocol.ChallengeNone)
	client.poll = authorizedPoll("sensitive-token-in-consumer-error")
	manager := newTestManager(t, client, clock, entropyBlocks(0x23, 4), 2)
	defer manager.Close()
	binding := testBinding("account-a")
	status := beginTestSession(t, manager, binding)
	clock.Advance(5 * time.Second)
	status, err := manager.Poll(context.Background(), binding, status.Handle)
	if err != nil {
		t.Fatal(err)
	}
	var borrowed []byte
	err = manager.Consume(binding, status.Handle, func(_ uint64, _ []byte, accessToken []byte, _ []byte, _ []byte, _ bool) error {
		borrowed = accessToken
		return errors.New("vault write failed with sensitive-token-in-consumer-error")
	})
	assertFlowKind(t, err, ErrorConsumer)
	assertNoSecrets(t, err, "sensitive-token-in-consumer-error")
	if !allZero(borrowed) {
		t.Fatal("credentials survived failed consumer")
	}
	_, err = manager.Status(binding, status.Handle)
	assertFlowKind(t, err, ErrorGone)
}

func TestConsumerPanicStillWipesAndConsumes(t *testing.T) {
	clock := newFakeClock()
	client := defaultFakeClient(t, protocol.ChallengeNone)
	client.poll = authorizedPoll("panic-token-secret")
	manager := newTestManager(t, client, clock, entropyBlocks(0x24, 4), 2)
	defer manager.Close()
	binding := testBinding("account-a")
	status := beginTestSession(t, manager, binding)
	clock.Advance(5 * time.Second)
	status, err := manager.Poll(context.Background(), binding, status.Handle)
	if err != nil {
		t.Fatal(err)
	}
	var borrowed []byte
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected consumer panic")
			}
		}()
		_ = manager.Consume(binding, status.Handle, func(_ uint64, _ []byte, accessToken []byte, _ []byte, _ []byte, _ bool) error {
			borrowed = accessToken
			panic("consumer panic with panic-token-secret")
		})
	}()
	if !allZero(borrowed) {
		t.Fatal("credentials survived consumer panic")
	}
	_, err = manager.Status(binding, status.Handle)
	assertFlowKind(t, err, ErrorGone)
}

func TestCancelActivePollCancelsClientAndDestroysSession(t *testing.T) {
	clock := newFakeClock()
	client := defaultFakeClient(t, protocol.ChallengeDeviceConfirmation)
	started := make(chan protocol.AuthSession, 1)
	client.poll = func(ctx context.Context, session protocol.AuthSession, _ time.Duration) (protocol.PollResult, error) {
		started <- session
		<-ctx.Done()
		return protocol.PollResult{}, ctx.Err()
	}
	manager := newTestManager(t, client, clock, entropyBlocks(0x31, 4), 2)
	defer manager.Close()
	binding := testBinding("account-a")
	status := beginTestSession(t, manager, binding)
	clock.Advance(5 * time.Second)
	result := make(chan error, 1)
	go func() {
		_, err := manager.Poll(context.Background(), binding, status.Handle)
		result <- err
	}()
	session := <-started
	_, err := manager.Status(binding, status.Handle)
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Poll(context.Background(), binding, status.Handle)
	assertFlowKind(t, err, ErrorBusy)
	if err := manager.Cancel(binding, status.Handle); err != nil {
		t.Fatal(err)
	}
	assertFlowKind(t, <-result, ErrorGone)
	assertProtocolSessionDestroyed(t, session)
}

func TestClientErrorAndPanicRemoveReservationAndDestroySession(t *testing.T) {
	clock := newFakeClock()
	client := defaultFakeClient(t, protocol.ChallengeNone)
	var lastSession protocol.AuthSession
	client.poll = func(_ context.Context, session protocol.AuthSession, _ time.Duration) (protocol.PollResult, error) {
		lastSession = session
		return protocol.PollResult{}, errors.New("transport leaked password-secret")
	}
	manager := newTestManager(t, client, clock, entropyBlocks(0x32, 8), 2)
	defer manager.Close()
	binding := testBinding("account-a")
	status := beginTestSession(t, manager, binding)
	clock.Advance(5 * time.Second)
	_, err := manager.Poll(context.Background(), binding, status.Handle)
	assertFlowKind(t, err, ErrorProtocol)
	assertNoSecrets(t, err, "password-secret")
	assertProtocolSessionDestroyed(t, lastSession)
	_, err = manager.Status(binding, status.Handle)
	assertFlowKind(t, err, ErrorGone)

	panicClient := defaultFakeClient(t, protocol.ChallengeNone)
	panicClient.begin = func(context.Context, protocol.PasswordCredentialsRequest, []byte, time.Duration) (protocol.BeginCredentialsResult, error) {
		panic("client panic with password-secret")
	}
	panicManager := newTestManager(t, panicClient, clock, uniqueEntropy(8), 2)
	defer panicManager.Close()
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected client panic")
			}
		}()
		_, _ = panicManager.Begin(context.Background(), binding, testPasswordRequest(), []byte("password-secret"))
	}()
	panicClient.begin = defaultFakeClient(t, protocol.ChallengeNone).begin
	if _, err := panicManager.Begin(context.Background(), binding, testPasswordRequest(), []byte("new-password")); err != nil {
		t.Fatalf("panic left account reserved: %v", err)
	}
}

func TestExpiryCapacityAndHandleReplayBounds(t *testing.T) {
	clock := newFakeClock()
	client := defaultFakeClient(t, protocol.ChallengeNone)
	entropy := bytes.NewReader(append(append(bytes.Repeat([]byte{0x51}, handleBytes), bytes.Repeat([]byte{0x51}, handleBytes)...), bytes.Repeat([]byte{0x52}, handleBytes)...))
	manager := newTestManager(t, client, clock, entropy, 1)
	defer manager.Close()
	firstBinding := testBinding("account-a")
	first := beginTestSession(t, manager, firstBinding)
	_, err := manager.Begin(context.Background(), testBinding("account-b"), testPasswordRequest(), []byte("password"))
	assertFlowKind(t, err, ErrorCapacity)
	clock.Advance(DefaultSessionTTL)
	manager.PurgeExpired()
	_, err = manager.Status(firstBinding, first.Handle)
	assertFlowKind(t, err, ErrorGone)
	second, err := manager.Begin(context.Background(), testBinding("account-b"), testPasswordRequest(), []byte("password"))
	if err != nil {
		t.Fatal(err)
	}
	if second.Handle == first.Handle {
		t.Fatal("expired handle was replayed")
	}
	manager.mu.Lock()
	tombCount := len(manager.tombs)
	manager.mu.Unlock()
	if tombCount > manager.config.TombstoneCapacity {
		t.Fatalf("tombstones = %d", tombCount)
	}
}

func TestEntropyAndCanceledBeginAreSanitized(t *testing.T) {
	clock := newFakeClock()
	client := defaultFakeClient(t, protocol.ChallengeNone)
	manager := newTestManager(t, client, clock, bytes.NewReader(nil), 1)
	_, err := manager.Begin(context.Background(), testBinding("account-a"), testPasswordRequest(), []byte("password-secret"))
	assertFlowKind(t, err, ErrorProtocol)
	assertNoSecrets(t, err, "password-secret")
	if client.beginCalls.Load() != 0 {
		t.Fatal("entropy failure reached client")
	}
	manager.Close()

	cancelClient := defaultFakeClient(t, protocol.ChallengeNone)
	cancelClient.begin = func(ctx context.Context, _ protocol.PasswordCredentialsRequest, _ []byte, _ time.Duration) (protocol.BeginCredentialsResult, error) {
		<-ctx.Done()
		return protocol.BeginCredentialsResult{}, ctx.Err()
	}
	cancelManager := newTestManager(t, cancelClient, clock, uniqueEntropy(4), 1)
	defer cancelManager.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = cancelManager.Begin(ctx, testBinding("account-a"), testPasswordRequest(), []byte("password-secret"))
	assertFlowKind(t, err, ErrorCanceled)
	assertNoSecrets(t, err, "password-secret")
}

func TestOperationDeadlineAndConcurrentBeginReservation(t *testing.T) {
	clock := newFakeClock()
	client := defaultFakeClient(t, protocol.ChallengeNone)
	started := make(chan struct{}, 1)
	client.begin = func(ctx context.Context, _ protocol.PasswordCredentialsRequest, _ []byte, _ time.Duration) (protocol.BeginCredentialsResult, error) {
		started <- struct{}{}
		<-ctx.Done()
		return protocol.BeginCredentialsResult{}, ctx.Err()
	}
	manager, err := New(client, Config{
		Capacity:          2,
		SessionTTL:        time.Minute,
		OperationTimeout:  20 * time.Millisecond,
		SweepInterval:     time.Second,
		TombstoneTTL:      time.Minute,
		TombstoneCapacity: 8,
		Clock:             clock,
		Entropy:           uniqueEntropy(8),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	binding := testBinding("account-a")
	firstResult := make(chan error, 1)
	go func() {
		_, err := manager.Begin(context.Background(), binding, testPasswordRequest(), []byte("deadline-password"))
		firstResult <- err
	}()
	<-started
	_, err = manager.Begin(context.Background(), binding, testPasswordRequest(), []byte("other-password"))
	assertFlowKind(t, err, ErrorConflict)
	err = <-firstResult
	assertFlowKind(t, err, ErrorTimeout)
	assertNoSecrets(t, err, "deadline-password", "other-password")
}

func TestBackgroundExpiryCancelsInFlightBegin(t *testing.T) {
	clock := newFakeClock()
	client := defaultFakeClient(t, protocol.ChallengeNone)
	started := make(chan struct{})
	client.begin = func(ctx context.Context, _ protocol.PasswordCredentialsRequest, _ []byte, _ time.Duration) (protocol.BeginCredentialsResult, error) {
		close(started)
		<-ctx.Done()
		return protocol.BeginCredentialsResult{}, ctx.Err()
	}
	manager := newTestManager(t, client, clock, uniqueEntropy(4), 1)
	eventually(t, func() bool { return clock.WaiterCount() > 0 })
	binding := testBinding("account-a")
	result := make(chan error, 1)
	go func() {
		_, err := manager.Begin(context.Background(), binding, testPasswordRequest(), []byte("password"))
		result <- err
	}()
	<-started
	clock.Advance(DefaultSessionTTL + DefaultSweepInterval)
	eventually(t, func() bool {
		select {
		case err := <-result:
			return errorKind(err) == ErrorGone
		default:
			return false
		}
	})
	manager.Close()
}

func TestCloseDestroysAllSessions(t *testing.T) {
	clock := newFakeClock()
	client := defaultFakeClient(t, protocol.ChallengeNone)
	var sessions []protocol.AuthSession
	original := client.begin
	client.begin = func(ctx context.Context, request protocol.PasswordCredentialsRequest, password []byte, timeout time.Duration) (protocol.BeginCredentialsResult, error) {
		result, err := original(ctx, request, password, timeout)
		sessions = append(sessions, result.Session)
		return result, err
	}
	manager := newTestManager(t, client, clock, uniqueEntropy(8), 2)
	beginTestSession(t, manager, testBinding("account-a"))
	beginTestSession(t, manager, testBinding("account-b"))
	manager.Close()
	manager.Close()
	for _, session := range sessions {
		assertProtocolSessionDestroyed(t, session)
	}
	_, err := manager.Begin(context.Background(), testBinding("account-c"), testPasswordRequest(), []byte("password"))
	assertFlowKind(t, err, ErrorClosed)
}

func defaultFakeClient(t *testing.T, challenge protocol.ChallengeType) *fakeClient {
	t.Helper()
	client := &fakeClient{}
	client.begin = func(context.Context, protocol.PasswordCredentialsRequest, []byte, time.Duration) (protocol.BeginCredentialsResult, error) {
		session := newProtocolSession(t, challenge)
		state := protocol.AuthResultChallengeRequired
		if challenge == protocol.ChallengeNone {
			state = protocol.AuthResultWaiting
		}
		return protocol.BeginCredentialsResult{State: state, Session: session, ServerMessage: "request-id-secret"}, nil
	}
	client.submit = func(context.Context, protocol.AuthSession, protocol.ChallengeType, []byte, time.Duration) (protocol.ChallengeResult, error) {
		return protocol.ChallengeResult{State: protocol.AuthResultChallengeAccepted}, nil
	}
	client.poll = func(_ context.Context, session protocol.AuthSession, _ time.Duration) (protocol.PollResult, error) {
		return protocol.PollResult{State: protocol.AuthResultWaiting, Session: session}, nil
	}
	return client
}

func newTestManager(t *testing.T, client Client, clock Clock, entropy io.Reader, capacity int) *Manager {
	t.Helper()
	manager, err := New(client, Config{
		Capacity:          capacity,
		SessionTTL:        DefaultSessionTTL,
		OperationTimeout:  time.Second,
		SweepInterval:     DefaultSweepInterval,
		TombstoneTTL:      DefaultTombstoneTTL,
		TombstoneCapacity: 8,
		Clock:             clock,
		Entropy:           entropy,
	})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func beginTestSession(t *testing.T, manager *Manager, binding Binding) Status {
	t.Helper()
	status, err := manager.Begin(context.Background(), binding, testPasswordRequest(), []byte("password"))
	if err != nil {
		t.Fatal(err)
	}
	return status
}

func testBinding(account string) Binding {
	return Binding{AccountID: account, ExpectedSteamID: testSteamID, VaultGeneration: "019f8acb-7ffd-7120-ad85-96bffa12a272", CapabilityID: "capability-for-test"}
}

func testPasswordRequest() protocol.PasswordCredentialsRequest {
	return protocol.PasswordCredentialsRequest{
		DeviceFriendlyName: "TcNo Account Switcher",
		AccountName:        "account_name",
		Platform:           protocol.PlatformMobileApp,
		Persistence:        protocol.PersistencePersistent,
		WebsiteID:          "Mobile",
		Device: protocol.DeviceDetails{
			FriendlyName: "TcNo Account Switcher",
			Platform:     protocol.PlatformMobileApp,
			OSType:       32,
			MachineID:    []byte("machine-id"),
			App:          protocol.AppTypeSteamMobile,
		},
		QoSLevel: 2,
	}
}

func newProtocolSession(t *testing.T, challenge protocol.ChallengeType) protocol.AuthSession {
	t.Helper()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		challengeMessage := appendProtoVarint(nil, 1, uint64(challenge))
		challengeMessage = appendProtoBytes(challengeMessage, 2, []byte("private-email@example.test"))
		body := appendProtoVarint(nil, 1, 4123)
		body = appendProtoBytes(body, 2, []byte("request-id-secret"))
		body = appendProtoFixed32(body, 3, math.Float32bits(5))
		body = appendProtoBytes(body, 4, challengeMessage)
		body = appendProtoVarint(body, 5, testSteamID)
		return &http.Response{
			StatusCode:    http.StatusOK,
			Status:        "200 OK",
			Header:        make(http.Header),
			Body:          io.NopCloser(bytes.NewReader(body)),
			ContentLength: int64(len(body)),
			Request:       request,
		}, nil
	})
	auth := protocol.NewAuthenticationClient(protocol.NewClient(protocol.Options{Transport: transport}))
	encrypted := base64.StdEncoding.EncodeToString(make([]byte, 256))
	result, err := auth.BeginAuthSessionViaCredentials(context.Background(), protocol.BeginCredentialsRequest{
		DeviceFriendlyName:  "TcNo Account Switcher",
		AccountName:         "account_name",
		EncryptedPassword:   encrypted,
		EncryptionTimestamp: 123456,
		Platform:            protocol.PlatformMobileApp,
		Persistence:         protocol.PersistenceEphemeral,
		WebsiteID:           "Mobile",
		Device: protocol.DeviceDetails{
			FriendlyName: "TcNo Account Switcher",
			Platform:     protocol.PlatformMobileApp,
			OSType:       32,
			MachineID:    []byte("machine-id"),
			App:          protocol.AppTypeSteamMobile,
		},
		QoSLevel: 2,
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return result.Session
}

func appendProtoVarint(destination []byte, field int, value uint64) []byte {
	destination = binary.AppendUvarint(destination, uint64(field<<3))
	return binary.AppendUvarint(destination, value)
}

func appendProtoBytes(destination []byte, field int, value []byte) []byte {
	destination = binary.AppendUvarint(destination, uint64(field<<3|2))
	destination = binary.AppendUvarint(destination, uint64(len(value)))
	return append(destination, value...)
}

func appendProtoFixed32(destination []byte, field int, value uint32) []byte {
	destination = binary.AppendUvarint(destination, uint64(field<<3|5))
	return binary.LittleEndian.AppendUint32(destination, value)
}

func assertProtocolSessionDestroyed(t *testing.T, session protocol.AuthSession) {
	t.Helper()
	var calls atomic.Int32
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("unexpected transport call")
	})
	auth := protocol.NewAuthenticationClient(protocol.NewClient(protocol.Options{Transport: transport}))
	_, err := auth.PollAuthSessionStatus(context.Background(), session, time.Second)
	var protocolErr *protocol.Error
	if !errors.As(err, &protocolErr) || protocolErr.Code != protocol.CodeInvalidRequest || calls.Load() != 0 {
		t.Fatalf("session was not destroyed: err=%v calls=%d", err, calls.Load())
	}
}

func authorizedPoll(token string) func(context.Context, protocol.AuthSession, time.Duration) (protocol.PollResult, error) {
	return func(_ context.Context, session protocol.AuthSession, _ time.Duration) (protocol.PollResult, error) {
		return protocol.PollResult{State: protocol.AuthResultAuthorized, Session: session, AccessToken: token}, nil
	}
}

func entropyBlocks(value byte, blocks int) io.Reader {
	return bytes.NewReader(bytes.Repeat([]byte{value}, handleBytes*blocks))
}

func uniqueEntropy(blocks int) io.Reader {
	data := make([]byte, 0, handleBytes*blocks)
	for block := 0; block < blocks; block++ {
		data = append(data, bytes.Repeat([]byte{byte(block + 1)}, handleBytes)...)
	}
	return bytes.NewReader(data)
}

func assertFlowKind(t *testing.T, err error, expected ErrorKind) {
	t.Helper()
	if got := errorKind(err); got != expected {
		t.Fatalf("error kind = %q, want %q (err=%v)", got, expected, err)
	}
}

func errorKind(err error) ErrorKind {
	var flowErr *Error
	if errors.As(err, &flowErr) {
		return flowErr.Kind
	}
	return ""
}

func assertNoSecrets(t *testing.T, value any, secrets ...string) {
	t.Helper()
	encoded, _ := json.Marshal(value)
	representations := string(encoded) + fmt.Sprintf(" %v %#v", value, value)
	for _, secret := range secrets {
		if secret != "" && strings.Contains(representations, secret) {
			t.Fatalf("secret %q leaked in %q", secret, representations)
		}
	}
}

func allZero(value []byte) bool {
	for _, current := range value {
		if current != 0 {
			return false
		}
	}
	return true
}

func eventually(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition was not met")
}
