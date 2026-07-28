package steamguard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/capability"
	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

const confirmationOperationTimeout = 45 * time.Second

// confirmationsLogger is the component logger for the confirmations flow. Steam
// identifiers and failure classes only; never credentials, nonces or hashes.
func confirmationsLogger() *slog.Logger {
	return slog.Default().With("component", "steamguard.confirmations")
}

type steamConfirmationClient interface {
	List(context.Context, confirmationapi.Credentials) ([]confirmationapi.Confirmation, error)
	FetchDetails(context.Context, confirmationapi.Credentials, confirmationapi.Confirmation) (confirmationapi.Details, error)
	FetchItemClass(context.Context, confirmationapi.Credentials, string, string, string) (confirmationapi.ItemClass, error)
	Decide(context.Context, confirmationapi.Credentials, confirmationapi.Confirmation, confirmationapi.Decision) error
	DecideBatch(context.Context, confirmationapi.Credentials, []confirmationapi.Confirmation, confirmationapi.Decision) error
	CloseIdleConnections()
}

type ConfirmationTextView struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type ConfirmationRowView struct {
	Handle string `json:"handle"`
	// Kind classifies the confirmation independently of TypeLabel, which is a
	// display string Steam may localise or reword. The window reviews a trade
	// full-window and everything else in a bar at the foot of the list.
	Kind      string `json:"kind"`
	TypeLabel string `json:"typeLabel"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	Icon      string `json:"icon,omitempty"`
	// CreationTime is Steam's Unix seconds, 0 when the row omits it.
	CreationTime int64 `json:"creationTime,omitempty"`
	// Details stays empty for a list refresh; it is populated only when details
	// are fetched on demand for a single row.
	Details     []ConfirmationTextView `json:"details,omitempty"`
	AcceptLabel string                 `json:"acceptLabel"`
	DenyLabel   string                 `json:"denyLabel"`
}

// confirmationKind names the two shapes the window lays out differently, and
// groups everything else: a trade moves items between two people and needs the
// room, a listing or an account change is a few lines of text.
func confirmationKind(value int) string {
	switch value {
	case 2:
		return "trade"
	case 3:
		return "market"
	default:
		return "other"
	}
}

type ConfirmationsResult struct {
	State        string                `json:"state"`
	AccountLabel string                `json:"accountLabel,omitempty"`
	Rows         []ConfirmationRowView `json:"rows,omitempty"`
	FetchedAt    int64                 `json:"fetchedAt,omitempty"`
	RetryAfterMS int64                 `json:"retryAfterMs,omitempty"`
}

type ConfirmationDecisionResult struct {
	State        string `json:"state"`
	RetryAfterMS int64  `json:"retryAfterMs,omitempty"`
}

func (s *Service) ListConfirmations(token string) (ConfirmationsResult, error) {
	binding, account, err := s.confirmationAccount(token)
	if err != nil {
		return ConfirmationsResult{}, err
	}
	log := confirmationsLogger()
	credentials, err := confirmationCredentials(account, binding.AccountID)
	if err != nil {
		log.Warn("confirmation credentials unusable", "operation", "list", "steamId64", binding.AccountID, "error", err)
		// The label rides along on every outcome: the window is titled with it, and
		// a failure is exactly when knowing which account it belongs to matters.
		return ConfirmationsResult{State: "reauth", AccountLabel: account.AccountName}, nil
	}
	log.Debug("listing Steam confirmations", "steamId64", binding.AccountID)
	ctx, operation := s.beginConfirmationOperation()
	defer s.endConfirmationOperation(operation)

	confirmations, err := s.confirmationClient.List(ctx, credentials)
	if err != nil {
		result := confirmationFailure(err)
		result.AccountLabel = account.AccountName
		logConfirmationFailure(log, "list", binding.AccountID, result.State, err)
		return result, nil
	}
	// Rows render from getlist alone. The per-row /details fetch used to abort the
	// whole refresh on the first failure; details are on-demand only now.
	rows := make([]ConfirmationRowView, 0, len(confirmations))
	privateRows := make(map[string]confirmationapi.Confirmation, len(confirmations))
	cachedIcons := make([]string, 0, len(confirmations))
	for _, confirmation := range confirmations {
		item := confirmation.Item()
		typeLabel := item.TypeName
		if typeLabel == "" {
			typeLabel = item.TypeLabel
		}
		// The webview cannot load a Steam URL, so the icon is fetched, sanitized
		// and served from the app's own origin.
		icon := s.confirmationIcons.Ensure(ctx, item.Icon)
		if icon != "" {
			cachedIcons = append(cachedIcons, icon)
		}
		rows = append(rows, ConfirmationRowView{
			Handle: item.Handle, Kind: confirmationKind(item.Type), TypeLabel: typeLabel, Title: item.Title,
			Summary: strings.Join(item.Summary, " · "), Icon: icon, CreationTime: item.CreationTime,
			AcceptLabel: item.AcceptLabel, DenyLabel: item.DenyLabel,
		})
		privateRows[item.Handle] = confirmation
	}
	// Anything neither the new list nor the open detail refers to is gone: a
	// decided confirmation takes its icons with it rather than leaving them behind.
	s.confirmationWindowMu.Lock()
	cachedIcons = append(cachedIcons, s.confirmationDetailIcons...)
	s.confirmationWindowMu.Unlock()
	// Not while a detail or an item hover is fetching: those download icons this
	// refresh has no way to name, and pruning deleted them the moment they landed.
	// Skipping costs one refresh's worth of files; the next prune takes them.
	if !s.confirmationIconsBusy() {
		s.confirmationIcons.Prune(cachedIcons)
	}
	if _, _, err := s.confirmationAccount(token); err != nil {
		return ConfirmationsResult{}, err
	}
	s.confirmationWindowMu.Lock()
	if operation != s.confirmationOperation {
		s.confirmationWindowMu.Unlock()
		return ConfirmationsResult{State: "canceled"}, nil
	}
	s.confirmationRows = privateRows
	s.confirmationWindowMu.Unlock()
	log.Info("Steam confirmations listed", "steamId64", binding.AccountID, "rows", len(rows))
	return ConfirmationsResult{
		State: "fresh", AccountLabel: account.AccountName, Rows: rows, FetchedAt: time.Now().UnixMilli(),
	}, nil
}

func (s *Service) DecideConfirmation(handle, decision, token string) (ConfirmationDecisionResult, error) {
	binding, account, err := s.confirmationAccount(token)
	if err != nil {
		return ConfirmationDecisionResult{}, err
	}
	log := confirmationsLogger()
	credentials, err := confirmationCredentials(account, binding.AccountID)
	if err != nil {
		log.Warn("confirmation credentials unusable", "operation", "decide", "steamId64", binding.AccountID, "error", err)
		return ConfirmationDecisionResult{State: "reauth"}, nil
	}
	handle = strings.TrimSpace(handle)
	s.confirmationWindowMu.Lock()
	confirmation, ok := s.confirmationRows[handle]
	s.confirmationWindowMu.Unlock()
	if !ok {
		log.Warn("confirmation decision for an unknown row", "steamId64", binding.AccountID, "decision", decision)
		return ConfirmationDecisionResult{}, capability.ErrInvalidCapability
	}
	action := confirmationapi.Allow
	if decision == "deny" {
		action = confirmationapi.Deny
	} else if decision != "accept" {
		log.Warn("unsupported confirmation decision", "steamId64", binding.AccountID, "decision", decision)
		return ConfirmationDecisionResult{}, capability.ErrInvalidCapability
	}
	log.Debug("deciding Steam confirmation", "steamId64", binding.AccountID, "decision", decision)
	ctx, operation := s.beginConfirmationOperation()
	defer s.endConfirmationOperation(operation)
	if err := s.confirmationClient.Decide(ctx, credentials, confirmation, action); err != nil {
		failure := confirmationFailure(err)
		logConfirmationFailure(log, "decide", binding.AccountID, failure.State, err)
		return ConfirmationDecisionResult{State: failure.State, RetryAfterMS: failure.RetryAfterMS}, nil
	}
	log.Info("Steam confirmation decided", "steamId64", binding.AccountID, "decision", decision)
	s.confirmationWindowMu.Lock()
	if operation == s.confirmationOperation {
		delete(s.confirmationRows, handle)
	}
	s.confirmationWindowMu.Unlock()
	return ConfirmationDecisionResult{State: "ok"}, nil
}

// DecideConfirmations decides several confirmations as one Steam request. Not a
// loop over DecideConfirmation: single decisions signed within the same second
// carry identical signatures, and Steam rejects the repeats as replays.
func (s *Service) DecideConfirmations(handles []string, decision, token string) (ConfirmationDecisionResult, error) {
	binding, account, err := s.confirmationAccount(token)
	if err != nil {
		return ConfirmationDecisionResult{}, err
	}
	log := confirmationsLogger()
	credentials, err := confirmationCredentials(account, binding.AccountID)
	if err != nil {
		log.Warn("confirmation credentials unusable", "operation", "decide-batch", "steamId64", binding.AccountID, "error", err)
		return ConfirmationDecisionResult{State: "reauth"}, nil
	}
	if len(handles) == 0 || len(handles) > 256 {
		return ConfirmationDecisionResult{}, capability.ErrInvalidCapability
	}
	confirmations := make([]confirmationapi.Confirmation, 0, len(handles))
	trimmed := make([]string, 0, len(handles))
	seen := make(map[string]struct{}, len(handles))
	s.confirmationWindowMu.Lock()
	for _, handle := range handles {
		handle = strings.TrimSpace(handle)
		if _, duplicate := seen[handle]; duplicate {
			s.confirmationWindowMu.Unlock()
			return ConfirmationDecisionResult{}, capability.ErrInvalidCapability
		}
		seen[handle] = struct{}{}
		confirmation, ok := s.confirmationRows[handle]
		if !ok {
			s.confirmationWindowMu.Unlock()
			log.Warn("confirmation decision for an unknown row", "steamId64", binding.AccountID, "decision", decision)
			return ConfirmationDecisionResult{}, capability.ErrInvalidCapability
		}
		confirmations = append(confirmations, confirmation)
		trimmed = append(trimmed, handle)
	}
	s.confirmationWindowMu.Unlock()
	action := confirmationapi.Allow
	if decision == "deny" {
		action = confirmationapi.Deny
	} else if decision != "accept" {
		log.Warn("unsupported confirmation decision", "steamId64", binding.AccountID, "decision", decision)
		return ConfirmationDecisionResult{}, capability.ErrInvalidCapability
	}
	log.Debug("deciding Steam confirmations", "steamId64", binding.AccountID, "decision", decision, "count", len(confirmations))
	ctx, operation := s.beginConfirmationOperation()
	defer s.endConfirmationOperation(operation)
	if err := s.confirmationClient.DecideBatch(ctx, credentials, confirmations, action); err != nil {
		failure := confirmationFailure(err)
		logConfirmationFailure(log, "decide-batch", binding.AccountID, failure.State, err)
		return ConfirmationDecisionResult{State: failure.State, RetryAfterMS: failure.RetryAfterMS}, nil
	}
	log.Info("Steam confirmations decided", "steamId64", binding.AccountID, "decision", decision, "count", len(confirmations))
	s.confirmationWindowMu.Lock()
	if operation == s.confirmationOperation {
		for _, handle := range trimmed {
			delete(s.confirmationRows, handle)
		}
	}
	s.confirmationWindowMu.Unlock()
	return ConfirmationDecisionResult{State: "ok"}, nil
}

func (s *Service) CancelConfirmations(token string) error {
	binding, _, err := s.confirmationAccount(token)
	if err != nil {
		return err
	}
	confirmationsLogger().Debug("cancelling Steam confirmation operation", "steamId64", binding.AccountID)
	s.confirmationWindowMu.Lock()
	if s.confirmationCancel != nil {
		s.confirmationCancel()
		s.confirmationCancel = nil
	}
	s.confirmationOperation++
	s.confirmationWindowMu.Unlock()
	return nil
}

func (s *Service) confirmationAccount(token string) (capability.Binding, mafile.Account, error) {
	s.confirmationWindowMu.Lock()
	if s.confirmationAccountID == "" || s.confirmationInstanceID == "" || s.capabilities == nil {
		s.confirmationWindowMu.Unlock()
		return capability.Binding{}, mafile.Account{}, capability.ErrInvalidCapability
	}
	binding := capability.Binding{
		WindowName: confirmationsWindowName, AccountID: s.confirmationAccountID,
		Scope: confirmationsCapabilityScope, LeaseID: s.confirmationInstanceID,
		VaultGeneration: s.confirmationGeneration,
	}
	if err := s.capabilities.Validate(binding, token); err != nil {
		s.confirmationWindowMu.Unlock()
		return capability.Binding{}, mafile.Account{}, err
	}
	s.confirmationWindowMu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.requireVaultLocked()
	if err != nil {
		return capability.Binding{}, mafile.Account{}, err
	}
	if v.IsLocked() {
		return capability.Binding{}, mafile.Account{}, vault.ErrLocked
	}
	if v.Generation() != binding.VaultGeneration {
		return capability.Binding{}, mafile.Account{}, capability.ErrInvalidCapability
	}
	records, err := v.List()
	if err != nil {
		return capability.Binding{}, mafile.Account{}, err
	}
	for _, record := range records {
		if record.SteamID64 == binding.AccountID {
			account, accountErr := accountFromRecord(v, record.ID)
			return binding, account, accountErr
		}
	}
	return capability.Binding{}, mafile.Account{}, ErrAccountNotFound
}

func confirmationCredentials(account mafile.Account, accountID string) (confirmationapi.Credentials, error) {
	if account.Session == nil || account.Session.SteamID == 0 || strconv.FormatUint(account.Session.SteamID, 10) != accountID {
		return confirmationapi.Credentials{}, ErrSteamMobileSession
	}
	credentials := confirmationapi.Credentials{
		SteamID: accountID, DeviceID: account.DeviceID, IdentitySecret: account.IdentitySecret,
		AccessToken: account.Session.AccessToken, SessionID: account.Session.SessionID,
	}
	if credentials.AccessToken == "" {
		return confirmationapi.Credentials{}, ErrSteamMobileSession
	}
	if credentials.SessionID == "" {
		// SDA-imported maFiles often carry no sessionid. Steam accepts any
		// client-chosen value, so mint one for this operation only; persisting it
		// would rotate the vault generation and kill the live capability.
		sessionID, err := newSessionID()
		if err != nil {
			return confirmationapi.Credentials{}, err
		}
		credentials.SessionID = sessionID
	}
	return credentials, nil
}

// newSessionID returns 32 uppercase hex characters, the shape Steam's own
// clients use for the sessionid cookie.
func newSessionID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", errors.Join(ErrSteamMobileSession, err)
	}
	return strings.ToUpper(hex.EncodeToString(raw)), nil
}

func confirmationFailure(err error) ConfirmationsResult {
	result := ConfirmationsResult{State: "error"}
	var apiErr *confirmationapi.Error
	if !errors.As(err, &apiErr) {
		return result
	}
	switch apiErr.Kind {
	case confirmationapi.FailureReauth:
		result.State = "reauth"
	case confirmationapi.FailureRateLimit:
		result.State = "rate-limit"
	case confirmationapi.FailureOffline:
		result.State = "offline"
	case confirmationapi.FailureCanceled:
		result.State = "canceled"
	}
	if apiErr.HasRetryAfter && apiErr.RetryAfter > 0 {
		result.RetryAfterMS = apiErr.RetryAfter.Milliseconds()
	}
	return result
}

// logConfirmationFailure records the underlying confirmationapi classification
// that confirmationFailure collapses into a single UI state string.
func logConfirmationFailure(log *slog.Logger, operation, steamID64, state string, err error) {
	attributes := []any{"operation", operation, "steamId64", steamID64, "state", state}
	var apiErr *confirmationapi.Error
	if errors.As(err, &apiErr) {
		attributes = append(attributes, "kind", string(apiErr.Kind))
		if apiErr.StatusCode != 0 {
			attributes = append(attributes, "status", apiErr.StatusCode)
		}
		if apiErr.HasRetryAfter {
			attributes = append(attributes, "retryAfter", apiErr.RetryAfter)
		}
	}
	if state == "reauth" {
		// Expected whenever the stored session went stale; the window answers it
		// with the "Login again" banner rather than an error.
		log.Info("Steam confirmations need re-authentication", attributes...)
		return
	}
	log.Warn("Steam confirmation request failed", append(attributes, "error", err)...)
}

func (s *Service) beginConfirmationOperation() (context.Context, uint64) {
	s.confirmationWindowMu.Lock()
	if s.confirmationCancel != nil {
		s.confirmationCancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), confirmationOperationTimeout)
	s.confirmationOperation++
	operation := s.confirmationOperation
	s.confirmationCancel = cancel
	s.confirmationWindowMu.Unlock()
	return ctx, operation
}

func (s *Service) endConfirmationOperation(operation uint64) {
	s.confirmationWindowMu.Lock()
	if operation == s.confirmationOperation && s.confirmationCancel != nil {
		s.confirmationCancel()
		s.confirmationCancel = nil
	}
	s.confirmationWindowMu.Unlock()
}

// beginConfirmationDetail returns the context for one row's detail fetch, on a
// cancel slot of its own.
//
// It used to share the window's single slot, which made the ten-second list poll
// and an opening trade cancel each other: a poll landing before the detail page
// arrived left the trade blank, and one landing part-way through its icons left
// some tiles empty — both fixed by closing the trade and opening it again, since
// by then the page and the images were cached. Opening another row still cancels
// the row before it, and tearing the window down still cancels whatever runs.
func (s *Service) beginConfirmationDetail() (context.Context, uint64) {
	s.confirmationWindowMu.Lock()
	if s.confirmationDetailCancel != nil {
		s.confirmationDetailCancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), confirmationOperationTimeout)
	s.confirmationDetailOperation++
	operation := s.confirmationDetailOperation
	s.confirmationDetailCancel = cancel
	s.confirmationWindowMu.Unlock()
	return ctx, operation
}

func (s *Service) endConfirmationDetail(operation uint64) {
	s.confirmationWindowMu.Lock()
	if operation == s.confirmationDetailOperation && s.confirmationDetailCancel != nil {
		s.confirmationDetailCancel()
		s.confirmationDetailCancel = nil
	}
	s.confirmationWindowMu.Unlock()
}

// holdConfirmationIcons marks that icons are being downloaded for something the
// current list does not know about, and returns the release. A refresh that
// pruned during that window deleted the images the moment they landed on disk,
// leaving an open trade with valid paths to files that no longer existed.
func (s *Service) holdConfirmationIcons() func() {
	s.confirmationWindowMu.Lock()
	s.confirmationDetailsInFlight++
	s.confirmationWindowMu.Unlock()
	return func() {
		s.confirmationWindowMu.Lock()
		s.confirmationDetailsInFlight--
		s.confirmationWindowMu.Unlock()
	}
}

func (s *Service) confirmationIconsBusy() bool {
	s.confirmationWindowMu.Lock()
	defer s.confirmationWindowMu.Unlock()
	return s.confirmationDetailsInFlight > 0
}

func (s *Service) resetConfirmationSession(clearContext bool) {
	s.confirmationWindowMu.Lock()
	s.resetConfirmationLocked(clearContext)
	s.confirmationWindowMu.Unlock()
}

func (s *Service) resetConfirmationLocked(clearContext bool) {
	if s.confirmationCancel != nil {
		s.confirmationCancel()
		s.confirmationCancel = nil
	}
	// The detail slot too: the window is going away, so a fetch still running
	// would keep writing icons into a folder that is about to be emptied.
	if s.confirmationDetailCancel != nil {
		s.confirmationDetailCancel()
		s.confirmationDetailCancel = nil
	}
	s.confirmationOperation++
	s.confirmationDetailOperation++
	clear(s.confirmationRows)
	if s.confirmationClient != nil {
		s.confirmationClient.CloseIdleConnections()
	}
	if clearContext {
		s.confirmationAccountID = ""
		s.confirmationGeneration = ""
		s.confirmationInstanceID = ""
	}
}

// ConfirmationTradeItemView is one item in a trade, with its image already served
// locally. The identifying triple stays so the item can be looked up in full.
type ConfirmationTradeItemView struct {
	AppID      string `json:"appId"`
	ClassID    string `json:"classId"`
	InstanceID string `json:"instanceId"`
	Icon       string `json:"icon,omitempty"`
}

type ConfirmationTradeSideView struct {
	Header string                      `json:"header"`
	Items  []ConfirmationTradeItemView `json:"items"`
}

// ConfirmationTradePartyView is the other side of a trade. Knowing who is
// receiving is the point of confirming one, so the name, avatar and profile link
// travel together.
type ConfirmationTradePartyView struct {
	Name       string `json:"name"`
	Avatar     string `json:"avatar,omitempty"`
	ProfileURL string `json:"profileUrl,omitempty"`
	Level      int    `json:"level,omitempty"`
	YearsBadge string `json:"yearsBadge,omitempty"`
}

type ConfirmationTradeView struct {
	Partner *ConfirmationTradePartyView `json:"partner,omitempty"`
	Give    ConfirmationTradeSideView   `json:"give"`
	Receive ConfirmationTradeSideView   `json:"receive"`
}

// ConfirmationListingView is a market listing reduced to what decides the answer:
// what the sale pays, how the market for the item looks, and what the item is.
//
// The amounts and counts travel as values, not as Steam's sentences, so the
// window words them from its own translations. Text and Prices carry Steam's
// wording only where a value could not be read.
type ConfirmationListingView struct {
	Receive   string                        `json:"receive,omitempty"`
	BuyerPays string                        `json:"buyerPays,omitempty"`
	Prices    []ConfirmationTextView        `json:"prices,omitempty"`
	Market    ConfirmationListingMarketView `json:"market"`
	Item      *ConfirmationItemView         `json:"item,omitempty"`
}

type ConfirmationListingMarketView struct {
	ForSale      int      `json:"forSale,omitempty"`
	ForSalePrice string   `json:"forSalePrice,omitempty"`
	SoldRecently int      `json:"soldRecently,omitempty"`
	Text         []string `json:"text,omitempty"`
}

// ConfirmationDetailsResult carries the per-confirmation detail Steam serves for
// one row. State mirrors ListConfirmations so the window handles a refused
// session the same way. DumpPath is set only when payload dumps are enabled.
type ConfirmationDetailsResult struct {
	State  string                 `json:"state"`
	Handle string                 `json:"handle"`
	Fields []ConfirmationTextView `json:"fields,omitempty"`
	// Trade is set only for trade offers.
	Trade *ConfirmationTradeView `json:"trade,omitempty"`
	// Listing is set only for market listings, and replaces Fields when it is.
	Listing  *ConfirmationListingView `json:"listing,omitempty"`
	DumpPath string                   `json:"dumpPath,omitempty"`
}

// InspectConfirmation loads the detail page for one confirmation. The list view
// carries only a headline, a few summary lines and a single icon, so this is the
// only source of anything richer — and with dumps enabled it records exactly what
// Steam sent, so what is actually available can be seen rather than assumed.
//
// Fetched when a row is selected rather than for the whole list: it is one
// request per confirmation, and batching them used to fail an entire refresh.
func (s *Service) InspectConfirmation(handle, token string) (ConfirmationDetailsResult, error) {
	binding, account, err := s.confirmationAccount(token)
	if err != nil {
		return ConfirmationDetailsResult{}, err
	}
	log := confirmationsLogger()
	handle = strings.TrimSpace(handle)
	credentials, err := confirmationCredentials(account, binding.AccountID)
	if err != nil {
		log.Warn("confirmation credentials unusable", "operation", "details", "steamId64", binding.AccountID, "error", err)
		return ConfirmationDetailsResult{State: "reauth", Handle: handle}, nil
	}
	s.confirmationWindowMu.Lock()
	confirmation, ok := s.confirmationRows[handle]
	s.confirmationWindowMu.Unlock()
	if !ok {
		return ConfirmationDetailsResult{}, capability.ErrInvalidCapability
	}

	ctx, operation := s.beginConfirmationDetail()
	defer s.endConfirmationDetail(operation)
	// Held until the icons this produces are recorded, so a refresh in between
	// cannot prune them.
	release := s.holdConfirmationIcons()
	defer release()
	details, err := s.confirmationClient.FetchDetails(ctx, credentials, confirmation)
	if err != nil {
		failure := confirmationFailure(err)
		logConfirmationFailure(log, "details", binding.AccountID, failure.State, err)
		return ConfirmationDetailsResult{State: failure.State, Handle: handle}, nil
	}
	dumpPath := dumpConfirmation(confirmation.Item(), details)
	fields := make([]ConfirmationTextView, 0, len(details.Fields))
	for _, field := range details.Fields {
		fields = append(fields, ConfirmationTextView{Label: field.Label, Value: field.Value})
	}
	trade := s.tradeView(ctx, details.Trade)
	listing := s.listingView(details.Listing)
	s.confirmationWindowMu.Lock()
	s.confirmationDetailIcons = tradeIcons(trade)
	s.confirmationWindowMu.Unlock()
	log.Debug("Steam confirmation detail loaded", "steamId64", binding.AccountID,
		"fields", len(fields), "trade", trade != nil, "listing", listing != nil)
	return ConfirmationDetailsResult{
		State: "ok", Handle: handle, Fields: fields,
		Trade: trade, Listing: listing, DumpPath: dumpPath,
	}, nil
}

// listingView renders a market listing for the window. Nothing here needs an
// image cached: a listing is one item, and the confirmation's own icon is it.
func (s *Service) listingView(listing *confirmationapi.ListingDetails) *ConfirmationListingView {
	if listing == nil {
		return nil
	}
	view := &ConfirmationListingView{
		Receive:   listing.Receive,
		BuyerPays: listing.BuyerPays,
		Prices:    make([]ConfirmationTextView, 0, len(listing.Prices)),
		Market: ConfirmationListingMarketView{
			ForSale:      listing.Market.ForSale,
			ForSalePrice: listing.Market.ForSalePrice,
			SoldRecently: listing.Market.SoldRecently,
			Text:         listing.Market.Text,
		},
	}
	for _, price := range listing.Prices {
		view.Prices = append(view.Prices, ConfirmationTextView{Label: price.Label, Value: price.Value})
	}
	if listing.Item != nil {
		item := itemDescriptionView(*listing.Item)
		view.Item = &item
	}
	return view
}

// tradeView caches every image the trade refers to and returns the view the window
// renders. Images go through the same sanitizing fetch as list icons, because the
// webview cannot load a Steam URL.
func (s *Service) tradeView(ctx context.Context, trade *confirmationapi.TradeDetails) *ConfirmationTradeView {
	if trade == nil {
		return nil
	}
	view := &ConfirmationTradeView{
		Give:    s.tradeSideView(ctx, trade.Give),
		Receive: s.tradeSideView(ctx, trade.Receive),
	}
	if trade.Partner != nil {
		view.Partner = &ConfirmationTradePartyView{
			Name:       trade.Partner.Name,
			Avatar:     s.confirmationIcons.Ensure(ctx, trade.Partner.AvatarURL),
			ProfileURL: trade.Partner.ProfileURL,
			Level:      trade.Partner.Level,
			YearsBadge: s.confirmationIcons.Ensure(ctx, trade.Partner.YearsBadgeURL),
		}
	}
	return view
}

// tradeIcons lists every locally served image an open trade uses, so a refresh
// running underneath it does not delete them.
func tradeIcons(trade *ConfirmationTradeView) []string {
	if trade == nil {
		return nil
	}
	icons := make([]string, 0, 16)
	if trade.Partner != nil {
		icons = append(icons, trade.Partner.Avatar, trade.Partner.YearsBadge)
	}
	for _, side := range []ConfirmationTradeSideView{trade.Give, trade.Receive} {
		for _, item := range side.Items {
			icons = append(icons, item.Icon)
		}
	}
	return icons
}

func (s *Service) tradeSideView(ctx context.Context, side confirmationapi.TradeSide) ConfirmationTradeSideView {
	items := make([]ConfirmationTradeItemView, 0, len(side.Items))
	for _, item := range side.Items {
		items = append(items, ConfirmationTradeItemView{
			AppID: item.AppID, ClassID: item.ClassID, InstanceID: item.InstanceID,
			Icon: s.confirmationIcons.Ensure(ctx, item.ImageURL),
		})
	}
	return ConfirmationTradeSideView{Header: side.Header, Items: items}
}

// ConfirmationItemView is the full description of one traded item, as Steam shows
// it when an item is hovered on the community site.
type ConfirmationItemView struct {
	State          string                 `json:"state"`
	Name           string                 `json:"name"`
	MarketHashName string                 `json:"marketHashName,omitempty"`
	Type           string                 `json:"type,omitempty"`
	NameColor      string                 `json:"nameColor,omitempty"`
	Icon           string                 `json:"icon,omitempty"`
	Tradable       bool                   `json:"tradable"`
	Marketable     bool                   `json:"marketable"`
	Descriptions   []ConfirmationItemLine `json:"descriptions,omitempty"`
	Tags           []ConfirmationItemTag  `json:"tags,omitempty"`
}

type ConfirmationItemLine struct {
	Value string `json:"value"`
	Color string `json:"color,omitempty"`
}

type ConfirmationItemTag struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
}

// itemView renders one of Steam's item descriptions and caches its image locally,
// which is what a hovered trade item needs. A market listing uses the description
// alone: the confirmation's own icon is already the item's image, so fetching it
// a second time would leave a file on disk that nothing ever shows.
func (s *Service) itemView(ctx context.Context, item confirmationapi.ItemClass) ConfirmationItemView {
	view := itemDescriptionView(item)
	view.Icon = s.confirmationIcons.Ensure(ctx, item.IconURL)
	return view
}

func itemDescriptionView(item confirmationapi.ItemClass) ConfirmationItemView {
	view := ConfirmationItemView{
		State: "ok", Name: item.Name, MarketHashName: item.MarketHashName, Type: item.Type,
		NameColor: item.NameColor, Tradable: item.Tradable, Marketable: item.Marketable,
	}
	for _, description := range item.Descriptions {
		view.Descriptions = append(view.Descriptions, ConfirmationItemLine{
			Value: description.Value, Color: description.Color,
		})
	}
	for _, tag := range item.Tags {
		view.Tags = append(view.Tags, ConfirmationItemTag{
			Category: tag.CategoryName, Name: tag.Name, Color: tag.Color,
		})
	}
	return view
}

// InspectTradeItem describes one item of an open trade. It is what makes a trade
// verifiable: the list says how many items are moving, this says which.
//
// Answers are cached for the window's lifetime, so hovering the same item
// repeatedly costs one request.
func (s *Service) InspectTradeItem(appID, classID, instanceID, token string) (ConfirmationItemView, error) {
	binding, account, err := s.confirmationAccount(token)
	if err != nil {
		return ConfirmationItemView{}, err
	}
	key := appID + "/" + classID + "/" + instanceID
	s.confirmationWindowMu.Lock()
	cached, ok := s.confirmationItems[key]
	s.confirmationWindowMu.Unlock()
	if ok {
		return cached, nil
	}

	log := confirmationsLogger()
	credentials, err := confirmationCredentials(account, binding.AccountID)
	if err != nil {
		return ConfirmationItemView{State: "reauth"}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), confirmationOperationTimeout)
	defer cancel()
	// Same hold as a detail fetch: this downloads an icon the current list has
	// never heard of, and a refresh would prune it out from under the hover.
	release := s.holdConfirmationIcons()
	defer release()
	item, err := s.confirmationClient.FetchItemClass(ctx, credentials, appID, classID, instanceID)
	if err != nil {
		failure := confirmationFailure(err)
		logConfirmationFailure(log, "itemclass", binding.AccountID, failure.State, err)
		return ConfirmationItemView{State: failure.State}, nil
	}
	view := s.itemView(ctx, item)
	s.confirmationWindowMu.Lock()
	if s.confirmationItems == nil {
		s.confirmationItems = make(map[string]ConfirmationItemView, 32)
	}
	s.confirmationItems[key] = view
	if view.Icon != "" {
		s.confirmationDetailIcons = append(s.confirmationDetailIcons, view.Icon)
	}
	s.confirmationWindowMu.Unlock()
	return view, nil
}
