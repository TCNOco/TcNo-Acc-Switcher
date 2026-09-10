//go:build !windows

package systemproxy

import (
	"net/http"
	"net/url"
)

// Proxy uses the conventional environment proxy settings on non-Windows hosts.
func Proxy(req *http.Request) (*url.URL, error) {
	if !validRequest(req) {
		return nil, errInvalidProxy
	}
	return http.ProxyFromEnvironment(req)
}
