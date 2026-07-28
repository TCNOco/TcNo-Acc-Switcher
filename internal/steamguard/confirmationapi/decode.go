package confirmationapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	maxConfirmations = 256
	maxSummaryLines  = 16
	maxTextInput     = 4096
	maxTextOutput    = 1024
	maxIconURL       = 512
)

type decimalString string

func (d *decimalString) UnmarshalJSON(raw []byte) error {
	if d == nil || len(raw) == 0 {
		return errInvalidResponse
	}
	var value string
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &value); err != nil {
			return errInvalidResponse
		}
	} else {
		value = string(raw)
	}
	if !validDecimal(value) {
		return errInvalidResponse
	}
	*d = decimalString(value)
	return nil
}

type unsignedString string

func (u *unsignedString) UnmarshalJSON(raw []byte) error {
	if u == nil || len(raw) == 0 {
		return errInvalidResponse
	}
	var value string
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &value); err != nil {
			return errInvalidResponse
		}
	} else {
		value = string(raw)
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || strconv.FormatUint(parsed, 10) != value {
		return errInvalidResponse
	}
	*u = unsignedString(value)
	return nil
}

type confirmationWire struct {
	ID        decimalString   `json:"id"`
	Nonce     decimalString   `json:"nonce"`
	CreatorID unsignedString  `json:"creator_id"`
	Headline  string          `json:"headline"`
	Summary   []string        `json:"summary"`
	Accept    string          `json:"accept"`
	Cancel    string          `json:"cancel"`
	Icon      string          `json:"icon"`
	Type      json.RawMessage `json:"type"`
	// Live getlist rows also carry these. TypeName and CreationTime are rendered;
	// Multi and Warn are accepted as raw values so a shape change cannot fail the
	// decode.
	TypeName     string          `json:"type_name"`
	CreationTime json.RawMessage `json:"creation_time"`
	Multi        json.RawMessage `json:"multi"`
	Warn         json.RawMessage `json:"warn"`
}

// lostAuthMarker is the mobile-era signal for a dead session: instead of the
// JSON envelope, Steam serves an HTML page whose links carry the
// steammobile://lostauth scheme.
var lostAuthMarker = []byte("lostauth")

func decodeList(raw []byte) ([]Confirmation, error) {
	var envelope struct {
		Success  bool               `json:"success"`
		Message  string             `json:"message"`
		NeedAuth bool               `json:"needauth"`
		Items    []confirmationWire `json:"conf"`
	}
	if err := decodeEnvelope(raw, &envelope); err != nil {
		if bytes.Contains(raw, lostAuthMarker) {
			clientLogger().Info("mobileconf session lost", "marker", "lostauth", "bodyBytes", len(raw))
			return nil, &Error{Kind: FailureReauth}
		}
		// The shape of what Steam sent instead is the only safe clue; the body
		// itself is never logged.
		trimmed := bytes.TrimLeft(raw, " \t\r\n")
		clientLogger().Warn("mobileconf getlist body was not the JSON envelope",
			"bodyBytes", len(raw), "htmlLike", len(trimmed) > 0 && trimmed[0] == '<')
		return nil, &Error{Kind: FailureFailed}
	}
	if envelope.NeedAuth {
		return nil, &Error{Kind: FailureReauth}
	}
	if !envelope.Success {
		// Steam answered 200 but refused the list. Its message is generic
		// service prose ("Oh nooo, we were unable to load..."), not account
		// data, and it is the only clue this failure mode leaves — the body
		// itself is never logged. Refused, not failed: the session probe
		// treats this verdict as a session that needs replacing.
		message := []rune(strings.Join(strings.Fields(envelope.Message), " "))
		if len(message) > 160 {
			message = message[:160]
		}
		clientLogger().Warn("mobileconf getlist refused",
			"confCount", len(envelope.Items), "message", string(message))
		return nil, &Error{Kind: FailureRefused}
	}
	if len(envelope.Items) > maxConfirmations {
		return nil, &Error{Kind: FailureFailed}
	}
	confirmations := make([]Confirmation, 0, len(envelope.Items))
	seen := make(map[string]struct{}, len(envelope.Items))
	for _, wire := range envelope.Items {
		id, nonce := string(wire.ID), string(wire.Nonce)
		handle := stableHandle(id)
		if _, duplicate := seen[handle]; duplicate {
			return nil, rejectRow("duplicate-id")
		}
		seen[handle] = struct{}{}
		confirmationType, err := parseConfirmationType(wire.Type)
		if err != nil {
			// The raw type is an enum number or name, never account data, and
			// it is exactly what a new Steam confirmation kind would trip on.
			value := wire.Type
			if len(value) > 32 {
				value = value[:32]
			}
			clientLogger().Warn("mobileconf confirmation row rejected", "reason", "type", "type", string(value))
			return nil, &Error{Kind: FailureFailed}
		}
		if len(wire.Summary) > maxSummaryLines {
			return nil, rejectRow("summary-count")
		}
		if len(wire.Icon) > maxTextInput {
			return nil, rejectRow("icon-length")
		}
		title, err := safeText(wire.Headline)
		if err != nil {
			return nil, rejectRow("headline")
		}
		accept, err := safeText(wire.Accept)
		if err != nil {
			return nil, rejectRow("accept-label")
		}
		deny, err := safeText(wire.Cancel)
		if err != nil {
			return nil, rejectRow("cancel-label")
		}
		summary := make([]string, 0, len(wire.Summary))
		for _, line := range wire.Summary {
			plain, err := safeText(line)
			if err != nil {
				return nil, rejectRow("summary-text")
			}
			if plain != "" {
				summary = append(summary, plain)
			}
		}
		typeName, err := safeText(wire.TypeName)
		if err != nil {
			return nil, rejectRow("type-name")
		}
		confirmations = append(confirmations, Confirmation{
			item: Item{
				Handle: handle, Type: confirmationType, TypeLabel: typeLabel(confirmationType),
				TypeName: typeName, Title: title, Summary: summary, AcceptLabel: accept, DenyLabel: deny,
				// A listing's icon is an economy image, and Steam sizes the one in
				// this list for its own phone row. Avatars carry no size segment and
				// pass through untouched.
				Icon:         ResizeEconomyImage(safeIconURL(wire.Icon)),
				CreationTime: parseCreationTime(wire.CreationTime),
			},
			id: id, nonce: nonce, creator: string(wire.CreatorID),
		})
	}
	return confirmations, nil
}

