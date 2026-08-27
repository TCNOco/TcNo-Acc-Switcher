package enrollmentflow

import (
	"context"
	"crypto/subtle"
	"errors"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/enrollmentapi"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
	"TcNo-Acc-Switcher/internal/steamguard/vaultrecord"
)

type Manager struct {
	client  APIClient
	vault   recordVault
	timeout time.Duration
	locksMu sync.Mutex
	locks   map[uint64]*sync.Mutex
}

// New creates an enrollment flow whose persisted state is stored only as a
// record inside the already-unlocked encrypted Steam Guard vault.
func New(client APIClient, encryptedVault *vault.Vault) *Manager {
	return newManager(client, encryptedVault, DefaultRequestTimeout)
}

func newManager(client APIClient, encryptedVault recordVault, timeout time.Duration) *Manager {
	return &Manager{client: client, vault: encryptedVault, timeout: timeout, locks: make(map[uint64]*sync.Mutex)}
}

func (m *Manager) accountLock(steamID uint64) *sync.Mutex {
	m.locksMu.Lock()
	defer m.locksMu.Unlock()
	lock := m.locks[steamID]
	if lock == nil {
		lock = &sync.Mutex{}
		m.locks[steamID] = lock
	}
	return lock
}

func (m *Manager) valid() bool {
	return m != nil && m.client != nil && m.vault != nil && m.timeout > 0 && m.timeout <= DefaultRequestTimeout
}

func (m *Manager) Start(ctx context.Context, request StartRequest) (Status, error) {
	if !m.valid() || ctx == nil || !validSteamID(request.SteamID) || !validToken(request.AccessToken) || request.AuthenticatorTime == 0 {
		return Status{}, ErrInvalidRequest
	}
	lock := m.accountLock(request.SteamID)
	lock.Lock()
	defer lock.Unlock()

	slot, err := m.existingLocked(request.SteamID)
	if err != nil {
		return Status{}, err
	}
	if slot.resumable {
		status := slot.status
		status.Resumed = true
		return status, nil
	}
	// Enrolling replaces whatever holds the slot, and for an authenticator that is
	// an unrecoverable loss. A session-only record is the one shape worth
	// replacing - it holds no secrets - and only when the caller asked for it.
	if slot.occupied && !(request.ReplaceLoginOnly && slot.kind == vaultrecord.KindLoginOnly) {
		return Status{}, ErrAlreadyEnrolled
	}

	token := append([]byte(nil), request.AccessToken...)
	defer wipe(token)
	result, err := m.client.AddAuthenticator(ctx, enrollmentapi.AddRequest{
		SteamID: request.SteamID, AccessToken: token, AuthenticatorTime: request.AuthenticatorTime,
	}, m.timeout)
	if err != nil {
		return Status{}, err
	}
	if result.Pending == nil {
		if !validTerminalAddState(result.State) {
			return Status{}, ErrInvalidPendingState
		}
		return statusFromAPI(result.State, retrySeconds(int64(result.RetryAfter/time.Second)), result.HasRetryAfter), nil
	}
	defer result.Pending.Destroy()
	if !validPendingState(result.State) || result.Pending.SteamID != request.SteamID {
		return Status{}, ErrInvalidPendingState
	}
	record, err := pendingFromAPI(result.Pending, result.State)
	if err != nil {
		return Status{}, err
	}
	defer record.destroy()
	// Carried from the login, not from the API response: Steam does not repeat the
	// refresh token here, and an authenticator persisted without one can only ever
	// be recovered with a password.
	record.RefreshToken = append([]byte(nil), request.RefreshToken...)
	record.PhoneHint = cleanPhoneHint(record.PhoneHint)
	record.RetryAfterSeconds = retrySeconds(int64(result.RetryAfter / time.Second))
	record.HasRetryAfter = result.HasRetryAfter
	raw, err := encodePending(&record)
	if err != nil {
		return Status{}, err
	}
	defer wipe(raw)
	if _, err := m.vault.PutRecord(steamIDString(request.SteamID), raw); err != nil {
		return Status{}, err
	}
	return recordStatus(&record, false), nil
}

func (m *Manager) Resume(steamID uint64) (Status, error) {
	if !m.valid() || !validSteamID(steamID) {
		return Status{}, ErrInvalidRequest
	}
	lock := m.accountLock(steamID)
	lock.Lock()
	defer lock.Unlock()
	slot, err := m.existingLocked(steamID)
	if err != nil {
		return Status{}, err
	}
	if !slot.resumable {
		return Status{}, ErrNoPendingEnrollment
	}
	status := slot.status
	status.Resumed = true
	return status, nil
}

