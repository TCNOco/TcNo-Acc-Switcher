package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/passwordpolicy"
	"TcNo-Acc-Switcher/internal/steamguard/securemem"
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Vault is safe for concurrent use. Disk mutations and lease access are
// serialized so a generation always represents one complete state.
type Vault struct {
	mu          sync.Mutex
	root        string
	opts        options
	header      header
	active      string
	lease       securemem.Handle
	outerLease  securemem.Handle
	leaseMode   LeaseMode
	leaseExpiry time.Time
}

func resolveOptions(opts []Option) options {
	o := options{
		clock: systemClock{}, protector: securemem.New(), hardener: defaultHardener(),
		kdf: DefaultKDFParams(), recoveryKDF: DefaultKDFParams(),
	}
	for _, apply := range opts {
		if apply != nil {
			apply(&o)
		}
	}
	return o
}

// Create initializes an empty, locked vault.
func Create(root, password string, opts ...Option) (*Vault, error) {
	o := resolveOptions(opts)
	if err := validateKDF(o.kdf); err != nil {
		return nil, err
	}
	if err := passwordpolicy.ValidateNew(password); err != nil {
		return nil, errors.Join(ErrInvalidPassword, err)
	}
	if _, err := os.Stat(filepath.Join(root, activeName)); err == nil {
		return nil, ErrAlreadyExists
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	if err := o.hardener.HardenDir(root); err != nil {
		return nil, err
	}
	vaultID, err := randomID()
	if err != nil {
		return nil, err
	}
	keyringID, err := randomID()
	if err != nil {
		return nil, err
	}
	vaultKey, err := randomBytes(keyBytes)
	if err != nil {
		return nil, err
	}
	defer wipe(vaultKey)
	h := header{Version: FormatVersion, VaultID: vaultID, KeyringID: keyringID}
	factor, err := newPasswordFactor(o.kdf)
	if err != nil {
		return nil, err
	}
	slot, err := buildSlot(h, "Password", []slotFactor{factor}, PasswordOnly(password), vaultKey)
	if err != nil {
		return nil, err
	}
	h.Slots = []keySlot{slot}
	v := &Vault{root: root, opts: o, header: h}
	ring := keyringPayload{Version: FormatVersion, Records: []recordEntry{}}
	if err := v.commitGenerationLocked(vaultKey, nil, h, ring, nil); err != nil {
		return nil, err
	}
	return v, nil
}

// Open recovers an interrupted transaction and opens the vault locked.
func Open(root string, opts ...Option) (*Vault, error) {
	o := resolveOptions(opts)
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, ErrInvalidFormat
	}
	if err := o.hardener.HardenDir(root); err != nil {
		return nil, err
	}
	if err := recoverTransaction(root); err != nil {
		return nil, err
	}
	active, err := readActive(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	genPath, err := generationPath(root, active)
	if err != nil {
		return nil, err
	}
	// Repair a generation folder that was created without its own rights, which
	// is what a plain copy of the vault produced while protection did not carry
	// down. Its path comes from the active file rather than from listing the
	// vault, so this works on the very folder that cannot be read, and the
	// owner keeps the right to fix its own directory.
	if err := o.hardener.HardenDir(genPath); err != nil {
		return nil, err
	}
	if err := o.hardener.HardenDir(filepath.Join(genPath, recordsName)); err != nil {
		return nil, err
	}
	var h header
	if err := readJSONFile(filepath.Join(genPath, headerName), maxHeader, &h); err != nil {
		return nil, err
	}
	if err := validateHeader(h); err != nil {
		return nil, err
	}
	return &Vault{root: root, opts: o, active: active, header: h}, nil
}

// FolderInfo describes a vault folder that has not been opened.
type FolderInfo struct {
	// HasRecoveryWrapper reports that the outer layer is wrapped with a
	// recovery password, which must be supplied to read the folder.
	HasRecoveryWrapper bool
}

// Inspect reports what a vault folder needs before it can be unlocked. Unlike
// Open it never writes: it skips journal recovery and directory hardening, so
// it is safe to point at a backup the user owns and expects to stay untouched.
func Inspect(root string) (FolderInfo, error) {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return FolderInfo{}, ErrNotFound
		}
		return FolderInfo{}, err
	}
	if !info.IsDir() {
		return FolderInfo{}, ErrInvalidFormat
	}
	active, err := readActive(root)
	if err != nil {
		if os.IsNotExist(err) {
			return FolderInfo{}, ErrNotFound
		}
		return FolderInfo{}, err
	}
	genPath, err := generationPath(root, active)
	if err != nil {
		return FolderInfo{}, err
	}
	var h header
	if err := readJSONFile(filepath.Join(genPath, headerName), maxHeader, &h); err != nil {
		if os.IsNotExist(err) {
			return FolderInfo{}, ErrNotFound
		}
		return FolderInfo{}, err
	}
	if err := validateHeader(h); err != nil {
		return FolderInfo{}, err
	}
	return FolderInfo{HasRecoveryWrapper: h.Recovery != nil}, nil
}

func validateHeader(h header) error {
	if h.Version != FormatVersion || !validID(h.VaultID) || !validID(h.KeyringID) {
		return ErrInvalidFormat
	}
	if h.OuterVersion != 0 && h.OuterVersion != OuterLayerVersion {
		return ErrInvalidFormat
	}
	if h.OuterVersion == 0 && (h.OuterProof.Nonce != "" || h.OuterProof.Ciphertext != "") {
		return ErrInvalidFormat
	}
	if h.OuterVersion == 0 && h.Recovery != nil {
		return ErrInvalidFormat
	}
	if h.OuterVersion != 0 {
		if _, err := decodeBounded(h.OuterProof.Nonce, 64); err != nil {
			return err
		}
		if _, err := decodeBounded(h.OuterProof.Ciphertext, 256); err != nil {
			return err
		}
	}
	if h.Recovery != nil {
		if h.Recovery.Version != RecoveryVersion {
			return ErrInvalidFormat
		}
		if err := validateKDF(h.Recovery.KDF); err != nil {
			return err
		}
		salt, err := decodeBounded(h.Recovery.Salt, saltBytes)
		if err != nil || len(salt) != saltBytes {
			return ErrInvalidFormat
		}
		if _, err := decodeBounded(h.Recovery.OuterKey.Nonce, 64); err != nil {
			return err
		}
		if _, err := decodeBounded(h.Recovery.OuterKey.Ciphertext, 256); err != nil {
			return err
		}
	}
	return validateSlots(h.Slots)
}

// maxSlots bounds how many derivations one header can ask an opener to attempt.
const maxSlots = 32

