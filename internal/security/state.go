package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/passwordpolicy"
	"TcNo-Acc-Switcher/internal/paths"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/hkdf"
)

const (
	securityVersion = 1
	vaultKeyBytes   = 32

	securityDirName  = "Security"
	securityFileName = "security.json"

	securityVerifierAAD = "tcno-security-verifier-v1"
	wrappedKeyAAD       = "tcno-security-vault-key-v1"
	steamGuardOuterInfo = "tcno-acc-switcher/steam-guard/outer-key/v1"

	kdfTargetMillis = 300
	kdfMaxTime      = 8

	// Read bounds for KDF parameters loaded from disk. argon2.IDKey allocates
	// the whole memory cost as one block and aborts the process if that fails,
	// so an out-of-range value cannot be caught later — it has to be rejected
	// before derivation. Wide enough to keep every parameter set this app has
	// ever written openable.
	minKDFMemoryKB uint32 = 8 * 1024
	maxKDFMemoryKB uint32 = 1024 * 1024
	maxKDFTime     uint32 = 16
	maxKDFThreads  uint8  = 16
)

var (
	ErrLocked             = errors.New("app is locked")
	ErrPasswordNotSet     = errors.New("app password is not set")
	ErrInvalidPassword    = errors.New("invalid app password")
	ErrPasswordAlreadySet = errors.New("app password is already set")
	ErrEmptyPassword      = errors.New("app password cannot be empty")
)

type Status struct {
	AppPasswordSet            bool `json:"appPasswordSet"`
	AppLocked                 bool `json:"appLocked"`
	SavedAccountDataEncrypted bool `json:"savedAccountDataEncrypted"`
	OperationBusy             bool `json:"operationBusy"`
	QuarantineCount           int  `json:"quarantineCount"`
	InterruptedRestorePending bool `json:"interruptedRestorePending"`
}

type KDFParams struct {
	Algorithm      string `json:"algorithm"`
	Time           uint32 `json:"time"`
	MemoryKB       uint32 `json:"memoryKb"`
	Threads        uint8  `json:"threads"`
	KeyLen         uint32 `json:"keyLen"`
	TargetMillis   uint32 `json:"targetMillis,omitempty"`
	MeasuredMillis uint32 `json:"measuredMillis,omitempty"`
}

type securityFile struct {
	Version                   int       `json:"version"`
	KDF                       KDFParams `json:"kdf"`
	Salt                      string    `json:"salt"`
	VerifierNonce             string    `json:"verifierNonce"`
	VerifierCiphertext        string    `json:"verifierCiphertext"`
	WrappedVaultKeyNonce      string    `json:"wrappedVaultKeyNonce"`
	WrappedVaultKeyCiphertext string    `json:"wrappedVaultKeyCiphertext"`
	SavedAccountDataEncrypted bool      `json:"savedAccountDataEncrypted"`
}

type manager struct {
	mu               sync.Mutex
	lifecycleMu      sync.Mutex
	masterKey        []byte
	operationBusy    bool
	saveSecurityFile func(securityFile) error
	removeFile       func(string) error
}

// SteamGuardLifecycleHook coordinates the app password with Steam Guard's
// independent outer encryption layer. Implementations must not retain key.
type SteamGuardLifecycleHook interface {
	EnableOuter(key []byte, recoveryPassword string) error
	DisableOuter(key []byte) error
	ChangeOuterPassword(oldPassword, newPassword string) error
	RevokeLeases() error
}

var (
	defaultManager   = &manager{}
	statusHookMu     sync.Mutex
	statusHook       func()
	steamGuardHookMu sync.RWMutex
	steamGuardHook   SteamGuardLifecycleHook
)

// SetSteamGuardLifecycleHook replaces the optional Steam Guard coordinator.
// Passing nil removes it.
func SetSteamGuardLifecycleHook(hook SteamGuardLifecycleHook) {
	steamGuardHookMu.Lock()
	steamGuardHook = hook
	steamGuardHookMu.Unlock()
}

func currentSteamGuardHook() SteamGuardLifecycleHook {
	steamGuardHookMu.RLock()
	hook := steamGuardHook
	steamGuardHookMu.RUnlock()
	return hook
}

func SetStatusChangedHook(fn func()) {
	statusHookMu.Lock()
	statusHook = fn
	statusHookMu.Unlock()
}

