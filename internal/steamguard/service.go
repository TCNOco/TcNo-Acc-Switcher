package steamguard

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/steamguard/authflow"
	"TcNo-Acc-Switcher/internal/steamguard/capability"
	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
	"TcNo-Acc-Switcher/internal/steamguard/enrollmentapi"
	"TcNo-Acc-Switcher/internal/steamguard/enrollmentflow"
	"TcNo-Acc-Switcher/internal/steamguard/hwkey"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/otp"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
	"TcNo-Acc-Switcher/internal/steamguard/qrattempt"
	"TcNo-Acc-Switcher/internal/steamguard/qrcapture"
	"TcNo-Acc-Switcher/internal/steamguard/qrregion"
	"TcNo-Acc-Switcher/internal/steamguard/registry"
	"TcNo-Acc-Switcher/internal/steamguard/secureclipboard"
	"TcNo-Acc-Switcher/internal/steamguard/securefile"
	"TcNo-Acc-Switcher/internal/steamguard/sessionrefresh"
	"TcNo-Acc-Switcher/internal/steamguard/timesync"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
	"TcNo-Acc-Switcher/internal/steamguard/vaultrecord"

	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/steam/accountstore"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// rememberSteamAccount makes a vault account visible in the Steam account list
// straight away, whether or not Steam has a loginusers.vdf row for it.
//
// The registration index next door holds the ID and state only, so this is the
// one place the login name reaches the list - without it a login-only account
// or a freshly imported maFile produces no tile at all. Seeding the store also
// means a later switch can rebuild the account's loginusers.vdf row.
//
// An empty accountName is fine: the store merges field-wise, so callers that
// only have the ID cannot erase a name an earlier sighting recorded.
func (s *Service) rememberSteamAccount(steamID64, accountName string) {
	steamID64 = strings.TrimSpace(steamID64)
	if steamID64 == "" {
		return
	}
	changed, err := accountstore.Upsert(accountstore.Record{
		SteamID64:   steamID64,
		AccountName: strings.TrimSpace(accountName),
		Source:      accountstore.SourceSteamGuard,
	})
	if err != nil {
		serviceLogger().Warn("Steam account store update failed",
			"steamId64", steamID64, "error", err)
		return
	}
	// Every registry write comes through here, so only ask for a refresh when
	// the store actually moved; an unchanged record has nothing new to fetch.
	if changed {
		steam.RequestProfileRefresh()
	}
}

var (
	ErrFeatureDisabled = errors.New("Steam Guard integration is disabled")
	ErrVaultNotReady   = errors.New("Steam Guard vault is not configured")
	ErrAccountNotFound = errors.New("Steam Guard account not found")
	ErrInvalidImport   = errors.New("invalid Steam Guard import")
	ErrPathNotAbsolute = errors.New("Steam Guard import path must be absolute")
	// ErrExportManifestExists reports an encrypted export whose maFile was written
	// but whose companion manifest was not, because one already existed there and
	// overwriting it would destroy the account list it holds.
	ErrExportManifestExists      = errors.New("a manifest.json already exists beside the exported maFile, so it was left untouched")
	ErrUnsupportedInput          = errors.New("unsupported Steam Guard import file")
	ErrLegacyManifest            = errors.New("legacy SDA manifest is required")
	ErrAppPassword               = errors.New("current app password is required")
	ErrSensitiveView             = errors.New("Steam Guard sensitive view is unavailable")
	ErrSensitiveLease            = errors.New("invalid Steam Guard sensitive view lease")
	ErrPasswordReuse             = errors.New("Steam Guard password must differ from the app password")
	ErrRetainedUnlockUnavailable = errors.New("protected memory is unavailable; only one-time Steam Guard code access is available")
)

const (
	SensitiveViewGrantEvent   = "steamguard:sensitive-view-grant"
	SensitiveViewRevokedEvent = "steamguard:sensitive-view-revoked"
	mainWindowName            = "main"
	modalCapabilityScope      = "steamguard-modal"
)

// serviceLogger is the component logger for the Steam Guard service layer. It
// goes through the process-wide logredact handler; never pass secrets to it.
func serviceLogger() *slog.Logger {
	return slog.Default().With("component", "steamguard")
}

type SettingsStatus struct {
	VaultConfigured            bool          `json:"vaultConfigured"`
	Unlocked                   bool          `json:"unlocked"`
	RememberPasswordForSession bool          `json:"rememberPasswordForSession"`
	FolderPath                 string        `json:"folderPath"`
	LastVerifiedBackup         *BackupStatus `json:"lastVerifiedBackup"`
	AppPasswordSet             bool          `json:"appPasswordSet"`
	// HasSecurityKey and PasswordOpens describe the ways into the vault, so the
	// unlock screen can offer what will actually work rather than assuming a
	// password. Both are read from the header and need no unlocking.
	HasSecurityKey            bool `json:"hasSecurityKey"`
	PasswordOpens             bool `json:"passwordOpens"`
	SavedAccountDataEncrypted bool `json:"savedAccountDataEncrypted"`
}

type BackupStatus struct {
	VerifiedAt string `json:"verifiedAt"`
	Path       string `json:"path"`
}

type CodeView struct {
	SteamID64         string            `json:"steamId64"`
	AccountName       string            `json:"accountName"`
	Code              string            `json:"code"`
	ExpiresAt         int64             `json:"expiresAt"`
	TimeStatus        string            `json:"timeStatus"`
	UnlockPersistence UnlockPersistence `json:"unlockPersistence"`
}

type UnlockPersistence string

const (
	UnlockPersistenceCached       UnlockPersistence = "cached"
	UnlockPersistenceOneOperation UnlockPersistence = "one_operation"
)

// AccountKind names a vault record's shape for the UI. A discriminant rather
// than a boolean because the vault has grown a third shape once already.
type AccountKind = string

const (
	AccountKindAuthenticator AccountKind = "authenticator"
	AccountKindLoginOnly     AccountKind = "login-only"
	AccountKindPending       AccountKind = "pending"
)

// SessionStatus is what the stored Steam session looks like from here, without
// asking Steam. Unknown is deliberately the zero value: an unreadable token is
// not evidence of anything, and must not be shown as either working or lapsed.
type SessionStatus = string

const (
	SessionStatusUnknown    SessionStatus = ""
	SessionStatusValid      SessionStatus = "valid"
	SessionStatusNeedsLogin SessionStatus = "needs_login"
)

type AccountSummary struct {
	SteamID64   string `json:"steamId64"`
	AccountName string `json:"accountName"`
	// Kind decides which actions the picker offers. A login-only account has no
	// shared secret, so asking it for a code would fail.
	Kind AccountKind `json:"kind"`
	// SessionStatus lets the picker show which accounts need a fresh sign-in
	// without opening each one. Decided from the decrypted record alone: no Steam
	// request, no vault write, so listing costs no more than it did.
	SessionStatus SessionStatus `json:"sessionStatus"`
}

type SensitiveViewGrant struct {
	Capability string `json:"capability"`
	Lease      string `json:"lease"`
	AccountID  string `json:"accountId"`
	RequestID  string `json:"requestId"`
}

type sensitiveViewLease struct {
	binding capability.Binding
}

type ImportResult struct {
	Path            string   `json:"path"`
	SteamID64       string   `json:"steamId64,omitempty"`
	AccountName     string   `json:"accountName,omitempty"`
	DiscardedFields []string `json:"discardedFields,omitempty"`
	Imported        bool     `json:"imported"`
	ErrorCode       string   `json:"errorCode,omitempty"`
	// CapabilityRefreshRequired is true when this entry wrote to the vault.
	// The write rotates the vault generation, so any capability the UI already
	// holds is invalid and must be re-acquired.
	CapabilityRefreshRequired bool `json:"capabilityRefreshRequired"`
}

