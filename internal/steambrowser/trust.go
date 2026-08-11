// Package steambrowser opens a browser window already signed in as a chosen
// account, using the session the Steam Guard vault already holds.
//
// Everything in this package is portable except the host backend. Trust
// classification, certificate reporting and session minting must stay free of
// build tags and of any webview type, so a macOS or Linux backend only has to
// supply host_<goos>.go.
package steambrowser

import (
	"net/url"
	"strings"
)

// Platform names the account platform a window belongs to. Steam is the only one
// today; the map below is keyed by platform so another one contributes its
// domains without touching how matching works.
type Platform string

const PlatformSteam Platform = "Steam"

// trustedDomains lists the registrable domains a window may show while still
// presenting itself as the platform's own site. Subdomains are included by the
// suffix rule in hostMatches, so store./help./login./api. need no entry.
var trustedDomains = map[Platform][]string{
	PlatformSteam: {
		"steamcommunity.com",
		"steampowered.com",
		"steamstatic.com", // Steam's asset and CDN domain
	},
}

// PageTrust is what the URL bar paints itself from.
type PageTrust struct {
	URL  string `json:"url"`
	Host string `json:"host"`
	// Secure reports an https origin. A trusted page is always secure; the two
	// are separate because an http page on a listed domain is still not safe.
	Secure  bool `json:"secure"`
	Trusted bool `json:"trusted"`
}

// Classify describes a URL for display. An unparseable or non-https URL is
// reported as neither secure nor trusted rather than rejected, because the URL
// bar still has to show something.
func Classify(platform Platform, rawURL string) PageTrust {
	trust := PageTrust{URL: rawURL}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return trust
	}
	trust.Host = canonicalHost(parsed.Hostname())
	trust.Secure = strings.EqualFold(parsed.Scheme, "https")
	trust.Trusted = trust.Secure && hostTrusted(platform, trust.Host)
	return trust
}

// IsTrusted reports whether a URL may be opened in a session window rather than
// handed to the system browser. It is derived here rather than taken from the
// frontend, so a value that crossed the boundary cannot widen the list.
func IsTrusted(platform Platform, rawURL string) bool {
	return Classify(platform, rawURL).Trusted
}

// canonicalHost lowercases a hostname and drops the root label's trailing dot,
// so "STEAMCOMMUNITY.COM." and "steamcommunity.com" compare equal.
func canonicalHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(host), ".")
}

// hostTrusted matches a host against the platform's domains, either exactly or
// as a subdomain.
//
// The suffix always includes the separating dot. Matching on a bare suffix would
// accept notsteampowered.com, and matching on a substring would accept
// steampowered.com.evil.tld — both of which would paint an attacker's page with
// the trusted styling.
func hostTrusted(platform Platform, host string) bool {
	if host == "" {
		return false
	}
	for _, domain := range trustedDomains[platform] {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

// TrustedDomains returns a platform's domains, for display in the UI. The copy
// keeps a caller from mutating the table.
func TrustedDomains(platform Platform) []string {
	domains := trustedDomains[platform]
	out := make([]string, len(domains))
	copy(out, domains)
	return out
}
