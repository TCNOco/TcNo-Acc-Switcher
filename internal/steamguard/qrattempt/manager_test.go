package qrattempt

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/securemem"
)

const testChallenge = "https://s.team/q/1/1234567890123456789"

var (
	testBinding = Binding{AccountID: "76561198000000001", VaultGeneration: "generation-a"}
	errCallback = errors.New("callback failed")
)

func TestCreateConsumeSingleUseAndWipes(t *testing.T) {
	manager, protector, _ := newTestManager(t)
	payload := []byte(testChallenge)
	id, err := manager.Create(testBinding, payload, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !validID(id) {
		t.Fatalf("invalid generated ID length %d", len(id))
	}
	requireZeroed(t, payload)
	requireZeroed(t, protector.input(0))

	var got, retained []byte
	if err := manager.Consume(id, testBinding, func(secret []byte) error {
		got = append([]byte(nil), secret...)
		retained = secret
		secret[0] ^= 0xff
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if string(got) != testChallenge {
		t.Fatalf("callback payload = %q", got)
	}
	requireZeroed(t, retained)
	requireDestroyed(t, protector.handle(0))
	if err := manager.Consume(id, testBinding, func([]byte) error { return nil }); !errors.Is(err, ErrNotFound) {
		t.Fatalf("replay error = %v", err)
	}
}

func TestConsumeEnforcesAccountAndGenerationBinding(t *testing.T) {
	manager, _, _ := newTestManager(t)
	id := createAttempt(t, manager, testBinding, time.Minute)
	called := 0
	callback := func([]byte) error {
		called++
		return nil
	}

	wrongAccount := testBinding
	wrongAccount.AccountID = "76561198000000002"
	if err := manager.Consume(id, wrongAccount, callback); !errors.Is(err, ErrBindingMismatch) {
		t.Fatalf("cross-account error = %v", err)
	}
	wrongGeneration := testBinding
	wrongGeneration.VaultGeneration = "generation-b"
	if err := manager.Consume(id, wrongGeneration, callback); !errors.Is(err, ErrBindingMismatch) {
		t.Fatalf("cross-generation error = %v", err)
	}
	if called != 0 {
		t.Fatalf("callback called %d times", called)
	}
	if err := manager.Consume(id, testBinding, callback); err != nil {
		t.Fatal(err)
	}
}

func TestInspectDoesNotConsumeAndWipesCallbackCopy(t *testing.T) {
	manager, protector, _ := newTestManager(t)
	id := createAttempt(t, manager, testBinding, time.Minute)
	var retained []byte
	if err := manager.Inspect(id, testBinding, func(payload []byte) error {
		if string(payload) != testChallenge {
			t.Fatalf("inspection payload = %q", payload)
		}
		retained = payload
		payload[0] ^= 0xff
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	requireZeroed(t, retained)
	if protector.handle(0).destroyed {
		t.Fatal("inspection destroyed the protected attempt")
	}
	if err := manager.Consume(id, testBinding, func(payload []byte) error {
		if string(payload) != testChallenge {
			t.Fatalf("stored payload was mutated: %q", payload)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestInspectEnforcesBindingExpiryAndCallbackFailure(t *testing.T) {
	manager, _, clock := newTestManager(t)
	id := createAttempt(t, manager, testBinding, time.Minute)
	wrong := testBinding
	wrong.AccountID = "76561198000000002"
	if err := manager.Inspect(id, wrong, func([]byte) error { return nil }); !errors.Is(err, ErrBindingMismatch) {
		t.Fatalf("binding error = %v", err)
	}
	if err := manager.Inspect(id, testBinding, func([]byte) error { return errCallback }); !errors.Is(err, errCallback) {
		t.Fatalf("callback error = %v", err)
	}
	if err := manager.Consume(id, testBinding, func([]byte) error { return nil }); err != nil {
		t.Fatalf("callback failure consumed attempt: %v", err)
	}

	expired := createAttempt(t, manager, testBinding, time.Minute)
	clock.Add(time.Minute)
	if err := manager.Inspect(expired, testBinding, func([]byte) error {
		t.Fatal("expired inspection callback ran")
		return nil
	}); !errors.Is(err, ErrExpired) {
		t.Fatalf("expiry error = %v", err)
	}
}

func TestExpiryAndSynchronousCleanup(t *testing.T) {
	manager, protector, clock := newTestManager(t)
	first := createAttempt(t, manager, testBinding, 10*time.Second)
	clock.Add(10 * time.Second)
	if err := manager.Consume(first, testBinding, func([]byte) error {
		t.Fatal("expired callback was invoked")
		return nil
	}); !errors.Is(err, ErrExpired) {
		t.Fatalf("expiry error = %v", err)
	}
	requireDestroyed(t, protector.handle(0))
	if err := manager.Consume(first, testBinding, func([]byte) error { return nil }); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired replay error = %v", err)
	}

	secondBinding := Binding{AccountID: "76561198000000002", VaultGeneration: "generation-a"}
	thirdBinding := Binding{AccountID: "76561198000000003", VaultGeneration: "generation-a"}
	second := createAttempt(t, manager, secondBinding, 20*time.Second)
	_ = createAttempt(t, manager, thirdBinding, 40*time.Second)
	clock.Add(20 * time.Second)
	removed, err := manager.CleanupExpired()
	if err != nil || removed != 1 {
		t.Fatalf("cleanup = %d, %v", removed, err)
	}
	if err := manager.Consume(second, secondBinding, func([]byte) error { return nil }); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cleaned attempt error = %v", err)
	}
	if len(manager.byID) != 1 {
		t.Fatalf("active attempts = %d", len(manager.byID))
	}
}

func TestCreateRejectsInvalidInputsAndWipesPayload(t *testing.T) {
	tests := []struct {
		name    string
		binding Binding
		payload string
		ttl     time.Duration
		want    error
	}{
		{name: "binding", binding: Binding{}, payload: testChallenge, ttl: time.Minute, want: ErrInvalidBinding},
		{name: "zero ttl", binding: testBinding, payload: testChallenge, ttl: 0, want: ErrInvalidTTL},
		{name: "long ttl", binding: testBinding, payload: testChallenge, ttl: MaxTTL + time.Nanosecond, want: ErrInvalidTTL},
		{name: "challenge", binding: testBinding, payload: "https://evil.example/q/1/1", ttl: time.Minute, want: ErrInvalidChallenge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager, protector, _ := newTestManager(t)
			payload := []byte(test.payload)
			if _, err := manager.Create(test.binding, payload, test.ttl); !errors.Is(err, test.want) {
				t.Fatalf("error = %v", err)
			}
			requireZeroed(t, payload)
			if len(protector.handles) != 0 {
				t.Fatal("invalid input reached secure memory")
			}
		})
	}
}

func TestCreateReplacesTheAccountsAttempt(t *testing.T) {
	manager, protector, _ := newTestManager(t)
	first := createAttempt(t, manager, testBinding, time.Minute)
	replacementBinding := testBinding
	replacementBinding.VaultGeneration = "generation-b"
	second := createAttempt(t, manager, replacementBinding, time.Minute)
	if first == second {
		t.Fatal("replacement reused an attempt ID")
	}
	requireDestroyed(t, protector.handle(0))
	if len(manager.byID) != 1 || manager.byAccount[testBinding.AccountID] != second {
		t.Fatal("replacement did not leave exactly one account attempt")
	}
	if err := manager.Consume(first, testBinding, func([]byte) error { return nil }); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old attempt error = %v", err)
	}
	if err := manager.Consume(second, testBinding, func([]byte) error { return nil }); !errors.Is(err, ErrBindingMismatch) {
		t.Fatalf("old generation error = %v", err)
	}
	if err := manager.Consume(second, replacementBinding, func([]byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestEntropyFailureAndCollisionAreFailClosed(t *testing.T) {
	t.Run("read failure", func(t *testing.T) {
		clock := newFakeClock()
		protector := &testProtector{}
		manager, err := NewWithDependencies(clock, failingReader{}, protector)
		if err != nil {
			t.Fatal(err)
		}
		payload := []byte(testChallenge)
		if _, err := manager.Create(testBinding, payload, time.Minute); !errors.Is(err, ErrEntropy) {
			t.Fatalf("error = %v", err)
		}
		requireZeroed(t, payload)
		requireDestroyed(t, protector.handle(0))
		if len(manager.byID) != 0 {
			t.Fatal("entropy failure left an active attempt")
		}
	})

	t.Run("collision", func(t *testing.T) {
		clock := newFakeClock()
		protector := &testProtector{}
		manager, err := NewWithDependencies(clock, zeroReader{}, protector)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = manager.RevokeAll() })
		first := createAttempt(t, manager, testBinding, time.Minute)
		secondBinding := Binding{AccountID: "76561198000000002", VaultGeneration: "generation-a"}
		payload := []byte(testChallenge)
		if _, err := manager.Create(secondBinding, payload, time.Minute); !errors.Is(err, ErrEntropy) {
			t.Fatalf("collision error = %v", err)
		}
		requireZeroed(t, payload)
		requireDestroyed(t, protector.handle(1))
		if len(manager.byID) != 1 || manager.byAccount[testBinding.AccountID] != first {
			t.Fatal("collision changed the existing attempt")
		}
	})
}

func TestSecureMemoryFailuresAreTypedAndSingleUse(t *testing.T) {
	t.Run("store", func(t *testing.T) {
		clock := newFakeClock()
		protector := &testProtector{failStore: true}
		manager, err := NewWithDependencies(clock, &sequenceEntropy{}, protector)
		if err != nil {
			t.Fatal(err)
		}
		payload := []byte(testChallenge)
		if _, err := manager.Create(testBinding, payload, time.Minute); !errors.Is(err, ErrSecureMemory) {
			t.Fatalf("error = %v", err)
		}
		requireZeroed(t, payload)
		requireZeroed(t, protector.input(0))
	})

	t.Run("consume", func(t *testing.T) {
		clock := newFakeClock()
		protector := &testProtector{failWith: true}
		manager, err := NewWithDependencies(clock, &sequenceEntropy{}, protector)
		if err != nil {
			t.Fatal(err)
		}
		id := createAttempt(t, manager, testBinding, time.Minute)
		if err := manager.Consume(id, testBinding, func([]byte) error {
			t.Fatal("secure-memory failure invoked callback")
			return nil
		}); !errors.Is(err, ErrSecureMemory) {
			t.Fatalf("error = %v", err)
		}
		requireDestroyed(t, protector.handle(0))
		if err := manager.Consume(id, testBinding, func([]byte) error { return nil }); !errors.Is(err, ErrNotFound) {
			t.Fatalf("replay error = %v", err)
		}
	})

	t.Run("destroy", func(t *testing.T) {
		clock := newFakeClock()
		protector := &testProtector{destroyFailures: 1}
		manager, err := NewWithDependencies(clock, &sequenceEntropy{}, protector)
		if err != nil {
			t.Fatal(err)
		}
		id := createAttempt(t, manager, testBinding, time.Minute)
		if err := manager.Consume(id, testBinding, func([]byte) error { return nil }); !errors.Is(err, ErrSecureMemory) {
			t.Fatalf("error = %v", err)
		}
		requireDestroyed(t, protector.handle(0))
		if err := manager.Consume(id, testBinding, func([]byte) error { return nil }); !errors.Is(err, ErrNotFound) {
			t.Fatalf("replay error = %v", err)
		}
	})
}

func TestConsumeCallbackErrorAndPanicStillDestroy(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		manager, protector, _ := newTestManager(t)
		id := createAttempt(t, manager, testBinding, time.Minute)
		var retained []byte
		err := manager.Consume(id, testBinding, func(secret []byte) error {
			retained = secret
			return errCallback
		})
		if !errors.Is(err, errCallback) {
			t.Fatalf("error = %v", err)
		}
		requireZeroed(t, retained)
		requireDestroyed(t, protector.handle(0))
	})

	t.Run("panic", func(t *testing.T) {
		manager, protector, _ := newTestManager(t)
		id := createAttempt(t, manager, testBinding, time.Minute)
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("callback panic was swallowed")
				}
			}()
			_ = manager.Consume(id, testBinding, func([]byte) error { panic("boom") })
		}()
		requireDestroyed(t, protector.handle(0))
	})
}

func TestConsumeIsAtomicallySingleUse(t *testing.T) {
	manager, _, _ := newTestManager(t)
	id := createAttempt(t, manager, testBinding, time.Minute)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- manager.Consume(id, testBinding, func([]byte) error {
			close(entered)
			<-release
			return nil
		})
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first consume did not enter callback")
	}
	if err := manager.Consume(id, testBinding, func([]byte) error { return nil }); !errors.Is(err, ErrNotFound) {
		t.Fatalf("concurrent replay error = %v", err)
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("first consume did not finish")
	}
}

