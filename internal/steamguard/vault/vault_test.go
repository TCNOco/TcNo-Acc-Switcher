package vault

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/passwordpolicy"
	"TcNo-Acc-Switcher/internal/steamguard/securemem"
)

type testHardener struct{}

func (testHardener) HardenDir(string) error  { return nil }
func (testHardener) HardenFile(string) error { return nil }

type testProtector struct{ fail bool }
type testHandle struct {
	mu    sync.Mutex
	value []byte
	dead  bool
}

func (p testProtector) Store(secret []byte) (securemem.Handle, error) {
	if p.fail {
		return nil, securemem.ErrUnavailable
	}
	return &testHandle{value: append([]byte(nil), secret...)}, nil
}
func (h *testHandle) With(fn func([]byte) error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.dead {
		return securemem.ErrUnavailable
	}
	b := append([]byte(nil), h.value...)
	defer wipe(b)
	return fn(b)
}
func (h *testHandle) Destroy() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	wipe(h.value)
	h.value = nil
	h.dead = true
	return nil
}

type destroyTrackingProtector struct {
	mu      sync.Mutex
	handles []*destroyTrackingHandle
}

type destroyTrackingHandle struct {
	mu          sync.Mutex
	value       []byte
	failDestroy bool
	destroyed   bool
}

func (p *destroyTrackingProtector) Store(secret []byte) (securemem.Handle, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	h := &destroyTrackingHandle{value: append([]byte(nil), secret...), failDestroy: len(p.handles) == 0}
	p.handles = append(p.handles, h)
	return h, nil
}

func (h *destroyTrackingHandle) With(fn func([]byte) error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.destroyed {
		return securemem.ErrUnavailable
	}
	b := append([]byte(nil), h.value...)
	defer wipe(b)
	return fn(b)
}

func (h *destroyTrackingHandle) Destroy() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.failDestroy {
		return errors.New("simulated destroy failure")
	}
	wipe(h.value)
	h.value = nil
	h.destroyed = true
	return nil
}

type recordingHardener struct {
	dirs []string
	err  error
}

func (h *recordingHardener) HardenDir(path string) error {
	h.dirs = append(h.dirs, path)
	return h.err
}
func (*recordingHardener) HardenFile(string) error { return nil }

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *testClock) Add(d time.Duration) { c.mu.Lock(); c.now = c.now.Add(d); c.mu.Unlock() }

func fastOptions(extra ...Option) []Option {
	fastKDF := KDFParams{Algorithm: "argon2id", MemoryKiB: minMemoryKiB, Passes: 1, Lanes: 1, KeyBytes: keyBytes}
	base := []Option{
		WithKDFParams(fastKDF), WithRecoveryKDFParams(fastKDF),
		WithHardener(testHardener{}), WithSecureMemory(testProtector{}),
	}
	return append(base, extra...)
}

func newUnlocked(t *testing.T, extra ...Option) *Vault {
	t.Helper()
	v, err := Create(t.TempDir(), "correct horse battery staple", fastOptions(extra...)...)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Unlock("correct horse battery staple", FixedLease); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestWrongPasswordAndLock(t *testing.T) {
	const password = "right password with enough length"
	v, err := Create(t.TempDir(), password, fastOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Unlock("wrong", FixedLease); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("wrong password: %v", err)
	}
	if err := v.Unlock(password, FixedLease); err != nil {
		t.Fatal(err)
	}
	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}
	if _, err := v.List(); !errors.Is(err, ErrLocked) {
		t.Fatalf("list after lock: %v", err)
	}
}

func TestCreateAndChangeAcceptUserSelectedPasswords(t *testing.T) {
	v, err := Create(t.TempDir(), "x", fastOptions()...)
	if err != nil {
		t.Fatalf("Create(one character) error = %v", err)
	}
	if err := v.Unlock("x", FixedLease); err != nil {
		t.Fatal(err)
	}
	if err := v.ChangePassword("x", "password"); err != nil {
		t.Fatalf("ChangePassword(common value) error = %v", err)
	}
	if err := v.Unlock("password", FixedLease); err != nil {
		t.Fatalf("changed password did not unlock: %v", err)
	}
	if _, err := Create(t.TempDir(), "", fastOptions()...); !errors.Is(err, ErrInvalidPassword) || !errors.Is(err, passwordpolicy.ErrEmpty) {
		t.Fatalf("Create(empty) error = %v", err)
	}
}