func validateSlots(slots []keySlot) error {
	if len(slots) == 0 || len(slots) > maxSlots {
		return ErrInvalidFormat
	}
	seen := make(map[string]bool, len(slots))
	for _, slot := range slots {
		if !validID(slot.ID) || seen[slot.ID] || len(slot.Factors) == 0 || len(slot.Factors) > 4 {
			return ErrInvalidFormat
		}
		seen[slot.ID] = true
		if len(slot.Label) > MaxSlotLabelBytes {
			return ErrInvalidFormat
		}
		for _, factor := range slot.Factors {
			salt, err := decodeBounded(factor.Salt, saltBytes)
			if err != nil || len(salt) != saltBytes {
				return ErrInvalidFormat
			}
			switch factor.Type {
			case FactorPassword:
				if factor.KDF == nil {
					return ErrInvalidFormat
				}
				if err := validateKDF(*factor.KDF); err != nil {
					return err
				}
				// A password factor describes no device and no file. Left
				// unchecked these are free-form strings that slotAAD folds in
				// between its NUL separators, where a crafted value can spell
				// out the encoding of a different factor list entirely.
				if factor.KeyfileID != "" || factor.CredentialID != "" || factor.RPID != "" || factor.UV {
					return ErrInvalidFormat
				}
			case FactorKeyfile, FactorRecoveryCode:
				if factor.KDF != nil || !validID(factor.KeyfileID) {
					return ErrInvalidFormat
				}
				if factor.CredentialID != "" || factor.RPID != "" || factor.UV {
					return ErrInvalidFormat
				}
			case FactorSecurityKey:
				if factor.KDF != nil || factor.KeyfileID != "" {
					return ErrInvalidFormat
				}
				// The descriptors have to survive a backup unchanged, so they
				// are bounded rather than free-form.
				if _, err := decodeCredentialID(factor.CredentialID); err != nil {
					return err
				}
				if factor.CredentialID == "" || !validRPID(factor.RPID) {
					return ErrInvalidFormat
				}
			default:
				return ErrInvalidFormat
			}
		}
		if _, err := decodeBounded(slot.VaultKey.Nonce, 64); err != nil {
			return err
		}
		if _, err := decodeBounded(slot.VaultKey.Ciphertext, 256); err != nil {
			return err
		}
	}
	return nil
}

// Unlock starts either a fixed five-minute lease or a process-session lease.
// A secure-memory failure never leaves a plaintext cached key behind.
func (v *Vault) Unlock(password string, mode LeaseMode) error {
	return v.unlock(PasswordOnly(password), nil, mode)
}

// UnlockWith unlocks using whatever enrolled factors the caller holds.
func (v *Vault) UnlockWith(creds Credentials, mode LeaseMode) error {
	return v.unlock(creds, nil, mode)
}

// UnlockWithOuter unlocks a double-encrypted vault with the app-derived key.
func (v *Vault) UnlockWithOuter(password string, outerKey []byte, mode LeaseMode) error {
	return v.unlock(PasswordOnly(password), outerKey, mode)
}

// UnlockWithFactorsAndOuter is the double-encrypted counterpart to UnlockWith.
func (v *Vault) UnlockWithFactorsAndOuter(creds Credentials, outerKey []byte, mode LeaseMode) error {
	return v.unlock(creds, outerKey, mode)
}

func (v *Vault) unlock(creds Credentials, outerKey []byte, mode LeaseMode) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.unlockLocked(creds, outerKey, mode)
}

func (v *Vault) unlockLocked(creds Credentials, outerKey []byte, mode LeaseMode) error {
	if mode != FixedLease && mode != ProcessLease {
		return ErrInvalidFormat
	}
	if v.header.OuterVersion != 0 && len(outerKey) != keyBytes {
		return ErrOuterKeyRequired
	}
	key, err := openVaultKey(v.header, creds)
	if err != nil {
		return err
	}
	defer wipe(key)
	if err := validateOuterProof(outerKey, v.header); err != nil {
		return err
	}
	if _, err := v.loadKeyring(key, outerKey); err != nil {
		if v.header.OuterVersion != 0 {
			return errors.Join(ErrInvalidOuterKey, err)
		}
		return err
	}
	h, err := v.opts.protector.Store(key)
	if err != nil {
		return errors.Join(ErrSecureMemory, ErrOneOperationRequired, err)
	}
	var outerHandle securemem.Handle
	if v.header.OuterVersion != 0 {
		outerHandle, err = v.opts.protector.Store(outerKey)
		if err != nil {
			return errors.Join(ErrSecureMemory, ErrOneOperationRequired, err, h.Destroy())
		}
	}
	if v.outerLease != nil {
		if err := v.outerLease.Destroy(); err != nil {
			return errors.Join(ErrSecureMemory, err, h.Destroy(), destroyHandle(outerHandle))
		}
		v.outerLease = nil
	}
	if v.lease != nil {
		if err := v.lease.Destroy(); err != nil {
			return errors.Join(ErrSecureMemory, err, h.Destroy(), destroyHandle(outerHandle))
		}
		v.lease = nil
	}
	v.lease = h
	v.outerLease = outerHandle
	v.leaseMode = mode
	if mode == FixedLease {
		v.leaseExpiry = v.opts.clock.Now().Add(FixedLeaseLength)
	} else {
		v.leaseExpiry = time.Time{}
	}
	return nil
}

// UnlockWithRecovery restores access using the two passwords contained in a
// copied double-encrypted vault. It never exposes the recovered outer key.
func (v *Vault) UnlockWithRecovery(vaultPassword, recoveryPassword string, mode LeaseMode) error {
	return v.UnlockWithFactorsAndRecovery(PasswordOnly(vaultPassword), recoveryPassword, mode)
}

// UnlockWithFactorsAndRecovery is UnlockWithRecovery for a vault whose slots need
// more than a password.
func (v *Vault) UnlockWithFactorsAndRecovery(creds Credentials, recoveryPassword string, mode LeaseMode) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	outerKey, err := openRecoveryOuterKey(recoveryPassword, v.header)
	if err != nil {
		return err
	}
	defer wipe(outerKey)
	return v.unlockLocked(creds, outerKey, mode)
}

func destroyHandle(handle securemem.Handle) error {
	if handle == nil {
		return nil
	}
	return handle.Destroy()
}

// VerifyCredentials reports whether these factors open some enrolled way in,
// without changing the lock state. An already-unlocked vault will do as it is
// told regardless of what the caller typed, so anything that asks the user to
// prove themselves before changing who can open the vault has to check the
// answer here rather than relying on the unlock path to do it.
func (v *Vault) VerifyCredentials(creds Credentials) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	key, err := openVaultKey(v.header, creds)
	if err != nil {
		return err
	}
	wipe(key)
	return nil
}

func (v *Vault) withKeyLocked(fn func([]byte) error) error {
	return v.withKeysLocked(func(key, _ []byte) error { return fn(key) })
}

