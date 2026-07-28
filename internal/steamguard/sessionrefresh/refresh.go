// Package sessionrefresh renews Steam web access tokens and writes them back to
// the encrypted Steam Guard vault without returning bearer tokens to callers.
package sessionrefresh

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

const (
	DefaultRequestTimeout = 20 * time.Second
	maxRequestTimeout     = 20 * time.Second
	maxTokenBytes         = 8192
	pendingKind           = "steamguard-enrollment-pending"
)

// Code is a stable, secret-free error classification.
type Code string

const (
	CodeInvalidRequest  Code = "invalid_request"
	CodeVaultLocked     Code = "vault_locked"
	CodeVaultRead       Code = "vault_read"
	CodeAccountNotFound Code = "account_not_found"
	CodeDuplicate       Code = "duplicate_account"
	CodePending         Code = "pending_enrollment"
	CodeCorruptRecord   Code = "corrupt_record"
	CodeWrongAccount    Code = "wrong_account"
	CodeNoRefreshToken  Code = "no_refresh_token"
	CodeCanceled        Code = "canceled"
	CodeTimedOut        Code = "timed_out"
	CodeRemote          Code = "remote_failure"
	CodeInvalidResponse Code = "invalid_token_response"
	CodePersist         Code = "persist_failure"
)

// Error deliberately omits wrapped transport, filesystem, and token details.
type Error struct{ Code Code }

func (e *Error) Error() string {
	if e == nil {
		return "Steam session refresh failed"
	}
	switch e.Code {
	case CodeInvalidRequest:
		return "invalid Steam session refresh request"
	case CodeVaultLocked:
		return "Steam Guard vault is locked"
	case CodeVaultRead:
		return "Steam Guard vault could not be read"
	case CodeAccountNotFound:
		return "Steam Guard account was not found"
	case CodeDuplicate:
		return "duplicate Steam Guard account records"
	case CodePending:
		return "Steam Guard enrollment is still pending"
	case CodeCorruptRecord:
		return "Steam Guard account record is invalid"
	case CodeWrongAccount:
		return "Steam Guard account record does not match the requested account"
	case CodeNoRefreshToken:
		return "Steam Guard account has no usable refresh token"
	case CodeCanceled:
		return "Steam session refresh was canceled"
	case CodeTimedOut:
		return "Steam session refresh timed out"
	case CodeRemote:
		return "Steam rejected or could not complete the session refresh"
	case CodeInvalidResponse:
		return "Steam returned an invalid session refresh response"
	case CodePersist:
		return "refreshed Steam session could not be saved"
	default:
		return "Steam session refresh failed"
	}
}

// Is supports errors.Is without retaining an underlying error that could
// contain request, transport, or filesystem details.
func (e *Error) Is(target error) bool {
	other, ok := target.(*Error)
	return ok && e != nil && other != nil && e.Code == other.Code
}

var (
	ErrInvalidRequest  = &Error{Code: CodeInvalidRequest}
	ErrVaultLocked     = &Error{Code: CodeVaultLocked}
	ErrVaultRead       = &Error{Code: CodeVaultRead}
	ErrAccountNotFound = &Error{Code: CodeAccountNotFound}
	ErrDuplicate       = &Error{Code: CodeDuplicate}
	ErrPending         = &Error{Code: CodePending}
	ErrCorruptRecord   = &Error{Code: CodeCorruptRecord}
	ErrWrongAccount    = &Error{Code: CodeWrongAccount}
	ErrNoRefreshToken  = &Error{Code: CodeNoRefreshToken}
	ErrCanceled        = &Error{Code: CodeCanceled}
	ErrTimedOut        = &Error{Code: CodeTimedOut}
	ErrRemote          = &Error{Code: CodeRemote}
	ErrInvalidResponse = &Error{Code: CodeInvalidResponse}
	ErrPersist         = &Error{Code: CodePersist}
)

// TokenClient is the only Steam protocol operation needed by Refresher.
type TokenClient interface {
	GenerateAccessTokenForApp(context.Context, protocol.GenerateAccessTokenRequest, time.Duration) (protocol.TokenResult, error)
}

// UnlockedVault is the narrow encrypted-storage boundary used by Refresher.
// Implementations must provide an atomic PutRecord operation.
type UnlockedVault interface {
	ListRecords() ([]vault.RecordInfo, error)
	GetRecord(id string) ([]byte, error)
	PutRecord(steamID64 string, plaintext []byte) (string, error)
}

