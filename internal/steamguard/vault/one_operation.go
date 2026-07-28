package vault

import (
	"errors"
	"sort"
	"sync"
)

// OneOperationAccess provides read-only vault access while a password-derived
// key is in scope. It never exposes that key and expires when the callback
// passed to WithOneOperation returns.
type OneOperationAccess struct {
	mu       sync.Mutex
	vault    *Vault
	vaultKey []byte
	outerKey []byte
	active   bool
}

// WithOneOperation authenticates and runs one bounded read operation without
// creating a cached unlock lease. Callers may use this after Unlock returns
// ErrOneOperationRequired.
func (v *Vault) WithOneOperation(password string, fn func(*OneOperationAccess) error) error {
	return v.withOneOperation(password, nil, fn)
}

// WithOneOperationWithOuter is the double-encrypted counterpart to
// WithOneOperation. The supplied outer key is copied and wiped before return.
func (v *Vault) WithOneOperationWithOuter(password string, outerKey []byte, fn func(*OneOperationAccess) error) error {
	return v.withOneOperation(password, outerKey, fn)
}

func (v *Vault) withOneOperation(password string, outerKey []byte, fn func(*OneOperationAccess) error) error {
	if fn == nil {
		return ErrInvalidOneOperation
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.header.OuterVersion != 0 && len(outerKey) != keyBytes {
		return ErrOuterKeyRequired
	}
	vaultKey, err := v.unwrapVaultKey(password, v.header)
	if err != nil {
		return err
	}
	defer wipe(vaultKey)
	if err := validateOuterProof(outerKey, v.header); err != nil {
		return err
	}
	if _, err := v.loadKeyring(vaultKey, outerKey); err != nil {
		if v.header.OuterVersion != 0 {
			return v.revokeOnIntegrityLocked(errors.Join(ErrInvalidOuterKey, err))
		}
		return v.revokeOnIntegrityLocked(err)
	}
	outerCopy := append([]byte(nil), outerKey...)
	defer wipe(outerCopy)
	access := &OneOperationAccess{
		vault: v, vaultKey: vaultKey, outerKey: outerCopy, active: true,
	}
	defer access.invalidate()
	err = fn(access)
	return v.revokeOnIntegrityLocked(err)
}

func (a *OneOperationAccess) invalidate() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.active = false
	a.vault = nil
	a.vaultKey = nil
	a.outerKey = nil
}

func (a *OneOperationAccess) validateLocked() error {
	if !a.active || a.vault == nil || len(a.vaultKey) != keyBytes {
		return ErrOneOperationExpired
	}
	return nil
}

// ListRecords returns authenticated record metadata inside the current scope.
func (a *OneOperationAccess) ListRecords() ([]RecordInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.validateLocked(); err != nil {
		return nil, err
	}
	ring, err := a.vault.loadKeyring(a.vaultKey, a.outerKey)
	if err != nil {
		return nil, err
	}
	out := make([]RecordInfo, 0, len(ring.Records))
	for _, record := range ring.Records {
		out = append(out, RecordInfo{ID: record.ID, SteamID64: record.SteamID64})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SteamID64 < out[j].SteamID64 })
	return out, nil
}

// GetRecord decrypts one record inside the current scope. The returned
// plaintext belongs to the caller and should be wiped as soon as it is parsed.
func (a *OneOperationAccess) GetRecord(id string) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.validateLocked(); err != nil {
		return nil, err
	}
	return a.vault.getRecordWithKeysLocked(id, a.vaultKey, a.outerKey)
}
