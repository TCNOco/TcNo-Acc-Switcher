package authflow

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"io"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

const (
	handleBytes       = 24
	maxHandleAttempts = 4
	maxBindingBytes   = 256
	maxCapacity       = 64
	maxTombstones     = 1024
	maxSessionTTL     = 30 * time.Minute
)

type realClock struct{}

func (realClock) Now() time.Time                             { return time.Now() }
func (realClock) After(delay time.Duration) <-chan time.Time { return time.After(delay) }

type authorizedData struct {
	steamID              uint64
	accountName          []byte
	accessToken          []byte
	refreshToken         []byte
	guardData            []byte
	hadRemoteInteraction bool
}

type sessionEntry struct {
	handle string
	// accountKey indexes m.accounts. A QR session and a password session for one
	// account are two different sign-ins and both have to be able to be open at
	// once, because the unlock screen offers them side by side.
	accountKey   string
	viaQR        bool
	challengeURL string
	binding      Binding
	session      protocol.AuthSession
	state      State
	challenges []Challenge
	expiresAt  time.Time
	nextPoll   time.Time
	authorized *authorizedData
	busy       bool
	closed     bool
	destroyed  bool
	cancel     context.CancelFunc
}

type tombstone struct {
	handle    string
	expiresAt time.Time
}

// Manager owns bounded, short-lived authentication state. Close must be called
// during application shutdown so every remaining protocol session is destroyed.
type Manager struct {
	mu        sync.Mutex
	wg        sync.WaitGroup
	client    Client
	config    Config
	sessions  map[string]*sessionEntry
	accounts  map[string]*sessionEntry
	tombs     map[string]time.Time
	tombOrder []tombstone
	stop      chan struct{}
	done      chan struct{}
	closed    bool
}

