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

	"TcNo-Acc-Switcher/internal/steamguard/loginrecord"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
	"TcNo-Acc-Switcher/internal/steamguard/vaultrecord"
)

const (
	DefaultRequestTimeout = 20 * time.Second
	maxRequestTimeout     = 20 * time.Second
	maxTokenBytes         = 8192
	pendingKind           = vaultrecord.KindStringEnrollmentPending
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
// Implementations must provide atomic PutRecord and PutRecords operations.
type UnlockedVault interface {
	ListRecords() ([]vault.RecordInfo, error)
	GetRecord(id string) ([]byte, error)
	PutRecord(steamID64 string, plaintext []byte) (string, error)
	// PutRecords commits every update in one generation, all or nothing. A
	// batch must cost exactly one generation switch, because each one
	// invalidates every capability outstanding against the vault.
	PutRecords(updates []vault.RecordUpdate) error
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

	if record.SteamID64 != strconv.FormatUint(steamID, 10) {
		return Result{}, ErrWrongAccount
	}
	decoded, err := decodeRecord(raw, steamID)
	if err != nil {
		return Result{}, err
	}
	defer decoded.destroy()
	if !validToken(decoded.refreshToken()) {
		return Result{}, ErrNoRefreshToken
	}
	if err := contextError(ctx.Err()); err != nil {
		return Result{}, err
	}

	result, err := r.exchange(ctx, steamID, decoded.refreshToken())
	if err != nil {
		return Result{}, err
	}
	defer clearTokenResult(&result)

	renewed := result.RefreshToken != ""
	canonical, exportErr := decoded.apply(result)
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

// RefreshBatch renews several accounts and writes every success back in a
// single vault generation.
//
// A generation switch invalidates every capability outstanding against the
// vault, so refreshing N accounts one Refresh at a time costs N invalidations -
// the reason a background sweep is forbidden from refreshing at all. One commit
// for the whole batch is what makes a sweep-sized refresh affordable.
//
// An account that cannot be loaded, decoded or renewed is dropped from the
// batch rather than failing it: one lapsed refresh token must not deny every
// other account its new session. Only a bad request, a locked vault, a done
// context or a failed commit returns an error, and a failed commit leaves every
// record on the generation it was already on.
func (r *Refresher) RefreshBatch(ctx context.Context, steamIDs []uint64) ([]Result, error) {
	if r == nil || r.client == nil || r.vault == nil || ctx == nil ||
		r.timeout <= 0 || r.timeout > maxRequestTimeout {
		return nil, ErrInvalidRequest
	}
	if len(steamIDs) == 0 {
		return nil, nil
	}
	if err := contextError(ctx.Err()); err != nil {
		return nil, err
	}
	log := logger()

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := contextError(ctx.Err()); err != nil {
		return nil, err
	}

	loaded, err := r.loadBatch(ctx, steamIDs, log)
	if err != nil {
		return nil, err
	}
	defer func() {
		for _, entry := range loaded {
			entry.destroy()
		}
	}()

	updates := make([]vault.RecordUpdate, 0, len(loaded))
	defer func() {
		for i := range updates {
			wipe(updates[i].Plaintext)
		}
	}()
	results := make([]Result, 0, len(loaded))
	for _, entry := range loaded {
		if err := contextError(ctx.Err()); err != nil {
			return nil, err
		}
		result, exchangeErr := r.exchange(ctx, entry.steamID, entry.decoded.refreshToken())
		if exchangeErr != nil {
			// One account's own deadline is indistinguishable from the parent's
			// here, so the parent context is what decides whether a slow or
			// rejected account ends the batch or is merely dropped from it.
			if parentErr := contextError(ctx.Err()); parentErr != nil {
				return nil, parentErr
			}
			logDropped(log, entry.steamID, exchangeErr)
			continue
		}
		// Encoded in the same pass as the exchange rather than in a later one, so
		// the issued tokens are cleared as soon as they are inside the batch.
		canonical, exportErr := entry.decoded.apply(result)
		renewed := result.RefreshToken != ""
		clearTokenResult(&result)
		if exportErr != nil {
			wipe(canonical)
			logDropped(log, entry.steamID, ErrCorruptRecord)
			continue
		}
		updates = append(updates, vault.RecordUpdate{
			SteamID64: strconv.FormatUint(entry.steamID, 10),
			Plaintext: canonical,
		})
		results = append(results, Result{SteamID: entry.steamID, RefreshTokenRenewed: renewed})
	}

	if len(updates) > 0 {
		if err := contextError(ctx.Err()); err != nil {
			return nil, err
		}
		if putErr := r.vault.PutRecords(updates); putErr != nil {
			if errors.Is(putErr, vault.ErrLocked) || errors.Is(putErr, vault.ErrLeaseExpired) {
				return nil, ErrVaultLocked
			}
			return nil, ErrPersist
		}
	}
	log.Info("Steam sessions refreshed in one vault generation",
		"requested", len(steamIDs), "refreshed", len(results), "skipped", len(steamIDs)-len(results))
	return results, nil
}

// batchEntry is one account's decoded record, held from the load phase until
// the batch is written.
type batchEntry struct {
	steamID uint64
	raw     []byte
	decoded decodedRecord
}

func (e *batchEntry) destroy() {
	wipe(e.raw)
	e.raw = nil
	e.decoded.destroy()
}

// loadBatch reads and decodes every account that can take part in the batch,
// before any network work starts.
//
// Repeats in steamIDs are collapsed: vault.PutRecords rejects a batch naming
// one account twice, so a caller's duplicate would otherwise sink the lot.
func (r *Refresher) loadBatch(ctx context.Context, steamIDs []uint64, log *slog.Logger) ([]*batchEntry, error) {
	loaded := make([]*batchEntry, 0, len(steamIDs))
	abort := func(err error) ([]*batchEntry, error) {
		for _, entry := range loaded {
			entry.destroy()
		}
		return nil, err
	}
	seen := make(map[uint64]struct{}, len(steamIDs))
	for _, steamID := range steamIDs {
		if _, repeat := seen[steamID]; repeat {
			continue
		}
		seen[steamID] = struct{}{}
		if err := contextError(ctx.Err()); err != nil {
			return abort(err)
		}
		if !validSteamID(steamID) {
			logDropped(log, steamID, ErrInvalidRequest)
			continue
		}
		record, raw, err := r.loadActiveRecord(ctx, steamID)
		if err != nil {
			// A locked vault or a done context would fail every remaining account
			// too, so they end the batch instead of emptying it one row at a time.
			if errors.Is(err, ErrVaultLocked) || errors.Is(err, ErrCanceled) || errors.Is(err, ErrTimedOut) {
				return abort(err)
			}
			logDropped(log, steamID, err)
			continue
		}
		entry := &batchEntry{steamID: steamID, raw: raw}
		if record.SteamID64 != strconv.FormatUint(steamID, 10) {
			entry.destroy()
			logDropped(log, steamID, ErrWrongAccount)
			continue
		}
		decoded, decodeErr := decodeRecord(raw, steamID)
		if decodeErr != nil {
			entry.destroy()
			logDropped(log, steamID, decodeErr)
			continue
		}
		entry.decoded = decoded
		if !validToken(decoded.refreshToken()) {
			entry.destroy()
			logDropped(log, steamID, ErrNoRefreshToken)
			continue
		}
		loaded = append(loaded, entry)
	}
	return loaded, nil
}

// logDropped records why one account left the batch. The batch itself reports
// only counts, so this is the sole place a per-account reason appears - as the
// stable Code, never a token or a transport detail.
func logDropped(log *slog.Logger, steamID uint64, err error) {
	log.Debug("account dropped from Steam session batch", "steamId64", steamID, "code", failureCode(err))
}

// decodedRecord is one vault record in the shape it was stored in.
//
// Both shapes carry a refresh token, and both must be written back the way they
// arrived: a login-only record has no authenticator secrets, so re-exporting it
// as a maFile would fail validation and lose the session.
type decodedRecord struct {
	account     mafile.Account
	login       loginrecord.Record
	isLoginOnly bool
}

// decodeRecord decodes raw as whichever shape it holds. Anything it decoded is
// destroyed before it returns an error, so a rejected record leaves nothing
// behind for the caller to clean up.
func decodeRecord(raw []byte, steamID uint64) (decodedRecord, error) {
	switch vaultrecord.Sniff(raw) {
	case vaultrecord.KindLoginOnly:
		login, err := loginrecord.Decode(raw)
		if err != nil {
			return decodedRecord{}, ErrCorruptRecord
		}
		if login.SteamID != steamID {
			login.Destroy()
			return decodedRecord{}, ErrWrongAccount
		}
		return decodedRecord{login: login, isLoginOnly: true}, nil
	case vaultrecord.KindEnrollmentPending:
		return decodedRecord{}, ErrPending
	default:
		parsed, err := mafile.ParsePlaintext(raw)
		if err != nil {
			if pendingRecord(raw) {
				return decodedRecord{}, ErrPending
			}
			return decodedRecord{}, ErrCorruptRecord
		}
		account := parsed.Account
		if account.Session == nil || account.Session.SteamID != steamID {
			clearAccount(&account)
			return decodedRecord{}, ErrWrongAccount
		}
		return decodedRecord{account: account}, nil
	}
}

func (d *decodedRecord) refreshToken() string {
	if d.isLoginOnly {
		return d.login.RefreshToken
	}
	if d.account.Session == nil {
		return ""
	}
	return d.account.Session.RefreshToken
}

// apply folds the issued credentials into the record and re-encodes it in its
// stored shape. An empty result.RefreshToken means Steam chose not to rotate
// it, and the stored one has to survive.
func (d *decodedRecord) apply(result protocol.TokenResult) ([]byte, error) {
	if d.isLoginOnly {
		d.login.AccessToken = result.AccessToken
		if result.RefreshToken != "" {
			d.login.RefreshToken = result.RefreshToken
		}
		return loginrecord.Encode(d.login)
	}
	d.account.Session.AccessToken = result.AccessToken
	if result.RefreshToken != "" {
		d.account.Session.RefreshToken = result.RefreshToken
	}
	return mafile.ExportPlaintext(d.account, mafile.ExportOptions{IncludeTokens: true})
}

func (d *decodedRecord) destroy() {
	d.login.Destroy()
	clearAccount(&d.account)
}

// exchange renews one account's tokens. A successful result is the caller's to
// clearTokenResult; a failed one is already cleared and classified.
func (r *Refresher) exchange(ctx context.Context, steamID uint64, refreshToken string) (protocol.TokenResult, error) {
	refreshBuffer := append([]byte(nil), refreshToken...)
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
	// Checked before remoteErr: a result that arrives after the deadline must not
	// reach the vault, however well-formed it looks.
	if err := contextError(requestContextErr); err != nil {
		clearTokenResult(&result)
		return protocol.TokenResult{}, err
	}
	if remoteErr != nil {
		clearTokenResult(&result)
		if err := contextError(remoteErr); err != nil {
			return protocol.TokenResult{}, err
		}
		var protocolErr *protocol.Error
		if errors.As(remoteErr, &protocolErr) &&
			(protocolErr.Code == protocol.CodeInvalidResponse || protocolErr.Code == protocol.CodeResponseTooLarge) {
			return protocol.TokenResult{}, ErrInvalidResponse
		}
		return protocol.TokenResult{}, ErrRemote
	}
	if result.State != protocol.AuthResultTokenIssued || !validToken(result.AccessToken) ||
		(result.RefreshToken != "" && !validToken(result.RefreshToken)) {
		clearTokenResult(&result)
		return protocol.TokenResult{}, ErrInvalidResponse
	}
	return result, nil
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