func (v *Vault) withKeysLocked(fn func([]byte, []byte) error) error {
	if v.lease == nil {
		return ErrLocked
	}
	if v.leaseMode == FixedLease && !v.opts.clock.Now().Before(v.leaseExpiry) {
		_ = v.destroyLeasesLocked()
		return ErrLeaseExpired
	}
	if v.header.OuterVersion != 0 && v.outerLease == nil {
		return ErrOuterKeyRequired
	}
	var operationErr error
	err := v.lease.With(func(key []byte) error {
		if v.outerLease == nil {
			operationErr = fn(key, nil)
			return nil
		}
		return v.outerLease.With(func(outerKey []byte) error {
			operationErr = fn(key, outerKey)
			return nil
		})
	})
	if err != nil {
		_ = v.destroyLeasesLocked()
		return errors.Join(ErrSecureMemory, ErrOneOperationRequired, err)
	}
	return v.revokeOnIntegrityLocked(operationErr)
}

func isIntegrityFailure(err error) bool {
	return errors.Is(err, ErrInvalidFormat) || errors.Is(err, ErrInvalidOuterKey)
}

func (v *Vault) revokeOnIntegrityLocked(err error) error {
	if !isIntegrityFailure(err) {
		return err
	}
	return errors.Join(err, v.destroyLeasesLocked())
}

// Lock destroys the cached vault key immediately.
func (v *Vault) Lock() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.destroyLeasesLocked()
}

// SetLeaseMode changes the lifetime of an existing protected key lease.
func (v *Vault) SetLeaseMode(mode LeaseMode) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if mode != FixedLease && mode != ProcessLease {
		return ErrInvalidFormat
	}
	if v.lease == nil {
		return ErrLocked
	}
	if v.leaseMode == FixedLease && !v.opts.clock.Now().Before(v.leaseExpiry) {
		return errors.Join(ErrLeaseExpired, v.destroyLeasesLocked())
	}
	v.leaseMode = mode
	if mode == FixedLease {
		v.leaseExpiry = v.opts.clock.Now().Add(FixedLeaseLength)
	} else {
		v.leaseExpiry = time.Time{}
	}
	return nil
}

func (v *Vault) destroyLeasesLocked() error {
	err := errors.Join(destroyHandle(v.lease), destroyHandle(v.outerLease))
	v.lease = nil
	v.outerLease = nil
	v.leaseExpiry = time.Time{}
	return err
}

func (v *Vault) IsLocked() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.lease == nil {
		return true
	}
	if v.leaseMode == FixedLease && !v.opts.clock.Now().Before(v.leaseExpiry) {
		_ = v.destroyLeasesLocked()
		return true
	}
	return false
}

// Generation returns the authenticated snapshot identifier currently opened by
// this process. It is non-secret and can bind short-lived UI capabilities.
func (v *Vault) Generation() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.active
}

func (v *Vault) loadKeyring(key, outerKey []byte) (keyringPayload, error) {
	genPath, err := generationPath(v.root, v.active)
	if err != nil {
		return keyringPayload{}, err
	}
	raw, err := os.ReadFile(filepath.Join(genPath, keyringName))
	if err != nil || len(raw) > maxKeyring {
		if err != nil {
			return keyringPayload{}, errors.Join(ErrInvalidFormat, err)
		}
		return keyringPayload{}, ErrInvalidFormat
	}
	raw, err = openOuterFile(outerKey, raw, v.header.OuterVersion,
		outerAAD(v.header.VaultID, v.active, keyringName))
	if err != nil {
		return keyringPayload{}, err
	}
	defer wipe(raw)
	var env envelope
	if err := unmarshalStrict(raw, &env); err != nil {
		return keyringPayload{}, err
	}
	plain, err := openEnvelope(key, env, aad(v.header.Version, v.header.VaultID, v.header.KeyringID, "", "keyring"))
	if err != nil {
		return keyringPayload{}, errors.Join(ErrInvalidFormat, err)
	}
	defer wipe(plain)
	var ring keyringPayload
	dec := json.NewDecoder(bytes.NewReader(plain))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ring); err != nil || ring.Version != FormatVersion {
		return keyringPayload{}, ErrInvalidFormat
	}
	seen := make(map[string]bool, len(ring.Records))
	for _, entry := range ring.Records {
		if !validID(entry.ID) || !validID(entry.Filename) || entry.SteamID64 == "" || seen[entry.ID] {
			return keyringPayload{}, ErrInvalidFormat
		}
		seen[entry.ID] = true
	}
	return ring, nil
}

