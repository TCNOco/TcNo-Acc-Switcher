package steamguard

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"TcNo-Acc-Switcher/internal/steamguard/authflow"
	"TcNo-Acc-Switcher/internal/steamguard/capability"
	"TcNo-Acc-Switcher/internal/steamguard/enrollmentapi"
	"TcNo-Acc-Switcher/internal/steamguard/enrollmentflow"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
	"TcNo-Acc-Switcher/internal/steamguard/registry"
	"TcNo-Acc-Switcher/internal/steamguard/sessionrefresh"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

var (
	ErrSteamAuthenticationPurpose  = errors.New("invalid Steam authentication purpose")
	ErrSteamAuthenticationState    = errors.New("Steam authentication session is unavailable")
	ErrSteamOperationBusy          = errors.New("another Steam account operation is in progress")
	ErrRevocationAcknowledgment    = errors.New("Steam Guard revocation code acknowledgment is required")
	ErrRevocationViewAlreadyIssued = errors.New("Steam Guard revocation code was already revealed for this view")
)

// authflowLogger covers the credential-login and enrollment flows. It records
// stable failure kinds and states only; never passwords, codes or tokens.
func authflowLogger() *slog.Logger {
	return slog.Default().With("component", "steamguard.authflow")
}

// authflowFailureKind extracts authflow's stable classification so a collapsed
// error still names its cause in the log.
func authflowFailureKind(err error) string {
	var flowErr *authflow.Error
	if errors.As(err, &flowErr) && flowErr != nil {
		return string(flowErr.Kind)
	}
	return "unknown"
}

// logAuthflowFailure logs conflicts at Warn (they block the user until the stale
// session expires) and every other kind at Info.
func logAuthflowFailure(operation, accountID string, err error) {
	kind := authflowFailureKind(err)
	log := authflowLogger()
	attributes := []any{"operation", operation, "steamId64", accountID, "kind", kind, "error", err}
	// The HTTP status and Steam result name the actual refusal; without them a
	// protocol failure is indistinguishable from a transient one in the log.
	var flowErr *authflow.Error
	if errors.As(err, &flowErr) && flowErr != nil {
		if flowErr.StatusCode != 0 {
			attributes = append(attributes, "status", flowErr.StatusCode)
		}
		if flowErr.HasEResult {
			attributes = append(attributes, "eresult", flowErr.EResult)
		}
	}
	if kind == string(authflow.ErrorConflict) {
		log.Warn("Steam credential flow conflict", attributes...)
		return
	}
	log.Info("Steam credential flow step failed", attributes...)
}

type SteamAuthPurpose string

const (
	SteamAuthPurposeLoginAgain       SteamAuthPurpose = "login_again"
	SteamAuthPurposeAddAuthenticator SteamAuthPurpose = "add_authenticator"
)

type SteamLoginResult struct {
	State                     string `json:"state"`
	RefreshTokenRenewed       bool   `json:"refreshTokenRenewed"`
	CapabilityRefreshRequired bool   `json:"capabilityRefreshRequired"`
	RegistryUpdated           bool   `json:"registryUpdated"`
}

type SteamCredentialResult struct {
	Handle                    string                 `json:"handle,omitempty"`
	State                     string                 `json:"state"`
	Challenges                []string               `json:"challenges,omitempty"`
	CanSubmitEmailCode        bool                   `json:"canSubmitEmailCode"`
	CanSubmitDeviceCode       bool                   `json:"canSubmitDeviceCode"`
	CanPoll                   bool                   `json:"canPoll"`
	PollAfterMillis           int64                  `json:"pollAfterMillis,omitempty"`
	ExpiresAtUnix             int64                  `json:"expiresAtUnix,omitempty"`
	Outcome                   string                 `json:"outcome,omitempty"`
	Enrollment                *SteamEnrollmentStatus `json:"enrollment,omitempty"`
	CapabilityRefreshRequired bool                   `json:"capabilityRefreshRequired"`
	RegistryUpdated           bool                   `json:"registryUpdated"`
}

type SteamEnrollmentStatus struct {
	State                     string `json:"state"`
	Confirmation              string `json:"confirmation"`
	PhoneHint                 string `json:"phoneHint,omitempty"`
	RetryAfterSeconds         int64  `json:"retryAfterSeconds,omitempty"`
	HasRetryAfter             bool   `json:"hasRetryAfter,omitempty"`
	Pending                   bool   `json:"pending"`
	Resumed                   bool   `json:"resumed,omitempty"`
	RevocationViewAvailable   bool   `json:"revocationViewAvailable,omitempty"`
	CapabilityRefreshRequired bool   `json:"capabilityRefreshRequired"`
	RegistryUpdated           bool   `json:"registryUpdated"`
}

// SteamRevocationView is the only enrollment DTO that intentionally contains a
// recovery secret. Each live UI capability may receive the value once.
type SteamRevocationView struct {
	Code                      string `json:"code"`
	CapabilityRefreshRequired bool   `json:"capabilityRefreshRequired"`
}

func (SteamRevocationView) String() string   { return "Steam Guard revocation view (secret redacted)" }
func (SteamRevocationView) GoString() string { return "Steam Guard revocation view (secret redacted)" }