func New(client Client, config Config) (*Manager, error) {
	if client == nil {
		return nil, flowError(ErrorInvalid)
	}
	if config.Capacity == 0 {
		config.Capacity = DefaultCapacity
	}
	if config.SessionTTL == 0 {
		config.SessionTTL = DefaultSessionTTL
	}
	if config.OperationTimeout == 0 {
		config.OperationTimeout = DefaultOperationTimeout
	}
	if config.SweepInterval == 0 {
		config.SweepInterval = DefaultSweepInterval
	}
	if config.TombstoneTTL == 0 {
		config.TombstoneTTL = DefaultTombstoneTTL
	}
	if config.TombstoneCapacity == 0 {
		config.TombstoneCapacity = DefaultTombstoneCapacity
	}
	if config.Entropy == nil {
		config.Entropy = rand.Reader
	}
	if config.Clock == nil {
		config.Clock = realClock{}
	}
	if config.Capacity < 1 || config.Capacity > maxCapacity ||
		config.SessionTTL <= 0 || config.SessionTTL > maxSessionTTL ||
		config.OperationTimeout <= 0 || config.OperationTimeout > protocol.MaxRequestTimeout ||
		config.SweepInterval <= 0 || config.SweepInterval > config.SessionTTL ||
		config.TombstoneTTL <= 0 || config.TombstoneTTL > maxSessionTTL ||
		config.TombstoneCapacity < config.Capacity || config.TombstoneCapacity > maxTombstones {
		return nil, flowError(ErrorInvalid)
	}
	manager := &Manager{
		client:   client,
		config:   config,
		sessions: make(map[string]*sessionEntry, config.Capacity),
		accounts: make(map[string]*sessionEntry, config.Capacity),
		tombs:    make(map[string]time.Time, config.TombstoneCapacity),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go manager.reapLoop()
	return manager, nil
}

// Begin starts one credentials session for binding.AccountID. Password is
// borrowed only for this call and is never copied into manager state.
func (m *Manager) Begin(ctx context.Context, binding Binding, request protocol.PasswordCredentialsRequest, password []byte) (status Status, err error) {
	if ctx == nil || !validBinding(binding) || len(password) == 0 {
		return Status{}, flowError(ErrorInvalid)
	}
	now := m.config.Clock.Now()
	m.mu.Lock()
	m.expireLocked(now)
	if m.closed {
		m.mu.Unlock()
		return Status{}, flowError(ErrorClosed)
	}
	if _, exists := m.accounts[binding.AccountID]; exists {
		m.mu.Unlock()
		return Status{}, flowError(ErrorConflict)
	}
	if len(m.sessions) >= m.config.Capacity {
		m.mu.Unlock()
		return Status{}, flowError(ErrorCapacity)
	}
	handle, handleErr := m.newHandleLocked()
	if handleErr != nil {
		m.mu.Unlock()
		return Status{}, handleErr
	}
	opCtx, cancel := context.WithTimeout(ctx, m.config.OperationTimeout)
	entry := &sessionEntry{
		handle:     handle,
		accountKey: binding.AccountID,
		binding:    binding,
		expiresAt:  now.Add(m.config.SessionTTL),
		busy:       true,
		cancel:     cancel,
	}
	m.sessions[handle] = entry
	m.accounts[entry.accountKey] = entry
	m.wg.Add(1)
	m.mu.Unlock()
	defer m.wg.Done()
	defer m.recoverOperation(entry)
	defer runtime.KeepAlive(password)

	result, clientErr := m.client.Begin(opCtx, request, password, m.config.OperationTimeout)
	cancel()
	m.mu.Lock()
	entry.busy = false
	entry.cancel = nil
	if entry.closed {
		result.Session.Destroy()
		m.destroyEntryLocked(entry)
		m.mu.Unlock()
		return Status{}, flowError(ErrorGone)
	}
	if clientErr != nil {
		result.Session.Destroy()
		m.closeEntryLocked(entry, m.config.Clock.Now())
		m.mu.Unlock()
		return Status{}, classifyClientError(clientErr)
	}
	if !entry.expiresAt.After(m.config.Clock.Now()) {
		result.Session.Destroy()
		m.closeEntryLocked(entry, m.config.Clock.Now())
		m.mu.Unlock()
		return Status{}, flowError(ErrorGone)
	}
	state, stateOK := mapResultState(result.State)
	if !stateOK || !validSession(result.Session, binding.ExpectedSteamID) {
		result.Session.Destroy()
		m.closeEntryLocked(entry, m.config.Clock.Now())
		m.mu.Unlock()
		return Status{}, flowError(ErrorProtocol)
	}
	entry.session = result.Session
	entry.state = state
	entry.challenges = projectChallenges(result.Session.Challenges())
	entry.nextPoll = m.config.Clock.Now().Add(result.Session.PollInterval())
	status = m.statusLocked(entry, m.config.Clock.Now())
	m.mu.Unlock()
	return status, nil
}

// BeginQR starts a session that is authorised by scanning, for the account named
// in binding.ExpectedAccountName.
//
// It occupies a different slot from Begin, so the QR code and the password form
// on the same screen are two live sign-ins rather than one refusing the other.
// The account name is mandatory: without it nothing checks who scanned, and the
// session would hand back a sign-in for whoever did.
func (m *Manager) BeginQR(ctx context.Context, binding Binding, request protocol.BeginQRRequest) (status Status, err error) {
	if ctx == nil || !validBinding(binding) || !validBoundedString(binding.ExpectedAccountName) {
		return Status{}, flowError(ErrorInvalid)
	}
	accountKey := qrAccountKey(binding.AccountID)
	now := m.config.Clock.Now()
	m.mu.Lock()
	m.expireLocked(now)
	if m.closed {
		m.mu.Unlock()
		return Status{}, flowError(ErrorClosed)
	}
	if _, exists := m.accounts[accountKey]; exists {
		m.mu.Unlock()
		return Status{}, flowError(ErrorConflict)
	}
	if len(m.sessions) >= m.config.Capacity {
		m.mu.Unlock()
		return Status{}, flowError(ErrorCapacity)
	}
	handle, handleErr := m.newHandleLocked()
	if handleErr != nil {
		m.mu.Unlock()
		return Status{}, handleErr
	}
	opCtx, cancel := context.WithTimeout(ctx, m.config.OperationTimeout)
	entry := &sessionEntry{
		handle:     handle,
		accountKey: accountKey,
		viaQR:      true,
		binding:    binding,
		expiresAt:  now.Add(m.config.SessionTTL),
		busy:       true,
		cancel:     cancel,
	}
	m.sessions[handle] = entry
	m.accounts[accountKey] = entry
	m.wg.Add(1)
	m.mu.Unlock()
	defer m.wg.Done()
	defer m.recoverOperation(entry)

	result, clientErr := m.client.BeginQR(opCtx, request, m.config.OperationTimeout)
	cancel()
	m.mu.Lock()
	entry.busy = false
	entry.cancel = nil
	if entry.closed {
		result.Session.Destroy()
		m.destroyEntryLocked(entry)
		m.mu.Unlock()
		return Status{}, flowError(ErrorGone)
	}
	if clientErr != nil {
		result.Session.Destroy()
		m.closeEntryLocked(entry, m.config.Clock.Now())
		m.mu.Unlock()
		return Status{}, classifyClientError(clientErr)
	}
	if !entry.expiresAt.After(m.config.Clock.Now()) {
		result.Session.Destroy()
		m.closeEntryLocked(entry, m.config.Clock.Now())
		m.mu.Unlock()
		return Status{}, flowError(ErrorGone)
	}
	if !validQRSession(result.Session) || result.ChallengeURL == "" {
		result.Session.Destroy()
		m.closeEntryLocked(entry, m.config.Clock.Now())
		m.mu.Unlock()
		return Status{}, flowError(ErrorProtocol)
	}
	entry.session = result.Session
	entry.state = StateWaiting
	entry.challengeURL = result.ChallengeURL
	entry.nextPoll = m.config.Clock.Now().Add(result.Session.PollInterval())
	status = m.statusLocked(entry, m.config.Clock.Now())
	m.mu.Unlock()
	return status, nil
}

// qrAccountKey keeps a QR session out of the slot the password session uses. The
// separator is a NUL so no account ID can be spelled to land in the other slot.
func qrAccountKey(accountID string) string {
	return "qr\x00" + accountID
}

// qrAccountMatches decides whether the account that scanned is the account the
// session was opened for. Steam treats account names case-insensitively, and the
// poll reports one rather than a SteamID, so this is the whole identity check.
func qrAccountMatches(expected, reported string) bool {
	expected = strings.TrimSpace(expected)
	reported = strings.TrimSpace(reported)
	if expected == "" || reported == "" {
		return false
	}
	return strings.EqualFold(expected, reported)
}

// SubmitCode sends one allowed email or device challenge answer. Code is
// borrowed only for the duration of this call.
func (m *Manager) SubmitCode(ctx context.Context, binding Binding, handle string, challenge Challenge, code []byte) (status Status, err error) {
	protocolChallenge, ok := submissionChallenge(challenge)
	if ctx == nil || !ok || !validChallengeCode(code) {
		return Status{}, flowError(ErrorInvalid)
	}
	entry, session, opCtx, cancel, startErr := m.startOperation(ctx, binding, handle, false, protocolChallenge)
	if startErr != nil {
		return Status{}, startErr
	}
	defer m.wg.Done()
	defer m.recoverOperation(entry)
	defer runtime.KeepAlive(code)

	result, clientErr := m.client.SubmitCode(opCtx, session, protocolChallenge, code, m.config.OperationTimeout)
	cancel()
	m.mu.Lock()
	entry.busy = false
	entry.cancel = nil
	if entry.closed {
		m.destroyEntryLocked(entry)
		m.mu.Unlock()
		return Status{}, flowError(ErrorGone)
	}
	if clientErr != nil {
		m.closeEntryLocked(entry, m.config.Clock.Now())
		m.mu.Unlock()
		return Status{}, classifyClientError(clientErr)
	}
	if !entry.expiresAt.After(m.config.Clock.Now()) {
		m.closeEntryLocked(entry, m.config.Clock.Now())
		m.mu.Unlock()
		return Status{}, flowError(ErrorGone)
	}
	switch result.State {
	case protocol.AuthResultChallengeAccepted:
		entry.state = StateCodeAccepted
		entry.challenges = nil
		entry.nextPoll = m.config.Clock.Now()
	case protocol.AuthResultAgreementRequired:
		entry.state = StateAgreementRequired
	default:
		m.closeEntryLocked(entry, m.config.Clock.Now())
		m.mu.Unlock()
		return Status{}, flowError(ErrorProtocol)
	}
	status = m.statusLocked(entry, m.config.Clock.Now())
	m.mu.Unlock()
	return status, nil
}

// Poll performs one bounded status poll. Calls before Steam's advertised poll
// interval return a typed delay without touching the remote service.
func (m *Manager) Poll(ctx context.Context, binding Binding, handle string) (status Status, err error) {
	entry, session, opCtx, cancel, startErr := m.startOperation(ctx, binding, handle, true, 0)
	if startErr != nil {
		return Status{}, startErr
	}
	defer m.wg.Done()
	defer m.recoverOperation(entry)

	result, clientErr := m.client.Poll(opCtx, session, m.config.OperationTimeout)
	cancel()
	m.mu.Lock()
	entry.busy = false
	entry.cancel = nil
	if entry.closed {
		result.Session.Destroy()
		wipePollResult(&result)
		m.destroyEntryLocked(entry)
		m.mu.Unlock()
		return Status{}, flowError(ErrorGone)
	}
	if clientErr != nil {
		result.Session.Destroy()
		wipePollResult(&result)
		m.closeEntryLocked(entry, m.config.Clock.Now())
		m.mu.Unlock()
		return Status{}, classifyClientError(clientErr)
	}
	if !entry.expiresAt.After(m.config.Clock.Now()) {
		result.Session.Destroy()
		wipePollResult(&result)
		m.closeEntryLocked(entry, m.config.Clock.Now())
		m.mu.Unlock()
		return Status{}, flowError(ErrorGone)
	}
	if !sessionStillValid(entry, result.Session, binding) {
		result.Session.Destroy()
		wipePollResult(&result)
		m.closeEntryLocked(entry, m.config.Clock.Now())
		m.mu.Unlock()
		return Status{}, flowError(ErrorProtocol)
	}
	entry.session = result.Session
	now := m.config.Clock.Now()
	// Steam rotates a QR code while it waits to be scanned, and reports the
	// replacement as a challenge. For this session that is not a challenge to
	// answer - it is the same sign-in with a new image - so it stays waiting.
	if entry.viaQR && result.State == protocol.AuthResultChallengeRequired && result.ChallengeURL != "" {
		entry.challengeURL = result.ChallengeURL
		result.State = protocol.AuthResultWaiting
	}
	switch result.State {
	case protocol.AuthResultAuthorized:
		if result.AccessToken == "" && result.RefreshToken == "" {
			wipePollResult(&result)
			m.closeEntryLocked(entry, now)
			m.mu.Unlock()
			return Status{}, flowError(ErrorProtocol)
		}
		if entry.viaQR && !qrAccountMatches(binding.ExpectedAccountName, result.AccountName) {
			// Somebody scanned it, but not the account this session was opened
			// for. Their tokens are never stored: they are wiped here, before
			// anything downstream can be handed a sign-in for the wrong account.
			wipePollResult(&result)
			m.closeEntryLocked(entry, now)
			m.mu.Unlock()
			return Status{}, flowError(ErrorBindingMismatch)
		}
		entry.authorized = &authorizedData{
			steamID:              result.Session.SteamID(),
			accountName:          []byte(result.AccountName),
			accessToken:          []byte(result.AccessToken),
			refreshToken:         []byte(result.RefreshToken),
			guardData:            []byte(result.GuardData),
			hadRemoteInteraction: result.HadRemoteInteraction,
		}
		wipePollResult(&result)
		entry.session.Destroy()
		entry.state = StateAuthorizedReady
		entry.challenges = nil
		entry.nextPoll = time.Time{}
	case protocol.AuthResultWaiting, protocol.AuthResultChallengeRequired, protocol.AuthResultAgreementRequired:
		state, _ := mapResultState(result.State)
		entry.state = state
		entry.nextPoll = now.Add(entry.session.PollInterval())
		wipePollResult(&result)
	default:
		wipePollResult(&result)
		m.closeEntryLocked(entry, now)
		m.mu.Unlock()
		return Status{}, flowError(ErrorProtocol)
	}
	status = m.statusLocked(entry, now)
	m.mu.Unlock()
	return status, nil
}

// Status returns the current safe projection and also enforces expiry.
func (m *Manager) Status(binding Binding, handle string) (Status, error) {
	if !validBinding(binding) || !validHandle(handle) {
		return Status{}, flowError(ErrorInvalid)
	}
	now := m.config.Clock.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked(now)
	entry, err := m.lookupLocked(binding, handle, now)
	if err != nil {
		return Status{}, err
	}
	return m.statusLocked(entry, now), nil
}

// Rebind moves every live session onto a new vault generation. A sign-in the
// user is halfway through - waiting on an emailed code, say - outlives a
// background renewal of stored session tokens, and that renewal changes nothing
// about who the session belongs to. Its caller must rebind the matching
// capability in the same pass, or the next call mismatches on the other side.
func (m *Manager) Rebind(generation string) {
	if !validBoundedString(generation) {
		return
	}
	m.mu.Lock()
	for _, entry := range m.sessions {
		entry.binding.VaultGeneration = generation
	}
	m.mu.Unlock()
}

// Cancel revokes a session. If a call is active, its context is canceled and
// the protocol session is destroyed as soon as the call releases it.
func (m *Manager) Cancel(binding Binding, handle string) error {
	if !validBinding(binding) || !validHandle(handle) {
		return flowError(ErrorInvalid)
	}
	now := m.config.Clock.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked(now)
	entry, err := m.lookupLocked(binding, handle, now)
	if err != nil {
		return err
	}
	m.closeEntryLocked(entry, now)
	return nil
}

// Consume transfers authorized credentials exactly once through a callback.
// Callback errors are deliberately replaced with a generic error.
func (m *Manager) Consume(binding Binding, handle string, consumer Consumer) error {
	if !validBinding(binding) || !validHandle(handle) || consumer == nil {
		return flowError(ErrorInvalid)
	}
	now := m.config.Clock.Now()
	m.mu.Lock()
	m.expireLocked(now)
	entry, err := m.lookupLocked(binding, handle, now)
	if err != nil {
		m.mu.Unlock()
		return err
	}
	if entry.busy {
		m.mu.Unlock()
		return flowError(ErrorBusy)
	}
	if entry.state != StateAuthorizedReady || entry.authorized == nil {
		m.mu.Unlock()
		return flowError(ErrorNotAuthorized)
	}
	credentials := entry.authorized
	entry.authorized = nil
	m.detachEntryLocked(entry, now)
	m.destroyEntryLocked(entry)
	m.mu.Unlock()
	defer destroyAuthorized(credentials)
	if consumeErr := consumer(credentials.steamID, credentials.accountName, credentials.accessToken, credentials.refreshToken, credentials.guardData, credentials.hadRemoteInteraction); consumeErr != nil {
		return flowError(ErrorConsumer)
	}
	return nil
}

// PurgeExpired allows lifecycle hooks and tests to force an immediate expiry
// sweep in addition to the manager's background sweep.
func (m *Manager) PurgeExpired() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.expireLocked(m.config.Clock.Now())
	m.mu.Unlock()
}