// PutRecord inserts or replaces the account identified by SteamID64.
func (v *Vault) PutRecord(steamID64 string, plaintext []byte) (string, error) {
	steamID64 = strings.TrimSpace(steamID64)
	if steamID64 == "" || len(plaintext) > maxPlainBytes {
		return "", ErrInvalidFormat
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	var result string
	err := v.withKeysLocked(func(vaultKey, outerKey []byte) error {
		ring, err := v.loadKeyring(vaultKey, outerKey)
		if err != nil {
			return err
		}
		staged := map[string][]byte{}
		recordID, err := v.stageRecordLocked(vaultKey, &ring, staged, steamID64, plaintext)
		if err != nil {
			return err
		}
		files, err := v.collectRecordFiles(ring, staged, outerKey)
		if err != nil {
			return err
		}
		if err := v.commitGenerationLocked(vaultKey, outerKey, v.header, ring, files); err != nil {
			return err
		}
		result = recordID
		return nil
	})
	return result, err
}

// RecordUpdate is one account's new plaintext in a batched write.
type RecordUpdate struct {
	SteamID64 string
	Plaintext []byte
}

// PutRecords replaces several accounts in a single generation.
//
// Every write here rotates the generation, and a rotation invalidates every
// outstanding capability. Doing that once per account turns a routine batch -
// refreshing expired access tokens for a whole vault - into one rotation per
// row, which is why the background sweeps are otherwise forbidden from writing
// at all. One commit for the whole batch is what makes such a batch affordable.
//
// It is all-or-nothing: the new generation is written, verified and only then
// made active, so a failure part-way leaves every record on the old one.
func (v *Vault) PutRecords(updates []RecordUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(updates))
	for _, update := range updates {
		steamID64 := strings.TrimSpace(update.SteamID64)
		// A duplicate would stage two record files for one account and leave the
		// keyring pointing at whichever won, orphaning the other.
		if steamID64 == "" || len(update.Plaintext) > maxPlainBytes || seen[steamID64] {
			return ErrInvalidFormat
		}
		seen[steamID64] = true
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	return v.withKeysLocked(func(vaultKey, outerKey []byte) error {
		ring, err := v.loadKeyring(vaultKey, outerKey)
		if err != nil {
			return err
		}
		staged := make(map[string][]byte, len(updates))
		for _, update := range updates {
			if _, err := v.stageRecordLocked(vaultKey, &ring, staged, strings.TrimSpace(update.SteamID64), update.Plaintext); err != nil {
				return err
			}
		}
		files, err := v.collectRecordFiles(ring, staged, outerKey)
		if err != nil {
			return err
		}
		return v.commitGenerationLocked(vaultKey, outerKey, v.header, ring, files)
	})
}

// stageRecordLocked seals one record and folds it into the pending keyring and
// file set without committing. Callers hold v.mu and are inside withKeysLocked.
//
// Each call mints a fresh filename and data key even when replacing an existing
// account, so a rewritten record never lands on the path its previous
// ciphertext occupied.
func (v *Vault) stageRecordLocked(vaultKey []byte, ring *keyringPayload, staged map[string][]byte, steamID64 string, plaintext []byte) (string, error) {
	recordID := ""
	for _, entry := range ring.Records {
		if entry.SteamID64 == steamID64 {
			recordID = entry.ID
			break
		}
	}
	var err error
	if recordID == "" {
		if recordID, err = randomID(); err != nil {
			return "", err
		}
	}
	filename, err := randomID()
	if err != nil {
		return "", err
	}
	dataKey, err := randomBytes(keyBytes)
	if err != nil {
		return "", err
	}
	defer wipe(dataKey)
	wrapped, err := seal(vaultKey, dataKey, aad(FormatVersion, v.header.VaultID, recordID, steamID64, "data-key"))
	if err != nil {
		return "", err
	}
	ciphertext, err := seal(dataKey, plaintext, aad(FormatVersion, v.header.VaultID, recordID, steamID64, "record"))
	if err != nil {
		return "", err
	}
	raw, err := marshalJSON(recordFile{Version: FormatVersion, RecordID: recordID, Ciphertext: ciphertext})
	if err != nil {
		return "", err
	}

	entry := recordEntry{ID: recordID, SteamID64: steamID64, Filename: filename, WrappedKey: wrapped}
	replaced := false
	for i := range ring.Records {
		if ring.Records[i].ID == recordID {
			ring.Records[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		ring.Records = append(ring.Records, entry)
	}
	staged[filename] = raw
	return recordID, nil
}

// Put is a shorthand for PutRecord.
func (v *Vault) Put(steamID64 string, plaintext []byte) (string, error) {
	return v.PutRecord(steamID64, plaintext)
}

func (v *Vault) GetRecord(id string) ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	var result []byte
	err := v.withKeysLocked(func(vaultKey, outerKey []byte) error {
		var err error
		result, err = v.getRecordWithKeysLocked(id, vaultKey, outerKey)
		return err
	})
	return result, err
}

func (v *Vault) getRecordWithKeysLocked(id string, vaultKey, outerKey []byte) ([]byte, error) {
	ring, err := v.loadKeyring(vaultKey, outerKey)
	if err != nil {
		return nil, err
	}
	for _, entry := range ring.Records {
		if entry.ID != id {
			continue
		}
		genPath, _ := generationPath(v.root, v.active)
		path, err := recordPath(genPath, entry.Filename)
		if err != nil {
			return nil, err
		}
		var rf recordFile
		raw, err := v.readInnerRecordFile(path, entry.Filename, outerKey)
		if err != nil {
			return nil, errors.Join(ErrInvalidFormat, err)
		}
		defer wipe(raw)
		if err := unmarshalStrict(raw, &rf); err != nil {
			return nil, err
		}
		if rf.Version != FormatVersion || rf.RecordID != entry.ID {
			return nil, ErrInvalidFormat
		}
		dataKey, err := openEnvelope(vaultKey, entry.WrappedKey, aad(FormatVersion, v.header.VaultID, entry.ID, entry.SteamID64, "data-key"))
		if err != nil {
			return nil, errors.Join(ErrInvalidFormat, err)
		}
		defer wipe(dataKey)
		if len(dataKey) != keyBytes {
			return nil, ErrInvalidFormat
		}
		result, err := openEnvelope(dataKey, rf.Ciphertext, aad(FormatVersion, v.header.VaultID, entry.ID, entry.SteamID64, "record"))
		if err != nil {
			return nil, errors.Join(ErrInvalidFormat, err)
		}
		return result, nil
	}
	return nil, ErrNotFound
}

func (v *Vault) Get(id string) ([]byte, error) { return v.GetRecord(id) }

func (v *Vault) ListRecords() ([]RecordInfo, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	var out []RecordInfo
	err := v.withKeysLocked(func(key, outerKey []byte) error {
		ring, err := v.loadKeyring(key, outerKey)
		if err != nil {
			return err
		}
		out = make([]RecordInfo, 0, len(ring.Records))
		for _, r := range ring.Records {
			out = append(out, RecordInfo{ID: r.ID, SteamID64: r.SteamID64})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].SteamID64 < out[j].SteamID64 })
		return nil
	})
	return out, err
}

func (v *Vault) List() ([]RecordInfo, error) { return v.ListRecords() }

func (v *Vault) DeleteRecord(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.withKeysLocked(func(key, outerKey []byte) error {
		ring, err := v.loadKeyring(key, outerKey)
		if err != nil {
			return err
		}
		next := ring.Records[:0]
		found := false
		for _, entry := range ring.Records {
			if entry.ID == id {
				found = true
				continue
			}
			next = append(next, entry)
		}
		if !found {
			return ErrNotFound
		}
		ring.Records = next
		files, err := v.collectRecordFiles(ring, nil, outerKey)
		if err != nil {
			return err
		}
		return v.commitGenerationLocked(key, outerKey, v.header, ring, files)
	})
}

func (v *Vault) Delete(id string) error { return v.DeleteRecord(id) }

// EnableOuter transactionally adds the app-password-derived AES-GCM layer to
// the authenticated keyring and every record file.
func (v *Vault) EnableOuter(outerKey []byte) error {
	return v.migrateOuter(OuterLayerVersion, outerKey, nil)
}

// EnableOuterWithRecovery adds the outer layer and its independently derived
// recovery wrapper in one verified generation.
func (v *Vault) EnableOuterWithRecovery(outerKey []byte, recoveryPassword string) error {
	return v.migrateOuter(OuterLayerVersion, outerKey, &recoveryPassword)
}

// DisableOuter transactionally removes the app-password-derived layer. Inner
// Steam Guard encryption remains unchanged.
func (v *Vault) DisableOuter(outerKey []byte) error {
	return v.migrateOuter(0, outerKey, nil)
}

// DisableOuterWithRecovery removes the outer layer from a copied vault when
// the destination installation has no app password. The recovered key is never
// returned to the caller.
func (v *Vault) DisableOuterWithRecovery(recoveryPassword string) error {
	v.mu.Lock()
	outerKey, err := openRecoveryOuterKey(recoveryPassword, v.header)
	v.mu.Unlock()
	if err != nil {
		return err
	}
	defer wipe(outerKey)
	return v.migrateOuter(0, outerKey, nil)
}