type steamCredentialAuthManager interface {
	Begin(context.Context, authflow.Binding, protocol.PasswordCredentialsRequest, []byte) (authflow.Status, error)
	SubmitCode(context.Context, authflow.Binding, string, authflow.Challenge, []byte) (authflow.Status, error)
	Poll(context.Context, authflow.Binding, string) (authflow.Status, error)
	Cancel(authflow.Binding, string) error
	Consume(authflow.Binding, string, authflow.Consumer) error
	Close()
}

type steamEnrollmentManager interface {
	Start(context.Context, enrollmentflow.StartRequest) (enrollmentflow.Status, error)
	Resume(uint64) (enrollmentflow.Status, error)
	RevealRevocationCode(uint64) (enrollmentflow.RevocationView, error)
	AcknowledgeRevocationCode(uint64, []byte) (enrollmentflow.Status, error)
	Finalize(context.Context, enrollmentflow.FinalizeRequest) (enrollmentflow.Status, error)
	Cancel(uint64) error
}

type steamSessionRefresher interface {
	Refresh(context.Context, uint64) (sessionrefresh.Result, error)
}

type steamAuthOperation struct {
	binding authflow.Binding
	purpose SteamAuthPurpose
}

type revocationAcknowledgment struct {
	accountID    string
	generation   string
	capabilityID string
	digest       [sha256.Size]byte
	ready        bool
}

type steamBoundOperation struct {
	id     uint64
	cancel context.CancelFunc
}

// UnlockSteamGuardVault unlocks the vault for an account that does not have an
// active authenticator record yet. The password is not retained by this layer.
func (s *Service) UnlockSteamGuardVault(accountID, password string, rememberForSession bool, token string) error {
	passwordBytes := []byte(password)
	password = ""
	defer wipe(passwordBytes)
	defer runtime.KeepAlive(password)
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.requireVaultLocked()
	if err != nil {
		return err
	}
	if err := s.authorizeModalLocked(v, strings.TrimSpace(accountID), token); err != nil {
		return err
	}
	if err := s.unlockVaultLocked(v, string(passwordBytes), rememberForSession); err != nil {
		serviceLogger().Warn("Steam Guard vault unlock failed",
			"steamId64", strings.TrimSpace(accountID), "reason", unlockFailureReason(err), "error", err)
		return err
	}
	return nil
}

func (s *Service) LoginAgain(accountID, token string) (SteamLoginResult, error) {
	v, _, steamID, err := s.authorizeSteamFlow(accountID, token)
	if err != nil {
		return SteamLoginResult{}, err
	}
	if s.newSessionRefresher == nil {
		return SteamLoginResult{}, ErrSteamAuthenticationState
	}
	ctx, finish, err := s.startBoundSteamOperation(accountID)
	if err != nil {
		return SteamLoginResult{}, err
	}
	defer finish()
	result, err := s.newSessionRefresher(v).Refresh(ctx, steamID)
	if err != nil {
		if reason, ok := refreshFailureReason(err); ok {
			// Not a failure the UI should report as an error: the stored session
			// cannot produce a working access token, so the user must sign in
			// again. Every one of these classes aborts before the vault write in
			// sessionrefresh.Refresh, so no capability was invalidated and
			// CapabilityRefreshRequired stays false.
			serviceLogger().Info("Steam session refresh needs re-authentication", "reason", reason)
			return SteamLoginResult{State: "reauthentication_required", CapabilityRefreshRequired: false}, nil
		}
		serviceLogger().Warn("Steam session refresh failed", "error", err)
		return SteamLoginResult{}, err
	}
	return SteamLoginResult{
		State:                     "refreshed",
		RefreshTokenRenewed:       result.RefreshTokenRenewed,
		CapabilityRefreshRequired: true,
		RegistryUpdated:           s.upsertRegistry(accountID, registry.StateActive),
	}, nil
}

// refreshFailureReason reports whether a refresh failure means "the stored
// session cannot be renewed" — the outcome the UI answers with the credential
// form — and returns a stable log value for it. Infrastructure failures (vault,
// persistence, cancellation, timeouts) return false and stay errors. The value
// never includes token material.
func refreshFailureReason(err error) (string, bool) {
	switch {
	case errors.Is(err, sessionrefresh.ErrNoRefreshToken):
		return "no_refresh_token", true
	case errors.Is(err, sessionrefresh.ErrRemote):
		return "remote_rejected", true
	case errors.Is(err, sessionrefresh.ErrInvalidResponse):
		// Steam answered, but with nothing usable — an expired refresh token
		// looks exactly like this. Signing in again is the only way forward.
		return "invalid_token_response", true
	default:
		return "", false
	}
}

