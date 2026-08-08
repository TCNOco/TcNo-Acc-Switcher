// Package loginrecord encodes the "login only" vault record: a Steam session
// with no authenticator behind it.
//
// It exists because an account can be useful to the switcher without ever
// enrolling an authenticator - a stored session is enough to ask Steam
// account-private questions, such as whether the account is on a CS2 cooldown.
// It cannot reuse mafile.Account: validateAccount hard-requires a 20-byte shared
// secret, a 20-byte identity secret and an android: device ID, and it runs on
// export as well as parse, so a secret-less record could be neither written nor
// read back.
//
// This package is separate from steamguard so sessionrefresh can use the type
// without an import cycle.
package loginrecord

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"unicode/utf8"

	"TcNo-Acc-Switcher/internal/steamguard/vaultrecord"
)

const (
	Version = 1

	maxRecordBytes      = 32 << 10
	maxTokenBytes       = 8192
	maxJSONDepth        = 16
	maxJSONEntries      = 128
	maxJSONStringBytes  = 16 << 10
	maxAccountNameBytes = 64
	maxSessionIDBytes   = 64
)

// ErrInvalidRecord covers every malformed-input case. It is deliberately single
// and opaque: the caller can do nothing differently per failure mode, and a
// detailed error over vault plaintext is a disclosure risk.
var ErrInvalidRecord = errors.New("invalid Steam Guard login-only record")

// Record is the persisted shape. Fields are strings rather than []byte because
// every consumer (the cookie builder, sessionrefresh) takes strings anyway, and
// a []byte here would imply a wipe guarantee Go strings cannot honour.
type Record struct {
	Kind         string `json:"kind"`
	Version      int    `json:"version"`
	SteamID      uint64 `json:"steamId"`
	AccountName  string `json:"accountName,omitempty"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	SessionID    string `json:"sessionId,omitempty"`
}

// New builds a Record with the envelope fields already set.
func New(steamID uint64, accountName, accessToken, refreshToken string) Record {
	return Record{
		Kind:         vaultrecord.KindStringLoginOnly,
		Version:      Version,
		SteamID:      steamID,
		AccountName:  strings.TrimSpace(accountName),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
}

// Destroy zeroes the record. Go strings cannot be wiped in place, so this drops
// references rather than scrubbing memory - the same guarantee the rest of the
// package's callers already live with.
func (r *Record) Destroy() {
	if r == nil {
		return
	}
	*r = Record{}
}

func Encode(record Record) ([]byte, error) {
	record.Kind = vaultrecord.KindStringLoginOnly
	record.Version = Version
	if !record.valid() {
		return nil, ErrInvalidRecord
	}
	raw, err := json.Marshal(record)
	if err != nil || len(raw) == 0 || len(raw) > maxRecordBytes {
		return nil, ErrInvalidRecord
	}
	return raw, nil
}

func Decode(raw []byte) (Record, error) {
	if len(raw) == 0 || len(raw) > maxRecordBytes || !utf8.Valid(raw) || !validJSON(raw) {
		return Record{}, ErrInvalidRecord
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var record Record
	if err := dec.Decode(&record); err != nil || dec.Decode(&struct{}{}) != io.EOF {
		record.Destroy()
		return Record{}, ErrInvalidRecord
	}
	if !record.valid() {
		record.Destroy()
		return Record{}, ErrInvalidRecord
	}
	return record, nil
}

func (r Record) valid() bool {
	return r.Kind == vaultrecord.KindStringLoginOnly &&
		r.Version == Version &&
		validSteamID(r.SteamID) &&
		validToken(r.AccessToken) &&
		(r.RefreshToken == "" || validToken(r.RefreshToken)) &&
		len(r.AccountName) <= maxAccountNameBytes && validDisplayText(r.AccountName) &&
		len(r.SessionID) <= maxSessionIDBytes && validDisplayText(r.SessionID)
}

// validJSON rejects duplicate keys, excessive nesting and oversized strings
// before the real decode runs, mirroring enrollmentflow's validPendingJSON. A
// duplicate key is the interesting one: encoding/json silently takes the last
// occurrence, so without this a crafted record could carry a decoy token.
func validJSON(raw []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if !readUniqueJSONValue(dec, 0) {
		return false
	}
	_, err := dec.Token()
	return err == io.EOF
}

func readUniqueJSONValue(dec *json.Decoder, depth int) bool {
	if depth > maxJSONDepth {
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
				if len(seen) >= maxJSONEntries {
					return false
				}
				keyToken, err := dec.Token()
				key, ok := keyToken.(string)
				if err != nil || !ok || len(key) > maxJSONStringBytes || strings.ContainsRune(key, utf8.RuneError) {
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
				if entries >= maxJSONEntries || !readUniqueJSONValue(dec, depth+1) {
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
		return len(value) <= maxJSONStringBytes && !strings.ContainsRune(value, utf8.RuneError)
	case json.Number, bool, nil:
		return true
	default:
		return false
	}
}

func validSteamID(value uint64) bool {
	const min = uint64(76561197960265728)
	return value >= min && value <= min+uint64(^uint32(0)) && value <= math.MaxInt64
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