type Service struct {
	mu           sync.Mutex
	vault        *vault.Vault
	vaultOptions []vault.Option
	liveKDF      vault.KDFParams
	backupKDF    vault.KDFParams
	saveKeyfile  func(vault.Keyfile) (string, error)
	// Substituted in tests with a deterministic fake, so every security-key
	// path is exercised without hardware. Nil means the platform driver.
	authenticator hwkey.Authenticator
	// managementVerified is when the last verified management authentication
	// stops covering the factor change it was asked for. An unlocked vault does
	// not authorise anything on its own: the lease can last the whole session.
	managementVerified time.Time
	// restoreInProgress marks the window where the live vault folder exists but
	// is not yet a vault this process may adopt.
	restoreInProgress          bool
	restoreMergeStage          string
	restoreMergeSource         string
	timeState                  *otp.TimeState
	timeSync                   *timesync.Client
	timeSyncCancel             context.CancelFunc
	clipboard                  clipboardManager
	contentProtectionMu        sync.Mutex
	contentProtectionLeases    map[string]sensitiveViewLease
	capabilities               *capability.Manager
	setMainContentProtectionFn func(bool) error
	emitMainWindowEventFn      func(string, any) error
	// emitCooldownFn publishes a CS2 cooldown change to the account list. Left
	// nil in tests so the emission can be asserted rather than dispatched.
	emitCooldownFn func(steam.CS2CooldownPatch)
	cooldownSweep  cooldownSweepState
	// emitOwnedGamesFn publishes a library change to an open games view. Left
	// nil in tests for the same reason as emitCooldownFn.
	emitOwnedGamesFn func(steam.OwnedGamesPatch)
	ownedGamesSweep  ownedGamesSweepState
	// steamDataRefresh rate limits the whole-platform refresh that an unlock or
	// a new account kicks off.
	steamDataRefresh       steamDataRefreshState
	confirmationWindowMu   sync.Mutex
	confirmationAccountID  string
	confirmationGeneration string
	confirmationInstanceID string
	confirmationRows       map[string]confirmationapi.Confirmation
	confirmationCancel     context.CancelFunc
	confirmationOperation  uint64
	// A detail fetch gets its own cancel slot. Sharing the one above meant a
	// ten-second poll landing mid-fetch cancelled the open trade, and the trade
	// cancelled the poll right back.
	confirmationDetailCancel    context.CancelFunc
	confirmationDetailOperation uint64
	// How many detail or item fetches are downloading icons right now. A refresh
	// cannot know what they will produce, so it does not prune while any run.
	confirmationDetailsInFlight int
	confirmationClient          steamConfirmationClient
	confirmationIcons           *confirmationIconCache
	// Icons the open detail refers to. The list poll prunes everything it does
	// not reference, which would otherwise clear the images under an open trade.
	confirmationDetailIcons []string
	// Item descriptions already fetched for the open window, keyed by
	// appid/classid/instanceid, so hovering an item repeatedly costs one request.
	confirmationItems         map[string]ConfirmationItemView
	qrScanner                 steamQRScanner
	qrRegionSelector          steamQRRegionSelector
	qrRegionMu                sync.Mutex
	qrRegionCancel            context.CancelFunc
	qrRegionBinding           capability.Binding
	qrRegionOperation         uint64
	qrAttempts                *qrattempt.Manager
	qrAuth                    steamQRAuthenticator
	steamProtocol             *protocol.Client
	resolveSteamExecutableFn  func() (string, bool)
	authStateMu               sync.Mutex
	authManager               steamCredentialAuthManager
	newAuthManager            func() (steamCredentialAuthManager, error)
	authOperations            map[string]steamAuthOperation
	authManagerEpoch          uint64
	authShutdown              bool
	enrollmentManager         steamEnrollmentManager
	newEnrollmentManager      func(*vault.Vault) steamEnrollmentManager
	newSessionRefresher       func(*vault.Vault) steamSessionRefresher
	revocationAcknowledgments map[string]revocationAcknowledgment
	steamOperationCancels     map[string]steamBoundOperation
	steamOperationSequence    uint64
	registryUpsertFn          func(string, registry.State) error
	// pendingAdds are the in-flight add-account attempts, keyed by the pending
	// id issued for each. See add_account_service.go.
	pendingAddMu sync.Mutex
	pendingAdds  map[string]pendingAdd
}

type clipboardManager interface {
	Copy(string, time.Duration) error
	Clear() (bool, error)
	Close() error
}

type lifecycleHook struct{ service *Service }

func NewService() *Service {
	steamProtocol := protocol.NewClient(protocol.Options{})
	authenticationClient := protocol.NewAuthenticationClient(steamProtocol)
	authAdapter := authflow.NewProtocolClient(authenticationClient)
	timeState := otp.NewTimeState(nil)
	s := &Service{
		timeState:                  timeState,
		timeSync:                   timesync.NewClient(),
		clipboard:                  secureclipboard.New(),
		contentProtectionLeases:    make(map[string]sensitiveViewLease),
		capabilities:               capability.NewManager(),
		confirmationRows:           make(map[string]confirmationapi.Confirmation),
		confirmationClient:         newConfirmationClient(steamProtocol, timeState),
		confirmationIcons:          newConfirmationIconCache(),
		qrScanner:                  qrcapture.New(),
		qrRegionSelector:           qrregion.New(),
		qrAttempts:                 qrattempt.New(),
		qrAuth:                     authenticationClient,
		steamProtocol:              steamProtocol,
		resolveSteamExecutableFn:   resolveSteamExecutable,
		setMainContentProtectionFn: setMainContentProtection,
		emitMainWindowEventFn:      emitMainWindowEvent,
		emitCooldownFn:             steam.EmitCS2CooldownPatch,
		emitOwnedGamesFn:           steam.EmitOwnedGamesPatch,
		authOperations:             make(map[string]steamAuthOperation),
		revocationAcknowledgments:  make(map[string]revocationAcknowledgment),
		steamOperationCancels:      make(map[string]steamBoundOperation),
		registryUpsertFn:           registry.Upsert,
	}
	s.cooldownSweep.wake = make(chan struct{}, 1)
	s.ownedGamesSweep.wake = make(chan struct{}, 1)
	s.newAuthManager = func() (steamCredentialAuthManager, error) {
		return authflow.New(authAdapter, authflow.Config{})
	}
	s.newEnrollmentManager = func(v *vault.Vault) steamEnrollmentManager {
		return enrollmentflow.New(enrollmentapi.NewClient(steamProtocol), v)
	}
	s.newSessionRefresher = func(v *vault.Vault) steamSessionRefresher {
		return sessionrefresh.New(authenticationClient, v)
	}
	security.SetSteamGuardLifecycleHook(lifecycleHook{service: s})
	return s
}

func newServiceForTest(options ...vault.Option) *Service {
	steamProtocol := protocol.NewClient(protocol.Options{})
	authenticationClient := protocol.NewAuthenticationClient(steamProtocol)
	authAdapter := authflow.NewProtocolClient(authenticationClient)
	timeState := otp.NewTimeState(nil)
	// Tests must not pay production KDF cost: a real derivation per vault
	// creation dominates the package runtime. Caller-supplied options come
	// after these, so a test can still pin its own parameters.
	testKDF := vault.KDFParams{Algorithm: "argon2id", MemoryKiB: 8 * 1024, Passes: 1, Lanes: 1, KeyBytes: 32}
	testBackupKDF := testKDF
	testBackupKDF.MemoryKiB = 16 * 1024
	options = append([]vault.Option{
		vault.WithKDFParams(testKDF), vault.WithRecoveryKDFParams(testKDF),
	}, options...)
	s := &Service{
		vaultOptions:               options,
		liveKDF:                    testKDF,
		backupKDF:                  testBackupKDF,
		timeState:                  timeState,
		timeSync:                   timesync.NewClient(),
		clipboard:                  secureclipboard.New(),
		contentProtectionLeases:    make(map[string]sensitiveViewLease),
		capabilities:               capability.NewManager(),
		confirmationRows:           make(map[string]confirmationapi.Confirmation),
		confirmationClient:         newConfirmationClient(steamProtocol, timeState),
		confirmationIcons:          newConfirmationIconCache(),
		qrScanner:                  qrcapture.New(),
		qrRegionSelector:           qrregion.New(),
		qrAttempts:                 qrattempt.New(),
		qrAuth:                     authenticationClient,
		steamProtocol:              steamProtocol,
		resolveSteamExecutableFn:   resolveSteamExecutable,
		setMainContentProtectionFn: setMainContentProtection,
		emitMainWindowEventFn:      emitMainWindowEvent,
		authOperations:             make(map[string]steamAuthOperation),
		revocationAcknowledgments:  make(map[string]revocationAcknowledgment),
		steamOperationCancels:      make(map[string]steamBoundOperation),
		registryUpsertFn:           registry.Upsert,
	}
	// Left with a nil emitCooldownFn and emitOwnedGamesFn on purpose: tests
	// assert the emission rather than dispatching it into an application that is
	// not running.
	s.cooldownSweep.wake = make(chan struct{}, 1)
	s.ownedGamesSweep.wake = make(chan struct{}, 1)
	s.newAuthManager = func() (steamCredentialAuthManager, error) {
		return authflow.New(authAdapter, authflow.Config{})
	}
	s.newEnrollmentManager = func(v *vault.Vault) steamEnrollmentManager {
		return enrollmentflow.New(enrollmentapi.NewClient(steamProtocol), v)
	}
	s.newSessionRefresher = func(v *vault.Vault) steamSessionRefresher {
		return sessionrefresh.New(authenticationClient, v)
	}
	return s
}

func (s *Service) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s.mu.Lock()
	if s.timeSyncCancel != nil {
		s.timeSyncCancel()
	}
	syncCtx, cancel := context.WithCancel(ctx)
	s.timeSyncCancel = cancel
	client := s.timeSync
	state := s.timeState
	s.mu.Unlock()
	if client != nil && state != nil {
		go runTimeSync(syncCtx, client, state)
	}
	s.startCooldownSweeper(ctx)
	s.startOwnedGamesSweeper(ctx)
	steam.SetOwnedGamesSweepHook(s.signalOwnedGamesSweep)
	steam.RegisterSteamGuardSweepTrigger(s.signalSteamGuardSweeps)
	registerLoginAgainHandoff()
	return nil
}