func (s *Service) BeginCredentialLogin(accountID, token, accountName, password string, purpose SteamAuthPurpose) (SteamCredentialResult, error) {
	passwordBytes := []byte(password)
	password = ""
	defer wipe(passwordBytes)
	defer runtime.KeepAlive(password)
	if !validSteamAuthPurpose(purpose) || !validAccountName(accountName) || len(passwordBytes) == 0 {
		// Rejected before any Steam call: without this line a mistyped purpose or
		// an empty field looks like the Sign in button doing nothing.
		authflowLogger().Warn("Steam credential login request rejected", "steamId64", strings.TrimSpace(accountID),
			"purpose", string(purpose), "validPurpose", validSteamAuthPurpose(purpose),
			"validAccountName", validAccountName(accountName), "hasPassword", len(passwordBytes) > 0)
		return SteamCredentialResult{}, ErrSteamAuthenticationPurpose
	}
	_, binding, _, err := s.authorizeSteamFlow(accountID, token)
	if err != nil {
		return SteamCredentialResult{}, err
	}
	manager, epoch, err := s.authenticationManager()
	if err != nil {
		return SteamCredentialResult{}, err
	}
	ctx, finish, err := s.startBoundSteamOperation(accountID)
	if err != nil {
		return SteamCredentialResult{}, err
	}
	defer finish()
	authflowLogger().Debug("beginning Steam credential login", "steamId64", accountID, "purpose", string(purpose))
	status, err := manager.Begin(ctx, binding, passwordAuthRequest(accountName), passwordBytes)
	if err != nil {
		logAuthflowFailure("begin", accountID, err)
		return SteamCredentialResult{}, err
	}
	if _, currentBinding, _, authorizeErr := s.authorizeSteamFlow(accountID, token); authorizeErr != nil || currentBinding != binding {
		s.cancelAuthflowSession(manager, binding, status.Handle, "begin-reauthorize")
		if authorizeErr != nil {
			return SteamCredentialResult{}, authorizeErr
		}
		return SteamCredentialResult{}, ErrSteamAuthenticationState
	}
	s.authStateMu.Lock()
	if s.authShutdown || s.authManager != manager || s.authManagerEpoch != epoch {
		s.authStateMu.Unlock()
		s.cancelAuthflowSession(manager, binding, status.Handle, "begin-manager-replaced")
		return SteamCredentialResult{}, ErrSteamAuthenticationState
	}
	if s.authOperations == nil {
		s.authOperations = make(map[string]steamAuthOperation)
	}
	s.authOperations[status.Handle] = steamAuthOperation{binding: binding, purpose: purpose}
	s.authStateMu.Unlock()
	authflowLogger().Info("Steam credential login started",
		"steamId64", accountID, "purpose", string(purpose), "state", string(status.State))
	return credentialResult(status), nil
}

// cancelAuthflowSession replaces the discarded Cancel results at the abort paths
// so a failed cleanup is visible.
func (s *Service) cancelAuthflowSession(manager steamCredentialAuthManager, binding authflow.Binding, handle, reason string) {
	if err := manager.Cancel(binding, handle); err != nil {
		authflowLogger().Warn("Steam credential session cleanup failed",
			"reason", reason, "steamId64", binding.AccountID, "kind", authflowFailureKind(err), "error", err)
	}
}

func (s *Service) SubmitCredentialCode(accountID, token, handle, challenge, code string) (SteamCredentialResult, error) {
	codeBytes := []byte(code)
	code = ""
	defer wipe(codeBytes)
	defer runtime.KeepAlive(code)
	challengeType, ok := submittedAuthChallenge(challenge)
	if !ok {
		return SteamCredentialResult{}, ErrSteamAuthenticationState
	}
	manager, _, binding, err := s.authenticationOperation(accountID, token, handle)
	if err != nil {
		return SteamCredentialResult{}, err
	}
	ctx, finish, err := s.startBoundSteamOperation(accountID)
	if err != nil {
		return SteamCredentialResult{}, err
	}
	defer finish()
	status, err := manager.SubmitCode(ctx, binding, handle, challengeType, codeBytes)
	if err != nil {
		logAuthflowFailure("submit-code", accountID, err)
		return SteamCredentialResult{}, err
	}
	authflowLogger().Info("Steam credential challenge submitted",
		"steamId64", accountID, "challenge", string(challengeType), "state", string(status.State))
	return credentialResult(status), nil
}