func (v *Vault) migrateOuter(target int, outerKey []byte, recoveryPassword *string) error {
	if target != 0 && target != OuterLayerVersion {
		return ErrInvalidFormat
	}
	if len(outerKey) != keyBytes {
		return ErrOuterKeyRequired
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.header.OuterVersion == target && recoveryPassword == nil {
		if target == OuterLayerVersion {
			if err := validateOuterProof(outerKey, v.header); err != nil {
				return err
			}
			if v.lease != nil {
				return v.replaceOuterLeaseLocked(outerKey)
			}
		}
		return nil
	}
	if v.header.OuterVersion != 0 {
		if err := validateOuterProof(outerKey, v.header); err != nil {
			return err
		}
	}
	keyringRaw, files, err := v.collectOuterMigrationFilesLocked(outerKey)
	if err != nil {
		return err
	}
	previousHeader := v.header
	nextHeader := v.header
	nextHeader.OuterVersion = target
	nextHeader.OuterProof = envelope{}
	nextHeader.Recovery = nil
	targetKey := outerKey
	if target == 0 {
		targetKey = nil
	} else {
		nextHeader.OuterProof, err = sealOuterProof(outerKey, nextHeader)
		if err != nil {
			return err
		}
		if recoveryPassword != nil {
			nextHeader.Recovery, err = createRecoveryWrapper(*recoveryPassword, outerKey, nextHeader, v.opts.recoveryKDF)
			if err != nil {
				return err
			}
		} else {
			nextHeader.Recovery = v.header.Recovery
		}
	}
	if err := v.commitOuterMigrationLocked(targetKey, nextHeader, keyringRaw, files); err != nil {
		rollbackErr := v.rollbackHeaderMigrationLocked(outerKey, previousHeader, keyringRaw, files)
		return errors.Join(err, rollbackErr)
	}
	if target == OuterLayerVersion {
		if v.lease != nil {
			if err := v.replaceOuterLeaseLocked(outerKey); err != nil {
				rollbackErr := v.rollbackHeaderMigrationLocked(outerKey, previousHeader, keyringRaw, files)
				return errors.Join(err, rollbackErr)
			}
		}
		return nil
	}
	return v.clearOuterLeaseLocked()
}

// HasRecoveryWrapper reports whether a copied vault can recover its outer key
// without the original app security file.
func (v *Vault) HasRecoveryWrapper() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.header.Recovery != nil
}

// ConfigureRecovery adds or replaces the recovery wrapper after authenticating
// the current outer key. The account ciphertext is not decrypted.
func (v *Vault) ConfigureRecovery(outerKey []byte, recoveryPassword string) error {
	return v.migrateOuter(OuterLayerVersion, outerKey, &recoveryPassword)
}

// ChangeRecoveryPassword rewraps the outer key after validating the old
// recovery password. It does not rewrite account plaintext.
func (v *Vault) ChangeRecoveryPassword(oldPassword, newPassword string) error {
	if strings.TrimSpace(newPassword) == "" {
		return ErrInvalidPassword
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	outerKey, err := openRecoveryOuterKey(oldPassword, v.header)
	if err != nil {
		return err
	}
	defer wipe(outerKey)
	return v.configureRecoveryLocked(outerKey, newPassword)
}

func (v *Vault) configureRecoveryLocked(outerKey []byte, password string) error {
	if v.header.OuterVersion != OuterLayerVersion {
		return ErrOuterKeyRequired
	}
	if err := validateOuterProof(outerKey, v.header); err != nil {
		return err
	}
	keyringRaw, files, err := v.collectOuterMigrationFilesLocked(outerKey)
	if err != nil {
		return err
	}
	previousHeader := v.header
	nextHeader := v.header
	nextHeader.Recovery, err = createRecoveryWrapper(password, outerKey, nextHeader, v.opts.recoveryKDF)
	if err != nil {
		return err
	}
	if err := v.commitOuterMigrationLocked(outerKey, nextHeader, keyringRaw, files); err != nil {
		rollbackErr := v.rollbackHeaderMigrationLocked(outerKey, previousHeader, keyringRaw, files)
		return errors.Join(err, rollbackErr)
	}
	return nil
}

// VerifyRecovery authenticates both password wrappers and every encrypted
// record in the active generation without creating an unlock lease.
func (v *Vault) VerifyRecovery(vaultPassword, recoveryPassword string) error {
	return v.VerifyRecoveryWith(PasswordOnly(vaultPassword), recoveryPassword)
}

// VerifyRecoveryWith is VerifyRecovery for a vault whose slots need more than a
// password.
func (v *Vault) VerifyRecoveryWith(creds Credentials, recoveryPassword string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	vaultKey, err := openVaultKey(v.header, creds)
	if err != nil {
		return err
	}
	defer wipe(vaultKey)
	outerKey, err := openRecoveryOuterKey(recoveryPassword, v.header)
	if err != nil {
		return err
	}
	defer wipe(outerKey)
	genPath, err := generationPath(v.root, v.active)
	if err != nil {
		return err
	}
	return verifyGeneration(genPath, v.active, v.header, vaultKey, outerKey)
}

// RestoreOuterFromRecovery rotates a copied vault from its recovered outer key
// to the current installation's key, then writes a recovery wrapper for the
// new app password. No key is returned to the caller.
func (v *Vault) RestoreOuterFromRecovery(oldRecoveryPassword string, newOuterKey []byte, newRecoveryPassword string) error {
	if len(newOuterKey) != keyBytes {
		return ErrOuterKeyRequired
	}
	if strings.TrimSpace(newRecoveryPassword) == "" {
		return ErrInvalidPassword
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	oldOuterKey, err := openRecoveryOuterKey(oldRecoveryPassword, v.header)
	if err != nil {
		return err
	}
	defer wipe(oldOuterKey)
	keyringRaw, files, err := v.collectOuterMigrationFilesLocked(oldOuterKey)
	if err != nil {
		return err
	}
	previousHeader := v.header
	nextHeader := v.header
	nextHeader.OuterProof, err = sealOuterProof(newOuterKey, nextHeader)
	if err != nil {
		return err
	}
	nextHeader.Recovery, err = createRecoveryWrapper(newRecoveryPassword, newOuterKey, nextHeader, v.opts.recoveryKDF)
	if err != nil {
		return err
	}
	if err := v.commitOuterMigrationLocked(newOuterKey, nextHeader, keyringRaw, files); err != nil {
		rollbackErr := v.rollbackHeaderMigrationLocked(oldOuterKey, previousHeader, keyringRaw, files)
		return errors.Join(err, rollbackErr)
	}
	if v.lease != nil {
		if err := v.replaceOuterLeaseLocked(newOuterKey); err != nil {
			rollbackErr := v.rollbackHeaderMigrationLocked(oldOuterKey, previousHeader, keyringRaw, files)
			return errors.Join(err, rollbackErr)
		}
	}
	return nil
}

func (v *Vault) collectOuterMigrationFilesLocked(outerKey []byte) ([]byte, map[string][]byte, error) {
	genPath, err := generationPath(v.root, v.active)
	if err != nil {
		return nil, nil, err
	}
	keyringPath := filepath.Join(genPath, keyringName)
	keyringInfo, err := os.Lstat(keyringPath)
	if err != nil || !keyringInfo.Mode().IsRegular() {
		return nil, nil, ErrInvalidFormat
	}
	keyringRaw, err := os.ReadFile(keyringPath)
	if err != nil || len(keyringRaw) > maxKeyring {
		return nil, nil, ErrInvalidFormat
	}
	keyringRaw, err = openOuterFile(outerKey, keyringRaw, v.header.OuterVersion,
		outerAAD(v.header.VaultID, v.active, keyringName))
	if err != nil {
		return nil, nil, err
	}
	recordDir := filepath.Join(genPath, recordsName)
	entries, err := os.ReadDir(recordDir)
	if err != nil {
		wipe(keyringRaw)
		return nil, nil, err
	}
	files := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".bin")
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() || entry.Name() != name+".bin" || !validID(name) {
			wipe(keyringRaw)
			return nil, nil, ErrInvalidFormat
		}
		raw, err := os.ReadFile(filepath.Join(recordDir, entry.Name()))
		if err != nil || len(raw) > maxRecord {
			wipe(keyringRaw)
			return nil, nil, ErrInvalidFormat
		}
		inner, err := openOuterFile(outerKey, raw, v.header.OuterVersion,
			outerAAD(v.header.VaultID, v.active, name))
		if err != nil {
			wipe(keyringRaw)
			return nil, nil, err
		}
		files[name] = inner
	}
	return keyringRaw, files, nil
}