func (s *Service) ServiceShutdown() error {
	s.closeAuthenticationManager(true)
	s.stopCooldownSweeper()
	steam.SetOwnedGamesSweepHook(nil)
	steam.RegisterSteamGuardSweepTrigger(nil)
	s.stopOwnedGamesSweeper()
	s.mu.Lock()
	cancel := s.timeSyncCancel
	s.timeSyncCancel = nil
	clipboard := s.clipboard
	steamProtocol := s.steamProtocol
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if steamProtocol != nil {
		steamProtocol.CloseIdleConnections()
	}
	revocationErr := s.revokeLeases()
	if clipboard != nil {
		return errors.Join(revocationErr, clipboard.Close())
	}
	return revocationErr
}

// steamAlignedClock reports the timesync-corrected time. Confirmation
// signatures must use the same clock Steam does, or the k hash is rejected.
type steamAlignedClock struct{ state *otp.TimeState }

func (c steamAlignedClock) Now() time.Time {
	if c.state == nil {
		return time.Now()
	}
	now, _ := c.state.Now()
	return now
}

func newConfirmationClient(steamProtocol *protocol.Client, state *otp.TimeState) *confirmationapi.Client {
	return confirmationapi.NewClient(confirmationapi.Options{
		Protocol: steamProtocol, Clock: steamAlignedClock{state: state},
	})
}

func runTimeSync(ctx context.Context, client *timesync.Client, state *otp.TimeState) {
	const interval = 5 * time.Minute
	log := slog.Default().With("component", "steamguard.timesync")
	syncOnce(ctx, client, state, log)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			syncOnce(ctx, client, state, log)
		}
	}
}

func syncOnce(ctx context.Context, client *timesync.Client, state *otp.TimeState, log *slog.Logger) {
	result, err := client.Sync(ctx, state)
	if err != nil {
		if ctx.Err() == nil {
			log.Warn("Steam time sync failed", "error", err, "freshness", state.Freshness())
		}
		return
	}
	log.Debug("Steam time synchronised", "offset", result.Offset, "roundTrip", result.RoundTrip)
}

func setMainContentProtection(enabled bool) error {
	app := application.Get()
	if app == nil || app.Window == nil {
		return ErrSensitiveView
	}
	window, ok := app.Window.GetByName("main")
	if !ok {
		return ErrSensitiveView
	}
	// The lease still exists and is still released; only the exclusion is skipped,
	// so nothing else about a sensitive view changes when capture is allowed.
	window.SetContentProtection(enabled && contentProtectionEnabled())
	return nil
}

func emitMainWindowEvent(name string, data any) error {
	app := application.Get()
	if app == nil || app.Window == nil {
		return ErrSensitiveView
	}
	window, ok := app.Window.GetByName(mainWindowName)
	if !ok {
		return ErrSensitiveView
	}
	window.DispatchWailsEvent(&application.CustomEvent{Name: name, Sender: "native", Data: data})
	return nil
}

// RequestSensitiveView enables content protection and delivers an account-bound
// capability directly to the main window. The singleton service call itself
// returns no bearer credential.
func (s *Service) RequestSensitiveView(accountID, requestID string) error {
	accountID = strings.TrimSpace(accountID)
	if _, err := strconv.ParseUint(accountID, 10, 64); err != nil {
		return ErrSensitiveView
	}
	return s.issueSensitiveView(accountID, requestID)
}

// issueSensitiveView is the body shared with RequestAddAccountView. The callers
// own the identity check - this one only ever sees an id its caller has already
// vouched for - so that an add-account attempt cannot borrow the numeric gate
// above, nor an arbitrary string the numeric gate.
func (s *Service) issueSensitiveView(accountID, requestID string) error {
	accountID = strings.TrimSpace(accountID)
	requestID = strings.TrimSpace(requestID)
	if accountID == "" || len(requestID) < 16 || len(requestID) > 128 {
		return ErrSensitiveView
	}
	s.mu.Lock()
	v, exists, err := s.openVaultLocked()
	var generation string
	if err == nil && exists {
		generation = v.Generation()
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	leaseBytes := make([]byte, 16)
	if _, err := rand.Read(leaseBytes); err != nil {
		return errors.Join(ErrSensitiveView, err)
	}
	lease := base64.RawURLEncoding.EncodeToString(leaseBytes)

	s.contentProtectionMu.Lock()
	defer s.contentProtectionMu.Unlock()
	if s.setMainContentProtectionFn == nil || s.emitMainWindowEventFn == nil || s.capabilities == nil {
		return ErrSensitiveView
	}
	if len(s.contentProtectionLeases) == 0 {
		if err := s.setMainContentProtectionFn(true); err != nil {
			return errors.Join(ErrSensitiveView, err)
		}
	}
	if s.contentProtectionLeases == nil {
		s.contentProtectionLeases = make(map[string]sensitiveViewLease)
	}
	binding := capability.Binding{
		WindowName:      mainWindowName,
		AccountID:       accountID,
		Scope:           modalCapabilityScope,
		LeaseID:         lease,
		VaultGeneration: generation,
	}
	token, err := s.capabilities.Issue(binding)
	if err != nil {
		if len(s.contentProtectionLeases) == 0 {
			_ = s.setMainContentProtectionFn(false)
		}
		return errors.Join(ErrSensitiveView, err)
	}
	s.contentProtectionLeases[lease] = sensitiveViewLease{binding: binding}
	grant := SensitiveViewGrant{Capability: token, Lease: lease, AccountID: accountID, RequestID: requestID}
	if err := s.emitMainWindowEventFn(SensitiveViewGrantEvent, grant); err != nil {
		delete(s.contentProtectionLeases, lease)
		s.capabilities.Revoke(token)
		if len(s.contentProtectionLeases) == 0 {
			_ = s.setMainContentProtectionFn(false)
		}
		return errors.Join(ErrSensitiveView, err)
	}
	return nil
}

func (s *Service) EndSensitiveView(token, lease string) error {
	s.contentProtectionMu.Lock()
	defer s.contentProtectionMu.Unlock()
	if lease == "" || token == "" || s.capabilities == nil {
		return ErrSensitiveLease
	}
	viewLease, ok := s.contentProtectionLeases[lease]
	if !ok || s.capabilities.Validate(viewLease.binding, token) != nil {
		return ErrSensitiveLease
	}
	s.cancelAuthenticationForBinding(viewLease.binding, token)
	s.clearUnacknowledgedRevocationForCapability(viewLease.binding.AccountID, token)
	s.cancelQRRegionSelection(lease)
	delete(s.contentProtectionLeases, lease)
	if len(s.contentProtectionLeases) != 0 {
		s.capabilities.Revoke(token)
		return s.revokeQRAttempt(viewLease.binding.AccountID)
	}
	if s.setMainContentProtectionFn == nil {
		s.contentProtectionLeases[lease] = viewLease
		return ErrSensitiveView
	}
	if err := s.setMainContentProtectionFn(false); err != nil {
		s.contentProtectionLeases[lease] = viewLease
		return errors.Join(ErrSensitiveView, err)
	}
	s.capabilities.Revoke(token)
	return s.revokeQRAttempt(viewLease.binding.AccountID)
}

// carryCapabilitiesAcross moves every live window capability, and the flow state
// keyed alongside it, onto the vault generation a background session-token
// renewal has just committed.
//
// The generation a capability is bound to exists so a token cannot be spent
// against a vault state it was never authorised for. Renewing stored Steam
// session tokens produces no such state: the key is the same, the same records
// exist, and the same person may read them. Orphaning the open windows'
// capabilities over it only ever surfaced as "invalid Steam Guard window
// capability" on whatever the user clicked next - and worst on the flows that
// sit waiting on the user, where a QR region drag or a file picker straddles the
// sweep and the scan is refused after the work is already done.
//
// Only the session-token sweeps may call this. Every other write - a re-key, an
// import, a record removed - must keep rotating capabilities out.
//
// The three locks are taken one at a time on purpose: confirmationAccount holds
// confirmationWindowMu while it takes s.mu, and authorizeModalLocked holds s.mu
// while it takes contentProtectionMu, so nesting any two of these here would
// close a cycle.
func (s *Service) carryCapabilitiesAcross(generation string) {
	generation = strings.TrimSpace(generation)
	if generation == "" || s.capabilities == nil {
		return
	}

	s.contentProtectionMu.Lock()
	for lease, view := range s.contentProtectionLeases {
		view.binding.VaultGeneration = generation
		s.contentProtectionLeases[lease] = view
	}
	s.contentProtectionMu.Unlock()

	s.confirmationWindowMu.Lock()
	if s.confirmationInstanceID != "" {
		s.confirmationGeneration = generation
	}
	s.confirmationWindowMu.Unlock()

	s.authStateMu.Lock()
	for handle, operation := range s.authOperations {
		operation.binding.VaultGeneration = generation
		s.authOperations[handle] = operation
	}
	manager := s.authManager
	s.authStateMu.Unlock()
	// Optional rather than part of steamCredentialAuthManager: the operations
	// above and the manager's own entries are two halves of one binding, so a
	// fake that implements neither simply keeps today's behaviour.
	if rebinder, ok := manager.(interface{ Rebind(string) }); ok {
		rebinder.Rebind(generation)
	}

	if s.qrAttempts != nil {
		// A scanned code outlives the scan: the user still has to read who is
		// asking and press Approve, and that gap is wider than the sweep.
		s.qrAttempts.Rebind(generation)
	}

	s.capabilities.Rebind(generation)
}

func (s *Service) revokeQRAttempt(accountID string) error {
	if s.qrAttempts == nil {
		return nil
	}
	return s.qrAttempts.RevokeAccount(accountID)
}

func (s *Service) GetSettingsStatus() (SettingsStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings, err := LoadSettings()
	if err != nil {
		return SettingsStatus{}, err
	}
	securityStatus, err := security.GetStatus()
	if err != nil {
		return SettingsStatus{}, err
	}
	root, err := VaultFolderPath()
	if err != nil {
		return SettingsStatus{}, err
	}
	v, exists, err := s.openVaultLocked()
	if err != nil {
		return SettingsStatus{}, err
	}
	status := SettingsStatus{
		VaultConfigured:            exists,
		RememberPasswordForSession: settings.RememberPasswordForSession,
		FolderPath:                 root,
		AppPasswordSet:             securityStatus.AppPasswordSet,
		SavedAccountDataEncrypted:  securityStatus.SavedAccountDataEncrypted,
	}
	if exists {
		status.Unlocked = !v.IsLocked()
		// Read from the header, which needs no unlocking - the unlock screen has
		// to know a security key is enrolled before it can offer to use one.
		factors := summariseFactors(v.ListSlots())
		status.HasSecurityKey = factors.SecurityKeyCount > 0
		status.PasswordOpens = factors.PasswordOpens
	}
	if settings.LastVerifiedBackup != "" {
		status.LastVerifiedBackup = &BackupStatus{
			VerifiedAt: settings.LastVerifiedBackup,
			Path:       settings.LastVerifiedBackupPath,
		}
	}
	return status, nil
}

func (s *Service) SetFeatureEnabled(enabled bool) error {
	// Locked around the read-modify-write only. revokeLeases takes s.mu itself,
	// and every other writer of this file holds it - without that a toggle
	// landing mid password-change put back the verified-backup stamp the change
	// had just cleared, and the screen then claimed a backup the new password
	// cannot open.
	if err := func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		settings, err := LoadSettings()
		if err != nil {
			return err
		}
		settings.FeatureEnabled = enabled
		return SaveSettings(settings)
	}(); err != nil {
		return err
	}
	if !enabled {
		_ = s.revokeLeases()
	}
	return nil
}