func emitStatusChanged() {
	statusHookMu.Lock()
	fn := statusHook
	statusHookMu.Unlock()
	if fn != nil {
		fn()
	}
}

func defaultKDFParams() KDFParams {
	// Calibration tunes Time to hit TargetMillis, so raising memory buys
	// hardness against parallel attackers without costing the user more time.
	return KDFParams{
		Algorithm:    "argon2id",
		Time:         2,
		MemoryKB:     256 * 1024,
		Threads:      1,
		KeyLen:       vaultKeyBytes,
		TargetMillis: kdfTargetMillis,
	}
}

func GetStatus() (Status, error) {
	return defaultManager.status()
}

func SetAppPassword(password string) error {
	return defaultManager.setAppPassword(password)
}

func UnlockApp(password string) error {
	return defaultManager.unlockApp(password)
}

func ChangePassword(oldPassword, newPassword string) error {
	return defaultManager.changePassword(oldPassword, newPassword)
}

func Lock() error {
	return defaultManager.lock()
}

func RemoveAppPassword(password string) error {
	return defaultManager.removeAppPassword(password)
}

func RequireUnlocked() error {
	return defaultManager.requireUnlocked()
}

func AppLocked() bool {
	_, locked, _, err := defaultManager.lockState()
	return err == nil && locked
}

func SavedAccountDataEncrypted() bool {
	_, _, encrypted, err := defaultManager.lockState()
	return err == nil && encrypted
}

// DeriveSteamGuardOuterKey derives a purpose-specific key from the unlocked
// random app master key. The caller owns the returned buffer and must wipe it.
func DeriveSteamGuardOuterKey() ([]byte, error) {
	master, err := defaultManager.unlockedMasterKey()
	if err != nil {
		return nil, err
	}
	defer wipeBytes(master)
	return deriveSteamGuardOuterKey(master)
}

// VerifyAppPassword authenticates a policy-eligible app password without
// changing lock state. Steam Guard uses this before creating an outer layer.
func VerifyAppPassword(password string) error {
	sf, ok, err := loadSecurityFile()
	if err != nil {
		return err
	}
	if !ok {
		return ErrPasswordNotSet
	}
	// ValidateExisting, not ValidateNew: this password was set under whatever
	// policy applied at the time, and rejecting it here would lock a legacy
	// user out of every Steam Guard operation while the app still unlocks.
	if err := passwordpolicy.ValidateExisting(password); err != nil {
		return err
	}
	key, err := unlockSecurityFileWithPassword(sf, password)
	if err != nil {
		return err
	}
	wipeBytes(key)
	return nil
}

func deriveSteamGuardOuterKey(master []byte) ([]byte, error) {
	if len(master) != vaultKeyBytes {
		return nil, ErrLocked
	}
	key := make([]byte, vaultKeyBytes)
	reader := hkdf.New(sha256.New, master, nil, []byte(steamGuardOuterInfo))
	if _, err := io.ReadFull(reader, key); err != nil {
		wipeBytes(key)
		return nil, err
	}
	return key, nil
}

// WipeSecret clears a caller-owned derived secret buffer.
func WipeSecret(secret []byte) { wipeBytes(secret) }

func securityDir() (string, error) {
	root, err := paths.DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, securityDirName), nil
}

func securityPath() (string, error) {
	dir, err := securityDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, securityFileName), nil
}