func (s *Service) PollCredentialLogin(accountID, token, handle string) (SteamCredentialResult, error) {
	manager, operation, binding, err := s.authenticationOperation(accountID, token, handle)
	if err != nil {
		return SteamCredentialResult{}, err
	}
	ctx, finish, err := s.startBoundSteamOperation(accountID)
	if err != nil {
		return SteamCredentialResult{}, err
	}
	defer finish()
	status, err := manager.Poll(ctx, binding, handle)
	if err != nil {
		logAuthflowFailure("poll", accountID, err)
		return SteamCredentialResult{}, err
	}
	result := credentialResult(status)
	if status.State != authflow.StateAuthorizedReady {
		authflowLogger().Debug("Steam credential login still pending", "steamId64", accountID, "state", string(status.State))
		return result, nil
	}

	v, currentBinding, steamID, err := s.authorizeSteamFlow(accountID, token)
	if err != nil || currentBinding != binding {
		s.cancelAuthflowSession(manager, binding, handle, "poll-reauthorize")
		if err != nil {
			return SteamCredentialResult{}, err
		}
		return SteamCredentialResult{}, ErrSteamAuthenticationState
	}
	var enrollmentStatus *SteamEnrollmentStatus
	// authflow sanitizes whatever the consumer returns, because a consumer error
	// can carry vault paths or tokens. Sentinels with nothing sensitive in them
	// are captured here instead, so the user is told "already enrolled" rather
	// than a generic transfer failure that names no cause at all.
	var enrollmentRefusal error
	registryUpdated := false
	consumeErr := manager.Consume(binding, handle, func(authorizedSteamID uint64, accountName, accessToken, refreshToken, guardData []byte, hadRemoteInteraction bool) error {
		defer runtime.KeepAlive(guardData)
		defer runtime.KeepAlive(hadRemoteInteraction)
		if authorizedSteamID != steamID {
			return ErrSteamAuthenticationState
		}
		switch operation.purpose {
		case SteamAuthPurposeLoginAgain:
			// Under the service lock: the poll that gets here ran with it
			// released, and ExportMaFile relocks the vault around its export.
			// A login landing inside that window failed to store the session
			// Steam had just authorised, and the tokens cannot be replayed -
			// the user has to redo the whole credential and 2FA ceremony.
			//
			// Only this branch is covered. The enrollmentflow calls below and in
			// Resume/Acknowledge/Finalize write the vault with s.mu unheld too,
			// and cannot simply be wrapped: they make network calls, and holding
			// the service lock across those is what stopped Lock Now working.
			// Closing that properly means enrollmentflow taking the lock around
			// its own writes.
			if err := s.withServiceLock(func() error {
				return updateMafileSession(v, steamID, accountName, accessToken, refreshToken)
			}); err != nil {
				return err
			}
			registryUpdated = s.upsertRegistry(accountID, registry.StateActive)
			return nil
		case SteamAuthPurposeAddAuthenticator:
			manager := s.enrollmentFlow(v)
			if manager == nil {
				return ErrSteamAuthenticationState
			}
			status, err := manager.Start(ctx, enrollmentflow.StartRequest{
				SteamID: authorizedSteamID, AccessToken: accessToken, AuthenticatorTime: uint64(s.authenticatorTime()),
			})
			if err != nil {
				authflowLogger().Warn("Steam Guard enrollment could not be started", "steamId64", accountID, "error", err)
				if errors.Is(err, enrollmentflow.ErrAlreadyEnrolled) {
					enrollmentRefusal = enrollmentflow.ErrAlreadyEnrolled
				}
				return err
			}
			projected := enrollmentResult(status)
			if status.Pending {
				projected.CapabilityRefreshRequired = true
				projected.RegistryUpdated = s.upsertRegistry(accountID, registry.StatePending)
			}
			enrollmentStatus = &projected
			return nil
		default:
			return ErrSteamAuthenticationPurpose
		}
	})
	s.removeAuthenticationOperation(handle)
	if consumeErr != nil {
		authflowLogger().Warn("Steam credential login could not be applied",
			"steamId64", accountID, "purpose", string(operation.purpose), "error", consumeErr)
		if enrollmentRefusal != nil {
			return SteamCredentialResult{}, enrollmentRefusal
		}
		return SteamCredentialResult{}, consumeErr
	}
	result.Handle = ""
	result.Outcome = "session_updated"
	result.RegistryUpdated = registryUpdated
	result.CapabilityRefreshRequired = true
	if operation.purpose == SteamAuthPurposeAddAuthenticator {
		result.Outcome = "enrollment_not_started"
		result.Enrollment = enrollmentStatus
		if enrollmentStatus != nil && enrollmentStatus.Pending {
			result.Outcome = "enrollment_pending"
			result.CapabilityRefreshRequired = true
		} else {
			result.CapabilityRefreshRequired = false
		}
	}
	authflowLogger().Info("Steam credential login authorized",
		"steamId64", accountID, "purpose", string(operation.purpose), "outcome", result.Outcome)
	return result, nil
}

func (s *Service) CancelCredentialLogin(accountID, token, handle string) error {
	manager, _, binding, err := s.authenticationOperation(accountID, token, handle)
	if err != nil {
		return err
	}
	err = manager.Cancel(binding, handle)
	s.removeAuthenticationOperation(handle)
	if err != nil {
		logAuthflowFailure("cancel", accountID, err)
		return err
	}
	authflowLogger().Debug("Steam credential login cancelled", "steamId64", accountID)
	return nil
}

func (s *Service) ResumeSteamGuardEnrollment(accountID, token string) (SteamEnrollmentStatus, error) {
	v, _, steamID, err := s.authorizeSteamFlow(accountID, token)
	if err != nil {
		return SteamEnrollmentStatus{}, err
	}
	manager := s.enrollmentFlow(v)
	if manager == nil {
		return SteamEnrollmentStatus{}, ErrSteamAuthenticationState
	}
	status, err := manager.Resume(steamID)
	if err != nil {
		if enrollmentflow.IsNoPending(err) {
			authflowLogger().Debug("no pending Steam Guard enrollment to resume", "steamId64", accountID)
			return SteamEnrollmentStatus{State: "not_started", Confirmation: "unknown"}, nil
		}
		authflowLogger().Warn("Steam Guard enrollment could not be resumed", "steamId64", accountID, "error", err)
		return SteamEnrollmentStatus{}, err
	}
	result := enrollmentResult(status)
	result.RegistryUpdated = s.upsertRegistry(accountID, registry.StatePending)
	authflowLogger().Info("Steam Guard enrollment resumed",
		"steamId64", accountID, "state", result.State, "pending", result.Pending)
	return result, nil
}