func (s *Service) SetRememberPasswordForSession(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings, err := LoadSettings()
	if err != nil {
		return err
	}
	settings.RememberPasswordForSession = enabled
	if err := SaveSettings(settings); err != nil {
		return err
	}
	v, exists, err := s.openVaultLocked()
	if err != nil || !exists || v.IsLocked() {
		return err
	}
	mode := vault.FixedLease
	if enabled {
		mode = vault.ProcessLease
	}
	return v.SetLeaseMode(mode)
}

func (s *Service) Initialize(password, appPassword string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(password) == "" {
		return "", vault.ErrInvalidPassword
	}
	outerKey, err := appOuterKeyForRecovery(appPassword)
	if err != nil {
		return "", err
	}
	defer security.WipeSecret(outerKey)
	if len(outerKey) != 0 && samePassword(password, appPassword) {
		return "", ErrPasswordReuse
	}
	root, err := VaultFolderPath()
	if err != nil {
		return "", err
	}
	if _, exists, err := s.openVaultLocked(); err != nil {
		return "", err
	} else if exists {
		return "", vault.ErrAlreadyExists
	}
	v, err := vault.Create(root, password, s.vaultOptions...)
	if err != nil {
		return "", err
	}
	if len(outerKey) != 0 {
		if err := v.EnableOuterWithRecovery(outerKey, appPassword); err != nil {
			return "", err
		}
	}
	s.vault = v
	// Create returns a locked vault, but a vault is only ever created in order
	// to be used immediately: the 2-Factor flow goes straight from here into
	// enrollment, which refuses a locked vault. Unlocking here also avoids
	// asking for the password again moments after the user chose it.
	if err := s.unlockVaultLocked(v, password, false); err != nil {
		return "", err
	}
	settings, err := LoadSettings()
	if err != nil {
		return "", err
	}
	settings.FeatureEnabled = true
	if err := SaveSettings(settings); err != nil {
		return "", err
	}
	return root, nil
}

func (s *Service) UnlockAccount(steamID64, password string, rememberForSession bool, token string) (CodeView, error) {
	return s.unlockAccountWith(steamID64, vault.PasswordOnly(password), rememberForSession, token)
}

// UnlockAccountWithFactors unlocks a vault whose slots need more than a
// password. keyfilePath is read here rather than in the frontend, so keyfile
// material never crosses into the webview. Empty arguments are simply absent
// factors, so this also serves a password-only vault.
func (s *Service) UnlockAccountWithFactors(
	steamID64, password, keyfilePath, backupKey string,
	rememberForSession bool,
	token string,
) (CodeView, error) {
	creds, err := buildVaultCredentials(password, keyfilePath, backupKey)
	if err != nil {
		return CodeView{}, err
	}
	defer wipe(creds.Keyfile)
	defer wipe(creds.RecoveryCode)
	return s.unlockAccountWith(steamID64, creds, rememberForSession, token)
}

// buildVaultCredentials turns what the user supplied into vault credentials.
// A malformed backup key or keyfile is reported as such rather than being
// passed on to fail later as a wrong password.
func buildVaultCredentials(password, keyfilePath, backupKey string) (vault.Credentials, error) {
	creds := vault.Credentials{Password: password}
	if path := strings.TrimSpace(keyfilePath); path != "" {
		if !filepath.IsAbs(path) {
			return vault.Credentials{}, vault.ErrInvalidKeyfile
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return vault.Credentials{}, errors.Join(vault.ErrInvalidKeyfile, err)
		}
		defer wipe(raw)
		keyfile, err := vault.ParseKeyfile(raw)
		if err != nil {
			return vault.Credentials{}, err
		}
		creds.Keyfile = keyfile.Secret
		// Carried, not discarded: it is what lets the vault tell a stale keyfile
		// from a way in that simply has a password of its own.
		creds.KeyfileID = keyfile.ID
	}
	if code := strings.TrimSpace(backupKey); code != "" {
		raw, err := vault.ParseRecoveryCode(code)
		if err != nil {
			wipe(creds.Keyfile)
			return vault.Credentials{}, err
		}
		creds.RecoveryCode = raw
	}
	return creds, nil
}

// needsSecurityKey reports whether a failed unlock is the kind a security key
// could still satisfy: a slot needing a factor that was not supplied, or one
// whose supplied factors did not open it.
func needsSecurityKey(err error) bool {
	return errors.Is(err, vault.ErrFactorRequired) || errors.Is(err, vault.ErrInvalidPassword)
}

func (s *Service) unlockAccountWith(steamID64 string, creds vault.Credentials, rememberForSession bool, token string) (CodeView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.requireVaultLocked()
	if err != nil {
		return CodeView{}, err
	}
	if err := s.authorizeModalLocked(v, steamID64, token); err != nil {
		return CodeView{}, err
	}
	// Tried with what the caller supplied first, and only then with a security
	// key. Prompting for a touch before knowing one is needed would interrupt
	// every unlock of a vault that has a key enrolled as an alternative.
	unlockErr := s.unlockVaultWithLocked(v, creds, rememberForSession)
	if unlockErr != nil && len(creds.SecurityKey) == 0 && needsSecurityKey(unlockErr) {
		if credID, secret, keyErr := s.evaluateSecurityKeyLocked(v); keyErr == nil && len(secret) != 0 {
			creds.SecurityKey = secret
			creds.SecurityKeyID = credID
			unlockErr = s.unlockVaultWithLocked(v, creds, rememberForSession)
			wipe(secret)
			creds.SecurityKey = nil
			creds.SecurityKeyID = ""
		} else if keyErr != nil {
			serviceLogger().Warn("Steam Guard security key could not be used",
				"steamId64", strings.TrimSpace(steamID64), "error", keyErr)
			// Joined, not just logged: where the key was the only thing offered
			// there is no password to have been rejected, and reporting one made
			// a cancelled prompt read as a typo over an empty field.
			unlockErr = joinSecurityKeyFailure(unlockErr, keyErr, creds)
		}
	}
	if err := unlockErr; err != nil {
		if !errors.Is(err, vault.ErrOneOperationRequired) {
			serviceLogger().Warn("Steam Guard vault unlock failed",
				"steamId64", strings.TrimSpace(steamID64), "reason", unlockFailureReason(err), "error", err)
			return CodeView{}, err
		}
		serviceLogger().Info("Steam Guard vault requires a one-operation unlock", "steamId64", strings.TrimSpace(steamID64))
		if lockErr := v.Lock(); lockErr != nil {
			return CodeView{}, errors.Join(ErrRetainedUnlockUnavailable, lockErr)
		}
		var view CodeView
		operationErr := s.withOneOperationCredentialsLocked(v, creds, func(access *vault.OneOperationAccess) error {
			var err error
			view, err = s.codeFromReader(access, steamID64, UnlockPersistenceOneOperation)
			return err
		})
		if operationErr != nil {
			serviceLogger().Warn("Steam Guard one-operation unlock failed",
				"steamId64", strings.TrimSpace(steamID64), "reason", unlockFailureReason(operationErr), "error", operationErr)
			return CodeView{}, operationErr
		}
		return view, nil
	}
	return s.codeLocked(v, steamID64)
}