func (m *manager) status() (Status, error) {
	sf, ok, err := loadSecurityFile()
	if err != nil {
		return Status{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	st := Status{
		AppPasswordSet:            ok,
		AppLocked:                 ok && len(m.masterKey) == 0,
		OperationBusy:             m.operationBusy,
		QuarantineCount:           countQuarantines(),
		InterruptedRestorePending: hasInterruptedRestoreJournal(),
	}
	if ok {
		st.SavedAccountDataEncrypted = sf.SavedAccountDataEncrypted
	}
	return st, nil
}

func (m *manager) setAppPassword(password string) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	if _, ok, err := loadSecurityFile(); err != nil {
		return err
	} else if ok {
		return ErrPasswordAlreadySet
	}
	if password == "" {
		return ErrEmptyPassword
	}
	if err := passwordpolicy.ValidateNew(password); err != nil {
		return err
	}
	salt, err := randomBytes(16)
	if err != nil {
		return err
	}
	kdf, derived := calibrateAndDeriveKey(password, salt)
	defer wipeBytes(derived)
	master, err := randomBytes(vaultKeyBytes)
	if err != nil {
		return err
	}
	defer wipeBytes(master)
	verifierNonce, verifierCipher, err := sealWithKey(derived, []byte("tcno-security-ok"), []byte(securityVerifierAAD))
	if err != nil {
		return err
	}
	wrapNonce, wrapped, err := sealWithKey(derived, master, []byte(wrappedKeyAAD))
	if err != nil {
		return err
	}
	sf := securityFile{
		Version:                   securityVersion,
		KDF:                       kdf,
		Salt:                      encode(salt),
		VerifierNonce:             encode(verifierNonce),
		VerifierCiphertext:        encode(verifierCipher),
		WrappedVaultKeyNonce:      encode(wrapNonce),
		WrappedVaultKeyCiphertext: encode(wrapped),
	}
	if err := m.save(sf); err != nil {
		return err
	}
	m.replaceMasterKey(append([]byte(nil), master...))
	emitStatusChanged()
	return nil
}

func (m *manager) unlockApp(password string) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	sf, ok, err := loadSecurityFile()
	if err != nil {
		return err
	}
	if !ok {
		return ErrPasswordNotSet
	}
	key, err := unlockSecurityFileWithPassword(sf, password)
	if err != nil {
		return err
	}
	if sf.SavedAccountDataEncrypted {
		outerKey, err := deriveSteamGuardOuterKey(key)
		if err != nil {
			wipeBytes(key)
			return err
		}
		defer wipeBytes(outerKey)
		if hook := currentSteamGuardHook(); hook != nil {
			if err := hook.EnableOuter(outerKey, password); err != nil {
				wipeBytes(key)
				return err
			}
		}
	}
	m.replaceMasterKey(key)
	emitStatusChanged()
	return nil
}

func (m *manager) changePassword(oldPassword, newPassword string) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	if newPassword == "" {
		return ErrEmptyPassword
	}
	if err := passwordpolicy.ValidateNew(newPassword); err != nil {
		return err
	}
	sf, ok, err := loadSecurityFile()
	if err != nil {
		return err
	}
	if !ok {
		return ErrPasswordNotSet
	}
	master, err := unlockSecurityFileWithPassword(sf, oldPassword)
	if err != nil {
		return err
	}
	defer wipeBytes(master)

	salt, err := randomBytes(16)
	if err != nil {
		return err
	}
	kdf, derived := calibrateAndDeriveKey(newPassword, salt)
	defer wipeBytes(derived)
	verifierNonce, verifierCipher, err := sealWithKey(derived, []byte("tcno-security-ok"), []byte(securityVerifierAAD))
	if err != nil {
		return err
	}
	wrapNonce, wrapped, err := sealWithKey(derived, master, []byte(wrappedKeyAAD))
	if err != nil {
		return err
	}
	sf.KDF = kdf
	sf.Salt = encode(salt)
	sf.VerifierNonce = encode(verifierNonce)
	sf.VerifierCiphertext = encode(verifierCipher)
	sf.WrappedVaultKeyNonce = encode(wrapNonce)
	sf.WrappedVaultKeyCiphertext = encode(wrapped)
	hook := currentSteamGuardHook()
	if hook != nil {
		if err := hook.RevokeLeases(); err != nil {
			return err
		}
		if sf.SavedAccountDataEncrypted {
			if err := hook.ChangeOuterPassword(oldPassword, newPassword); err != nil {
				return err
			}
		}
	}
	if err := m.save(sf); err != nil {
		if hook != nil && sf.SavedAccountDataEncrypted {
			return errors.Join(err, hook.ChangeOuterPassword(newPassword, oldPassword))
		}
		return err
	}
	m.replaceMasterKey(append([]byte(nil), master...))
	emitStatusChanged()
	return nil
}

func (m *manager) lock() error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	if _, ok, err := loadSecurityFile(); err != nil {
		return err
	} else if !ok {
		return ErrPasswordNotSet
	}
	m.replaceMasterKey(nil)
	var revokeErr error
	if hook := currentSteamGuardHook(); hook != nil {
		revokeErr = hook.RevokeLeases()
	}
	emitStatusChanged()
	return revokeErr
}