func (s *Service) RevealSteamGuardRevocationCode(accountID, token string) (SteamRevocationView, error) {
	v, _, steamID, err := s.authorizeSteamFlow(accountID, token)
	if err != nil {
		return SteamRevocationView{}, err
	}
	manager := s.enrollmentFlow(v)
	if manager == nil {
		return SteamRevocationView{}, ErrSteamAuthenticationState
	}
	capabilityID := authCapabilityID(token)
	s.authStateMu.Lock()
	if s.revocationAcknowledgments == nil {
		s.revocationAcknowledgments = make(map[string]revocationAcknowledgment)
	}
	_, alreadyRevealed := s.revocationAcknowledgments[capabilityID]
	reservation := revocationAcknowledgment{
		accountID: accountID, generation: v.Generation(), capabilityID: capabilityID,
	}
	if !alreadyRevealed {
		s.revocationAcknowledgments[capabilityID] = reservation
	}
	s.authStateMu.Unlock()
	if alreadyRevealed {
		authflowLogger().Warn("revocation code was already revealed for this view", "steamId64", accountID)
		return SteamRevocationView{}, ErrRevocationViewAlreadyIssued
	}
	view, err := manager.RevealRevocationCode(steamID)
	if err != nil {
		authflowLogger().Warn("revocation code could not be revealed", "steamId64", accountID, "error", err)
		s.authStateMu.Lock()
		if current, ok := s.revocationAcknowledgments[capabilityID]; ok && current == reservation {
			delete(s.revocationAcknowledgments, capabilityID)
		}
		s.authStateMu.Unlock()
		return SteamRevocationView{}, err
	}
	defer view.Destroy()
	digest := revocationDigest([]byte(view.Code))
	s.authStateMu.Lock()
	current, reserved := s.revocationAcknowledgments[capabilityID]
	if !reserved || current != reservation {
		s.authStateMu.Unlock()
		return SteamRevocationView{}, ErrRevocationAcknowledgment
	}
	reservation.digest = digest
	reservation.ready = true
	s.revocationAcknowledgments[capabilityID] = reservation
	s.authStateMu.Unlock()
	return SteamRevocationView{Code: view.Code, CapabilityRefreshRequired: false}, nil
}

// AcknowledgeSteamGuardRevocationCode persists proof the recovery code was
// written down. It writes to the vault, so the returned status always carries
// CapabilityRefreshRequired.
func (s *Service) AcknowledgeSteamGuardRevocationCode(accountID, token, code string) (SteamEnrollmentStatus, error) {
	codeBytes := []byte(code)
	code = ""
	defer wipe(codeBytes)
	defer runtime.KeepAlive(code)
	v, _, steamID, err := s.authorizeSteamFlow(accountID, token)
	if err != nil {
		return SteamEnrollmentStatus{}, err
	}
	capabilityID := authCapabilityID(token)
	digest := revocationDigest(codeBytes)
	s.authStateMu.Lock()
	ack, ok := s.revocationAcknowledgments[capabilityID]
	s.authStateMu.Unlock()
	if !ok || !ack.ready || ack.accountID != accountID || ack.generation != v.Generation() ||
		subtle.ConstantTimeCompare([]byte(ack.capabilityID), []byte(capabilityID)) != 1 ||
		subtle.ConstantTimeCompare(ack.digest[:], digest[:]) != 1 {
		authflowLogger().Warn("revocation code acknowledgment did not match the revealed code", "steamId64", accountID)
		return SteamEnrollmentStatus{}, ErrRevocationAcknowledgment
	}
	manager := s.enrollmentFlow(v)
	if manager == nil {
		return SteamEnrollmentStatus{}, ErrSteamAuthenticationState
	}
	status, err := manager.AcknowledgeRevocationCode(steamID, codeBytes)
	if err != nil {
		authflowLogger().Warn("revocation code acknowledgment could not be saved", "steamId64", accountID, "error", err)
		return SteamEnrollmentStatus{}, err
	}
	authflowLogger().Info("revocation code acknowledged", "steamId64", accountID)
	s.authStateMu.Lock()
	if current, ok := s.revocationAcknowledgments[capabilityID]; ok && current == ack {
		current.digest = [sha256.Size]byte{}
		delete(s.revocationAcknowledgments, capabilityID)
	}
	s.authStateMu.Unlock()
	result := enrollmentResult(status)
	result.CapabilityRefreshRequired = true
	result.RegistryUpdated = s.upsertRegistry(accountID, registry.StatePending)
	return result, nil
}