func (s *Service) GetCode(steamID64, token string) (*CodeView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.requireVaultLocked()
	if err != nil {
		return nil, err
	}
	if err := s.authorizeModalLocked(v, steamID64, token); err != nil {
		return nil, err
	}
	if v.IsLocked() {
		return nil, nil
	}
	view, err := s.codeLocked(v, steamID64)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func (s *Service) CopyCode(steamID64, token string) error {
	s.mu.Lock()
	v, err := s.requireVaultLocked()
	if err != nil {
		s.mu.Unlock()
		return err
	}
	if err := s.authorizeModalLocked(v, steamID64, token); err != nil {
		s.mu.Unlock()
		return err
	}
	if v.IsLocked() {
		s.mu.Unlock()
		return vault.ErrLocked
	}
	view, err := s.codeLocked(v, steamID64)
	clipboard := s.clipboard
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if clipboard == nil {
		return secureclipboard.ErrUnavailable
	}
	lifetime := time.Until(time.UnixMilli(view.ExpiresAt))
	if lifetime > secureclipboard.MaxClipboardLife {
		lifetime = secureclipboard.MaxClipboardLife
	}
	err = clipboard.Copy(view.Code, lifetime)
	view.Code = ""
	return err
}

// ListAccounts returns every vault record, anchored on one the caller already
// holds a capability for: knowing a SteamID64 the vault has is the price of
// reading the rest. An anchor the vault does not hold - a pending add attempt,
// or an account still on its setup page - is refused as an invalid capability
// even when the capability itself is sound, so the two cases are logged apart.
func (s *Service) ListAccounts(accountID, token string) ([]AccountSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.requireVaultLocked()
	if err != nil {
		return nil, err
	}
	if err := s.authorizeModalLocked(v, accountID, token); err != nil {
		return nil, err
	}
	records, err := v.List()
	if err != nil {
		return nil, err
	}
	result := make([]AccountSummary, 0, len(records))
	anchorFound := false
	// One instant for the whole list, so two rows whose tokens lapse in the same
	// second cannot disagree about which side of the boundary they fell.
	now := time.Now()
	for _, record := range records {
		if record.SteamID64 == accountID {
			anchorFound = true
		}
		loaded, err := recordFromVault(v, record.ID)
		if err != nil {
			// One unreadable record must not blank the whole picker: a single
			// half-finished enrollment would otherwise hide every other account.
			serviceLogger().Warn("skipping unreadable vault record",
				"steamId64", record.SteamID64, "error", err)
			continue
		}
		result = append(result, AccountSummary{
			SteamID64:     record.SteamID64,
			AccountName:   loaded.AccountName(),
			Kind:          summaryKind(loaded.Kind),
			SessionStatus: localSessionStatus(loaded.Kind, loaded.AccessToken(), loaded.RefreshToken(), now),
		})
		loaded.destroy()
	}
	if !anchorFound {
		// Deliberately the same error as a bad capability: telling the two apart
		// would answer "is this account in the vault?" for any id a capability
		// can be minted for, which is every numeric one.
		serviceLogger().Warn("refusing to list accounts: the anchor is not a vault record",
			"accountId", accountID)
		return nil, capability.ErrInvalidCapability
	}
	// The vault is the only place a restored account's login name exists, and it
	// has just been read. Without this the switcher shows those accounts nameless
	// until Steam answers with a community name, and login-only records - which
	// may have no Steam profile to answer with - stay nameless indefinitely.
	for _, summary := range result {
		s.rememberSteamAccount(summary.SteamID64, summary.AccountName)
	}
	return result, nil
}

// RegisterAccountNameResolver wires the vault into the switcher's lookup for an
// account it knows only by SteamID64. Package-level rather than a method, so it
// does not become a bound frontend call.
func RegisterAccountNameResolver(s *Service) {
	steam.RegisterAccountNameResolver(s.accountNameForSteamID)
}

// RegisterForgetHandler wires the vault into the switcher's Forget, so an account
// that leaves the list leaves its session-only Steam Guard record behind with it -
// and so forgetting an account that has an authenticator is refused rather than
// half-done. Package-level for the same reason as the resolver above.
func RegisterForgetHandler(s *Service) {
	steam.RegisterSteamGuardForgetHandler(s.forgetLoginOnlyRecord)
}

// accountNameForSteamID reads the login name out of the vault. Only a name: the
// same string loginusers.vdf holds in plain text for every other account. An
// unopened or locked vault simply has no answer.
func (s *Service) accountNameForSteamID(steamID64 string) (string, bool) {
	steamID64 = strings.TrimSpace(steamID64)
	if steamID64 == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, exists, err := s.openVaultLocked()
	if err != nil || !exists || v.IsLocked() {
		return "", false
	}
	records, err := v.List()
	if err != nil {
		return "", false
	}
	for _, record := range records {
		if record.SteamID64 != steamID64 {
			continue
		}
		loaded, err := recordFromVault(v, record.ID)
		if err != nil {
			return "", false
		}
		name := strings.TrimSpace(loaded.AccountName())
		loaded.destroy()
		return name, name != ""
	}
	return "", false
}

func summaryKind(kind vaultrecord.Kind) AccountKind {
	switch kind {
	case vaultrecord.KindLoginOnly:
		return AccountKindLoginOnly
	case vaultrecord.KindEnrollmentPending:
		return AccountKindPending
	default:
		return AccountKindAuthenticator
	}
}

func (s *Service) ChangePassword(currentPassword, newPassword, appPassword string) error {
	return s.ChangePasswordWithFactors(currentPassword, newPassword, appPassword, "", "")
}

// ChangePasswordWithFactors changes the vault password on a vault whose slots
// need more than a password. The other factors are unavoidable here: a slot's
// key is derived from every factor it lists, so rebuilding the password part
// still needs the keyfile that sits beside it.
func (s *Service) ChangePasswordWithFactors(currentPassword, newPassword, appPassword, keyfilePath, backupKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(newPassword) == "" {
		return vault.ErrInvalidPassword
	}
	securityStatus, err := security.GetStatus()
	if err != nil {
		return err
	}
	if securityStatus.SavedAccountDataEncrypted {
		if appPassword == "" {
			return ErrAppPassword
		}
		if err := security.VerifyAppPassword(appPassword); err != nil {
			return err
		}
		if samePassword(newPassword, appPassword) {
			return ErrPasswordReuse
		}
	}
	v, err := s.requireVaultLocked()
	if err != nil {
		return err
	}
	oldCreds, err := buildVaultCredentials(currentPassword, keyfilePath, backupKey)
	if err != nil {
		return err
	}
	defer wipe(oldCreds.Keyfile)
	defer wipe(oldCreds.RecoveryCode)
	// A closure, because the secret may be filled in below and the argument to a
	// plain defer is evaluated where it is written.
	defer func() { wipe(oldCreds.SecurityKey) }()
	if err := s.unlockWithSecurityKeyFallbackLocked(v, &oldCreds); err != nil {
		return err
	}
	// A way in that pairs the password with a security key can only be re-keyed
	// with the device present, and the unlock above will not have asked for it if
	// the password alone opened some other slot. Asked for explicitly here, so
	// the change succeeds instead of being refused for a factor the user is
	// holding but was never prompted for.
	if len(oldCreds.SecurityKey) == 0 && pairsPasswordWithSecurityKey(v.ListSlots()) {
		if credID, secret, keyErr := s.evaluateSecurityKeyLocked(v); keyErr == nil && len(secret) != 0 {
			oldCreds.SecurityKey = secret
			oldCreds.SecurityKeyID = credID
		}
		// The ceremony above runs with s.mu released, so Lock Now or the app
		// lock can have closed the vault while the user was reaching for their
		// key. Said plainly here rather than surfacing further down as a missing
		// outer key, which names nothing the user can act on.
		if _, err := s.reopenedVaultLocked(); err != nil {
			return err
		}
	}
	if err := v.ChangePasswordWith(oldCreds, newPassword); err != nil {
		return err
	}
	// Cleared only once the change is committed. Clearing it first meant a
	// refused change still told the user their verified backup no longer counted.
	//
	// Best effort from here: the password HAS changed. Returning a bookkeeping
	// error would report a failure the user then acts on by keeping the old
	// password, which no longer opens anything.
	if err := clearVerifiedBackupStamp(); err != nil {
		serviceLogger().Warn("Steam Guard password changed but the verified-backup stamp could not be cleared", "error", err)
	}
	return nil
}

// clearVerifiedBackupStamp forgets the last verified backup, which a password
// change invalidates: the backup still opens with the password it was taken
// under, and that is no longer the vault's.
func clearVerifiedBackupStamp() error {
	settings, err := LoadSettings()
	if err != nil {
		return err
	}
	settings.LastVerifiedBackup = ""
	settings.LastVerifiedBackupPath = ""
	return SaveSettings(settings)
}

func (s *Service) LockNow() error { return s.revokeLeases() }

func (s *Service) OpenFolder() error {
	root, err := VaultFolderPath()
	if err != nil {
		return err
	}
	return platform.OpenPathInFileManager(root)
}

func (s *Service) PickMaFiles() ([]string, error) {
	app := application.Get()
	if app == nil {
		return nil, errors.New("application not initialised")
	}
	dialog := app.Dialog.OpenFile().
		SetTitle("Import Steam Desktop Authenticator maFiles").
		AddFilter("Steam authenticator files", "*.maFile")
	if owner := dialogOwnerWindow(); owner != nil {
		dialog = dialog.AttachToWindow(owner)
	}
	selected, err := dialog.PromptForMultipleSelection()
	logDialogOutcome("import-mafiles", len(selected) > 0, err)
	if err != nil {
		if dialogCancelled(err) {
			return []string{}, nil
		}
		return nil, err
	}
	paths := make([]string, 0, len(selected))
	for _, path := range selected {
		path = strings.TrimSpace(path)
		if path != "" && filepath.IsAbs(path) && strings.EqualFold(filepath.Ext(path), ".maFile") {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

// MaFileExportResult reports where the export went. Path is empty when the user
// cancelled the save dialog. ManifestSkipped means the maFile was written but its
// companion manifest was not, so SDA cannot import it until one is supplied — a
// warning, not a failure, and the two are worth telling apart.
type MaFileExportResult struct {
	Path            string `json:"path"`
	ManifestSkipped bool   `json:"manifestSkipped"`
}

// ExportMaFile writes one account as an SDA maFile. password re-verifies the Steam
// Guard vault before the secret leaves it. maFilePassword is optional: when set,
// the file is encrypted the way SDA encrypts, and a manifest.json carrying the salt
// and IV is written beside it, because SDA reads them from there.
func (s *Service) ExportMaFile(steamID64, password, maFilePassword string, includeSessionTokens bool, token string) (MaFileExportResult, error) {
	steamID64 = strings.TrimSpace(steamID64)
	if steamID64 == "" {
		return MaFileExportResult{}, ErrAccountNotFound
	}
	s.mu.Lock()
	v, err := s.requireVaultLocked()
	if err == nil {
		err = s.authorizeModalLocked(v, steamID64, token)
	}
	s.mu.Unlock()
	if err != nil {
		return MaFileExportResult{}, err
	}
	app := application.Get()
	if app == nil {
		return MaFileExportResult{}, errors.New("application not initialised")
	}
	destination, err := app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:    "Export Steam Desktop Authenticator maFile",
		Filename: steamID64 + ".maFile",
		Filters: []application.FileFilter{{
			DisplayName: "Steam authenticator file",
			Pattern:     "*.maFile",
		}},
		Window: dialogOwnerWindow(),
	}).
		PromptForSingleSelection()
	logDialogOutcome("export-mafile", strings.TrimSpace(destination) != "", err)
	if err != nil {
		if dialogCancelled(err) {
			// Cancel is a clean outcome: an empty path with no error.
			return MaFileExportResult{}, nil
		}
		return MaFileExportResult{}, err
	}
	if strings.TrimSpace(destination) == "" {
		return MaFileExportResult{}, nil
	}
	destination = strings.TrimSpace(destination)
	if !strings.EqualFold(filepath.Ext(destination), ".maFile") {
		destination += ".maFile"
	}
	if !filepath.IsAbs(destination) {
		return MaFileExportResult{}, ErrPathNotAbsolute
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	v, err = s.requireVaultLocked()
	if err != nil {
		return MaFileExportResult{}, err
	}
	if err := s.authorizeModalLocked(v, steamID64, token); err != nil {
		return MaFileExportResult{}, err
	}
	if lockErr := v.Lock(); lockErr != nil {
		serviceLogger().Warn("could not relock Steam Guard vault before export re-authentication", "error", lockErr)
	}
	if err := s.unlockVaultLocked(v, password, false); err != nil {
		serviceLogger().Warn("maFile export re-authentication failed",
			"steamId64", steamID64, "reason", unlockFailureReason(err), "error", err)
		return MaFileExportResult{}, err
	}
	defer func() {
		if lockErr := v.Lock(); lockErr != nil {
			serviceLogger().Warn("could not relock Steam Guard vault after export", "error", lockErr)
		}
	}()
	// The chosen destination is never logged; only the outcome is.
	if err := exportAccountToPath(v, steamID64, destination, includeSessionTokens, maFilePassword); err != nil {
		if errors.Is(err, ErrExportManifestExists) {
			// The maFile itself was written; only its companion manifest was not.
			serviceLogger().Info("maFile exported without a manifest", "steamId64", steamID64,
				"encrypted", true)
			return MaFileExportResult{Path: destination, ManifestSkipped: true}, nil
		}
		serviceLogger().Warn("maFile export failed", "steamId64", steamID64,
			"includeSessionTokens", includeSessionTokens, "error", err)
		return MaFileExportResult{}, err
	}
	serviceLogger().Info("maFile exported", "steamId64", steamID64,
		"includeSessionTokens", includeSessionTokens, "encrypted", maFilePassword != "")
	return MaFileExportResult{Path: destination}, nil
}

// writeLegacyExportManifest writes the manifest.json SDA needs beside an encrypted
// maFile: SDA reads the salt and IV from there, not from the file itself, so an
// encrypted export without it cannot be imported anywhere.
//
// It never overwrites an existing manifest. Exporting into a real SDA maFiles
// folder would otherwise destroy the manifest listing every one of that user's
// accounts, and losing those is unrecoverable.
func writeLegacyExportManifest(destination, steamID64 string, export mafile.LegacyEncryptedExport) error {
	steamID, err := strconv.ParseUint(steamID64, 10, 64)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(filepath.Dir(destination), "manifest.json")
	if _, statErr := os.Stat(manifestPath); statErr == nil {
		serviceLogger().Warn("kept the existing manifest.json beside an encrypted export",
			"steamId64", steamID64)
		return ErrExportManifestExists
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	manifest, err := json.Marshal(map[string]any{
		"encrypted": true,
		"entries": []map[string]any{{
			"filename":        filepath.Base(destination),
			"steamid":         steamID,
			"encryption_salt": export.Salt,
			"encryption_iv":   export.IV,
		}},
	})
	if err != nil {
		return err
	}
	file, err := securefile.CreateNew(manifestPath)
	if err != nil {
		return err
	}
	if _, err := file.Write(manifest); err != nil {
		_ = file.Close()
		_ = os.Remove(manifestPath)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(manifestPath)
		return err
	}
	return file.Close()
}

func exportAccountToPath(v *vault.Vault, steamID64, destination string, includeSessionTokens bool, maFilePassword string) error {
	records, err := v.List()
	if err != nil {
		return err
	}
	var recordID string
	for _, record := range records {
		if record.SteamID64 == steamID64 {
			recordID = record.ID
			break
		}
	}
	if recordID == "" {
		return ErrAccountNotFound
	}
	account, err := accountFromRecord(v, recordID)
	if err != nil {
		return err
	}
	options := mafile.ExportOptions{IncludeTokens: includeSessionTokens}
	var body []byte
	var encrypted mafile.LegacyEncryptedExport
	if maFilePassword == "" {
		body, err = mafile.ExportPlaintext(account, options)
	} else {
		encrypted, err = mafile.ExportLegacyEncrypted(account, options, maFilePassword)
		body = encrypted.Body
	}
	if err != nil {
		return err
	}
	defer wipe(body)
	file, err := securefile.CreateNew(destination)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = file.Close()
			_ = os.Remove(destination)
		}
	}()
	if _, err := file.Write(body); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	committed = true
	if maFilePassword == "" {
		return nil
	}
	// The maFile is written and keeps its value even if the manifest cannot be:
	// the caller reports that separately rather than discarding the export.
	return writeLegacyExportManifest(destination, steamID64, encrypted)
}

func (s *Service) ImportPlaintext(paths []string, password string, rememberForSession bool) ([]ImportResult, error) {
	return s.importFiles(paths, password, "", rememberForSession, false)
}

func (s *Service) ImportMaFiles(paths []string, password, legacyPassword string, rememberForSession bool) ([]ImportResult, error) {
	return s.importFiles(paths, password, legacyPassword, rememberForSession, true)
}

func (s *Service) importFiles(paths []string, password, legacyPassword string, rememberForSession, allowLegacy bool) ([]ImportResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	enabled, err := s.featureEnabledLocked()
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, ErrFeatureDisabled
	}
	settings, err := LoadSettings()
	if err != nil {
		return nil, err
	}
	v, err := s.requireVaultLocked()
	if err != nil {
		return nil, err
	}
	rememberForSession = rememberForSession || settings.RememberPasswordForSession
	if v.IsLocked() {
		if err := s.unlockVaultLocked(v, password, rememberForSession); err != nil {
			serviceLogger().Warn("Steam Guard import could not unlock the vault",
				"reason", unlockFailureReason(err), "error", err)
			return nil, err
		}
	} else if rememberForSession {
		if err := v.SetLeaseMode(vault.ProcessLease); err != nil {
			return nil, err
		}
	}
	results := make([]ImportResult, 0, len(paths))
	imported, failed := 0, 0
	defer func() {
		serviceLogger().Info("Steam Guard import finished",
			"legacy", allowLegacy, "requested", len(paths), "imported", imported, "failed", failed)
	}()
	for _, source := range paths {
		result := ImportResult{Path: source}
		var steamID64, accountName string
		var discarded []string
		var importErr error
		if allowLegacy {
			steamID64, accountName, discarded, importErr = importMaFile(v, source, legacyPassword)
		} else {
			steamID64, accountName, discarded, importErr = importPlaintextFile(v, source)
		}
		if importErr != nil {
			failed++
			result.ErrorCode = importErrorCode(importErr)
			// The source path is user data; only the error code is recorded.
			serviceLogger().Warn("Steam Guard maFile import failed", "errorCode", result.ErrorCode)
			results = append(results, result)
			continue
		}
		imported++
		result.SteamID64 = steamID64
		result.AccountName = accountName
		result.DiscardedFields = discarded
		result.Imported = true
		result.CapabilityRefreshRequired = true
		if err := registry.Upsert(steamID64, registry.StateActive); err != nil {
			serviceLogger().Warn("Steam Guard registry update failed after import", "steamId64", steamID64, "error", err)
			return results, err
		}
		// Import writes the registration index directly rather than through
		// upsertRegistry, so unlike the enrollment paths this call is the only
		// thing putting the account in the list - an imported maFile is often
		// for an account Steam has never signed in on this machine.
		s.rememberSteamAccount(steamID64, accountName)
		results = append(results, result)
	}
	if imported > 0 {
		s.signalSteamDataRefresh(true)
	}
	return results, nil
}

func (h lifecycleHook) EnableOuter(key []byte, recoveryPassword string) error {
	s := h.service
	s.mu.Lock()
	defer s.mu.Unlock()
	v, exists, err := s.openVaultLocked()
	if err != nil || !exists {
		return err
	}
	if v.HasRecoveryWrapper() {
		return v.EnableOuter(key)
	}
	return v.EnableOuterWithRecovery(key, recoveryPassword)
}

func (h lifecycleHook) DisableOuter(key []byte) error {
	s := h.service
	s.mu.Lock()
	defer s.mu.Unlock()
	v, exists, err := s.openVaultLocked()
	if err != nil || !exists {
		return err
	}
	return v.DisableOuter(key)
}

func (h lifecycleHook) ChangeOuterPassword(oldPassword, newPassword string) error {
	s := h.service
	s.mu.Lock()
	defer s.mu.Unlock()
	v, exists, err := s.openVaultLocked()
	if err != nil || !exists || !v.HasRecoveryWrapper() {
		return err
	}
	return v.ChangeRecoveryPassword(oldPassword, newPassword)
}

func (h lifecycleHook) RevokeLeases() error { return h.service.revokeLeases() }

func (s *Service) revokeLeases() error {
	s.closeAuthenticationManager(false)
	s.cancelQRRegionSelection("")
	s.mu.Lock()
	clipboard := s.clipboard
	qrAttempts := s.qrAttempts
	var vaultErr error
	if s.vault != nil {
		vaultErr = s.vault.Lock()
	}
	// Locking the vault retires the authentication that opened it: the next
	// factor change has to be asked for again.
	s.closeManagementGateLocked()
	s.mu.Unlock()
	s.resetConfirmationSession(false)

	s.contentProtectionMu.Lock()
	hadViews := len(s.contentProtectionLeases) != 0
	clear(s.contentProtectionLeases)
	if s.capabilities != nil {
		s.capabilities.RevokeAll()
	}
	var protectionErr error
	if hadViews {
		if s.setMainContentProtectionFn == nil {
			protectionErr = ErrSensitiveView
		} else {
			protectionErr = s.setMainContentProtectionFn(false)
		}
	}
	emitter := s.emitMainWindowEventFn
	s.contentProtectionMu.Unlock()

	var eventErr error
	if hadViews && emitter != nil {
		eventErr = emitter(SensitiveViewRevokedEvent, nil)
	}
	var clipboardErr error
	if clipboard != nil {
		_, clipboardErr = clipboard.Clear()
	}
	var qrAttemptErr error
	if qrAttempts != nil {
		qrAttemptErr = qrAttempts.RevokeAll()
	}
	return errors.Join(vaultErr, protectionErr, eventErr, clipboardErr, qrAttemptErr)
}

// adoptVaultWithoutSettingsLocked turns the integration on when a vault is
// present but no Steam Guard settings file is.
//
// The flag lives in Settings/SteamGuard.json, outside the SteamGuard folder, so
// copying that folder in - or restoring it by any route other than
// RestoreVerifiedBackup, which sets the flag itself - leaves a vault full of
// accounts that every call then refuses to touch. A vault existing is the
// user's opt-in; there is no decision to override, because no settings file
// means no decision was ever recorded. One that says false is left alone.
func (s *Service) adoptVaultWithoutSettingsLocked() (bool, error) {
	if settingsFileExists() {
		return false, nil
	}
	_, exists, err := s.openVaultLocked()
	if err != nil || !exists {
		return false, err
	}
	settings, err := LoadSettings()
	if err != nil {
		return false, err
	}
	settings.FeatureEnabled = true
	if err := SaveSettings(settings); err != nil {
		return false, err
	}
	serviceLogger().Info("adopted a Steam Guard vault that arrived without settings")
	return true, nil
}

// featureEnabledLocked reports whether the integration may run, adopting a
// vault that arrived without settings rather than refusing it.
func (s *Service) featureEnabledLocked() (bool, error) {
	settings, err := LoadSettings()
	if err != nil {
		return false, err
	}
	if settings.FeatureEnabled {
		return true, nil
	}
	return s.adoptVaultWithoutSettingsLocked()
}

func (s *Service) requireVaultLocked() (*vault.Vault, error) {
	enabled, err := s.featureEnabledLocked()
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, ErrFeatureDisabled
	}
	v, exists, err := s.openVaultLocked()
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrVaultNotReady
	}
	return v, nil
}