func (v *Vault) commitOuterMigrationLocked(outerKey []byte, nextHeader header,
	keyringRaw []byte, files map[string][]byte) error {
	nextID, err := randomID()
	if err != nil {
		return err
	}
	genPath, _ := generationPath(v.root, nextID)
	recordDir := filepath.Join(genPath, recordsName)
	if err := os.MkdirAll(recordDir, 0o700); err != nil {
		return err
	}
	if err := v.opts.hardener.HardenDir(genPath); err != nil {
		return err
	}
	if err := v.opts.hardener.HardenDir(recordDir); err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(genPath)
		}
	}()
	headerRaw, err := marshalJSON(nextHeader)
	if err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(genPath, headerName), headerRaw, 0o600, v.opts.hardener); err != nil {
		return err
	}
	sealedKeyring, err := sealOuterFile(outerKey, keyringRaw, nextHeader.OuterVersion,
		outerAAD(nextHeader.VaultID, nextID, keyringName))
	if err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(genPath, keyringName), sealedKeyring, 0o600, v.opts.hardener); err != nil {
		return err
	}
	for name, raw := range files {
		path, err := recordPath(genPath, name)
		if err != nil {
			return err
		}
		sealed, err := sealOuterFile(outerKey, raw, nextHeader.OuterVersion,
			outerAAD(nextHeader.VaultID, nextID, name))
		if err != nil {
			return err
		}
		if err := atomicWrite(path, sealed, 0o600, v.opts.hardener); err != nil {
			return err
		}
	}
	syncDir(recordDir)
	syncDir(genPath)
	if err := verifyOuterMigration(genPath, nextID, nextHeader, outerKey, keyringRaw, files); err != nil {
		return err
	}
	tx := transaction{Version: FormatVersion, Previous: v.active, Next: nextID}
	txRaw, _ := marshalJSON(tx)
	if err := atomicWrite(filepath.Join(v.root, journalName), txRaw, 0o600, v.opts.hardener); err != nil {
		return err
	}
	cleanup = false
	if v.opts.txnHook != nil {
		if err := v.opts.txnHook("after-journal"); err != nil {
			return err
		}
	}
	if err := atomicWrite(filepath.Join(v.root, activeName), []byte(nextID), 0o600, v.opts.hardener); err != nil {
		return err
	}
	v.active = nextID
	v.header = nextHeader
	if v.opts.txnHook != nil {
		if err := v.opts.txnHook("after-switch"); err != nil {
			return err
		}
	}
	if err := os.Remove(filepath.Join(v.root, journalName)); err != nil {
		return err
	}
	if tx.Previous != "" {
		if old, err := generationPath(v.root, tx.Previous); err == nil {
			_ = os.RemoveAll(old)
		}
	}
	syncDir(v.root)
	return nil
}

func verifyOuterMigration(genPath, generationID string, h header, outerKey, keyringRaw []byte,
	files map[string][]byte) error {
	var diskHeader header
	if err := readJSONFile(filepath.Join(genPath, headerName), maxHeader, &diskHeader); err != nil {
		return err
	}
	if !headersEqual(diskHeader, h) {
		return ErrInvalidFormat
	}
	if err := validateHeader(diskHeader); err != nil {
		return err
	}
	if err := validateOuterProof(outerKey, h); err != nil {
		return err
	}
	disk, err := os.ReadFile(filepath.Join(genPath, keyringName))
	if err != nil {
		return err
	}
	inner, err := openOuterFile(outerKey, disk, h.OuterVersion, outerAAD(h.VaultID, generationID, keyringName))
	if err != nil || !bytes.Equal(inner, keyringRaw) {
		return ErrInvalidFormat
	}
	wipe(inner)
	for name, want := range files {
		path, _ := recordPath(genPath, name)
		disk, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		inner, err := openOuterFile(outerKey, disk, h.OuterVersion, outerAAD(h.VaultID, generationID, name))
		if err != nil || !bytes.Equal(inner, want) {
			return ErrInvalidFormat
		}
		wipe(inner)
	}
	return nil
}

func (v *Vault) replaceOuterLeaseLocked(key []byte) error {
	handle, err := v.opts.protector.Store(key)
	if err != nil {
		return errors.Join(ErrSecureMemory, err)
	}
	if err := destroyHandle(v.outerLease); err != nil {
		return errors.Join(ErrSecureMemory, err, handle.Destroy())
	}
	v.outerLease = handle
	return nil
}

func (v *Vault) clearOuterLeaseLocked() error {
	err := destroyHandle(v.outerLease)
	v.outerLease = nil
	return err
}

func (v *Vault) rollbackHeaderMigrationLocked(outerKey []byte, previousHeader header,
	keyringRaw []byte, files map[string][]byte) error {
	if err := recoverTransaction(v.root); err != nil {
		return err
	}
	if headersEqual(v.header, previousHeader) {
		return nil
	}
	rollbackKey := outerKey
	if previousHeader.OuterVersion == 0 {
		rollbackKey = nil
	}
	hook := v.opts.txnHook
	v.opts.txnHook = nil
	err := v.commitOuterMigrationLocked(rollbackKey, previousHeader, keyringRaw, files)
	v.opts.txnHook = hook
	return err
}