func (s *Service) FinalizeSteamGuardEnrollment(accountID, token, confirmationCode string) (SteamEnrollmentStatus, error) {
	codeBytes := []byte(confirmationCode)
	confirmationCode = ""
	defer wipe(codeBytes)
	defer runtime.KeepAlive(confirmationCode)
	v, _, steamID, err := s.authorizeSteamFlow(accountID, token)
	if err != nil {
		return SteamEnrollmentStatus{}, err
	}
	ctx, finish, err := s.startBoundSteamOperation(accountID)
	if err != nil {
		return SteamEnrollmentStatus{}, err
	}
	defer finish()
	manager := s.enrollmentFlow(v)
	if manager == nil {
		return SteamEnrollmentStatus{}, ErrSteamAuthenticationState
	}
	status, err := manager.Finalize(ctx, enrollmentflow.FinalizeRequest{
		SteamID: steamID, ConfirmationCode: codeBytes, AuthenticatorTime: uint64(s.authenticatorTime()),
	})
	if err != nil {
		authflowLogger().Warn("Steam Guard enrollment could not be finalized", "steamId64", accountID, "error", err)
		return SteamEnrollmentStatus{}, err
	}
	result := enrollmentResult(status)
	result.CapabilityRefreshRequired = true
	authflowLogger().Info("Steam Guard enrollment finalize returned", "steamId64", accountID, "state", result.State)
	if status.State == enrollmentapi.StateComplete {
		s.clearRevocationAcknowledgment(accountID)
		result.RegistryUpdated = s.upsertRegistry(accountID, registry.StateActive)
		return result, nil
	}
	result.RegistryUpdated = s.upsertRegistry(accountID, registry.StatePending)
	return result, nil
}

func (s *Service) CancelSteamGuardEnrollment(accountID, token string) error {
	v, _, steamID, err := s.authorizeSteamFlow(accountID, token)
	if err != nil {
		return err
	}
	s.cancelBoundSteamOperation(accountID)
	manager := s.enrollmentFlow(v)
	if manager == nil {
		return ErrSteamAuthenticationState
	}
	status, err := manager.Resume(steamID)
	if err != nil && !enrollmentflow.IsNoPending(err) {
		authflowLogger().Warn("Steam Guard enrollment cancel could not read pending state", "steamId64", accountID, "error", err)
		return err
	}
	s.clearRevocationAcknowledgment(accountID)
	if status.Pending {
		s.upsertRegistry(accountID, registry.StatePending)
	}
	authflowLogger().Info("Steam Guard enrollment cancelled", "steamId64", accountID, "stillPending", status.Pending)
	return nil
}