func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	close(m.stop)
	now := m.config.Clock.Now()
	for _, entry := range m.sessions {
		m.closeEntryLocked(entry, now)
	}
	m.mu.Unlock()
	<-m.done
	m.wg.Wait()
}

func (m *Manager) startOperation(ctx context.Context, binding Binding, handle string, poll bool, required protocol.ChallengeType) (*sessionEntry, protocol.AuthSession, context.Context, context.CancelFunc, error) {
	if ctx == nil || !validBinding(binding) || !validHandle(handle) {
		return nil, protocol.AuthSession{}, nil, nil, flowError(ErrorInvalid)
	}
	now := m.config.Clock.Now()
	m.mu.Lock()
	m.expireLocked(now)
	if m.closed {
		m.mu.Unlock()
		return nil, protocol.AuthSession{}, nil, nil, flowError(ErrorClosed)
	}
	entry, err := m.lookupLocked(binding, handle, now)
	if err != nil {
		m.mu.Unlock()
		return nil, protocol.AuthSession{}, nil, nil, err
	}
	if entry.busy {
		m.mu.Unlock()
		return nil, protocol.AuthSession{}, nil, nil, flowError(ErrorBusy)
	}
	if entry.authorized != nil {
		m.mu.Unlock()
		return nil, protocol.AuthSession{}, nil, nil, flowError(ErrorNotAuthorized)
	}
	if poll && now.Before(entry.nextPoll) {
		retry := entry.nextPoll.Sub(now)
		m.mu.Unlock()
		return nil, protocol.AuthSession{}, nil, nil, &Error{Kind: ErrorTooSoon, RetryAfter: retry, HasRetryAfter: true}
	}
	if required != 0 && !sessionAllows(entry.session, required) {
		m.mu.Unlock()
		return nil, protocol.AuthSession{}, nil, nil, flowError(ErrorInvalid)
	}
	opCtx, cancel := context.WithTimeout(ctx, m.config.OperationTimeout)
	entry.busy = true
	entry.cancel = cancel
	m.wg.Add(1)
	session := entry.session
	m.mu.Unlock()
	return entry, session, opCtx, cancel, nil
}