func TestCRUDPasswordChangeAndReopen(t *testing.T) {
	const newPassword = "new password with sufficient length"
	v := newUnlocked(t)
	id, err := v.Put("76561198000000001", []byte("shared-secret-and-token"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := v.Get(id)
	if err != nil || string(got) != "shared-secret-and-token" {
		t.Fatalf("get=%q err=%v", got, err)
	}
	list, err := v.List()
	if err != nil || len(list) != 1 || list[0].SteamID64 != "76561198000000001" {
		t.Fatalf("list=%v err=%v", list, err)
	}
	lease := v.lease.(*testHandle)
	if err := v.ChangePassword("correct horse battery staple", newPassword); err != nil {
		t.Fatal(err)
	}
	if !v.IsLocked() {
		t.Fatal("password change did not revoke the active lease")
	}
	if !lease.dead {
		t.Fatal("password change did not destroy the protected key handle")
	}
	root := v.root
	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(root, fastOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Unlock("correct horse battery staple", FixedLease); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("old password: %v", err)
	}
	if err := reopened.Unlock(newPassword, FixedLease); err != nil {
		t.Fatal(err)
	}
	if err := reopened.Delete(id); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Get(id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get deleted: %v", err)
	}
}

func TestOuterLayerRoundTripWrongKeyTamperAndPlaintextSentinel(t *testing.T) {
	v := newUnlocked(t)
	const sentinel = "OUTER-PLAINTEXT-SENTINEL-7741"
	id, err := v.Put("76561198000000042", []byte(sentinel))
	if err != nil {
		t.Fatal(err)
	}
	innerGenPath, _ := generationPath(v.root, v.active)
	innerKeyring, err := os.ReadFile(filepath.Join(innerGenPath, keyringName))
	if err != nil {
		t.Fatal(err)
	}
	outerKey := bytes.Repeat([]byte{0x42}, keyBytes)
	if err := v.EnableOuter(outerKey); err != nil {
		t.Fatal(err)
	}
	if v.header.OuterVersion != OuterLayerVersion {
		t.Fatalf("outer version = %d", v.header.OuterVersion)
	}
	genPath, _ := generationPath(v.root, v.active)
	disk, err := os.ReadFile(filepath.Join(genPath, keyringName))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(disk, []byte(sentinel)) {
		t.Fatal("outer file contains plaintext sentinel")
	}
	if bytes.Contains(disk, innerKeyring) {
		t.Fatal("outer file contains the unchanged inner ciphertext record")
	}
	var wrapped outerFile
	if err := unmarshalStrict(disk, &wrapped); err != nil || wrapped.Version != OuterLayerVersion {
		t.Fatalf("keyring is not an outer record: %+v err=%v", wrapped, err)
	}
	root := v.root
	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(root, fastOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Unlock("correct horse battery staple", FixedLease); !errors.Is(err, ErrOuterKeyRequired) {
		t.Fatalf("unlock without outer key: %v", err)
	}
	wrongKey := bytes.Repeat([]byte{0x24}, keyBytes)
	if err := reopened.UnlockWithOuter("correct horse battery staple", wrongKey, FixedLease); !errors.Is(err, ErrInvalidOuterKey) {
		t.Fatalf("unlock with wrong outer key: %v", err)
	}
	if err := reopened.UnlockWithOuter("correct horse battery staple", outerKey, FixedLease); err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Get(id)
	if err != nil || string(got) != sentinel {
		t.Fatalf("get=%q err=%v", got, err)
	}

	genPath, _ = generationPath(reopened.root, reopened.active)
	keyringPath := filepath.Join(genPath, keyringName)
	tampered, err := os.ReadFile(keyringPath)
	if err != nil {
		t.Fatal(err)
	}
	tampered[len(tampered)/2] ^= 1
	if err := os.WriteFile(keyringPath, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := reopened.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := reopened.UnlockWithOuter("correct horse battery staple", outerKey, FixedLease); err == nil {
		t.Fatal("tampered outer keyring unlocked")
	}
}

func TestOuterMigrationRollsBackOnTransactionFailure(t *testing.T) {
	armed := false
	hook := func(stage string) error {
		if armed && stage == "after-switch" {
			armed = false
			return errors.New("simulated migration failure")
		}
		return nil
	}
	v := newUnlocked(t, WithTransactionHook(hook))
	id, err := v.Put("76561198000000043", []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	outerKey := bytes.Repeat([]byte{0x51}, keyBytes)
	armed = true
	if err := v.EnableOuter(outerKey); err == nil {
		t.Fatal("outer migration unexpectedly succeeded")
	}
	if v.header.OuterVersion != 0 {
		t.Fatalf("failed enable left outer version %d", v.header.OuterVersion)
	}
	if got, err := v.Get(id); err != nil || string(got) != "payload" {
		t.Fatalf("record after enable rollback=%q err=%v", got, err)
	}
	if err := v.EnableOuter(outerKey); err != nil {
		t.Fatal(err)
	}
	armed = true
	if err := v.DisableOuter(outerKey); err == nil {
		t.Fatal("outer removal unexpectedly succeeded")
	}
	if v.header.OuterVersion != OuterLayerVersion {
		t.Fatalf("failed disable left outer version %d", v.header.OuterVersion)
	}
	if got, err := v.Get(id); err != nil || string(got) != "payload" {
		t.Fatalf("record after disable rollback=%q err=%v", got, err)
	}
}

func TestOuterMigrationWorksWhileInnerVaultIsLocked(t *testing.T) {
	v := newUnlocked(t)
	id, err := v.Put("76561198000000044", []byte("locked-migration-payload"))
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}
	outerKey := bytes.Repeat([]byte{0x63}, keyBytes)
	if err := v.EnableOuter(outerKey); err != nil {
		t.Fatalf("enable while locked: %v", err)
	}
	if !v.IsLocked() {
		t.Fatal("outer migration unlocked the inner vault")
	}
	if err := v.UnlockWithOuter("correct horse battery staple", outerKey, FixedLease); err != nil {
		t.Fatal(err)
	}
	if got, err := v.Get(id); err != nil || string(got) != "locked-migration-payload" {
		t.Fatalf("get after locked enable=%q err=%v", got, err)
	}
	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := v.DisableOuter(outerKey); err != nil {
		t.Fatalf("disable while locked: %v", err)
	}
	if err := v.Unlock("correct horse battery staple", FixedLease); err != nil {
		t.Fatal(err)
	}
	if got, err := v.Get(id); err != nil || string(got) != "locked-migration-payload" {
		t.Fatalf("get after locked disable=%q err=%v", got, err)
	}
}

func TestOuterProofTamperIsRejected(t *testing.T) {
	v := newUnlocked(t)
	outerKey := bytes.Repeat([]byte{0x71}, keyBytes)
	if err := v.EnableOuter(outerKey); err != nil {
		t.Fatal(err)
	}
	genPath, _ := generationPath(v.root, v.active)
	headerPath := filepath.Join(genPath, headerName)
	var diskHeader header
	if err := readJSONFile(headerPath, maxHeader, &diskHeader); err != nil {
		t.Fatal(err)
	}
	last := 0
	replacement := byte('A')
	if diskHeader.OuterProof.Ciphertext[last] == replacement {
		replacement = 'B'
	}
	diskHeader.OuterProof.Ciphertext = diskHeader.OuterProof.Ciphertext[:last] + string(replacement) + diskHeader.OuterProof.Ciphertext[last+1:]
	raw, err := marshalJSON(diskHeader)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(headerPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(v.root, fastOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.UnlockWithOuter("correct horse battery staple", outerKey, FixedLease); !errors.Is(err, ErrInvalidOuterKey) {
		t.Fatalf("tampered outer proof error = %v", err)
	}
}

func TestPutRecordPlaintextBoundary(t *testing.T) {
	v := newUnlocked(t)
	tooLarge := make([]byte, maxPlainBytes+1)
	if _, err := v.Put("76561198000000101", tooLarge); !errors.Is(err, ErrInvalidFormat) {
		t.Fatalf("oversized plaintext: %v", err)
	}
	atLimit := make([]byte, maxPlainBytes)
	atLimit[0], atLimit[len(atLimit)-1] = 0x5a, 0xa5
	id, err := v.Put("76561198000000102", atLimit)
	if err != nil {
		t.Fatalf("plaintext at limit: %v", err)
	}
	got, err := v.Get(id)
	if err != nil || len(got) != len(atLimit) || got[0] != 0x5a || got[len(got)-1] != 0xa5 {
		t.Fatalf("boundary round trip len=%d err=%v", len(got), err)
	}
}

func TestFixedLeaseDoesNotSlideAndProcessLeasePersists(t *testing.T) {
	clock := &testClock{now: time.Unix(1_700_000_000, 0)}
	v := newUnlocked(t, WithClock(clock))
	clock.Add(4 * time.Minute)
	if _, err := v.List(); err != nil {
		t.Fatal(err)
	}
	clock.Add(61 * time.Second)
	if _, err := v.List(); !errors.Is(err, ErrLeaseExpired) {
		t.Fatalf("expected expiry, got %v", err)
	}
	if err := v.Unlock("correct horse battery staple", ProcessLease); err != nil {
		t.Fatal(err)
	}
	clock.Add(24 * time.Hour)
	if _, err := v.List(); err != nil {
		t.Fatalf("process lease slid/expired: %v", err)
	}
}

func TestSetLeaseModeChangesExistingProtectedLease(t *testing.T) {
	clock := &testClock{now: time.Unix(1_700_000_000, 0)}
	v := newUnlocked(t, WithClock(clock))
	if err := v.SetLeaseMode(ProcessLease); err != nil {
		t.Fatal(err)
	}
	clock.Add(24 * time.Hour)
	if _, err := v.List(); err != nil {
		t.Fatalf("promoted process lease expired: %v", err)
	}
	if err := v.SetLeaseMode(FixedLease); err != nil {
		t.Fatal(err)
	}
	clock.Add(FixedLeaseLength)
	if _, err := v.List(); !errors.Is(err, ErrLeaseExpired) {
		t.Fatalf("demoted fixed lease did not expire: %v", err)
	}
}

func TestFixedLeaseExpiresAtExactFiveMinuteBoundary(t *testing.T) {
	clock := &testClock{now: time.Unix(1_700_000_000, 0)}
	v := newUnlocked(t, WithClock(clock))
	lease := v.lease.(*testHandle)

	clock.Add(FixedLeaseLength - time.Nanosecond)
	if _, err := v.List(); err != nil {
		t.Fatalf("lease expired before boundary: %v", err)
	}
	if lease.dead {
		t.Fatal("lease handle destroyed before boundary")
	}

	clock.Add(time.Nanosecond)
	if _, err := v.List(); !errors.Is(err, ErrLeaseExpired) {
		t.Fatalf("exact-boundary access: %v", err)
	}
	if !lease.dead || !v.IsLocked() {
		t.Fatal("exact-boundary expiry did not destroy and revoke the lease")
	}
}

func TestKDFBoundsCheckedOnOpen(t *testing.T) {
	v, err := Create(t.TempDir(), "KDF bounds test password", fastOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	gen, _ := generationPath(v.root, v.active)
	var h header
	path := filepath.Join(gen, headerName)
	if err := readJSONFile(path, maxHeader, &h); err != nil {
		t.Fatal(err)
	}
	h.KDF.MemoryKiB = maxMemoryKiB + 1
	raw, _ := marshalJSON(h)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(v.root, fastOptions()...); !errors.Is(err, ErrKDFBounds) {
		t.Fatalf("expected bounds error, got %v", err)
	}
}

func TestCorruptWrapperCiphertextAndTruncation(t *testing.T) {
	t.Run("wrapper", func(t *testing.T) {
		const password = "corrupt wrapper test password"
		v, err := Create(t.TempDir(), password, fastOptions()...)
		if err != nil {
			t.Fatal(err)
		}
		gen, _ := generationPath(v.root, v.active)
		path := filepath.Join(gen, headerName)
		var h header
		if err := readJSONFile(path, maxHeader, &h); err != nil {
			t.Fatal(err)
		}
		h.VaultKey.Ciphertext = h.VaultKey.Ciphertext[:len(h.VaultKey.Ciphertext)-2] + "AA"
		raw, _ := marshalJSON(h)
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		o, err := Open(v.root, fastOptions()...)
		if err != nil {
			t.Fatal(err)
		}
		if err := o.Unlock(password, FixedLease); !errors.Is(err, ErrInvalidPassword) {
			t.Fatalf("corrupt wrapper: %v", err)
		}
	})
	t.Run("record", func(t *testing.T) {
		v := newUnlocked(t)
		id, err := v.Put("76561198000000002", []byte("secret"))
		if err != nil {
			t.Fatal(err)
		}
		var entry recordEntry
		if err := v.withKeyLocked(func(key []byte) error {
			ring, e := v.loadKeyring(key, nil)
			if e == nil {
				entry = ring.Records[0]
			}
			return e
		}); err != nil {
			t.Fatal(err)
		}
		gen, _ := generationPath(v.root, v.active)
		path, _ := recordPath(gen, entry.Filename)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw[:len(raw)/2], 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := v.Get(id); err == nil {
			t.Fatal("truncated record was accepted")
		}
	})
	t.Run("keyring", func(t *testing.T) {
		v := newUnlocked(t)
		gen, _ := generationPath(v.root, v.active)
		path := filepath.Join(gen, keyringName)
		raw, _ := os.ReadFile(path)
		_ = os.WriteFile(path, raw[:len(raw)/2], 0o600)
		if _, err := v.List(); err == nil {
			t.Fatal("truncated keyring was accepted")
		}
	})
}

func TestAccountSwapAndAADCorruptionRejected(t *testing.T) {
	v := newUnlocked(t)
	id1, _ := v.Put("76561198000000003", []byte("first-secret"))
	_, _ = v.Put("76561198000000004", []byte("second-secret"))
	var entries []recordEntry
	if err := v.withKeyLocked(func(key []byte) error { ring, err := v.loadKeyring(key, nil); entries = ring.Records; return err }); err != nil {
		t.Fatal(err)
	}
	gen, _ := generationPath(v.root, v.active)
	p1, _ := recordPath(gen, entries[0].Filename)
	p2, _ := recordPath(gen, entries[1].Filename)
	raw1, _ := os.ReadFile(p1)
	raw2, _ := os.ReadFile(p2)
	if err := os.WriteFile(p1, raw2, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p2, raw1, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Get(id1); err == nil {
		t.Fatal("swapped account records were accepted")
	}
}

func TestExplicitAADCorruptionRejected(t *testing.T) {
	v := newUnlocked(t)
	id, err := v.Put("76561198000000013", []byte("aad-bound-secret"))
	if err != nil {
		t.Fatal(err)
	}
	var entry recordEntry
	var malformed recordFile
	if err := v.withKeyLocked(func(key []byte) error {
		ring, err := v.loadKeyring(key, nil)
		if err != nil {
			return err
		}
		entry = ring.Records[0]
		dataKey, err := openEnvelope(key, entry.WrappedKey, aad(FormatVersion, v.header.VaultID, entry.ID, entry.SteamID64, "data-key"))
		if err != nil {
			return err
		}
		defer wipe(dataKey)
		ciphertext, err := seal(dataKey, []byte("aad-bound-secret"), aad(FormatVersion, v.header.VaultID, entry.ID, "76561198000000999", "record"))
		malformed = recordFile{Version: FormatVersion, RecordID: entry.ID, Ciphertext: ciphertext}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	gen, _ := generationPath(v.root, v.active)
	path, _ := recordPath(gen, entry.Filename)
	raw, _ := marshalJSON(malformed)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Get(id); err == nil {
		t.Fatal("record encrypted under different SteamID AAD was accepted")
	}
}

func TestNoncesAreUnique(t *testing.T) {
	v := newUnlocked(t)
	seen := map[string]bool{}
	for i := 0; i < 12; i++ {
		id := "765611980000000" + string(rune('A'+i))
		if _, err := v.Put(id, []byte{byte(i)}); err != nil {
			t.Fatal(err)
		}
		if err := v.withKeyLocked(func(key []byte) error {
			ring, err := v.loadKeyring(key, nil)
			if err != nil {
				return err
			}
			entry := ring.Records[len(ring.Records)-1]
			gen, _ := generationPath(v.root, v.active)
			path, _ := recordPath(gen, entry.Filename)
			var rf recordFile
			if err := readJSONFile(path, maxRecord, &rf); err != nil {
				return err
			}
			for _, nonce := range []string{entry.WrappedKey.Nonce, rf.Ciphertext.Nonce} {
				if seen[nonce] {
					t.Fatalf("reused nonce %s", nonce)
				}
				seen[nonce] = true
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInterruptedTransactionRecovery(t *testing.T) {
	fail := false
	hook := func(stage string) error {
		if fail && stage == "after-journal" {
			return errors.New("simulated crash")
		}
		return nil
	}
	v := newUnlocked(t, WithTransactionHook(hook))
	fail = true
	if _, err := v.Put("76561198000000005", []byte("never committed")); err == nil {
		t.Fatal("expected simulated crash")
	}
	fail = false
	recovered, err := Open(v.root, fastOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(v.root, journalName)); !os.IsNotExist(err) {
		t.Fatalf("journal remains: %v", err)
	}
	if err := recovered.Unlock("correct horse battery staple", FixedLease); err != nil {
		t.Fatal(err)
	}
	list, err := recovered.List()
	if err != nil || len(list) != 0 {
		t.Fatalf("rollback list=%v err=%v", list, err)
	}
}

func TestInterruptedTransactionRecoveryAfterSwitch(t *testing.T) {
	fail := false
	hook := func(stage string) error {
		if fail && stage == "after-switch" {
			return errors.New("simulated crash after switch")
		}
		return nil
	}
	v := newUnlocked(t, WithTransactionHook(hook))
	fail = true
	if _, err := v.Put("76561198000000015", []byte("committed before crash")); err == nil {
		t.Fatal("expected simulated crash")
	}
	recovered, err := Open(v.root, fastOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	if err := recovered.Unlock("correct horse battery staple", FixedLease); err != nil {
		t.Fatal(err)
	}
	list, err := recovered.List()
	if err != nil || len(list) != 1 || list[0].SteamID64 != "76561198000000015" {
		t.Fatalf("committed generation not recovered: list=%v err=%v", list, err)
	}
}

func TestNoPlaintextSentinelsOnDisk(t *testing.T) {
	v, err := Create(t.TempDir(), "PASSWORD-SENTINEL-9d31", fastOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Unlock("PASSWORD-SENTINEL-9d31", FixedLease); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Put("76561198999999999", []byte("USERNAME-SENTINEL token=TOKEN-SENTINEL secret=SECRET-SENTINEL")); err != nil {
		t.Fatal(err)
	}
	var disk bytes.Buffer
	err = filepath.WalkDir(v.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		disk.WriteString(d.Name())
		if !d.IsDir() {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			disk.Write(raw)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, sentinel := range []string{"PASSWORD-SENTINEL-9d31", "76561198999999999", "USERNAME-SENTINEL", "TOKEN-SENTINEL", "SECRET-SENTINEL"} {
		if bytes.Contains(disk.Bytes(), []byte(sentinel)) {
			t.Fatalf("plaintext sentinel found: %s", sentinel)
		}
	}
}

func TestSecureMemoryFailureFailsClosed(t *testing.T) {
	const password = "secure memory fallback password"
	v, err := Create(t.TempDir(), password, fastOptions(WithSecureMemory(testProtector{fail: true}))...)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Unlock(password, FixedLease); !errors.Is(err, ErrSecureMemory) || !errors.Is(err, ErrOneOperationRequired) {
		t.Fatalf("unlock: %v", err)
	}
	if !v.IsLocked() {
		t.Fatal("vault retained a lease after secure-memory failure")
	}
}

func TestSecureMemoryFailureAllowsOneScopedReadOperation(t *testing.T) {
	const password = "one operation fallback password"
	root := t.TempDir()
	v, err := Create(root, password, fastOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Unlock(password, FixedLease); err != nil {
		t.Fatal(err)
	}
	id, err := v.Put("76561198000000301", []byte("one-operation-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}

	fallback, err := Open(root, fastOptions(WithSecureMemory(testProtector{fail: true}))...)
	if err != nil {
		t.Fatal(err)
	}
	if err := fallback.Unlock(password, ProcessLease); !errors.Is(err, ErrOneOperationRequired) {
		t.Fatalf("cached unlock should advertise fallback: %v", err)
	}
	var escaped *OneOperationAccess
	err = fallback.WithOneOperation(password, func(access *OneOperationAccess) error {
		escaped = access
		records, err := access.ListRecords()
		if err != nil || len(records) != 1 || records[0].ID != id {
			return errors.New("scoped list failed")
		}
		plain, err := access.GetRecord(id)
		if err != nil {
			return err
		}
		defer wipe(plain)
		if string(plain) != "one-operation-secret" {
			return errors.New("scoped record mismatch")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !fallback.IsLocked() || fallback.lease != nil || fallback.outerLease != nil {
		t.Fatal("one-operation fallback retained a cached key")
	}
	if _, err := escaped.ListRecords(); !errors.Is(err, ErrOneOperationExpired) {
		t.Fatalf("escaped scope remained usable: %v", err)
	}
	var escapedAfterPanic *OneOperationAccess
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("one-operation callback did not panic")
			}
		}()
		_ = fallback.WithOneOperation(password, func(access *OneOperationAccess) error {
			escapedAfterPanic = access
			panic("simulated callback panic")
		})
	}()
	if _, err := escapedAfterPanic.ListRecords(); !errors.Is(err, ErrOneOperationExpired) {
		t.Fatalf("panic left scope usable: %v", err)
	}
}

func TestOneOperationFallbackSupportsOuterEncryption(t *testing.T) {
	const password = "outer fallback password"
	root := t.TempDir()
	outerKey := bytes.Repeat([]byte{0xa7}, keyBytes)
	v, err := Create(root, password, fastOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Unlock(password, FixedLease); err != nil {
		t.Fatal(err)
	}
	id, err := v.Put("76561198000000302", []byte("outer-fallback-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if err := v.EnableOuter(outerKey); err != nil {
		t.Fatal(err)
	}
	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}

	fallback, err := Open(root, fastOptions(WithSecureMemory(testProtector{fail: true}))...)
	if err != nil {
		t.Fatal(err)
	}
	if err := fallback.UnlockWithOuter(password, outerKey, ProcessLease); !errors.Is(err, ErrOneOperationRequired) {
		t.Fatalf("cached outer unlock should advertise fallback: %v", err)
	}
	err = fallback.WithOneOperationWithOuter(password, outerKey, func(access *OneOperationAccess) error {
		plain, err := access.GetRecord(id)
		if err != nil {
			return err
		}
		defer wipe(plain)
		if string(plain) != "outer-fallback-secret" {
			return errors.New("outer scoped record mismatch")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !fallback.IsLocked() {
		t.Fatal("outer one-operation fallback retained a cached key")
	}
}

func TestLockRevokesProcessLeaseAndDestroysHandle(t *testing.T) {
	v := newUnlocked(t)
	if err := v.Unlock("correct horse battery staple", ProcessLease); err != nil {
		t.Fatal(err)
	}
	lease := v.lease.(*testHandle)
	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}
	if !lease.dead {
		t.Fatal("lock did not destroy process lease handle")
	}
	if _, err := v.List(); !errors.Is(err, ErrLocked) {
		t.Fatalf("access after lock: %v", err)
	}
}

func TestIntegrityFailureRevokesLease(t *testing.T) {
	v := newUnlocked(t)
	id, err := v.Put("76561198000000303", []byte("integrity-secret"))
	if err != nil {
		t.Fatal(err)
	}
	lease := v.lease.(*testHandle)
	var entry recordEntry
	if err := v.withKeyLocked(func(key []byte) error {
		ring, err := v.loadKeyring(key, nil)
		if err == nil {
			entry = ring.Records[0]
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	gen, _ := generationPath(v.root, v.active)
	path, _ := recordPath(gen, entry.Filename)
	if err := os.WriteFile(path, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Get(id); !errors.Is(err, ErrInvalidFormat) {
		t.Fatalf("tampered record: %v", err)
	}
	if !lease.dead || !v.IsLocked() {
		t.Fatal("integrity failure did not destroy and revoke the lease")
	}
}

func TestUnlockRetainsOldLeaseWhenDestroyFails(t *testing.T) {
	const password = "destroy failure test password"
	protector := &destroyTrackingProtector{}
	v, err := Create(t.TempDir(), password, fastOptions(WithSecureMemory(protector))...)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Unlock(password, FixedLease); err != nil {
		t.Fatal(err)
	}
	oldLease := v.lease
	if err := v.Unlock(password, ProcessLease); !errors.Is(err, ErrSecureMemory) {
		t.Fatalf("second unlock: %v", err)
	}
	if v.lease != oldLease {
		t.Fatal("failed destroy lost the old lease handle")
	}
	if len(protector.handles) != 2 || !protector.handles[1].destroyed {
		t.Fatal("replacement handle was not destroyed after old-lease failure")
	}
}

func TestMaliciousJournalCannotDeleteActiveGeneration(t *testing.T) {
	v, err := Create(t.TempDir(), "journal integrity test password", fastOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	activePath, _ := generationPath(v.root, v.active)
	txRaw, _ := marshalJSON(transaction{Version: FormatVersion, Previous: v.active, Next: v.active})
	if err := os.WriteFile(filepath.Join(v.root, journalName), txRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(v.root, fastOptions()...); !errors.Is(err, ErrInvalidFormat) {
		t.Fatalf("crafted journal: %v", err)
	}
	if info, err := os.Stat(activePath); err != nil || !info.IsDir() {
		t.Fatalf("active generation was deleted: %v", err)
	}
}

func TestOpenHardensRootBeforeReadingVaultFiles(t *testing.T) {
	v, err := Create(t.TempDir(), "root hardening test password", fastOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v.root, activeName), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("harden first")
	hardener := &recordingHardener{err: sentinel}
	if _, err := Open(v.root, fastOptions(WithHardener(hardener))...); !errors.Is(err, sentinel) {
		t.Fatalf("open error=%v", err)
	}
	if len(hardener.dirs) != 1 || hardener.dirs[0] != v.root {
		t.Fatalf("hardener calls=%v", hardener.dirs)
	}
}

func TestRecoveryWrapperRoundTripAndPasswordRewrap(t *testing.T) {
	v := newUnlocked(t)
	id, err := v.Put("76561198000000201", []byte("recovery-record-secret"))
	if err != nil {
		t.Fatal(err)
	}
	outerKey := bytes.Repeat([]byte{0x81}, keyBytes)
	const recoveryPassword = "independent app password"
	if err := v.EnableOuterWithRecovery(outerKey, recoveryPassword); err != nil {
		t.Fatal(err)
	}
	if !v.HasRecoveryWrapper() || v.header.Recovery == nil {
		t.Fatal("recovery wrapper was not persisted")
	}
	if v.header.Recovery.Version != RecoveryVersion {
		t.Fatalf("recovery version=%d", v.header.Recovery.Version)
	}
	if v.header.Recovery.Salt == v.header.Salt {
		t.Fatal("recovery and vault wrappers reused a salt")
	}
	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := v.UnlockWithRecovery("correct horse battery staple", "wrong", FixedLease); !errors.Is(err, ErrInvalidRecoveryPassword) {
		t.Fatalf("wrong recovery password: %v", err)
	}
	if err := v.UnlockWithRecovery("correct horse battery staple", recoveryPassword, FixedLease); err != nil {
		t.Fatal(err)
	}
	if got, err := v.Get(id); err != nil || string(got) != "recovery-record-secret" {
		t.Fatalf("recovered record=%q err=%v", got, err)
	}
	if err := v.ChangeRecoveryPassword(recoveryPassword, "new app password"); err != nil {
		t.Fatal(err)
	}
	if err := v.VerifyRecovery("correct horse battery staple", recoveryPassword); !errors.Is(err, ErrInvalidRecoveryPassword) {
		t.Fatalf("old recovery password: %v", err)
	}
	if err := v.VerifyRecovery("correct horse battery staple", "new app password"); err != nil {
		t.Fatal(err)
	}
	if err := v.ChangePassword("correct horse battery staple", "new Steam Guard password"); err != nil {
		t.Fatal(err)
	}
	if err := v.VerifyRecovery("new Steam Guard password", "new app password"); err != nil {
		t.Fatalf("recovery after vault password change: %v", err)
	}
	if err := v.DisableOuter(outerKey); err != nil {
		t.Fatal(err)
	}
	if v.HasRecoveryWrapper() {
		t.Fatal("removing the outer layer retained recovery metadata")
	}
	if err := v.Unlock("new Steam Guard password", FixedLease); err != nil {
		t.Fatal(err)
	}
}

func TestRecoveryKDFIsDomainSeparated(t *testing.T) {
	params := KDFParams{Algorithm: "argon2id", MemoryKiB: minMemoryKiB, Passes: 1, Lanes: 1, KeyBytes: keyBytes}
	salt := bytes.Repeat([]byte{0x91}, saltBytes)
	standard, err := derive("same password", salt, params)
	if err != nil {
		t.Fatal(err)
	}
	defer wipe(standard)
	recovery, err := deriveRecovery("same password", salt, params, RecoveryVersion)
	if err != nil {
		t.Fatal(err)
	}
	defer wipe(recovery)
	if bytes.Equal(standard, recovery) {
		t.Fatal("recovery KDF reused the vault-password domain")
	}
}

func TestRecoveryWrapperRejectsTamperAndLeavesNoPasswordOnDisk(t *testing.T) {
	v := newUnlocked(t)
	if _, err := v.Put("76561198000000202", []byte("authenticated-record")); err != nil {
		t.Fatal(err)
	}
	outerKey := bytes.Repeat([]byte{0x82}, keyBytes)
	const recoveryPassword = "RECOVERY-PASSWORD-SENTINEL-2981"
	if err := v.EnableOuterWithRecovery(outerKey, recoveryPassword); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(v.root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(raw, []byte(recoveryPassword)) {
			t.Fatalf("recovery password persisted in %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	genPath, _ := generationPath(v.root, v.active)
	headerPath := filepath.Join(genPath, headerName)
	var diskHeader header
	if err := readJSONFile(headerPath, maxHeader, &diskHeader); err != nil {
		t.Fatal(err)
	}
	ciphertext := []byte(diskHeader.Recovery.OuterKey.Ciphertext)
	if ciphertext[0] == 'A' {
		ciphertext[0] = 'B'
	} else {
		ciphertext[0] = 'A'
	}
	diskHeader.Recovery.OuterKey.Ciphertext = string(ciphertext)
	raw, err := marshalJSON(diskHeader)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(headerPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(v.root, fastOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.UnlockWithRecovery("correct horse battery staple", recoveryPassword, FixedLease); !errors.Is(err, ErrInvalidRecoveryPassword) {
		t.Fatalf("tampered recovery wrapper: %v", err)
	}
}

func TestRecoveryMigrationRollbackAndLegacyOuterCompatibility(t *testing.T) {
	armed := false
	hook := func(stage string) error {
		if armed && stage == "after-switch" {
			armed = false
			return errors.New("simulated recovery migration failure")
		}
		return nil
	}
	v := newUnlocked(t, WithTransactionHook(hook))
	id, err := v.Put("76561198000000203", []byte("rollback-record"))
	if err != nil {
		t.Fatal(err)
	}
	outerKey := bytes.Repeat([]byte{0x83}, keyBytes)
	if err := v.EnableOuter(outerKey); err != nil {
		t.Fatal(err)
	}
	if v.HasRecoveryWrapper() {
		t.Fatal("legacy outer migration unexpectedly added recovery metadata")
	}
	if err := v.UnlockWithRecovery("correct horse battery staple", "app password", FixedLease); !errors.Is(err, ErrRecoveryNotConfigured) {
		t.Fatalf("legacy recovery error: %v", err)
	}
	armed = true
	if err := v.ConfigureRecovery(outerKey, "app password"); err == nil {
		t.Fatal("recovery migration unexpectedly succeeded")
	}
	if v.HasRecoveryWrapper() {
		t.Fatal("failed migration left recovery metadata active")
	}
	if got, err := v.Get(id); err != nil || string(got) != "rollback-record" {
		t.Fatalf("record after recovery rollback=%q err=%v", got, err)
	}
	if err := v.ConfigureRecovery(outerKey, "app password"); err != nil {
		t.Fatal(err)
	}
	if err := v.VerifyRecovery("correct horse battery staple", "app password"); err != nil {
		t.Fatal(err)
	}
}

func TestEnableOuterWithRecoveryRollsBackBothLayers(t *testing.T) {
	armed := false
	hook := func(stage string) error {
		if armed && stage == "after-switch" {
			armed = false
			return errors.New("simulated combined migration failure")
		}
		return nil
	}
	v := newUnlocked(t, WithTransactionHook(hook))
	id, err := v.Put("76561198000000205", []byte("combined-rollback-record"))
	if err != nil {
		t.Fatal(err)
	}
	outerKey := bytes.Repeat([]byte{0x86}, keyBytes)
	armed = true
	if err := v.EnableOuterWithRecovery(outerKey, "app password"); err == nil {
		t.Fatal("combined migration unexpectedly succeeded")
	}
	if v.header.OuterVersion != 0 || v.HasRecoveryWrapper() {
		t.Fatalf("failed migration left outer=%d recovery=%v", v.header.OuterVersion, v.HasRecoveryWrapper())
	}
	if got, err := v.Get(id); err != nil || string(got) != "combined-rollback-record" {
		t.Fatalf("record after combined rollback=%q err=%v", got, err)
	}
}

func TestRestoreOuterFromRecoveryRotatesInstallationKey(t *testing.T) {
	v := newUnlocked(t)
	id, err := v.Put("76561198000000204", []byte("clean-machine-restore-record"))
	if err != nil {
		t.Fatal(err)
	}
	oldOuterKey := bytes.Repeat([]byte{0x84}, keyBytes)
	newOuterKey := bytes.Repeat([]byte{0x85}, keyBytes)
	if err := v.EnableOuterWithRecovery(oldOuterKey, "old app password"); err != nil {
		t.Fatal(err)
	}
	root := v.root
	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(root, fastOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	if err := restored.VerifyRecovery("correct horse battery staple", "old app password"); err != nil {
		t.Fatal(err)
	}
	if err := restored.RestoreOuterFromRecovery("old app password", newOuterKey, "new app password"); err != nil {
		t.Fatal(err)
	}
	if err := restored.UnlockWithOuter("correct horse battery staple", oldOuterKey, FixedLease); !errors.Is(err, ErrInvalidOuterKey) {
		t.Fatalf("old installation key after restore: %v", err)
	}
	if err := restored.UnlockWithRecovery("correct horse battery staple", "old app password", FixedLease); !errors.Is(err, ErrInvalidRecoveryPassword) {
		t.Fatalf("old recovery password after restore: %v", err)
	}
	if err := restored.UnlockWithRecovery("correct horse battery staple", "new app password", FixedLease); err != nil {
		t.Fatal(err)
	}
	if got, err := restored.Get(id); err != nil || string(got) != "clean-machine-restore-record" {
		t.Fatalf("restored record=%q err=%v", got, err)
	}
}