func headersEqual(left, right header) bool {
	leftRaw, leftErr := json.Marshal(left)
	rightRaw, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftRaw, rightRaw)
}

// ChangePassword validates the old password and creates a generation whose
// vault-key wrapper is derived from the new password. Record keys are not
// re-encrypted.
func (v *Vault) ChangePassword(oldPassword, newPassword string) error {
	return v.ChangePasswordWith(PasswordOnly(oldPassword), newPassword)
}

// ChangePasswordWith changes the password on a vault whose slots need more than
// one factor. The other factors are carried into the rebuilt slot unchanged, so
// a password-and-keyfile slot still needs the same keyfile afterwards.
func (v *Vault) ChangePasswordWith(oldCreds Credentials, newPassword string) error {
	if err := passwordpolicy.ValidateNew(newPassword); err != nil {
		return errors.Join(ErrInvalidPassword, err)
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	newCreds := oldCreds
	newCreds.Password = newPassword
	vaultKey, err := openVaultKey(v.header, oldCreds)
	if err != nil {
		return err
	}
	defer wipe(vaultKey)
	var ring keyringPayload
	var outerKey []byte
	if v.header.OuterVersion != 0 {
		if v.outerLease == nil {
			return ErrOuterKeyRequired
		}
		if err := v.outerLease.With(func(key []byte) error {
			outerKey = append([]byte(nil), key...)
			return nil
		}); err != nil {
			return errors.Join(ErrSecureMemory, ErrOneOperationRequired, err, v.destroyLeasesLocked())
		}
		defer wipe(outerKey)
	}
	ring, err = v.loadKeyring(vaultKey, outerKey)
	if err != nil {
		return v.revokeOnIntegrityLocked(err)
	}
	nextHeader := v.header
	outcome, err := reissueSlots(v.header, oldCreds, newCreds, v.opts.kdf, vaultKey)
	if err != nil {
		return err
	}
	nextHeader.Slots = outcome.slots
	if outcome.passwordSlots == 0 {
		return ErrNoPasswordEnrolled
	}
	// Nothing is written if some way in would keep answering to the old
	// password. Committing here would report a password change that did not
	// happen, and the user would believe the old one was retired when anyone
	// holding that keyfile or key could still use it.
	if len(outcome.stillOnOldPassword) != 0 {
		return fmt.Errorf("%w: %s", ErrPasswordStillInUse, strings.Join(outcome.stillOnOldPassword, ", "))
	}
	// The vault key can be recovered by a factor that is not the password at
	// all — a backup key, a security key — and that opens every path below
	// without anything having checked the password being retired. Rebuilding
	// no password slot means none of them opened, so the old one was never
	// proven and is about to survive a change reported as successful.
	if outcome.passwordReissued == 0 {
		return ErrInvalidPassword
	}
	files, err := v.collectRecordFiles(ring, nil, outerKey)
	if err != nil {
		return v.revokeOnIntegrityLocked(err)
	}
	if err := v.commitGenerationLocked(vaultKey, outerKey, nextHeader, ring, files); err != nil {
		return err
	}
	if err := v.destroyLeasesLocked(); err != nil {
		return errors.Join(ErrSecureMemory, err)
	}
	return nil
}

// Rekey re-wraps the vault key under new KDF parameters, keeping the same
// password. The vault key itself, the per-record data keys and every record
// ciphertext are unchanged: only the header envelope is rebuilt. A vault whose
// outer layer is enabled must be unlocked, so that the new generation can be
// re-sealed; use RekeyWithRecovery to rekey a copy that is still locked.
func (v *Vault) Rekey(password string, params KDFParams) error {
	return v.RekeyWith(PasswordOnly(password), params)
}

// RekeyWith rekeys a vault whose slots need more than a password. Every factor
// the slot lists is re-derived under the new parameters, so a password-and-keyfile
// slot still needs the same keyfile afterwards.
func (v *Vault) RekeyWith(creds Credentials, params KDFParams) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	var outerKey []byte
	if v.header.OuterVersion != 0 {
		if v.outerLease == nil {
			return ErrOuterKeyRequired
		}
		if err := v.outerLease.With(func(key []byte) error {
			outerKey = append([]byte(nil), key...)
			return nil
		}); err != nil {
			return errors.Join(ErrSecureMemory, ErrOneOperationRequired, err, v.destroyLeasesLocked())
		}
		defer wipe(outerKey)
	}
	return v.rekeyLocked(creds, outerKey, params)
}

// RekeyWithRecovery rekeys a double-encrypted vault that has not been
// unlocked, recovering the outer key from the recovery wrapper. The recovered
// key is never returned to the caller.
func (v *Vault) RekeyWithRecovery(vaultPassword, recoveryPassword string, params KDFParams) error {
	return v.RekeyWithRecoveryAndFactors(PasswordOnly(vaultPassword), recoveryPassword, params)
}

// RekeyWithRecoveryAndFactors is RekeyWithRecovery for a vault whose slots need
// more than a password.
func (v *Vault) RekeyWithRecoveryAndFactors(creds Credentials, recoveryPassword string, params KDFParams) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	outerKey, err := openRecoveryOuterKey(recoveryPassword, v.header)
	if err != nil {
		return err
	}
	defer wipe(outerKey)
	return v.rekeyLocked(creds, outerKey, params)
}

func (v *Vault) rekeyLocked(creds Credentials, outerKey []byte, params KDFParams) error {
	if err := validateKDF(params); err != nil {
		return err
	}
	vaultKey, err := openVaultKey(v.header, creds)
	if err != nil {
		return err
	}
	defer wipe(vaultKey)
	ring, err := v.loadKeyring(vaultKey, outerKey)
	if err != nil {
		return v.revokeOnIntegrityLocked(err)
	}
	nextHeader := v.header
	// Skipped slots are not a problem here: rekeying only changes derivation
	// cost, so a slot left alone keeps working with the same factors.
	outcome, err := reissueSlots(v.header, creds, creds, params, vaultKey)
	if err != nil {
		return err
	}
	nextHeader.Slots = outcome.slots
	files, err := v.collectRecordFiles(ring, nil, outerKey)
	if err != nil {
		return v.revokeOnIntegrityLocked(err)
	}
	return v.commitGenerationLocked(vaultKey, outerKey, nextHeader, ring, files)
}