var _ UnlockedVault = (*vault.Vault)(nil)

// Result contains no bearer credentials and is safe to pass to callers.
type Result struct {
	SteamID             uint64
	RefreshTokenRenewed bool
}

// Refresher serializes read/exchange/write operations made through one
// instance. The vault itself makes each final generation switch atomic.
type Refresher struct {
	client  TokenClient
	vault   UnlockedVault
	timeout time.Duration
	mu      sync.Mutex
}

func New(client TokenClient, unlockedVault UnlockedVault) *Refresher {
	return &Refresher{client: client, vault: unlockedVault, timeout: DefaultRequestTimeout}
}

// NewWithTimeout is intended for callers whose parent context has a shorter
// deadline. Durations above the protocol maximum are rejected at Refresh.
func NewWithTimeout(client TokenClient, unlockedVault UnlockedVault, timeout time.Duration) *Refresher {
	return &Refresher{client: client, vault: unlockedVault, timeout: timeout}
}

// logger records the Steam ID and the stable error Code only. Access and
// refresh tokens never reach it.
func logger() *slog.Logger {
	return slog.Default().With("component", "steamguard.sessionrefresh")
}

// failureCode maps an error to its stable, secret-free classification.
func failureCode(err error) string {
	var refreshErr *Error
	if errors.As(err, &refreshErr) && refreshErr != nil {
		return string(refreshErr.Code)
	}
	return "unknown"
}

// Refresh logs the attempt outcome and delegates to refresh.
func (r *Refresher) Refresh(ctx context.Context, steamID uint64) (Result, error) {
	log := logger()
	log.Debug("refreshing Steam session", "steamId64", steamID)
	result, err := r.refresh(ctx, steamID)
	if err != nil {
		log.Warn("Steam session refresh failed", "steamId64", steamID, "code", failureCode(err))
		return Result{}, err
	}
	log.Info("Steam session refreshed", "steamId64", steamID, "refreshTokenRenewed", result.RefreshTokenRenewed)
	return result, nil
}