func (m *Manager) recoverOperation(entry *sessionEntry) {
	panicValue := recover()
	if panicValue == nil {
		return
	}
	m.mu.Lock()
	if entry.cancel != nil {
		entry.cancel()
		entry.cancel = nil
	}
	entry.busy = false
	if entry.closed {
		m.destroyEntryLocked(entry)
	} else {
		m.closeEntryLocked(entry, m.config.Clock.Now())
	}
	m.mu.Unlock()
	panic(panicValue)
}

func (m *Manager) reapLoop() {
	defer close(m.done)
	for {
		select {
		case <-m.stop:
			return
		case <-m.config.Clock.After(m.config.SweepInterval):
			m.PurgeExpired()
		}
	}
}

func (m *Manager) lookupLocked(binding Binding, handle string, now time.Time) (*sessionEntry, error) {
	entry, exists := m.sessions[handle]
	if !exists {
		if expiry, gone := m.tombs[handle]; gone && expiry.After(now) {
			return nil, flowError(ErrorGone)
		}
		return nil, flowError(ErrorNotFound)
	}
	if !sameBinding(entry.binding, binding) {
		return nil, flowError(ErrorBindingMismatch)
	}
	return entry, nil
}

func (m *Manager) statusLocked(entry *sessionEntry, now time.Time) Status {
	status := Status{
		Handle:        entry.handle,
		State:         entry.state,
		Challenges:    append([]Challenge(nil), entry.challenges...),
		CanPoll:       entry.authorized == nil,
		ChallengeURL:  entry.challengeURL,
		ExpiresAtUnix: entry.expiresAt.Unix(),
	}
	for _, challenge := range entry.challenges {
		status.CanSubmitEmailCode = status.CanSubmitEmailCode || challenge == ChallengeEmailCode
		status.CanSubmitDeviceCode = status.CanSubmitDeviceCode || challenge == ChallengeDeviceCode
	}
	if status.CanPoll && now.Before(entry.nextPoll) {
		status.PollAfterMillis = int64((entry.nextPoll.Sub(now) + time.Millisecond - 1) / time.Millisecond)
	}
	return status
}