func TestRevokeAccountAndAll(t *testing.T) {
	manager, protector, _ := newTestManager(t)
	first := createAttempt(t, manager, testBinding, time.Minute)
	secondBinding := Binding{AccountID: "76561198000000002", VaultGeneration: "generation-a"}
	second := createAttempt(t, manager, secondBinding, time.Minute)
	if err := manager.RevokeAccount(testBinding.AccountID); err != nil {
		t.Fatal(err)
	}
	requireDestroyed(t, protector.handle(0))
	if err := manager.Consume(first, testBinding, func([]byte) error { return nil }); !errors.Is(err, ErrNotFound) {
		t.Fatalf("account revoke error = %v", err)
	}
	if err := manager.RevokeAll(); err != nil {
		t.Fatal(err)
	}
	requireDestroyed(t, protector.handle(1))
	if err := manager.Consume(second, secondBinding, func([]byte) error { return nil }); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoke-all error = %v", err)
	}
}

func TestManagerCapacityIsBounded(t *testing.T) {
	manager, protector, _ := newTestManager(t)
	for index := 0; index < MaxActiveAttempts; index++ {
		binding := Binding{
			AccountID:       accountID(index),
			VaultGeneration: "generation-a",
		}
		_ = createAttempt(t, manager, binding, MaxTTL)
	}
	overflow := Binding{AccountID: accountID(MaxActiveAttempts), VaultGeneration: "generation-a"}
	payload := []byte(testChallenge)
	if _, err := manager.Create(overflow, payload, MaxTTL); !errors.Is(err, ErrCapacity) {
		t.Fatalf("capacity error = %v", err)
	}
	requireZeroed(t, payload)
	requireDestroyed(t, protector.handle(MaxActiveAttempts))
	if len(manager.byID) != MaxActiveAttempts {
		t.Fatalf("active attempts = %d", len(manager.byID))
	}
}