func (m *manager) removeAppPassword(password string) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	sf, ok, err := loadSecurityFile()
	if err != nil {
		return err
	}
	if !ok {
		return ErrPasswordNotSet
	}
	key, err := unlockWithPassword(password)
	if err != nil {
		return err
	}
	defer wipeBytes(key)
	journal, err := writeJournal("remove-password", map[string]any{
		"encrypted": sf.SavedAccountDataEncrypted,
	})
	if err != nil {
		return err
	}
	encryptionDisabled := false
	rollback := func(cause error) error {
		if encryptionDisabled {
			cause = errors.Join(cause, EnableSavedAccountEncryption(password))
		}
		_ = os.Remove(journal)
		return cause
	}
	hook := currentSteamGuardHook()
	if hook != nil {
		if err := hook.RevokeLeases(); err != nil {
			return rollback(err)
		}
	}
	m.replaceMasterKey(append([]byte(nil), key...))
	if sf.SavedAccountDataEncrypted {
		if err := disableSavedAccountEncryptionWithKey(password, key, true); err != nil {
			return rollback(err)
		}
		encryptionDisabled = true
	}
	p, err := securityPath()
	if err != nil {
		return rollback(err)
	}
	if err := m.remove(p); err != nil && !os.IsNotExist(err) {
		return rollback(err)
	}
	_ = os.Remove(journal)
	m.replaceMasterKey(nil)
	emitStatusChanged()
	return nil
}

func (m *manager) save(sf securityFile) error {
	if m.saveSecurityFile != nil {
		return m.saveSecurityFile(sf)
	}
	return saveSecurityFile(sf)
}

func (m *manager) remove(path string) error {
	if m.removeFile != nil {
		return m.removeFile(path)
	}
	return os.Remove(path)
}

// lockState answers the lock and encryption predicates without the quarantine
// and restore-journal directory scans status() runs for its UI-only fields.
func (m *manager) lockState() (passwordSet, locked, encrypted bool, err error) {
	sf, ok, err := loadSecurityFile()
	if err != nil {
		return false, false, false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return ok, ok && len(m.masterKey) == 0, ok && sf.SavedAccountDataEncrypted, nil
}

func (m *manager) requireUnlocked() error {
	_, locked, _, err := m.lockState()
	if err != nil {
		return err
	}
	if locked {
		return ErrLocked
	}
	return nil
}

func (m *manager) unlockedMasterKey() ([]byte, error) {
	if err := m.requireUnlocked(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.masterKey) == 0 {
		return nil, ErrLocked
	}
	return append([]byte(nil), m.masterKey...), nil
}

func (m *manager) setBusy(v bool) {
	m.mu.Lock()
	m.operationBusy = v
	m.mu.Unlock()
	emitStatusChanged()
}

func unlockWithPassword(password string) ([]byte, error) {
	sf, ok, err := loadSecurityFile()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrPasswordNotSet
	}
	return unlockSecurityFileWithPassword(sf, password)
}

func unlockSecurityFileWithPassword(sf securityFile, password string) ([]byte, error) {
	salt, err := decode(sf.Salt)
	if err != nil {
		return nil, err
	}
	derived := deriveKey(password, salt, sf.KDF)
	defer wipeBytes(derived)
	verifierNonce, err := decode(sf.VerifierNonce)
	if err != nil {
		return nil, err
	}
	verifierCipher, err := decode(sf.VerifierCiphertext)
	if err != nil {
		return nil, err
	}
	verifier, err := openWithKey(derived, verifierNonce, verifierCipher, []byte(securityVerifierAAD))
	if err != nil {
		return nil, ErrInvalidPassword
	}
	wipeBytes(verifier)
	wrapNonce, err := decode(sf.WrappedVaultKeyNonce)
	if err != nil {
		return nil, err
	}
	wrapped, err := decode(sf.WrappedVaultKeyCiphertext)
	if err != nil {
		return nil, err
	}
	master, err := openWithKey(derived, wrapNonce, wrapped, []byte(wrappedKeyAAD))
	if err != nil {
		return nil, ErrInvalidPassword
	}
	if len(master) != vaultKeyBytes {
		wipeBytes(master)
		return nil, fmt.Errorf("invalid vault key length")
	}
	return master, nil
}

func (m *manager) replaceMasterKey(key []byte) {
	m.mu.Lock()
	wipeBytes(m.masterKey)
	m.masterKey = key
	m.mu.Unlock()
}

// Go cannot guarantee erasure of compiler or runtime copies beyond this reachable buffer.
func wipeBytes(buf []byte) {
	for i := range buf {
		buf[i] = 0
	}
	runtime.KeepAlive(buf)
}