// backupKDFParams reports the cost applied to a verified backup's own header.
// The zero value means the shipped profile, so only tests need to set it.
func (s *Service) backupKDFParams() vault.KDFParams {
	if s.backupKDF.Algorithm == "" {
		return vault.BackupKDFParams()
	}
	return s.backupKDF
}

// liveKDFParams reports the cost a routinely unlocked vault carries. Restoring
// a backup rekeys down to this.
func (s *Service) liveKDFParams() vault.KDFParams {
	if s.liveKDF.Algorithm == "" {
		return vault.DefaultKDFParams()
	}
	return s.liveKDF
}

func (s *Service) openVaultLocked() (*vault.Vault, bool, error) {
	if s.vault != nil {
		return s.vault, true, nil
	}
	// A restore builds the live vault folder in place and deletes it again if any
	// later step fails. It can release s.mu in between - a security key enrolled
	// in the backup has to be asked - and opening the half-built folder in that
	// window would cache a vault this process then keeps forever, over a
	// directory that no longer exists.
	if s.restoreInProgress {
		return nil, false, ErrRestoreInProgress
	}
	root, err := VaultFolderPath()
	if err != nil {
		return nil, false, err
	}
	v, err := vault.Open(root, s.vaultOptions...)
	if errors.Is(err, vault.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	s.vault = v
	return v, true, nil
}

func appOuterKeyForRecovery(appPassword string) ([]byte, error) {
	status, err := security.GetStatus()
	if err != nil || !status.SavedAccountDataEncrypted {
		return nil, err
	}
	if appPassword == "" {
		return nil, ErrAppPassword
	}
	if err := security.VerifyAppPassword(appPassword); err != nil {
		return nil, err
	}
	return security.DeriveSteamGuardOuterKey()
}

func (s *Service) unlockVaultLocked(v *vault.Vault, password string, remember bool) error {
	return s.unlockVaultWithLocked(v, vault.PasswordOnly(password), remember)
}

func (s *Service) unlockVaultWithLocked(v *vault.Vault, creds vault.Credentials, remember bool) error {
	if !remember {
		settings, err := LoadSettings()
		if err != nil {
			return err
		}
		remember = settings.RememberPasswordForSession
	}
	mode := vault.FixedLease
	if remember {
		mode = vault.ProcessLease
	}
	status, err := security.GetStatus()
	if err != nil {
		return err
	}
	if !status.SavedAccountDataEncrypted {
		unlockErr := retainedUnlockError(v.UnlockWith(creds, mode))
		if unlockErr == nil {
			s.signalSteamDataRefresh(false)
		}
		return unlockErr
	}
	key, err := security.DeriveSteamGuardOuterKey()
	if err != nil {
		return err
	}
	defer security.WipeSecret(key)
	unlockErr := retainedUnlockError(v.UnlockWithFactorsAndOuter(creds, key, mode))
	if unlockErr == nil {
		s.signalSteamDataRefresh(false)
	}
	return unlockErr
}

// unlockFailureReason separates a wrong password from vault, secure-memory and
// app-password problems. It never sees the password itself.
func unlockFailureReason(err error) string {
	switch {
	case errors.Is(err, vault.ErrInvalidPassword):
		return "invalid_password"
	case errors.Is(err, vault.ErrInvalidOuterKey), errors.Is(err, vault.ErrOuterKeyRequired):
		return "outer_key"
	case errors.Is(err, vault.ErrLocked), errors.Is(err, vault.ErrLeaseExpired):
		return "vault_locked"
	case errors.Is(err, vault.ErrInvalidFormat):
		return "vault_format"
	case errors.Is(err, vault.ErrSecureMemory), errors.Is(err, vault.ErrOneOperationRequired),
		errors.Is(err, vault.ErrOneOperationExpired):
		return "secure_memory"
	case errors.Is(err, ErrAppPassword):
		return "app_password"
	default:
		return "vault_error"
	}
}

func retainedUnlockError(err error) error {
	if errors.Is(err, vault.ErrOneOperationRequired) {
		return errors.Join(ErrRetainedUnlockUnavailable, err)
	}
	return err
}

func (s *Service) withOneOperationLocked(v *vault.Vault, password string, fn func(*vault.OneOperationAccess) error) error {
	return s.withOneOperationCredentialsLocked(v, vault.PasswordOnly(password), fn)
}

func (s *Service) withOneOperationCredentialsLocked(v *vault.Vault, creds vault.Credentials, fn func(*vault.OneOperationAccess) error) error {
	status, err := security.GetStatus()
	if err != nil {
		return err
	}
	if !status.SavedAccountDataEncrypted {
		return v.WithOneOperationCredentials(creds, fn)
	}
	key, err := security.DeriveSteamGuardOuterKey()
	if err != nil {
		return err
	}
	defer security.WipeSecret(key)
	return v.WithOneOperationCredentialsAndOuter(creds, key, fn)
}

func (s *Service) authorizeModalLocked(v *vault.Vault, accountID, token string) error {
	accountID = strings.TrimSpace(accountID)
	if s.capabilities == nil || accountID == "" || token == "" {
		return capability.ErrInvalidCapability
	}
	binding, err := s.capabilities.Resolve(token)
	if err != nil {
		return err
	}
	expected := capability.Binding{
		WindowName:      mainWindowName,
		AccountID:       accountID,
		Scope:           modalCapabilityScope,
		LeaseID:         binding.LeaseID,
		VaultGeneration: v.Generation(),
	}
	if err := s.capabilities.Validate(expected, token); err != nil {
		return err
	}
	s.contentProtectionMu.Lock()
	lease, ok := s.contentProtectionLeases[binding.LeaseID]
	s.contentProtectionMu.Unlock()
	if !ok || lease.binding != expected {
		return capability.ErrInvalidCapability
	}
	return nil
}

func (s *Service) codeLocked(v *vault.Vault, steamID64 string) (CodeView, error) {
	return s.codeFromReader(v, steamID64, UnlockPersistenceCached)
}

type accountRecordReader interface {
	ListRecords() ([]vault.RecordInfo, error)
	GetRecord(string) ([]byte, error)
}

func (s *Service) codeFromReader(reader accountRecordReader, steamID64 string, persistence UnlockPersistence) (CodeView, error) {
	steamID64 = strings.TrimSpace(steamID64)
	records, err := reader.ListRecords()
	if err != nil {
		return CodeView{}, err
	}
	for _, record := range records {
		if record.SteamID64 != steamID64 {
			continue
		}
		account, err := accountFromReader(reader, record.ID)
		if err != nil {
			return CodeView{}, err
		}
		now, freshness := s.timeState.Now()
		code, err := otp.Generate(account.SharedSecret, now)
		if err != nil {
			return CodeView{}, err
		}
		return CodeView{
			SteamID64:         steamID64,
			AccountName:       account.AccountName,
			Code:              code.Value,
			ExpiresAt:         time.Now().Add(code.ExpiresAt.Sub(now)).UnixMilli(),
			TimeStatus:        timeStatus(freshness),
			UnlockPersistence: persistence,
		}, nil
	}
	return CodeView{}, ErrAccountNotFound
}

func accountFromRecord(v *vault.Vault, recordID string) (mafile.Account, error) {
	return accountFromReader(v, recordID)
}

// accountFromReader decrypts a record that must hold an authenticator.
//
// The vault stores three record shapes. Every caller of this function needs a
// shared or identity secret, so the other two shapes are turned into errors here
// rather than at each call site - that way code generation, confirmations, QR
// approval and maFile export all decline a login-only account correctly without
// any of them knowing the kind exists.
func accountFromReader(reader accountRecordReader, recordID string) (mafile.Account, error) {
	raw, err := reader.GetRecord(recordID)
	if err != nil {
		return mafile.Account{}, err
	}
	defer wipe(raw)
	switch vaultrecord.Sniff(raw) {
	case vaultrecord.KindLoginOnly:
		return mafile.Account{}, ErrNotAuthenticator
	case vaultrecord.KindEnrollmentPending:
		return mafile.Account{}, ErrRecordPending
	}
	parsed, err := mafile.ParsePlaintext(raw)
	if err != nil {
		return mafile.Account{}, errors.Join(ErrInvalidImport, err)
	}
	return parsed.Account, nil
}

func importPlaintextFile(v *vault.Vault, source string) (string, string, []string, error) {
	raw, err := readImportFile(source)
	if err != nil {
		return "", "", nil, err
	}
	defer wipe(raw)
	parsed, err := mafile.ParsePlaintext(raw)
	if err != nil {
		return "", "", nil, ErrInvalidImport
	}
	return commitImportedAccount(v, parsed)
}

func importMaFile(v *vault.Vault, source, legacyPassword string) (string, string, []string, error) {
	raw, err := readImportFile(source)
	if err != nil {
		return "", "", nil, err
	}
	defer wipe(raw)
	parsed, plainErr := mafile.ParsePlaintext(raw)
	if plainErr == nil {
		return commitImportedAccount(v, parsed)
	}
	manifestPath := filepath.Join(filepath.Dir(source), "manifest.json")
	manifest, err := readBoundedRegularFile(manifestPath, mafile.MaxInputBytes)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil, ErrLegacyManifest
		}
		return "", "", nil, err
	}
	defer wipe(manifest)
	parsed, err = mafile.ImportLegacyEncrypted(raw, manifest, filepath.Base(source), legacyPassword)
	if err != nil {
		return "", "", nil, err
	}
	return commitImportedAccount(v, parsed)
}

