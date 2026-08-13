package tradelink

import (
	"strings"
	"testing"
)

// page wraps a trade URL the way the privacy page does: inside an attribute, so
// the ampersand arrives escaped.
func page(url string) []byte {
	return []byte(`<html><head><title>Steam Community</title></head><body>` +
		`<input type="text" id="trade_offer_access_url" value="` + url + `" readonly>` +
		`</body></html>`)
}

func TestParseReadsTheLinkAndCanonicalisesIt(t *testing.T) {
	result := Parse(page("https://steamcommunity.com/tradeoffer/new/?partner=12345678&amp;token=aB3-_xYz"))

	if result.Outcome != OutcomeParsed {
		t.Fatalf("outcome = %v, want parsed", result.Outcome)
	}
	if result.Partner != "12345678" || result.Token != "aB3-_xYz" {
		t.Fatalf("partner/token = %q/%q", result.Partner, result.Token)
	}
	// The escaping must not survive into the clipboard.
	want := "https://steamcommunity.com/tradeoffer/new/?partner=12345678&token=aB3-_xYz"
	if result.URL != want {
		t.Fatalf("url = %q, want %q", result.URL, want)
	}
}

func TestParseAcceptsTheShapesSteamActuallyServes(t *testing.T) {
	cases := map[string]string{
		"bare ampersand":  "https://steamcommunity.com/tradeoffer/new/?partner=1&token=aA1",
		"www host":        "https://www.steamcommunity.com/tradeoffer/new/?partner=1&amp;token=aA1",
		"http scheme":     "http://steamcommunity.com/tradeoffer/new/?partner=1&amp;token=aA1",
		"no trailing sla": "https://steamcommunity.com/tradeoffer/new?partner=1&amp;token=aA1",
	}
	for name, url := range cases {
		t.Run(name, func(t *testing.T) {
			result := Parse(page(url))
			if result.Outcome != OutcomeParsed {
				t.Fatalf("outcome = %v, want parsed", result.Outcome)
			}
			// Whatever shape it arrived in, one canonical form comes back out.
			if result.URL != "https://steamcommunity.com/tradeoffer/new/?partner=1&token=aA1" {
				t.Fatalf("url = %q", result.URL)
			}
		})
	}
}

func TestParseReportsTheLoginPageRatherThanNoLink(t *testing.T) {
	// The distinction is the whole point: "you have no trade URL" would send the
	// user to Steam's settings, when what they need is to sign in again.
	for _, marker := range []string{"g_steamID = false", "<title>Sign In</title>"} {
		result := Parse([]byte(`<html><head>` + marker + `</head><body></body></html>`))
		if result.Outcome != OutcomeNotSignedIn {
			t.Fatalf("%q: outcome = %v, want not-signed-in", marker, result.Outcome)
		}
		if result.URL != "" {
			t.Fatalf("%q: url = %q, want empty", marker, result.URL)
		}
	}
}

func TestParseFailsClosed(t *testing.T) {
	cases := map[string][]byte{
		"empty":              nil,
		"unrelated page":     []byte(`<html><body>Nothing to see</body></html>`),
		"oversized":          []byte(strings.Repeat("x", maxBodyBytes+1)),
		"link but no token":  page("https://steamcommunity.com/tradeoffer/new/?partner=12345678"),
		"partner not a numb": page("https://steamcommunity.com/tradeoffer/new/?partner=abc&amp;token=aA1"),
		"partner zero":       page("https://steamcommunity.com/tradeoffer/new/?partner=0&amp;token=aA1"),
		"partner overflows":  page("https://steamcommunity.com/tradeoffer/new/?partner=4294967296&amp;token=aA1"),
		"another host":       page("https://evil.example.com/tradeoffer/new/?partner=1&amp;token=aA1"),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			result := Parse(body)
			if result.Outcome != OutcomeUnrecognised {
				t.Fatalf("outcome = %v, want unrecognised", result.Outcome)
			}
			if result.URL != "" || result.Partner != "" || result.Token != "" {
				t.Fatalf("unrecognised result carried data: %+v", result)
			}
		})
	}
}

// A login page that also happens to mention a trade URL must still be reported
// as a login page: the URL on it would not be this account's.
func TestParsePrefersTheLoginVerdict(t *testing.T) {
	body := []byte(`<html><head><title>Sign In</title></head><body>` +
		`https://steamcommunity.com/tradeoffer/new/?partner=1&amp;token=aA1</body></html>`)
	if got := Parse(body).Outcome; got != OutcomeNotSignedIn {
		t.Fatalf("outcome = %v, want not-signed-in", got)
	}
}