func (s *Service) authorizeSteamFlow(accountID, token string) (*vault.Vault, authflow.Binding, uint64, error) {
	accountID = strings.TrimSpace(accountID)
	steamID, err := canonicalSteamID(accountID)
	if err != nil {
		return nil, authflow.Binding{}, 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.requireVaultLocked()
	if err != nil {
		return nil, authflow.Binding{}, 0, err
	}
	if v.IsLocked() {
		return nil, authflow.Binding{}, 0, vault.ErrLocked
	}
	if err := s.authorizeModalLocked(v, accountID, token); err != nil {
		return nil, authflow.Binding{}, 0, err
	}
	return v, authflow.Binding{
		AccountID: accountID, ExpectedSteamID: steamID, VaultGeneration: v.Generation(), CapabilityID: authCapabilityID(token),
	}, steamID, nil
}

func (s *Service) authenticationManager() (steamCredentialAuthManager, uint64, error) {
	s.authStateMu.Lock()
	defer s.authStateMu.Unlock()
	if s.authShutdown || s.newAuthManager == nil {
		return nil, 0, ErrSteamAuthenticationState
	}
	if s.authManager == nil {
		manager, err := s.newAuthManager()
		if err != nil {
			return nil, 0, err
		}
		s.authManager = manager
		s.authManagerEpoch++
	}
	return s.authManager, s.authManagerEpoch, nil
}

func (s *Service) authenticationOperation(accountID, token, handle string) (steamCredentialAuthManager, steamAuthOperation, authflow.Binding, error) {
	_, binding, _, err := s.authorizeSteamFlow(accountID, token)
	if err != nil {
		return nil, steamAuthOperation{}, authflow.Binding{}, err
	}
	s.authStateMu.Lock()
	defer s.authStateMu.Unlock()
	operation, ok := s.authOperations[handle]
	if !ok || s.authManager == nil || !sameAuthBinding(operation.binding, binding) {
		return nil, steamAuthOperation{}, authflow.Binding{}, ErrSteamAuthenticationState
	}
	return s.authManager, operation, binding, nil
}

func (s *Service) removeAuthenticationOperation(handle string) {
	s.authStateMu.Lock()
	delete(s.authOperations, handle)
	s.authStateMu.Unlock()
}

func (s *Service) closeAuthenticationManager(shutdown bool) {
	s.authStateMu.Lock()
	manager := s.authManager
	s.authManager = nil
	s.authManagerEpoch++
	if shutdown {
		s.authShutdown = true
	}
	for accountID, operation := range s.steamOperationCancels {
		operation.cancel()
		delete(s.steamOperationCancels, accountID)
	}
	clear(s.authOperations)
	s.enrollmentManager = nil
	for capabilityID := range s.revocationAcknowledgments {
		delete(s.revocationAcknowledgments, capabilityID)
	}
	s.authStateMu.Unlock()
	if manager != nil {
		manager.Close()
	}
}

func (s *Service) cancelAuthenticationForBinding(binding capability.Binding, token string) {
	s.authStateMu.Lock()
	manager := s.authManager
	operations := make([]struct {
		handle  string
		binding authflow.Binding
	}, 0, 1)
	for handle, operation := range s.authOperations {
		if operation.binding.AccountID == binding.AccountID && operation.binding.VaultGeneration == binding.VaultGeneration &&
			subtle.ConstantTimeCompare([]byte(operation.binding.CapabilityID), []byte(authCapabilityID(token))) == 1 {
			operations = append(operations, struct {
				handle  string
				binding authflow.Binding
			}{handle: handle, binding: operation.binding})
			delete(s.authOperations, handle)
		}
	}
	if operation, ok := s.steamOperationCancels[binding.AccountID]; ok {
		operation.cancel()
		delete(s.steamOperationCancels, binding.AccountID)
	}
	s.authStateMu.Unlock()
	if manager != nil {
		for _, operation := range operations {
			s.cancelAuthflowSession(manager, operation.binding, operation.handle, "lease-revoked")
		}
	}
}

func (s *Service) enrollmentFlow(v *vault.Vault) steamEnrollmentManager {
	s.authStateMu.Lock()
	defer s.authStateMu.Unlock()
	if s.authShutdown || s.newEnrollmentManager == nil {
		return nil
	}
	if s.enrollmentManager == nil {
		s.enrollmentManager = s.newEnrollmentManager(v)
	}
	return s.enrollmentManager
}

func (s *Service) startBoundSteamOperation(accountID string) (context.Context, func(), error) {
	s.authStateMu.Lock()
	defer s.authStateMu.Unlock()
	if s.authShutdown {
		return nil, nil, ErrSteamAuthenticationState
	}
	if _, busy := s.steamOperationCancels[accountID]; busy {
		return nil, nil, ErrSteamOperationBusy
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.steamOperationSequence++
	operationID := s.steamOperationSequence
	s.steamOperationCancels[accountID] = steamBoundOperation{id: operationID, cancel: cancel}
	return ctx, func() {
		cancel()
		s.authStateMu.Lock()
		if current, ok := s.steamOperationCancels[accountID]; ok && current.id == operationID {
			delete(s.steamOperationCancels, accountID)
		}
		s.authStateMu.Unlock()
	}, nil
}

func (s *Service) cancelBoundSteamOperation(accountID string) {
	s.authStateMu.Lock()
	operation, ok := s.steamOperationCancels[accountID]
	delete(s.steamOperationCancels, accountID)
	s.authStateMu.Unlock()
	if ok {
		operation.cancel()
	}
}

func (s *Service) clearRevocationAcknowledgment(accountID string) {
	s.authStateMu.Lock()
	for capabilityID, ack := range s.revocationAcknowledgments {
		if ack.accountID == accountID {
			ack.digest = [sha256.Size]byte{}
			delete(s.revocationAcknowledgments, capabilityID)
		}
	}
	s.authStateMu.Unlock()
}

func (s *Service) clearUnacknowledgedRevocationForCapability(accountID, token string) {
	capabilityID := authCapabilityID(token)
	s.authStateMu.Lock()
	if ack, ok := s.revocationAcknowledgments[capabilityID]; ok && ack.accountID == accountID {
		ack.digest = [sha256.Size]byte{}
		delete(s.revocationAcknowledgments, capabilityID)
	}
	s.authStateMu.Unlock()
}

func (s *Service) authenticatorTime() int64 {
	if s.timeState == nil {
		return time.Now().Unix()
	}
	now, _ := s.timeState.Now()
	return now.Unix()
}

func (s *Service) upsertRegistry(accountID string, state registry.State) bool {
	if s.registryUpsertFn == nil {
		return false
	}
	if err := s.registryUpsertFn(accountID, state); err != nil {
		serviceLogger().Warn("Steam Guard registry update failed", "state", state, "error", err)
		return false
	}
	return true
}

func credentialResult(status authflow.Status) SteamCredentialResult {
	challenges := make([]string, 0, len(status.Challenges))
	for _, challenge := range status.Challenges {
		challenges = append(challenges, string(challenge))
	}
	return SteamCredentialResult{
		Handle: status.Handle, State: string(status.State), Challenges: challenges,
		CanSubmitEmailCode: status.CanSubmitEmailCode, CanSubmitDeviceCode: status.CanSubmitDeviceCode,
		CanPoll: status.CanPoll, PollAfterMillis: status.PollAfterMillis, ExpiresAtUnix: status.ExpiresAtUnix,
	}
}

func enrollmentResult(status enrollmentflow.Status) SteamEnrollmentStatus {
	confirmation := "unknown"
	switch status.Confirmation {
	case enrollmentapi.ConfirmationSMS:
		confirmation = "sms"
	case enrollmentapi.ConfirmationEmail:
		confirmation = "email"
	}
	return SteamEnrollmentStatus{
		State: string(status.State), Confirmation: confirmation, PhoneHint: status.PhoneHint,
		RetryAfterSeconds: status.RetryAfterSeconds, HasRetryAfter: status.HasRetryAfter,
		Pending: status.Pending, Resumed: status.Resumed, RevocationViewAvailable: status.RevocationViewAvailable,
	}
}

func submittedAuthChallenge(value string) (authflow.Challenge, bool) {
	switch authflow.Challenge(strings.TrimSpace(value)) {
	case authflow.ChallengeEmailCode:
		return authflow.ChallengeEmailCode, true
	case authflow.ChallengeDeviceCode:
		return authflow.ChallengeDeviceCode, true
	default:
		return "", false
	}
}

func passwordAuthRequest(accountName string) protocol.PasswordCredentialsRequest {
	const friendlyName = "TcNo Account Switcher"
	return protocol.PasswordCredentialsRequest{
		DeviceFriendlyName: friendlyName,
		AccountName:        accountName,
		RememberLogin:      true,
		Platform:           protocol.PlatformMobileApp,
		Persistence:        protocol.PersistencePersistent,
		WebsiteID:          "Mobile",
		Device: protocol.DeviceDetails{
			FriendlyName: friendlyName, Platform: protocol.PlatformMobileApp, OSType: 32, App: protocol.AppTypeSteamMobile,
		},
		QoSLevel: 2,
	}
}

func updateMafileSession(v *vault.Vault, steamID uint64, accountName, accessToken, refreshToken []byte) error {
	if v == nil || !utf8.Valid(accountName) || !validBearerBytes(accessToken) || (len(refreshToken) != 0 && !validBearerBytes(refreshToken)) {
		return ErrSteamAuthenticationState
	}
	records, err := v.ListRecords()
	if err != nil {
		return err
	}
	wanted := strconv.FormatUint(steamID, 10)
	var recordID string
	for _, record := range records {
		if record.SteamID64 == wanted {
			if recordID != "" {
				return ErrSteamAuthenticationState
			}
			recordID = record.ID
		}
	}
	if recordID == "" {
		return ErrAccountNotFound
	}
	account, err := accountFromRecord(v, recordID)
	if err != nil || account.Session == nil || account.Session.SteamID != steamID {
		return ErrSteamAuthenticationState
	}
	defer clearMafileAccount(&account)
	if len(accountName) != 0 {
		account.AccountName = string(accountName)
	}
	account.Session.AccessToken = string(accessToken)
	if len(refreshToken) != 0 {
		account.Session.RefreshToken = string(refreshToken)
	}
	canonical, err := mafile.ExportPlaintext(account, mafile.ExportOptions{IncludeTokens: true})
	if err != nil {
		wipe(canonical)
		return ErrSteamAuthenticationState
	}
	defer wipe(canonical)
	_, err = v.PutRecord(wanted, canonical)
	return err
}

func clearMafileAccount(account *mafile.Account) {
	if account == nil {
		return
	}
	account.SharedSecret = ""
	account.IdentitySecret = ""
	account.Secret1 = ""
	account.RevocationCode = ""
	account.URI = ""
	if account.Session != nil {
		*account.Session = mafile.SessionData{}
	}
	*account = mafile.Account{}
}

func canonicalSteamID(accountID string) (uint64, error) {
	steamID, err := strconv.ParseUint(accountID, 10, 64)
	const minimum = uint64(76561197960265728)
	if err != nil || steamID < minimum || steamID > minimum+uint64(^uint32(0)) || strconv.FormatUint(steamID, 10) != accountID {
		return 0, ErrAccountNotFound
	}
	return steamID, nil
}

func validSteamAuthPurpose(purpose SteamAuthPurpose) bool {
	return purpose == SteamAuthPurposeLoginAgain || purpose == SteamAuthPurposeAddAuthenticator
}

func validAccountName(value string) bool {
	if len(value) == 0 || len(value) > 64 || !utf8.ValidString(value) {
		return false
	}
	for _, current := range value {
		if current < 0x21 || current > 0x7e {
			return false
		}
	}
	return true
}

func validBearerBytes(value []byte) bool {
	if len(value) == 0 || len(value) > 8192 {
		return false
	}
	for _, current := range value {
		if current < 0x21 || current > 0x7e {
			return false
		}
	}
	return true
}

func authCapabilityID(token string) string {
	digest := sha256.Sum256([]byte("tcno-steamguard-auth-capability-v1\x00" + token))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func sameAuthBinding(left, right authflow.Binding) bool {
	return left.ExpectedSteamID == right.ExpectedSteamID &&
		subtle.ConstantTimeCompare([]byte(left.AccountID), []byte(right.AccountID)) == 1 &&
		subtle.ConstantTimeCompare([]byte(left.VaultGeneration), []byte(right.VaultGeneration)) == 1 &&
		subtle.ConstantTimeCompare([]byte(left.CapabilityID), []byte(right.CapabilityID)) == 1
}

func revocationDigest(code []byte) [sha256.Size]byte {
	buffer := make([]byte, 0, len(code)+48)
	buffer = append(buffer, "tcno-steamguard-revocation-ack-v1\x00"...)
	buffer = append(buffer, code...)
	digest := sha256.Sum256(buffer)
	wipe(buffer)
	return digest
}