func (m *Manager) expireLocked(now time.Time) {
	m.purgeTombstonesLocked(now)
	for _, entry := range m.sessions {
		if !entry.expiresAt.After(now) {
			m.closeEntryLocked(entry, now)
		}
	}
}

func (m *Manager) closeEntryLocked(entry *sessionEntry, now time.Time) {
	if entry == nil || entry.closed {
		return
	}
	m.detachEntryLocked(entry, now)
	if entry.cancel != nil {
		entry.cancel()
	}
	if !entry.busy {
		m.destroyEntryLocked(entry)
	}
}

func (m *Manager) detachEntryLocked(entry *sessionEntry, now time.Time) {
	if entry.closed {
		return
	}
	entry.closed = true
	if current := m.sessions[entry.handle]; current == entry {
		delete(m.sessions, entry.handle)
	}
	if current := m.accounts[entry.accountKey]; current == entry {
		delete(m.accounts, entry.accountKey)
	}
	m.addTombstoneLocked(entry.handle, now)
}

func (m *Manager) destroyEntryLocked(entry *sessionEntry) {
	if entry == nil || entry.destroyed {
		return
	}
	entry.session.Destroy()
	destroyAuthorized(entry.authorized)
	entry.authorized = nil
	entry.challenges = nil
	entry.binding = Binding{}
	entry.state = ""
	entry.cancel = nil
	entry.destroyed = true
}