func (r *Refresher) refresh(ctx context.Context, steamID uint64) (Result, error) {
	if r == nil || r.client == nil || r.vault == nil || ctx == nil ||
		!validSteamID(steamID) || r.timeout <= 0 || r.timeout > maxRequestTimeout {
		return Result{}, ErrInvalidRequest
	}
	if err := contextError(ctx.Err()); err != nil {
		return Result{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := contextError(ctx.Err()); err != nil {
		return Result{}, err
	}

	record, raw, err := r.loadActiveRecord(ctx, steamID)
	if err != nil {
		return Result{}, err
	}
	defer wipe(raw)

	parsed, parseErr := mafile.ParsePlaintext(raw)
	if parseErr != nil {
		if pendingRecord(raw) {
			return Result{}, ErrPending
		}
		return Result{}, ErrCorruptRecord
	}
	account := parsed.Account
	if account.Session == nil || account.Session.SteamID != steamID || record.SteamID64 != strconv.FormatUint(steamID, 10) {
		clearAccount(&account)
		return Result{}, ErrWrongAccount
	}
	defer clearAccount(&account)
	if !validToken(account.Session.RefreshToken) {
		return Result{}, ErrNoRefreshToken
	}
	if err := contextError(ctx.Err()); err != nil {
		return Result{}, err
	}

	refreshBuffer := append([]byte(nil), account.Session.RefreshToken...)
	defer wipe(refreshBuffer)
	request := protocol.GenerateAccessTokenRequest{
		SteamID:      steamID,
		RefreshToken: string(refreshBuffer),
		Renewal:      protocol.RenewalAllow,
	}
	requestCtx, cancel := context.WithTimeout(ctx, r.timeout)
	result, remoteErr := r.client.GenerateAccessTokenForApp(requestCtx, request, r.timeout)
	requestContextErr := requestCtx.Err()
	cancel()
	request.RefreshToken = ""
	runtime.KeepAlive(request)
	if err := contextError(requestContextErr); err != nil {
		clearTokenResult(&result)
		return Result{}, err
	}
	if remoteErr != nil {
		clearTokenResult(&result)
		if err := contextError(remoteErr); err != nil {
			return Result{}, err
		}
		var protocolErr *protocol.Error
		if errors.As(remoteErr, &protocolErr) &&
			(protocolErr.Code == protocol.CodeInvalidResponse || protocolErr.Code == protocol.CodeResponseTooLarge) {
			return Result{}, ErrInvalidResponse
		}
		return Result{}, ErrRemote
	}
	defer clearTokenResult(&result)
	if result.State != protocol.AuthResultTokenIssued || !validToken(result.AccessToken) ||
		(result.RefreshToken != "" && !validToken(result.RefreshToken)) {
		return Result{}, ErrInvalidResponse
	}

	account.Session.AccessToken = result.AccessToken
	renewed := result.RefreshToken != ""
	if renewed {
		account.Session.RefreshToken = result.RefreshToken
	}
	canonical, exportErr := mafile.ExportPlaintext(account, mafile.ExportOptions{IncludeTokens: true})
	if exportErr != nil {
		wipe(canonical)
		return Result{}, ErrCorruptRecord
	}
	defer wipe(canonical)
	if err := contextError(ctx.Err()); err != nil {
		return Result{}, err
	}
	if _, putErr := r.vault.PutRecord(strconv.FormatUint(steamID, 10), canonical); putErr != nil {
		if errors.Is(putErr, vault.ErrLocked) || errors.Is(putErr, vault.ErrLeaseExpired) {
			return Result{}, ErrVaultLocked
		}
		return Result{}, ErrPersist
	}
	return Result{SteamID: steamID, RefreshTokenRenewed: renewed}, nil
}

func (r *Refresher) loadActiveRecord(ctx context.Context, steamID uint64) (vault.RecordInfo, []byte, error) {
	records, err := r.vault.ListRecords()
	if err != nil {
		if errors.Is(err, vault.ErrLocked) || errors.Is(err, vault.ErrLeaseExpired) {
			return vault.RecordInfo{}, nil, ErrVaultLocked
		}
		return vault.RecordInfo{}, nil, ErrVaultRead
	}
	if err := contextError(ctx.Err()); err != nil {
		return vault.RecordInfo{}, nil, err
	}
	wanted := strconv.FormatUint(steamID, 10)
	var match vault.RecordInfo
	count := 0
	for _, candidate := range records {
		if candidate.SteamID64 == wanted {
			match = candidate
			count++
		}
	}
	if count == 0 {
		return vault.RecordInfo{}, nil, ErrAccountNotFound
	}
	if count != 1 || strings.TrimSpace(match.ID) == "" {
		return vault.RecordInfo{}, nil, ErrDuplicate
	}
	raw, err := r.vault.GetRecord(match.ID)
	if err != nil {
		wipe(raw)
		if errors.Is(err, vault.ErrLocked) || errors.Is(err, vault.ErrLeaseExpired) {
			return vault.RecordInfo{}, nil, ErrVaultLocked
		}
		return vault.RecordInfo{}, nil, ErrVaultRead
	}
	if len(raw) == 0 || len(raw) > mafile.MaxInputBytes {
		wipe(raw)
		return vault.RecordInfo{}, nil, ErrCorruptRecord
	}
	return match, raw, nil
}

func pendingRecord(raw []byte) bool {
	var marker struct {
		Kind string `json:"kind"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	return decoder.Decode(&marker) == nil && marker.Kind == pendingKind
}

func validSteamID(value uint64) bool {
	const min = uint64(76561197960265728)
	return value >= min && value <= min+uint64(^uint32(0))
}

func validToken(value string) bool {
	if len(value) == 0 || len(value) > maxTokenBytes {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 0x21 || value[i] > 0x7e {
			return false
		}
	}
	return true
}

func contextError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimedOut
	}
	if errors.Is(err, context.Canceled) {
		return ErrCanceled
	}
	return nil
}

func clearTokenResult(result *protocol.TokenResult) {
	if result == nil {
		return
	}
	result.AccessToken = ""
	result.RefreshToken = ""
	*result = protocol.TokenResult{}
	runtime.KeepAlive(result)
}

func clearAccount(account *mafile.Account) {
	if account == nil {
		return
	}
	if account.Session != nil {
		*account.Session = mafile.SessionData{}
	}
	*account = mafile.Account{}
	runtime.KeepAlive(account)
}

func wipe(value []byte) {
	for i := range value {
		value[i] = 0
	}
	runtime.KeepAlive(value)
}