func deriveKey(password string, salt []byte, p KDFParams) []byte {
	p = normalizeKDFParams(p)
	return argon2.IDKey([]byte(password), salt, p.Time, p.MemoryKB, p.Threads, p.KeyLen)
}

// validateKDFParams bounds parameters that came from a file rather than from
// this process. Callers must run it before any deriveKey on loaded params.
func validateKDFParams(p KDFParams) error {
	p = normalizeKDFParams(p)
	if p.Algorithm != "argon2id" {
		return fmt.Errorf("unsupported KDF algorithm %q", p.Algorithm)
	}
	if p.MemoryKB < minKDFMemoryKB || p.MemoryKB > maxKDFMemoryKB {
		return fmt.Errorf("KDF memory %d KiB out of range", p.MemoryKB)
	}
	if p.Time < 1 || p.Time > maxKDFTime {
		return fmt.Errorf("KDF time %d out of range", p.Time)
	}
	if p.Threads < 1 || p.Threads > maxKDFThreads {
		return fmt.Errorf("KDF threads %d out of range", p.Threads)
	}
	if p.KeyLen != vaultKeyBytes {
		return fmt.Errorf("KDF key length %d out of range", p.KeyLen)
	}
	return nil
}

func normalizeKDFParams(p KDFParams) KDFParams {
	def := defaultKDFParams()
	if p.Algorithm == "" {
		p.Algorithm = def.Algorithm
	}
	if p.Time == 0 {
		p.Time = def.Time
	}
	if p.MemoryKB == 0 {
		p.MemoryKB = def.MemoryKB
	}
	if p.Threads == 0 {
		p.Threads = def.Threads
	}
	if p.KeyLen == 0 {
		p.KeyLen = def.KeyLen
	}
	if p.TargetMillis == 0 {
		p.TargetMillis = def.TargetMillis
	}
	return p
}

func calibrateAndDeriveKey(password string, salt []byte) (KDFParams, []byte) {
	p := normalizeKDFParams(defaultKDFParams())
	p.Time = 1
	start := time.Now()
	key := deriveKey(password, salt, p)
	singleMillis := elapsedMillis(start)
	target := p.TargetMillis
	if target == 0 {
		target = kdfTargetMillis
	}
	nextTime := uint32(1)
	if singleMillis > 0 {
		nextTime = (target + singleMillis - 1) / singleMillis
	}
	if nextTime < 1 {
		nextTime = 1
	}
	if nextTime > kdfMaxTime {
		nextTime = kdfMaxTime
	}
	p.Time = nextTime
	if nextTime == 1 {
		p.MeasuredMillis = singleMillis
		return p, key
	}
	wipeBytes(key)
	start = time.Now()
	key = deriveKey(password, salt, p)
	p.MeasuredMillis = elapsedMillis(start)
	return p, key
}

func elapsedMillis(start time.Time) uint32 {
	ms := time.Since(start).Milliseconds()
	if ms < 1 {
		return 1
	}
	if ms > int64(^uint32(0)) {
		return ^uint32(0)
	}
	return uint32(ms)
}

func loadSecurityFile() (securityFile, bool, error) {
	p, err := securityPath()
	if err != nil {
		return securityFile{}, false, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return securityFile{}, false, nil
		}
		return securityFile{}, false, err
	}
	var sf securityFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return securityFile{}, false, err
	}
	if sf.Version != securityVersion {
		return securityFile{}, false, fmt.Errorf("unsupported security file version %d", sf.Version)
	}
	if err := validateKDFParams(sf.KDF); err != nil {
		return securityFile{}, false, err
	}
	return sf, true, nil
}

func saveSecurityFile(sf securityFile) error {
	p, err := securityPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomicDurable(p, append(data, '\n'), 0o600)
}

func updateSecurityFile(fn func(*securityFile) error) error {
	sf, ok, err := loadSecurityFile()
	if err != nil {
		return err
	}
	if !ok {
		return ErrPasswordNotSet
	}
	if err := fn(&sf); err != nil {
		return err
	}
	return saveSecurityFile(sf)
}

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

func sealWithKey(key, plaintext, aad []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce, err := randomBytes(aead.NonceSize())
	if err != nil {
		return nil, nil, err
	}
	return nonce, aead.Seal(nil, nonce, plaintext, aad), nil
}

func openWithKey(key, nonce, ciphertext, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ciphertext, aad)
}

func encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