// rejectRow names the check a confirmation row failed — the field, never its
// content — so a rejected list is diagnosable from the log.
func rejectRow(reason string) error {
	clientLogger().Warn("mobileconf confirmation row rejected", "reason", reason)
	return &Error{Kind: FailureFailed}
}

func decodeDecision(raw []byte) error {
	var envelope struct {
		Success  bool   `json:"success"`
		NeedAuth bool   `json:"needauth"`
		Message  string `json:"message"`
	}
	if err := decodeEnvelope(raw, &envelope); err != nil {
		if bytes.Contains(raw, lostAuthMarker) {
			return &Error{Kind: FailureReauth}
		}
		return &Error{Kind: FailureFailed}
	}
	if envelope.NeedAuth {
		return &Error{Kind: FailureReauth}
	}
	if !envelope.Success {
		return &Error{Kind: FailureFailed}
	}
	return nil
}

// decodeEnvelope reads exactly one JSON document. Unknown fields are tolerated
// because Steam adds keys to mobileconf responses without notice; trailing data
// is still rejected.
func decodeEnvelope(raw []byte, destination any) error {
	if len(raw) == 0 {
		return errInvalidResponse
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return errInvalidResponse
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errInvalidResponse
	}
	return nil
}

func parseConfirmationType(raw json.RawMessage) (int, error) {
	if len(raw) == 0 {
		return 0, errInvalidResponse
	}
	value := string(raw)
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &value); err != nil {
			return 0, errInvalidResponse
		}
		if named, ok := map[string]int{
			"Invalid": 0, "Test": 1, "Trade": 2, "MarketListing": 3,
			"FeatureOptOut": 4, "PhoneNumberChange": 5, "AccountRecovery": 6,
		}[value]; ok {
			return named, nil
		}
	}
	// Any small integer is a valid type: Steam adds confirmation kinds without
	// notice (API key requests, family invites, ...), and an unknown kind must
	// render as a generic confirmation rather than poison the whole list. The
	// named cases above and typeLabel cover the kinds that get richer handling.
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 || parsed > 9999 || strconv.Itoa(parsed) != value {
		return 0, errInvalidResponse
	}
	return parsed, nil
}

// safeIconURL keeps only bounded absolute HTTPS icon URLs. Anything else, such
// as a relative or javascript: value, is dropped rather than rejected.
func safeIconURL(raw string) string {
	if len(raw) > maxIconURL || !strings.HasPrefix(raw, "https://") {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return ""
	}
	return parsed.String()
}

// parseCreationTime accepts Steam's Unix seconds as a number or a string and
// returns 0 when it is absent or unusable.
func parseCreationTime(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	value := string(raw)
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &value); err != nil {
			return 0
		}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func typeLabel(value int) string {
	switch value {
	case 1:
		return "Test"
	case 2:
		return "Trade"
	case 3:
		return "Market listing"
	case 4:
		return "Feature opt-out"
	case 5:
		return "Phone number change"
	case 6:
		return "Account recovery"
	default:
		return "Confirmation"
	}
}

func safeText(raw string) (string, error) {
	if len(raw) > maxTextInput {
		return "", errInvalidResponse
	}
	contextNode := &nethtml.Node{Type: nethtml.ElementNode, DataAtom: atom.Body, Data: "body"}
	nodes, err := nethtml.ParseFragment(strings.NewReader(raw), contextNode)
	if err != nil {
		return "", errInvalidResponse
	}
	plain := visibleNodeText(nodes)
	plain = strings.Join(strings.FieldsFunc(plain, unicode.IsSpace), " ")
	if len(plain) > maxTextOutput {
		return "", errInvalidResponse
	}
	return plain, nil
}
