package enrollmentflow

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"TcNo-Acc-Switcher/internal/steamguard/enrollmentapi"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
)

const (
	pendingKind               = "steamguard-enrollment-pending"
	pendingVersion            = 2
	legacyPendingVersion      = 1
	maxPendingBytes           = 32 << 10
	maxTokenBytes             = 8192
	maxPendingJSONDepth       = 16
	maxPendingJSONEntries     = 128
	maxPendingJSONStringBytes = 16 << 10
)

var deviceIDPattern = regexp.MustCompile(`^android:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type pendingRecord struct {
	Kind                   string                         `json:"kind"`
	Version                int                            `json:"version"`
	State                  enrollmentapi.State            `json:"state"`
	RequestID              []byte                         `json:"requestId"`
	SteamID                uint64                         `json:"steamId"`
	AccessToken            []byte                         `json:"accessToken"`
	DeviceID               string                         `json:"deviceId"`
	SharedSecret           []byte                         `json:"sharedSecret"`
	IdentitySecret         []byte                         `json:"identitySecret"`
	Secret1                []byte                         `json:"secret1"`
	RevocationCode         []byte                         `json:"revocationCode"`
	URI                    []byte                         `json:"uri"`
	SerialNumber           uint64                         `json:"serialNumber"`
	ServerTime             uint64                         `json:"serverTime"`
	AccountName            string                         `json:"accountName,omitempty"`
	TokenGID               string                         `json:"tokenGid,omitempty"`
	PhoneHint              string                         `json:"phoneHint,omitempty"`
	Confirmation           enrollmentapi.ConfirmationType `json:"confirmation"`
	RevocationViewed       bool                           `json:"revocationViewed,omitempty"`
	RevocationAcknowledged bool                           `json:"revocationAcknowledged,omitempty"`
	RetryAfterSeconds      int64                          `json:"retryAfterSeconds,omitempty"`
	HasRetryAfter          bool                           `json:"hasRetryAfter,omitempty"`
}

func pendingFromAPI(p *enrollmentapi.PendingEnrollment, state enrollmentapi.State) (pendingRecord, error) {
	if p == nil {
		return pendingRecord{}, ErrInvalidPendingState
	}
	record := pendingRecord{
		Kind:           pendingKind,
		Version:        pendingVersion,
		State:          state,
		RequestID:      append([]byte(nil), p.RequestID...),
		SteamID:        p.SteamID,
		AccessToken:    append([]byte(nil), p.AccessToken...),
		DeviceID:       p.DeviceID,
		SharedSecret:   append([]byte(nil), p.SharedSecret...),
		IdentitySecret: append([]byte(nil), p.IdentitySecret...),
		Secret1:        append([]byte(nil), p.Secret1...),
		RevocationCode: append([]byte(nil), p.RevocationCode...),
		URI:            append([]byte(nil), p.URI...),
		SerialNumber:   p.SerialNumber,
		ServerTime:     p.ServerTime,
		AccountName:    p.AccountName,
		TokenGID:       p.TokenGID,
		PhoneHint:      p.PhoneHint,
		Confirmation:   p.Confirmation,
	}
	if !validPendingRecord(&record) {
		record.destroy()
		return pendingRecord{}, ErrInvalidPendingState
	}
	return record, nil
}

func (p *pendingRecord) toAPI() *enrollmentapi.PendingEnrollment {
	return &enrollmentapi.PendingEnrollment{
		RequestID:      append([]byte(nil), p.RequestID...),
		SteamID:        p.SteamID,
		AccessToken:    append([]byte(nil), p.AccessToken...),
		DeviceID:       p.DeviceID,
		SharedSecret:   append([]byte(nil), p.SharedSecret...),
		IdentitySecret: append([]byte(nil), p.IdentitySecret...),
		Secret1:        append([]byte(nil), p.Secret1...),
		RevocationCode: append([]byte(nil), p.RevocationCode...),
		URI:            append([]byte(nil), p.URI...),
		SerialNumber:   p.SerialNumber,
		ServerTime:     p.ServerTime,
		AccountName:    p.AccountName,
		TokenGID:       p.TokenGID,
		PhoneHint:      p.PhoneHint,
		Confirmation:   p.Confirmation,
	}
}

func (p *pendingRecord) destroy() {
	if p == nil {
		return
	}
	wipe(p.RequestID)
	wipe(p.AccessToken)
	wipe(p.SharedSecret)
	wipe(p.IdentitySecret)
	wipe(p.Secret1)
	wipe(p.RevocationCode)
	wipe(p.URI)
	*p = pendingRecord{}
}

func encodePending(record *pendingRecord) ([]byte, error) {
	if record != nil {
		record.Version = pendingVersion
		record.RevocationViewed = false
	}
	if !validPendingRecord(record) {
		return nil, ErrInvalidPendingState
	}
	raw, err := json.Marshal(record)
	if err != nil || len(raw) == 0 || len(raw) > maxPendingBytes {
		wipe(raw)
		return nil, ErrInvalidPendingState
	}
	return raw, nil
}

func decodePending(raw []byte) (pendingRecord, error) {
	if len(raw) == 0 || len(raw) > maxPendingBytes || !utf8.Valid(raw) || !validPendingJSON(raw) {
		return pendingRecord{}, ErrInvalidPendingState
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var record pendingRecord
	if err := dec.Decode(&record); err != nil || dec.Decode(&struct{}{}) != io.EOF {
		record.destroy()
		return pendingRecord{}, ErrInvalidPendingState
	}
	if record.Version == legacyPendingVersion {
		// Version 1 persisted a one-time "viewed" marker before the user could
		// acknowledge the code. Treat it as unacknowledged so a crash cannot
		// permanently strand enrollment.
		record.Version = pendingVersion
		record.RevocationViewed = false
		record.RevocationAcknowledged = false
	}
	if !validPendingRecord(&record) {
		record.destroy()
		return pendingRecord{}, ErrInvalidPendingState
	}
	return record, nil
}

func validPendingJSON(raw []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if !readUniqueJSONValue(dec, 0) {
		return false
	}
	_, err := dec.Token()
	return err == io.EOF
}

func readUniqueJSONValue(dec *json.Decoder, depth int) bool {
	if depth > maxPendingJSONDepth {
		return false
	}
	token, err := dec.Token()
	if err != nil {
		return false
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			seen := make(map[string]struct{})
			for dec.More() {
				if len(seen) >= maxPendingJSONEntries {
					return false
				}
				keyToken, err := dec.Token()
				key, ok := keyToken.(string)
				if err != nil || !ok || len(key) > maxPendingJSONStringBytes || strings.ContainsRune(key, utf8.RuneError) {
					return false
				}
				if _, exists := seen[key]; exists {
					return false
				}
				seen[key] = struct{}{}
				if !readUniqueJSONValue(dec, depth+1) {
					return false
				}
			}
			end, err := dec.Token()
			return err == nil && end == json.Delim('}')
		case '[':
			entries := 0
			for dec.More() {
				if entries >= maxPendingJSONEntries || !readUniqueJSONValue(dec, depth+1) {
					return false
				}
				entries++
			}
			end, err := dec.Token()
			return err == nil && end == json.Delim(']')
		default:
			return false
		}
	case string:
		return len(value) <= maxPendingJSONStringBytes && !strings.ContainsRune(value, utf8.RuneError)
	case json.Number, bool, nil:
		return true
	default:
		return false
	}
}

func validPendingRecord(p *pendingRecord) bool {
	if p == nil || p.Kind != pendingKind || p.Version != pendingVersion || !validPendingState(p.State) ||
		!validSteamID(p.SteamID) || len(p.RequestID) != 16 || !validToken(p.AccessToken) ||
		!deviceIDPattern.MatchString(p.DeviceID) || len(p.SharedSecret) != 20 ||
		len(p.IdentitySecret) != 20 || len(p.Secret1) != 20 || !validRevocationCode(p.RevocationCode) ||
		len(p.URI) == 0 || len(p.URI) > 1024 || !utf8.Valid(p.URI) || !bytes.HasPrefix(p.URI, []byte("otpauth://totp/Steam:")) ||
		p.SerialNumber == 0 || p.ServerTime < 1230768000 || p.ServerTime > 4102444800 ||
		len(p.AccountName) > 64 || len(p.TokenGID) > 64 || len(p.PhoneHint) > 32 ||
		!validDisplayText(p.AccountName) || !validDisplayText(p.TokenGID) || !validDisplayText(p.PhoneHint) ||
		p.RevocationViewed || p.RetryAfterSeconds < 0 || (!p.HasRetryAfter && p.RetryAfterSeconds != 0) {
		return false
	}
	return p.Confirmation == enrollmentapi.ConfirmationUnknown ||
		p.Confirmation == enrollmentapi.ConfirmationSMS || p.Confirmation == enrollmentapi.ConfirmationEmail
}

func validPendingState(state enrollmentapi.State) bool {
	switch state {
	case enrollmentapi.StateAwaitingSMS, enrollmentapi.StateAwaitingEmail,
		enrollmentapi.StateAwaitingConfirmation, enrollmentapi.StateRateLimited,
		enrollmentapi.StateReauthenticationRequired, enrollmentapi.StateConfirmationCodeRejected,
		enrollmentapi.StateAuthenticatorCodeRetry:
		return true
	default:
		return false
	}
}

func validTerminalAddState(state enrollmentapi.State) bool {
	switch state {
	case enrollmentapi.StatePhoneRequired, enrollmentapi.StateAlreadyHasAuthenticator,
		enrollmentapi.StateRateLimited, enrollmentapi.StateReauthenticationRequired:
		return true
	default:
		return false
	}
}

func validFinalizeState(state enrollmentapi.State) bool {
	return state == enrollmentapi.StateComplete || validPendingState(state)
}

func validSteamID(value uint64) bool {
	const min = uint64(76561197960265728)
	return value >= min && value <= min+uint64(^uint32(0)) && value <= math.MaxInt64
}

func validToken(value []byte) bool {
	if len(value) == 0 || len(value) > maxTokenBytes {
		return false
	}
	for _, b := range value {
		if b < 0x21 || b > 0x7e {
			return false
		}
	}
	return true
}

func validRevocationCode(value []byte) bool {
	if len(value) < 6 || len(value) > 32 || value[0] != 'R' {
		return false
	}
	for _, b := range value[1:] {
		if !((b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z')) {
			return false
		}
	}
	return true
}

func validDisplayText(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func recordStatus(record *pendingRecord, resumed bool) Status {
	return Status{
		State:                   record.State,
		Confirmation:            record.Confirmation,
		PhoneHint:               record.PhoneHint,
		RetryAfterSeconds:       record.RetryAfterSeconds,
		HasRetryAfter:           record.HasRetryAfter,
		Pending:                 true,
		Resumed:                 resumed,
		RevocationViewAvailable: !record.RevocationAcknowledged,
	}
}

func statusFromAPI(state enrollmentapi.State, retryAfterSeconds int64, hasRetryAfter bool) Status {
	return Status{State: state, RetryAfterSeconds: retryAfterSeconds, HasRetryAfter: hasRetryAfter}
}

func activeMaFile(record *pendingRecord) ([]byte, error) {
	if !validPendingRecord(record) {
		return nil, ErrInvalidPendingState
	}
	account := mafile.Account{
		SharedSecret:   base64.StdEncoding.EncodeToString(record.SharedSecret),
		SerialNumber:   strconv.FormatUint(record.SerialNumber, 10),
		RevocationCode: string(record.RevocationCode),
		URI:            string(record.URI),
		ServerTime:     int64(record.ServerTime),
		AccountName:    record.AccountName,
		TokenGID:       record.TokenGID,
		IdentitySecret: base64.StdEncoding.EncodeToString(record.IdentitySecret),
		Secret1:        base64.StdEncoding.EncodeToString(record.Secret1),
		Status:         1,
		DeviceID:       record.DeviceID,
		FullyEnrolled:  true,
		Session: &mafile.SessionData{
			SteamID:     record.SteamID,
			AccessToken: string(record.AccessToken),
		},
	}
	return mafile.ExportPlaintext(account, mafile.ExportOptions{IncludeTokens: true})
}

func steamIDString(value uint64) string { return strconv.FormatUint(value, 10) }

func retrySeconds(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func cleanPhoneHint(value string) string { return strings.TrimSpace(value) }