func destroyAuthorized(credentials *authorizedData) {
	if credentials == nil {
		return
	}
	wipe(credentials.accountName)
	wipe(credentials.accessToken)
	wipe(credentials.refreshToken)
	wipe(credentials.guardData)
	credentials.steamID = 0
	credentials.accountName = nil
	credentials.accessToken = nil
	credentials.refreshToken = nil
	credentials.guardData = nil
	credentials.hadRemoteInteraction = false
}

func wipePollResult(result *protocol.PollResult) {
	if result == nil {
		return
	}
	result.RefreshToken = ""
	result.AccessToken = ""
	result.AccountName = ""
	result.GuardData = ""
	result.ChallengeURL = ""
	result.AgreementURL = ""
	runtime.KeepAlive(result)
}

func wipe(value []byte) {
	for index := range value {
		value[index] = 0
	}
	runtime.KeepAlive(value)
}

func (m *Manager) addTombstoneLocked(handle string, now time.Time) {
	if handle == "" {
		return
	}
	expiresAt := now.Add(m.config.TombstoneTTL)
	m.tombs[handle] = expiresAt
	m.tombOrder = append(m.tombOrder, tombstone{handle: handle, expiresAt: expiresAt})
	for len(m.tombOrder) > m.config.TombstoneCapacity {
		oldest := m.tombOrder[0]
		m.tombOrder = m.tombOrder[1:]
		if current, exists := m.tombs[oldest.handle]; exists && current.Equal(oldest.expiresAt) {
			delete(m.tombs, oldest.handle)
		}
	}
}