// slotState is what the account's single record slot currently holds.
type slotState struct {
	// status is meaningful only when resumable: an enrollment to carry on with.
	status    Status
	resumable bool
	// occupied is a record in the slot, with kind naming its shape. Enrolling
	// over one replaces it.
	occupied bool
	kind     vaultrecord.Kind
}

func (m *Manager) existingLocked(steamID uint64) (slotState, error) {
	_, raw, found, err := m.loadRecord(steamID)
	if err != nil || !found {
		return slotState{occupied: found, kind: vaultrecord.KindUnknown}, err
	}
	defer wipe(raw)
	kind := vaultrecord.Sniff(raw)
	record, err := decodePending(raw)
	if err != nil {
		return slotState{occupied: true, kind: kind}, nil
	}
	defer record.destroy()
	if record.SteamID != steamID {
		return slotState{occupied: true, kind: kind}, ErrInvalidPendingState
	}
	return slotState{status: recordStatus(&record, true), resumable: true, occupied: true, kind: kind}, nil
}

// RevealRevocationCode returns the recovery code while it remains
// unacknowledged. The service layer limits each live UI capability to one view;
// keeping this read-only permits recovery after a crash before acknowledgment.
func (m *Manager) RevealRevocationCode(steamID uint64) (RevocationView, error) {
	if !m.valid() || !validSteamID(steamID) {
		return RevocationView{}, ErrInvalidRequest
	}
	lock := m.accountLock(steamID)
	lock.Lock()
	defer lock.Unlock()
	_, raw, found, err := m.loadRecord(steamID)
	if err != nil {
		return RevocationView{}, err
	}
	if !found {
		return RevocationView{}, ErrNoPendingEnrollment
	}
	defer wipe(raw)
	record, err := decodePending(raw)
	if err != nil || record.SteamID != steamID {
		return RevocationView{}, ErrNoPendingEnrollment
	}
	defer record.destroy()
	if record.RevocationAcknowledged {
		return RevocationView{}, ErrRevocationCodeAlreadyAcknowledged
	}
	return RevocationView{Code: string(record.RevocationCode)}, nil
}

// AcknowledgeRevocationCode atomically persists proof that the user typed the
// exact recovery code back. The code itself was already encrypted in pending
// state and is never returned from this method.
func (m *Manager) AcknowledgeRevocationCode(steamID uint64, code []byte) (Status, error) {
	if !m.valid() || !validSteamID(steamID) || !validRevocationCode(code) {
		return Status{}, ErrInvalidRequest
	}
	lock := m.accountLock(steamID)
	lock.Lock()
	defer lock.Unlock()
	_, raw, found, err := m.loadRecord(steamID)
	if err != nil {
		return Status{}, err
	}
	if !found {
		return Status{}, ErrNoPendingEnrollment
	}
	defer wipe(raw)
	record, err := decodePending(raw)
	if err != nil || record.SteamID != steamID {
		return Status{}, ErrNoPendingEnrollment
	}
	defer record.destroy()
	if record.RevocationAcknowledged {
		return Status{}, ErrRevocationCodeAlreadyAcknowledged
	}
	if subtle.ConstantTimeCompare(record.RevocationCode, code) != 1 {
		return Status{}, ErrInvalidRequest
	}
	record.RevocationAcknowledged = true
	next, err := encodePending(&record)
	if err != nil {
		return Status{}, err
	}
	defer wipe(next)
	if _, err := m.vault.PutRecord(steamIDString(steamID), next); err != nil {
		return Status{}, err
	}
	return recordStatus(&record, false), nil
}

