// Package tradelink parses the trade URL out of Steam's trade-offer privacy
// page.
//
// An account's trade URL is only readable while authenticated as that account,
// and Steam serves it as undocumented HTML. So the parser's job is less
// "extract the value" than "refuse to guess": a page it does not recognise must
// be indistinguishable, to the caller, from a page it never fetched.
package tradelink

import (
	"bytes"
	"regexp"
	"strconv"
)

// maxBodyBytes matches the fetch's own response cap. A body larger than this was
// never a privacy page.
const maxBodyBytes = 512 << 10

// Outcome says whether the response may be trusted.
//
// The zero value is OutcomeUnrecognised, so a parser path that forgets to set it
// fails closed rather than reporting a confident answer.
type Outcome int

const (
	// OutcomeUnrecognised is a 200 that carries no trade URL this build can read.
	OutcomeUnrecognised Outcome = iota
	// OutcomeNotSignedIn is Steam's login page, served because the session cookie
	// was rejected.
	OutcomeNotSignedIn
	// OutcomeParsed is a page a trade URL was read from end to end.
	OutcomeParsed
)

func (o Outcome) String() string {
	switch o {
	case OutcomeNotSignedIn:
		return "not-signed-in"
	case OutcomeParsed:
		return "parsed"
	default:
		return "unrecognised"
	}
}

// Result is one page's reading.
type Result struct {
	Outcome Outcome
	// URL is rebuilt from Partner and Token rather than echoed out of the page,
	// so what reaches the clipboard is canonical and carries no HTML escaping.
	URL string
	// Partner is the 32-bit account id the link points at. The caller checks it
	// against the account it asked about: a link for anyone else is a bug, and a
	// silent one, since a wrong trade URL looks exactly like a right one.
	Partner string
	Token   string
}

// loginMarkers identify the sign-in page Steam serves when the session cookie is
// missing or stale. Redirects are disabled on the request, but Steam also renders
// the login page with a 200 in some flows, so the body is authoritative.
var loginMarkers = [][]byte{
	[]byte("g_steamID = false"),
	[]byte("<title>Sign In"),
}

// tradeURLPattern reads the link wherever it appears rather than from a named
// element. Steam has moved this page's markup more than once and renamed the
// field with it; the URL's own shape is the part that has stayed put.
//
// The `&amp;` alternative is not optional: the value arrives HTML-escaped inside
// an attribute, so a pattern matching only a bare `&` finds nothing on the real
// page.
var tradeURLPattern = regexp.MustCompile(
	`https?://(?:www\.)?steamcommunity\.com/tradeoffer/new/?\?partner=([0-9]{1,10})(?:&amp;|&)token=([A-Za-z0-9_-]{1,32})`)

// Parse reads a trade-offer privacy page.
func Parse(body []byte) Result {
	if len(body) == 0 || len(body) > maxBodyBytes {
		return Result{}
	}
	// Byte-scan before matching: a login page carries no trade URL, and saying
	// "this account has none" is a very different answer from "sign in again".
	for _, marker := range loginMarkers {
		if bytes.Contains(body, marker) {
			return Result{Outcome: OutcomeNotSignedIn}
		}
	}
	match := tradeURLPattern.FindSubmatch(body)
	if match == nil {
		return Result{}
	}
	partner := string(match[1])
	// A partner that does not round-trip as a 32-bit account id came from
	// something that merely looked like a trade URL.
	parsed, err := strconv.ParseUint(partner, 10, 32)
	if err != nil || parsed == 0 || strconv.FormatUint(parsed, 10) != partner {
		return Result{}
	}
	token := string(match[2])
	return Result{
		Outcome: OutcomeParsed,
		URL:     "https://steamcommunity.com/tradeoffer/new/?partner=" + partner + "&token=" + token,
		Partner: partner,
		Token:   token,
	}
}