func TestNewWithDependenciesRejectsNil(t *testing.T) {
	clock := newFakeClock()
	protector := &testProtector{}
	if _, err := NewWithDependencies(nil, &sequenceEntropy{}, protector); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("clock error = %v", err)
	}
	if _, err := NewWithDependencies(clock, nil, protector); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("entropy error = %v", err)
	}
	if _, err := NewWithDependencies(clock, &sequenceEntropy{}, nil); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("protector error = %v", err)
	}
	var zero Manager
	if _, err := zero.Create(testBinding, []byte(testChallenge), time.Minute); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("zero-value manager error = %v", err)
	}
}

func createAttempt(t *testing.T, manager *Manager, binding Binding, ttl time.Duration) ID {
	t.Helper()
	id, err := manager.Create(binding, []byte(testChallenge), ttl)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func accountID(index int) string {
	return "account-" + time.Unix(int64(index), 0).UTC().Format("150405.000000000")
}

func newTestManager(t *testing.T) (*Manager, *testProtector, *fakeClock) {
	t.Helper()
	clock := newFakeClock()
	protector := &testProtector{}
	manager, err := NewWithDependencies(clock, &sequenceEntropy{}, protector)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.RevokeAll() })
	return manager, protector, clock
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Unix(1_700_000_000, 0)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Add(duration time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(duration)
	c.mu.Unlock()
}