func readImportFile(source string) ([]byte, error) {
	if !filepath.IsAbs(source) {
		return nil, ErrPathNotAbsolute
	}
	if !strings.EqualFold(filepath.Ext(source), ".maFile") {
		return nil, ErrUnsupportedInput
	}
	return readBoundedRegularFile(source, mafile.MaxInputBytes)
}

func readBoundedRegularFile(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxBytes {
		return nil, ErrInvalidImport
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil || int64(len(raw)) > maxBytes {
		wipe(raw)
		return nil, ErrInvalidImport
	}
	return raw, nil
}

func commitImportedAccount(v *vault.Vault, parsed mafile.ParseResult) (string, string, []string, error) {
	if parsed.Account.Session == nil || parsed.Account.Session.SteamID == 0 {
		return "", "", nil, ErrInvalidImport
	}
	steamID64 := strconv.FormatUint(parsed.Account.Session.SteamID, 10)
	canonical, err := mafile.ExportPlaintext(parsed.Account, mafile.ExportOptions{IncludeTokens: true})
	if err != nil {
		return "", "", nil, ErrInvalidImport
	}
	defer wipe(canonical)
	if _, err := v.PutRecord(steamID64, canonical); err != nil {
		return "", "", nil, err
	}
	return steamID64, parsed.Account.AccountName, parsed.DiscardedFields, nil
}

func importErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrPathNotAbsolute):
		return "path_not_absolute"
	case errors.Is(err, ErrUnsupportedInput):
		return "unsupported_input"
	case errors.Is(err, ErrInvalidImport):
		return "invalid_mafile"
	case errors.Is(err, ErrLegacyManifest):
		return "legacy_manifest_required"
	case errors.Is(err, mafile.ErrWrongPasswordOrCorruptSource):
		return "legacy_wrong_password_or_corrupt"
	default:
		return "read_failed"
	}
}

func timeStatus(freshness otp.Freshness) string {
	switch freshness {
	case otp.FreshnessFresh:
		return "fresh"
	case otp.FreshnessStale:
		return "stale"
	case otp.FreshnessUntrusted:
		return "untrusted"
	default:
		return "unavailable"
	}
}

func wipe(value []byte) {
	for i := range value {
		value[i] = 0
	}
}

func samePassword(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

var _ security.SteamGuardLifecycleHook = lifecycleHook{}
