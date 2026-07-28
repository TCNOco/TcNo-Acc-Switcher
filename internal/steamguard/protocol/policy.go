package protocol

import (
	"net/url"
	"strings"
)

// Route selects the fixed host allowlist used for an initial request.
type Route uint8

const (
	RouteRequest Route = iota + 1
	RouteTransfer
)

func validateEndpoint(raw string, route Route) (*url.URL, *Error) {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Opaque != "" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return nil, protocolError(CodeInvalidRequest, StateInvalid)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return nil, protocolError(CodeSchemeDenied, StateDenied)
	}
	if port := u.Port(); port != "" && port != "443" {
		return nil, protocolError(CodeHostDenied, StateDenied)
	}

	host := strings.ToLower(u.Hostname())
	if !hostAllowed(host, route) {
		return nil, protocolError(CodeHostDenied, StateDenied)
	}
	u.Scheme = "https"
	return u, nil
}

func validateRedirect(u *url.URL) *Error {
	if u == nil || !u.IsAbs() || u.Opaque != "" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return protocolError(CodeRedirectDenied, StateDenied)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return protocolError(CodeRedirectDenied, StateDenied)
	}
	if port := u.Port(); port != "" && port != "443" {
		return protocolError(CodeRedirectDenied, StateDenied)
	}
	if !redirectHostAllowed(strings.ToLower(u.Hostname())) {
		return protocolError(CodeRedirectDenied, StateDenied)
	}
	return nil
}

func hostAllowed(host string, route Route) bool {
	switch route {
	case RouteRequest:
		switch host {
		case "api.steampowered.com", "login.steampowered.com", "steamcommunity.com":
			return true
		}
	case RouteTransfer:
		switch host {
		case "steamcommunity.com", "store.steampowered.com", "help.steampowered.com":
			return true
		}
	}
	return false
}

func redirectHostAllowed(host string) bool {
	switch host {
	case "api.steampowered.com",
		"login.steampowered.com",
		"steamcommunity.com",
		"store.steampowered.com",
		"help.steampowered.com":
		return true
	default:
		return false
	}
}
