// Package vaultrecord identifies which shape a decrypted vault record holds.
//
// The vault itself is a blob store: it indexes by SteamID64 and knows nothing
// about the plaintext. Three shapes exist, and they are told apart by a
// top-level "kind" key. SDA maFiles have no schema envelope, so their absence of
// a "kind" IS the discriminator - mafile.ParsePlaintext deliberately rejects any
// record that carries one.
package vaultrecord

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"
)

// Kind is the shape of a decrypted vault record.
type Kind int

const (
	// KindUnknown is a record carrying a "kind" this build does not recognise.
	// Treat it as unreadable rather than guessing; a newer build may own it.
	KindUnknown Kind = iota
	// KindMaFile is an SDA-format authenticator. No "kind" key.
	KindMaFile
	// KindLoginOnly is a session-only record: tokens, no authenticator secrets.
	KindLoginOnly
	// KindEnrollmentPending is a half-finished authenticator enrollment.
	KindEnrollmentPending
)

// Kind strings as they appear in the record JSON. KindStringEnrollmentPending
// must stay in step with enrollmentflow's own constant; a test asserts it.
const (
	KindStringLoginOnly         = "steamguard-login-only"
	KindStringEnrollmentPending = "steamguard-enrollment-pending"
)

// maxSniffBytes bounds the decode. Every record shape is far smaller than this,
// and a record larger than it fails its own codec anyway.
const maxSniffBytes = 512 << 10

func (k Kind) String() string {
	switch k {
	case KindMaFile:
		return "authenticator"
	case KindLoginOnly:
		return "login-only"
	case KindEnrollmentPending:
		return "enrollment-pending"
	default:
		return "unknown"
	}
}

// Sniff reports the shape of raw without fully decoding it.
//
// It reads only the top-level "kind" string. Anything that is not valid JSON,
// not a JSON object, or oversized reports KindUnknown - callers must treat that
// as "cannot read", never as a default shape.
func Sniff(raw []byte) Kind {
	if len(raw) == 0 || len(raw) > maxSniffBytes || !utf8.Valid(raw) {
		return KindUnknown
	}
	// Require a JSON object. Every record shape is one, and without this check a
	// bare `null` would decode cleanly into the marker struct and be reported as
	// a maFile purely because it has no "kind".
	if trimmed := bytes.TrimLeft(raw, " \t\r\n"); len(trimmed) == 0 || trimmed[0] != '{' {
		return KindUnknown
	}
	var marker struct {
		Kind *string `json:"kind"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&marker); err != nil {
		return KindUnknown
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return KindUnknown
	}
	if marker.Kind == nil {
		// No envelope at all. That is what an SDA maFile looks like, and the
		// only shape mafile.ParsePlaintext will accept.
		return KindMaFile
	}
	switch *marker.Kind {
	case KindStringLoginOnly:
		return KindLoginOnly
	case KindStringEnrollmentPending:
		return KindEnrollmentPending
	default:
		return KindUnknown
	}
}
