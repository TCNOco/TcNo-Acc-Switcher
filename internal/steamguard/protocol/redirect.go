package protocol

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// A redirect chain is a handful of hops on one host, so these bound what a
// misbehaving response can make one request hold.
const (
	maxCarriedCookies    = 32
	maxCarriedCookieSize = 4 << 10
)

// carriedCookies keeps the cookies a host sets partway through a redirect chain
// so the next hop presents them.
//
// The client keeps no cookie jar on purpose, but a chain without one cannot
// finish a flow that hands out a cookie and redirects back to the page that
// wanted it: every hop replays the caller's original Cookie header unchanged,
// the destination asks for the same cookie again, and the chain ends only when
// the hop budget does - which reads, from the outside, as a refused session on
// an account whose session was fine.
//
// Deliberately narrower than a jar. Values are recorded only from the host the
// request started on, replayed only to that same host, and discarded with the
// request. Domain and Path are not honoured, because a cookie scoped anywhere
// else is one this would never send.
type carriedCookies struct {
	host  string
	inner http.RoundTripper

	mu sync.Mutex
	// order preserves first-seen order so the header this renders is stable.
	order []string
	value map[string]string
}

func newCarriedCookies(host string, inner http.RoundTripper) *carriedCookies {
	return &carriedCookies{host: host, inner: inner, value: make(map[string]string)}
}

// RoundTrip harvests Set-Cookie from every hop that stayed on the origin host.
func (c *carriedCookies) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := c.inner.RoundTrip(request)
	if err != nil || response == nil {
		return response, err
	}
	if request.URL != nil && strings.EqualFold(request.URL.Hostname(), c.host) {
		c.record(response.Cookies())
	}
	return response, nil
}

func (c *carriedCookies) record(cookies []*http.Cookie) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, cookie := range cookies {
		if cookie == nil || cookie.Name == "" || len(cookie.Name)+len(cookie.Value) > maxCarriedCookieSize {
			continue
		}
		if cookie.Value == "" || cookie.MaxAge < 0 || (!cookie.Expires.IsZero() && !cookie.Expires.After(now)) {
			// Cleared rather than set. Forgetting the recorded value falls back to
			// whatever the caller sent, which is as close to "unset" as this can get
			// without dropping the session cookie it was handed.
			delete(c.value, cookie.Name)
			continue
		}
		if _, tracked := c.value[cookie.Name]; !tracked && !c.ordered(cookie.Name) {
			if len(c.order) >= maxCarriedCookies {
				continue
			}
			c.order = append(c.order, cookie.Name)
		}
		c.value[cookie.Name] = cookie.Value
	}
}

func (c *carriedCookies) ordered(name string) bool {
	for _, existing := range c.order {
		if existing == name {
			return true
		}
	}
	return false
}

// header renders the Cookie header for the next hop: what the caller sent, with
// anything the chain has since reset replaced in place, then whatever it added.
func (c *carriedCookies) header(original string) string {
	if c == nil {
		return original
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.value) == 0 {
		return original
	}
	pairs := make([]string, 0, len(c.order)+8)
	sent := make(map[string]bool, len(c.value))
	for _, pair := range strings.Split(original, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		name := pair
		if index := strings.IndexByte(pair, '='); index >= 0 {
			name = pair[:index]
		}
		replacement, reset := c.value[name]
		if !reset {
			pairs = append(pairs, pair)
			continue
		}
		if sent[name] {
			continue
		}
		sent[name] = true
		pairs = append(pairs, name+"="+replacement)
	}
	for _, name := range c.order {
		value, present := c.value[name]
		if !present || sent[name] {
			continue
		}
		sent[name] = true
		pairs = append(pairs, name+"="+value)
	}
	return strings.Join(pairs, "; ")
}

// redirectTarget renders a followed hop for a caller's log: origin and path
// only. The query is dropped rather than trimmed, because a signed Steam
// request carries its signature there. The scheme is a literal because
// validateRedirect has already refused anything but https.
func redirectTarget(target *url.URL) string {
	if target == nil {
		return ""
	}
	return "https://" + strings.ToLower(target.Hostname()) + target.EscapedPath()
}