func (v *Vault) collectRecordFiles(ring keyringPayload, replacements map[string][]byte, outerKey []byte) (map[string][]byte, error) {
	files := make(map[string][]byte, len(ring.Records))
	genPath, _ := generationPath(v.root, v.active)
	for _, entry := range ring.Records {
		if raw, ok := replacements[entry.Filename]; ok {
			files[entry.Filename] = raw
			continue
		}
		path, err := recordPath(genPath, entry.Filename)
		if err != nil {
			return nil, err
		}
		raw, err := v.readInnerRecordFile(path, entry.Filename, outerKey)
		if err != nil {
			return nil, errors.Join(ErrInvalidFormat, err)
		}
		if len(raw) > maxRecord {
			return nil, ErrInvalidFormat
		}
		files[entry.Filename] = raw
	}
	return files, nil
}

func (v *Vault) readInnerRecordFile(path, filename string, outerKey []byte) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) > maxRecord {
		return nil, ErrInvalidFormat
	}
	inner, err := openOuterFile(outerKey, raw, v.header.OuterVersion,
		outerAAD(v.header.VaultID, v.active, filename))
	if err != nil {
		return nil, err
	}
	return inner, nil
}

func (v *Vault) commitGenerationLocked(key, outerKey []byte, nextHeader header, ring keyringPayload, files map[string][]byte) error {
	nextID, err := randomID()
	if err != nil {
		return err
	}
	genPath, _ := generationPath(v.root, nextID)
	recordDir := filepath.Join(genPath, recordsName)
	if err := os.MkdirAll(recordDir, 0o700); err != nil {
		return err
	}
	if err := v.opts.hardener.HardenDir(genPath); err != nil {
		return err
	}
	if err := v.opts.hardener.HardenDir(recordDir); err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(genPath)
		}
	}()
	headerRaw, err := marshalJSON(nextHeader)
	if err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(genPath, headerName), headerRaw, 0o600, v.opts.hardener); err != nil {
		return err
	}
	ringRaw, err := json.Marshal(ring)
	if err != nil {
		return err
	}
	// The keyring plaintext names every account in the vault. Every path that
	// decrypts one wipes its copy; the path that writes one has to as well.
	defer wipe(ringRaw)
	ringEnv, err := seal(key, ringRaw, aad(FormatVersion, nextHeader.VaultID, nextHeader.KeyringID, "", "keyring"))
	if err != nil {
		return err
	}
	ringFile, err := marshalJSON(ringEnv)
	if err != nil {
		return err
	}
	ringFile, err = sealOuterFile(outerKey, ringFile, nextHeader.OuterVersion,
		outerAAD(nextHeader.VaultID, nextID, keyringName))
	if err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(genPath, keyringName), ringFile, 0o600, v.opts.hardener); err != nil {
		return err
	}
	for name, raw := range files {
		path, err := recordPath(genPath, name)
		if err != nil {
			return err
		}
		raw, err = sealOuterFile(outerKey, raw, nextHeader.OuterVersion,
			outerAAD(nextHeader.VaultID, nextID, name))
		if err != nil {
			return err
		}
		if err := atomicWrite(path, raw, 0o600, v.opts.hardener); err != nil {
			return err
		}
	}
	syncDir(recordDir)
	syncDir(genPath)
	if err := verifyGeneration(genPath, nextID, nextHeader, key, outerKey); err != nil {
		return err
	}
	tx := transaction{Version: FormatVersion, Previous: v.active, Next: nextID}
	txRaw, _ := marshalJSON(tx)
	if err := atomicWrite(filepath.Join(v.root, journalName), txRaw, 0o600, v.opts.hardener); err != nil {
		return err
	}
	cleanup = false
	if v.opts.txnHook != nil {
		if err := v.opts.txnHook("after-journal"); err != nil {
			return err
		}
	}
	if err := atomicWrite(filepath.Join(v.root, activeName), []byte(nextID), 0o600, v.opts.hardener); err != nil {
		return err
	}
	v.active = nextID
	v.header = nextHeader
	if v.opts.txnHook != nil {
		if err := v.opts.txnHook("after-switch"); err != nil {
			return err
		}
	}
	if err := os.Remove(filepath.Join(v.root, journalName)); err != nil {
		return err
	}
	if tx.Previous != "" {
		if old, err := generationPath(v.root, tx.Previous); err == nil {
			_ = os.RemoveAll(old)
		}
	}
	syncDir(v.root)
	return nil
}

func verifyGeneration(genPath, generationID string, h header, key, outerKey []byte) error {
	var diskHeader header
	if err := readJSONFile(filepath.Join(genPath, headerName), maxHeader, &diskHeader); err != nil {
		return err
	}
	if err := validateHeader(diskHeader); err != nil {
		return err
	}
	if err := validateOuterProof(outerKey, diskHeader); err != nil {
		return err
	}
	if diskHeader.VaultID != h.VaultID || diskHeader.KeyringID != h.KeyringID {
		return ErrInvalidFormat
	}
	raw, err := os.ReadFile(filepath.Join(genPath, keyringName))
	if err != nil || len(raw) > maxKeyring {
		return ErrInvalidFormat
	}
	raw, err = openOuterFile(outerKey, raw, h.OuterVersion, outerAAD(h.VaultID, generationID, keyringName))
	if err != nil {
		return err
	}
	defer wipe(raw)
	var env envelope
	if err := unmarshalStrict(raw, &env); err != nil {
		return err
	}
	plain, err := openEnvelope(key, env, aad(FormatVersion, h.VaultID, h.KeyringID, "", "keyring"))
	if err != nil {
		return err
	}
	defer wipe(plain)
	var ring keyringPayload
	if err := json.Unmarshal(plain, &ring); err != nil || ring.Version != FormatVersion {
		return ErrInvalidFormat
	}
	for _, entry := range ring.Records {
		path, err := recordPath(genPath, entry.Filename)
		if err != nil {
			return err
		}
		recordRaw, err := os.ReadFile(path)
		if err != nil || len(recordRaw) > maxRecord {
			return ErrInvalidFormat
		}
		recordRaw, err = openOuterFile(outerKey, recordRaw, h.OuterVersion,
			outerAAD(h.VaultID, generationID, entry.Filename))
		if err != nil {
			return err
		}
		var rf recordFile
		if err := unmarshalStrict(recordRaw, &rf); err != nil {
			return err
		}
		if rf.Version != FormatVersion || rf.RecordID != entry.ID {
			return ErrInvalidFormat
		}
		dataKey, err := openEnvelope(key, entry.WrappedKey, aad(FormatVersion, h.VaultID, entry.ID, entry.SteamID64, "data-key"))
		if err != nil {
			return err
		}
		if len(dataKey) != keyBytes {
			wipe(dataKey)
			return ErrInvalidFormat
		}
		plainRecord, err := openEnvelope(dataKey, rf.Ciphertext, aad(FormatVersion, h.VaultID, entry.ID, entry.SteamID64, "record"))
		wipe(dataKey)
		wipe(plainRecord)
		if err != nil {
			return err
		}
	}
	return nil
}