func (m *Manager) purgeTombstonesLocked(now time.Time) {
	for len(m.tombOrder) > 0 && !m.tombOrder[0].expiresAt.After(now) {
		oldest := m.tombOrder[0]
		m.tombOrder = m.tombOrder[1:]
		if current, exists := m.tombs[oldest.handle]; exists && current.Equal(oldest.expiresAt) {
			delete(m.tombs, oldest.handle)
		}
	}
}

func (m *Manager) newHandleLocked() (string, *Error) {
	buffer := make([]byte, handleBytes)
	defer wipe(buffer)
	for attempt := 0; attempt < maxHandleAttempts; attempt++ {
		if _, err := io.ReadFull(m.config.Entropy, buffer); err != nil {
			return "", flowError(ErrorProtocol)
		}
		handle := base64.RawURLEncoding.EncodeToString(buffer)
		if _, live := m.sessions[handle]; live {
			continue
		}
		if _, replay := m.tombs[handle]; replay {
			continue
		}
		return handle, nil
	}
	return "", flowError(ErrorProtocol)
}

func mapResultState(state protocol.AuthResultState) (State, bool) {
	switch state {
	case protocol.AuthResultWaiting:
		return StateWaiting, true
	case protocol.AuthResultChallengeRequired:
		return StateChallengeRequired, true
	case protocol.AuthResultAgreementRequired:
		return StateAgreementRequired, true
	case protocol.AuthResultChallengeAccepted:
		return StateCodeAccepted, true
	default:
		return "", false
	}
}

