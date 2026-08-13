// Package confirmationapi implements Steam mobile confirmations behind the
// hardened Steam protocol transport. It never returns raw HTML or credentials.
package confirmationapi

import (
	"context"
	"errors"
	"time"
)

const (
	CommunityOrigin  = "https://steamcommunity.com"
	ListEndpoint     = CommunityOrigin + "/mobileconf/getlist"
	DetailsBase      = CommunityOrigin + "/mobileconf/details/"
	DecisionEndpoint = CommunityOrigin + "/mobileconf/ajaxop"
	// MultiDecisionEndpoint decides several confirmations in one POST, the way
	// the Steam app submits a multi-select.
	MultiDecisionEndpoint = CommunityOrigin + "/mobileconf/multiajaxop"
	RequestTimeout        = 15 * time.Second
	// MobileUserAgent matches the Steam mobile app's HTTP client. Steam serves
	// mobileconf only to that fingerprint.
	MobileUserAgent = "okhttp/3.12.12"
)

// FailureKind is a stable service-facing failure classification.
type FailureKind string

const (
	FailureInvalid   FailureKind = "invalid"
	FailureCanceled  FailureKind = "canceled"
	FailureTimeout   FailureKind = "timeout"
	FailureRateLimit FailureKind = "rate-limit"
	FailureReauth    FailureKind = "reauth"
	FailureOffline   FailureKind = "offline"
	FailureRetryable FailureKind = "retryable"
	FailureFailed    FailureKind = "failed"
	// FailureRefused is Steam answering the signed request and declining it
	// (success:false without needauth). Unlike FailureFailed it is a verdict
	// about the session, not about the transport or the response shape.
	FailureRefused FailureKind = "refused"
)

// Error contains no URL, token, cookie, response body, or underlying network
// error, so it is safe to pass through logs or Wails bindings.
type Error struct {
	Kind          FailureKind
	StatusCode    int
	RetryAfter    time.Duration
	HasRetryAfter bool
	// Detail is the transport's fixed label for which check refused the request,
	// carried through because Kind alone collapses very different causes onto the
	// same word. A denied redirect and a rejected session both read as reauth,
	// and telling them apart from a log line is the difference between a fix and
	// a guess.
	Detail string
}

func (e *Error) Error() string {
	if e == nil {
		return "Steam confirmation request failed"
	}
	switch e.Kind {
	case FailureInvalid:
		return "Steam confirmation request is invalid"
	case FailureCanceled:
		return "Steam confirmation request was canceled"
	case FailureTimeout:
		return "Steam confirmation request timed out"
	case FailureRateLimit:
		return "Steam confirmation requests are rate limited"
	case FailureReauth:
		return "Steam confirmation session requires authentication"
	case FailureOffline:
		return "Steam confirmation request is unavailable offline"
	case FailureRetryable:
		return "Steam confirmation request temporarily failed"
	case FailureRefused:
		return "Steam refused the confirmation request"
	default:
		return "Steam confirmation request failed"
	}
}

func (e *Error) Is(target error) bool {
	return e != nil && ((e.Kind == FailureCanceled && target == context.Canceled) ||
		(e.Kind == FailureTimeout && target == context.DeadlineExceeded))
}

// Credentials are supplied from an already-unlocked vault for each operation.
type Credentials struct {
	SteamID        string
	DeviceID       string
	IdentitySecret string
	AccessToken    string
	SessionID      string
}

func (Credentials) String() string   { return "Steam confirmation credentials [redacted]" }
func (Credentials) GoString() string { return "confirmationapi.Credentials{[redacted]}" }

// Item is the HTML-free DTO safe to expose to the frontend.
type Item struct {
	Handle string `json:"handle"`
	Type   int    `json:"type"`
	// TypeLabel is derived from Type. TypeName is Steam's own label when the row
	// carries one, and is preferred for display.
	TypeLabel    string   `json:"typeLabel"`
	TypeName     string   `json:"typeName,omitempty"`
	Title        string   `json:"title"`
	Summary      []string `json:"summary"`
	Icon         string   `json:"icon,omitempty"`
	CreationTime int64    `json:"creationTime,omitempty"`
	AcceptLabel  string   `json:"acceptLabel"`
	DenyLabel    string   `json:"denyLabel"`
}

// Confirmation contains the public item plus private Steam decision keys.
type Confirmation struct {
	item    Item
	id      string
	nonce   string
	creator string
}

func (c Confirmation) Item() Item {
	item := c.item
	item.Summary = append([]string(nil), c.item.Summary...)
	return item
}

func (c Confirmation) String() string {
	return "Steam confirmation " + c.item.Handle
}

func (c Confirmation) GoString() string {
	return "confirmationapi.Confirmation{Handle:" + c.item.Handle + "}"
}

// TextField is plain text extracted from Steam's details HTML.
type TextField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Details contains structured, plain-text confirmation information.
type Details struct {
	Handle string      `json:"handle"`
	Fields []TextField `json:"fields"`
	// Trade is set only for trade offers, whose detail page describes both sides
	// of the exchange as structured items rather than as sentences.
	Trade *TradeDetails `json:"trade,omitempty"`
	// Listing is set only for market listings, whose page is mostly boilerplate
	// wrapped around a few prices. When it is set, Fields is empty: everything
	// worth reading is here instead.
	Listing *ListingDetails `json:"listing,omitempty"`
	// Raw is the response body the fields were parsed from. It exists so a
	// diagnostics dump can show what Steam actually sends, which the parser
	// reduces to visible text. It carries trade details: never log it.
	Raw []byte `json:"-"`
}

// Decision selects Steam's allow or cancel operation.
type Decision string

const (
	Allow Decision = "allow"
	Deny  Decision = "deny"
)

var errInvalidResponse = errors.New("invalid confirmation response")