func (m *Manager) Finalize(ctx context.Context, request FinalizeRequest) (Status, error) {
	if !m.valid() || ctx == nil || !validSteamID(request.SteamID) || len(request.ConfirmationCode) == 0 || request.AuthenticatorTime == 0 {
		return Status{}, ErrInvalidRequest
	}
	lock := m.accountLock(request.SteamID)
	lock.Lock()
	defer lock.Unlock()
	_, raw, found, err := m.loadRecord(request.SteamID)
	if err != nil {
		return Status{}, err
	}
	if !found {
		return Status{}, ErrNoPendingEnrollment
	}
	defer wipe(raw)
	record, err := decodePending(raw)
	if err != nil || record.SteamID != request.SteamID {
		return Status{}, ErrNoPendingEnrollment
	}
	defer record.destroy()
	if !record.RevocationAcknowledged {
		return Status{}, ErrInvalidRequest
	}
	pending := record.toAPI()
	defer pending.Destroy()
	requestID := append([]byte(nil), record.RequestID...)
	defer wipe(requestID)
	confirmationCode := append([]byte(nil), request.ConfirmationCode...)
	defer wipe(confirmationCode)
	result, err := m.client.FinalizeAddAuthenticator(ctx, enrollmentapi.FinalizeRequest{
		Pending: pending, RequestID: requestID, ConfirmationCode: confirmationCode,
		AuthenticatorTime: request.AuthenticatorTime,
	}, m.timeout)
	if err != nil {
		// Adding an authenticator activates it on Steam; finalize only confirms
		// the user saw the code. A refusal here can therefore arrive after the
		// authenticator is already live - a confirmation code already spent, or
		// a result this build cannot map - and the secrets for it are already on
		// disk. Abandoning them would leave the account generating codes nobody
		// holds, so check with Steam before giving up.
		if committed, verifyErr := m.commitIfSteamAlreadyActivated(ctx, &record); committed {
			return Status{State: enrollmentapi.StateComplete}, nil
		} else if verifyErr != nil {
			return Status{}, errors.Join(err, verifyErr)
		}
		return Status{}, err
	}
	if !validFinalizeState(result.State) {
		return Status{}, ErrInvalidPendingState
	}
	if result.ServerTime != 0 {
		if result.ServerTime < 1230768000 || result.ServerTime > 4102444800 {
			return Status{}, ErrInvalidPendingState
		}
		record.ServerTime = result.ServerTime
	}
	if result.State == enrollmentapi.StateComplete {
		active, err := activeMaFile(&record)
		if err != nil {
			return Status{}, err
		}
		defer wipe(active)
		if _, err := m.vault.PutRecord(steamIDString(request.SteamID), active); err != nil {
			return Status{}, err
		}
		return Status{State: enrollmentapi.StateComplete}, nil
	}
	record.State = result.State
	record.RetryAfterSeconds = retrySeconds(int64(result.RetryAfter / time.Second))
	record.HasRetryAfter = result.HasRetryAfter
	next, err := encodePending(&record)
	if err != nil {
		return Status{}, err
	}
	defer wipe(next)
	if _, err := m.vault.PutRecord(steamIDString(request.SteamID), next); err != nil {
		return Status{}, err
	}
	return recordStatus(&record, false), nil
}

// commitIfSteamAlreadyActivated stores the pending secrets when Steam reports
// that the authenticator they belong to is the one active on the account.
//
// The token GID recorded when the authenticator was added is compared with the
// GID Steam reports now. Equal means these exact secrets generate this
// account's codes, which is a stronger statement than any finalize result: the
// enrollment is finished whatever the finalize call said. Unequal, absent, or
// unreachable means nothing is written, because committing secrets that are not
// the live authenticator would leave the user holding codes Steam rejects while
// the app claimed success.
func (m *Manager) commitIfSteamAlreadyActivated(ctx context.Context, record *pendingRecord) (bool, error) {
	if record == nil || record.TokenGID == "" || len(record.AccessToken) == 0 {
		return false, nil
	}
	status, err := m.client.QueryStatus(ctx, record.SteamID, record.AccessToken, m.timeout)
	if err != nil {
		return false, err
	}
	if status.TokenGID == "" || status.TokenGID != record.TokenGID {
		return false, nil
	}
	active, err := activeMaFile(record)
	if err != nil {
		return false, err
	}
	defer wipe(active)
	if _, err := m.vault.PutRecord(steamIDString(record.SteamID), active); err != nil {
		return false, err
	}
	return true, nil
}

// Cancel removes only a validated pending record. It never deletes an active
// maFile record for the same Steam account.
func (m *Manager) Cancel(steamID uint64) error {
	if !m.valid() || !validSteamID(steamID) {
		return ErrInvalidRequest
	}
	lock := m.accountLock(steamID)
	lock.Lock()
	defer lock.Unlock()
	id, raw, found, err := m.loadRecord(steamID)
	if err != nil {
		return err
	}
	if !found {
		return ErrNoPendingEnrollment
	}
	defer wipe(raw)
	record, err := decodePending(raw)
	if err != nil || record.SteamID != steamID {
		return ErrNoPendingEnrollment
	}
	record.destroy()
	return m.vault.DeleteRecord(id)
}

func (m *Manager) loadRecord(steamID uint64) (string, []byte, bool, error) {
	entries, err := m.vault.ListRecords()
	if err != nil {
		return "", nil, false, err
	}
	wanted := steamIDString(steamID)
	for _, entry := range entries {
		if entry.SteamID64 != wanted {
			continue
		}
		raw, err := m.vault.GetRecord(entry.ID)
		return entry.ID, raw, true, err
	}
	return "", nil, false, nil
}

func IsNoPending(err error) bool { return errors.Is(err, ErrNoPendingEnrollment) }