func projectChallenges(challenges []protocol.AllowedChallenge) []Challenge {
	projected := make([]Challenge, 0, len(challenges))
	for _, challenge := range challenges {
		var safe Challenge
		switch challenge.Type {
		case protocol.ChallengeNone:
			safe = ChallengeNone
		case protocol.ChallengeEmailCode:
			safe = ChallengeEmailCode
		case protocol.ChallengeDeviceCode:
			safe = ChallengeDeviceCode
		case protocol.ChallengeDeviceConfirmation:
			safe = ChallengeDeviceConfirmation
		case protocol.ChallengeEmailConfirmation:
			safe = ChallengeEmailConfirmation
		default:
			safe = ChallengeUnsupported
		}
		projected = append(projected, safe)
	}
	return projected
}

func submissionChallenge(challenge Challenge) (protocol.ChallengeType, bool) {
	switch challenge {
	case ChallengeEmailCode:
		return protocol.ChallengeEmailCode, true
	case ChallengeDeviceCode:
		return protocol.ChallengeDeviceCode, true
	default:
		return 0, false
	}
}

func sessionAllows(session protocol.AuthSession, expected protocol.ChallengeType) bool {
	for _, challenge := range session.Challenges() {
		if challenge.Type == expected {
			return true
		}
	}
	return false
}

func validSession(session protocol.AuthSession, expectedSteamID uint64) bool {
	return session.ID() != "" && session.ClientID() != 0 && session.SteamID() != 0 &&
		session.PollInterval() > 0 && (expectedSteamID == 0 || session.SteamID() == expectedSteamID)
}

// validQRSession is validSession for a sign-in nobody has scanned yet, which is
// why it names no account. The account arrives with the poll and is checked
// against the binding there.
func validQRSession(session protocol.AuthSession) bool {
	return session.ID() != "" && session.ClientID() != 0 && session.ViaQR() &&
		session.SteamID() == 0 && session.PollInterval() > 0
}

func sessionStillValid(entry *sessionEntry, session protocol.AuthSession, binding Binding) bool {
	if entry.viaQR {
		return validQRSession(session)
	}
	return validSession(session, binding.ExpectedSteamID)
}

func validBinding(binding Binding) bool {
	return validBoundedString(binding.AccountID) && validBoundedString(binding.VaultGeneration) && validBoundedString(binding.CapabilityID)
}

func validBoundedString(value string) bool {
	return len(value) > 0 && len(value) <= maxBindingBytes && utf8.ValidString(value)
}

func sameBinding(left, right Binding) bool {
	return left.ExpectedSteamID == right.ExpectedSteamID &&
		subtle.ConstantTimeCompare([]byte(left.ExpectedAccountName), []byte(right.ExpectedAccountName)) == 1 &&
		subtle.ConstantTimeCompare([]byte(left.AccountID), []byte(right.AccountID)) == 1 &&
		subtle.ConstantTimeCompare([]byte(left.VaultGeneration), []byte(right.VaultGeneration)) == 1 &&
		subtle.ConstantTimeCompare([]byte(left.CapabilityID), []byte(right.CapabilityID)) == 1
}

func validHandle(handle string) bool {
	if len(handle) != base64.RawURLEncoding.EncodedLen(handleBytes) {
		return false
	}
	decoded := make([]byte, handleBytes)
	defer wipe(decoded)
	count, err := base64.RawURLEncoding.Decode(decoded, []byte(handle))
	return err == nil && count == handleBytes
}

func validChallengeCode(code []byte) bool {
	if len(code) != 5 {
		return false
	}
	for _, current := range code {
		if (current < 'A' || current > 'Z') && (current < 'a' || current > 'z') && (current < '0' || current > '9') {
			return false
		}
	}
	return true
}
