// Package systemproxy resolves the user's proxy for outbound HTTP requests.
package systemproxy

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var errInvalidProxy = errors.New("system proxy configuration is invalid")

// proxyForConfig interprets Windows' manual proxy map or a WinHTTP PAC result.
// A configured but invalid proxy fails the request instead of going direct.
func proxyForConfig(target *url.URL, server, bypass string) (*url.URL, error) {
	if bypasses(target, bypass) {
		return nil, nil
	}
	var fallback, socks, selected string
	for _, entry := range strings.FieldsFunc(server, func(r rune) bool { return r == ';' || r == ' ' || r == '\t' }) {
		key, value, mapped := strings.Cut(entry, "=")
		if mapped && strings.TrimSpace(value) == "" {
			return nil, errInvalidProxy
		}
		if !mapped {
			if fallback == "" {
				fallback = entry
			}
		} else if strings.EqualFold(key, target.Scheme) {
			if selected == "" {
				selected = value
			}
		} else if strings.EqualFold(key, "socks") {
			socks = value
		}
	}
	if selected == "" {
		selected = fallback
	}
	if selected == "" && socks != "" {
		selected = socks
		if !strings.Contains(selected, "://") {
			selected = "socks5://" + selected
		}
	}
	if selected == "" {
		return nil, nil
	}
	if !strings.Contains(selected, "://") {
		selected = "http://" + selected
	}
	proxy, err := url.Parse(selected)
	if err != nil || proxy.Hostname() == "" || proxy.User != nil || proxy.RawQuery != "" || proxy.Fragment != "" || (proxy.Path != "" && proxy.Path != "/") {
		return nil, errInvalidProxy
	}
	switch proxy.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, errInvalidProxy
	}
	return proxy, nil
}

func bypasses(target *url.URL, bypass string) bool {
	host := strings.ToLower(target.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	for _, rule := range strings.Split(bypass, ";") {
		rule = strings.ToLower(strings.TrimSpace(rule))
		if rule == "" {
			continue
		}
		if rule == "<local>" {
			if !strings.Contains(host, ".") && net.ParseIP(host) == nil {
				return true
			}
			continue
		}
		match := host
		if scheme, rest, ok := strings.Cut(rule, "://"); ok {
			if scheme != strings.ToLower(target.Scheme) {
				continue
			}
			rule = rest
		}
		if strings.Contains(rule, ":") {
			match = strings.ToLower(target.Host)
		}
		pattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(rule), "\\*", ".*") + "$"
		if ok, _ := regexp.MatchString(pattern, match); ok {
			return true
		}
	}
	return false
}

func validRequest(req *http.Request) bool {
	return req != nil && req.URL != nil && (req.URL.Scheme == "http" || req.URL.Scheme == "https")
}