type sequenceEntropy struct {
	mu       sync.Mutex
	sequence uint64
}

func (r *sequenceEntropy) Read(output []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sequence++
	clear(output)
	if len(output) >= 8 {
		binary.LittleEndian.PutUint64(output[:8], r.sequence)
	}
	for index := 8; index < len(output); index++ {
		output[index] = byte(index) ^ byte(r.sequence)
	}
	return len(output), nil
}

type zeroReader struct{}

func (zeroReader) Read(output []byte) (int, error) {
	clear(output)
	return len(output), nil
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

type testProtector struct {
	mu              sync.Mutex
	failStore       bool
	failWith        bool
	destroyFailures int
	inputs          [][]byte
	handles         []*testSecureHandle
}

func (p *testProtector) Store(secret []byte) (securemem.Handle, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.inputs = append(p.inputs, secret)
	if p.failStore {
		return nil, securemem.ErrUnavailable
	}
	handle := &testSecureHandle{
		value:           append([]byte(nil), secret...),
		failWith:        p.failWith,
		destroyFailures: p.destroyFailures,
	}
	p.handles = append(p.handles, handle)
	return handle, nil
}

func (p *testProtector) input(index int) []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.inputs[index]
}

func (p *testProtector) handle(index int) *testSecureHandle {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.handles[index]
}

type testSecureHandle struct {
	mu              sync.Mutex
	value           []byte
	failWith        bool
	destroyFailures int
	destroyed       bool
}

func (h *testSecureHandle) With(fn func([]byte) error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.destroyed || h.failWith {
		return securemem.ErrUnavailable
	}
	copyOfSecret := append([]byte(nil), h.value...)
	defer wipe(copyOfSecret)
	return fn(copyOfSecret)
}

func (h *testSecureHandle) Destroy() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.destroyed {
		wipe(h.value)
		h.value = nil
		h.destroyed = true
	}
	if h.destroyFailures > 0 {
		h.destroyFailures--
		return securemem.ErrUnavailable
	}
	return nil
}

func requireDestroyed(t *testing.T, handle *testSecureHandle) {
	t.Helper()
	handle.mu.Lock()
	defer handle.mu.Unlock()
	if !handle.destroyed || handle.value != nil {
		t.Fatal("secure-memory handle was not destroyed")
	}
}

func requireZeroed(t *testing.T, value []byte) {
	t.Helper()
	for _, item := range value {
		if item != 0 {
			t.Fatal("buffer was not wiped")
		}
	}
}
