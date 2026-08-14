package protocol

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestDoRejectsSchemeAndHostBeforeTransport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		endpoint string
		route    Route
		code     Code
	}{
		{name: "plain HTTP", endpoint: "http://api.steampowered.com/auth", route: RouteRequest, code: CodeSchemeDenied},
		{name: "lookalike host", endpoint: "https://api.steampowered.com.evil.example/auth", route: RouteRequest, code: CodeHostDenied},
		{name: "nonstandard port", endpoint: "https://api.steampowered.com:444/auth", route: RouteRequest, code: CodeHostDenied},
		{name: "request host used as transfer", endpoint: "https://api.steampowered.com/auth", route: RouteTransfer, code: CodeHostDenied},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			client := NewClient(Options{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				return nil, errors.New("transport must not run")
			})})
			_, err := client.Do(context.Background(), Request{
				Method:   http.MethodGet,
				Endpoint: test.endpoint,
				Route:    test.route,
				Timeout:  time.Second,
			})
			assertProtocolCode(t, err, test.code)
			if calls.Load() != 0 {
				t.Fatalf("transport called %d times", calls.Load())
			}
		})
	}
}

func TestDoRejectsRedirectOutsideAllowlist(t *testing.T) {
	t.Parallel()

	const secret = "redirect-secret"
	var calls atomic.Int32
	client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		return response(request, http.StatusFound, http.Header{
			"Location": {"https://evil.example/collect?token=" + secret},
		}, nil), nil
	})})

	_, err := client.Do(context.Background(), Request{
		Method:         http.MethodGet,
		Endpoint:       "https://steamcommunity.com/login",
		Route:          RouteRequest,
		Timeout:        time.Second,
		AllowRedirects: true,
	})
	assertProtocolCode(t, err, CodeRedirectDenied)
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "evil.example") {
		t.Fatalf("redirect error leaked destination: %q", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("transport called %d times, want 1", calls.Load())
	}
}

func TestDoScrubsHeadersOnAllowedRedirect(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch calls.Add(1) {
		case 1:
			return response(request, http.StatusFound, http.Header{
				"Location": {"https://store.steampowered.com/login/settoken"},
			}, nil), nil
		case 2:
			if request.Header.Get("Authorization") != "" || request.Header.Get("Cookie") != "" || request.Header.Get("Referer") != "" {
				t.Fatalf("sensitive header survived redirect: %#v", request.Header)
			}
			if request.Header.Get("User-Agent") != UserAgent {
				t.Fatalf("User-Agent = %q", request.Header.Get("User-Agent"))
			}
			return response(request, http.StatusNoContent, nil, nil), nil
		default:
			return nil, errors.New("too many requests")
		}
	})})

	result, err := client.Do(context.Background(), Request{
		Method:         http.MethodGet,
		Endpoint:       "https://steamcommunity.com/login",
		Route:          RouteRequest,
		Header:         http.Header{"Authorization": {"Bearer secret"}, "Cookie": {"session=secret"}},
		Timeout:        time.Second,
		AllowRedirects: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d", result.StatusCode)
	}
}

// Steam answers /profiles/<id64>/... with a 302 to /id/<vanity>/... for any
// account with a custom URL. Following that hop without the cookies lands on
// the login page, so a live session reads as "sign in again".
func TestDoCarriesHeadersAcrossASameHostRedirectWhenAsked(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch calls.Add(1) {
		case 1:
			return response(request, http.StatusFound, http.Header{
				"Location": {"https://steamcommunity.com/id/vanity/tradeoffers/privacy"},
			}, nil), nil
		case 2:
			if request.Header.Get("Cookie") != "session=secret" {
				t.Fatalf("cookie did not survive the hop: %#v", request.Header)
			}
			if request.Header.Get("User-Agent") != "okhttp/3.12.12" {
				t.Fatalf("User-Agent = %q", request.Header.Get("User-Agent"))
			}
			return response(request, http.StatusNoContent, nil, nil), nil
		default:
			return nil, errors.New("too many requests")
		}
	})})

	result, err := client.Do(context.Background(), Request{
		Method:   http.MethodGet,
		Endpoint: "https://steamcommunity.com/profiles/76561198000000000/tradeoffers/privacy",
		Route:    RouteRequest,
		Header: http.Header{
			"Cookie":     {"session=secret"},
			"User-Agent": {"okhttp/3.12.12"},
		},
		Timeout:                   time.Second,
		AllowRedirects:            true,
		PreserveHeadersOnRedirect: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d", result.StatusCode)
	}
}

// Same host is the entire guarantee. A hop to another allowlisted Steam host is
// still an origin that did not issue the cookie, so it gets the scrubbed set.
func TestDoScrubsHeadersAcrossHostsEvenWhenPreserving(t *testing.T) {
	t.Parallel()

	for _, target := range []string{
		"https://store.steampowered.com/login/settoken",
		"https://help.steampowered.com/wizard",
		"https://login.steampowered.com/jwt",
	} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				switch calls.Add(1) {
				case 1:
					return response(request, http.StatusFound, http.Header{"Location": {target}}, nil), nil
				case 2:
					if request.Header.Get("Cookie") != "" {
						t.Fatalf("cookie replayed to %s: %#v", target, request.Header)
					}
					return response(request, http.StatusNoContent, nil, nil), nil
				default:
					return nil, errors.New("too many requests")
				}
			})})

			if _, err := client.Do(context.Background(), Request{
				Method:                    http.MethodGet,
				Endpoint:                  "https://steamcommunity.com/profiles/1/tradeoffers/privacy",
				Route:                     RouteRequest,
				Header:                    http.Header{"Cookie": {"session=secret"}},
				Timeout:                   time.Second,
				AllowRedirects:            true,
				PreserveHeadersOnRedirect: true,
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The flag means nothing on its own: without AllowRedirects a 302 is still a
// denied redirect, so it cannot quietly widen anything it is set on.
func TestDoStillDeniesRedirectsWithoutAllowRedirects(t *testing.T) {
	t.Parallel()

	client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, http.StatusFound, http.Header{
			"Location": {"https://steamcommunity.com/id/vanity/tradeoffers/privacy"},
		}, nil), nil
	})})

	_, err := client.Do(context.Background(), Request{
		Method:                    http.MethodGet,
		Endpoint:                  "https://steamcommunity.com/profiles/1/tradeoffers/privacy",
		Route:                     RouteRequest,
		Header:                    http.Header{"Cookie": {"session=secret"}},
		Timeout:                   time.Second,
		PreserveHeadersOnRedirect: true,
	})
	assertProtocolCode(t, err, CodeRedirectDenied)
}

// Every redirect refusal collapses to the same Kind by the time it reaches a
// caller, so the label is the only thing that says which check fired. Getting
// this wrong costs a debugging round trip per failure.
func TestDoLabelsWhyARedirectWasDenied(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		location string
		allow    bool
		want     string
	}{
		"not allowed":  {"https://steamcommunity.com/id/vanity/x", false, "redirect_disabled"},
		"offsite host": {"https://evil.example.com/x", true, "redirect_host"},
		"downgraded":   {"http://steamcommunity.com/x", true, "redirect_scheme"},
		"odd port":     {"https://steamcommunity.com:8443/x", true, "redirect_port"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return response(request, http.StatusFound, http.Header{"Location": {tc.location}}, nil), nil
			})})
			_, err := client.Do(context.Background(), Request{
				Method:                    http.MethodGet,
				Endpoint:                  "https://steamcommunity.com/profiles/1/tradeoffers/privacy",
				Route:                     RouteRequest,
				Timeout:                   time.Second,
				AllowRedirects:            tc.allow,
				PreserveHeadersOnRedirect: true,
			})
			var protocolErr *Error
			if !errors.As(err, &protocolErr) {
				t.Fatalf("err = %v, want *Error", err)
			}
			if protocolErr.Code != CodeRedirectDenied {
				t.Fatalf("code = %v, want %v", protocolErr.Code, CodeRedirectDenied)
			}
			if protocolErr.Detail != tc.want {
				t.Fatalf("detail = %q, want %q", protocolErr.Detail, tc.want)
			}
		})
	}
}

// Steam may answer with a bare path. Go resolves a relative Location against the
// URL it came from before any policy runs, so it arrives as an absolute same-host
// URL and is followed with the session intact - it is not a malformed redirect.
func TestDoResolvesARelativeRedirectAgainstItsOrigin(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch calls.Add(1) {
		case 1:
			return response(request, http.StatusFound, http.Header{
				"Location": {"/id/vanity/tradeoffers/privacy"},
			}, nil), nil
		case 2:
			if request.URL.String() != "https://steamcommunity.com/id/vanity/tradeoffers/privacy" {
				t.Fatalf("resolved to %s", request.URL)
			}
			if request.Header.Get("Cookie") != "session=secret" {
				t.Fatalf("cookie did not survive: %#v", request.Header)
			}
			return response(request, http.StatusNoContent, nil, nil), nil
		default:
			return nil, errors.New("too many requests")
		}
	})})

	if _, err := client.Do(context.Background(), Request{
		Method:                    http.MethodGet,
		Endpoint:                  "https://steamcommunity.com/profiles/1/tradeoffers/privacy",
		Route:                     RouteRequest,
		Header:                    http.Header{"Cookie": {"session=secret"}},
		Timeout:                   time.Second,
		AllowRedirects:            true,
		PreserveHeadersOnRedirect: true,
	}); err != nil {
		t.Fatal(err)
	}
}

// A chain that never settles is refused by count, and says so. This is the one
// refusal a same-host, correctly-followed redirect can still hit.
func TestDoLabelsARedirectLoopByItsLimit(t *testing.T) {
	t.Parallel()

	client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		// Always somewhere else on the same host, so every other check passes.
		return response(request, http.StatusFound, http.Header{
			"Location": {"https://steamcommunity.com/id/vanity/" + strconv.Itoa(len(request.URL.Path))},
		}, nil), nil
	})})

	_, err := client.Do(context.Background(), Request{
		Method:                    http.MethodGet,
		Endpoint:                  "https://steamcommunity.com/profiles/1/tradeoffers/privacy",
		Route:                     RouteRequest,
		Timeout:                   time.Second,
		AllowRedirects:            true,
		PreserveHeadersOnRedirect: true,
	})
	var protocolErr *Error
	if !errors.As(err, &protocolErr) {
		t.Fatalf("err = %v, want *Error", err)
	}
	if protocolErr.Detail != "redirect_limit" {
		t.Fatalf("detail = %q, want redirect_limit", protocolErr.Detail)
	}
}

// The labels are compared across package boundaries, where only the string
// exists. If the builder and the constant ever disagree, a redirect refusal
// silently changes meaning for every caller reading it.
func TestRedirectLabelsMatchTheirExportedNames(t *testing.T) {
	t.Parallel()

	if got := redirectDenied("disabled").Detail; got != DetailRedirectDisabled {
		t.Fatalf("disabled label = %q, want %q", got, DetailRedirectDisabled)
	}
	if got := redirectDenied("limit").Detail; got != DetailRedirectLimit {
		t.Fatalf("limit label = %q, want %q", got, DetailRedirectLimit)
	}
}

// Steam hands out a session cookie partway through and redirects back to the
// page that wanted it. With nothing carrying that cookie the destination asks
// again, and the chain only ends when the budget does.
func TestDoCarriesACookieSetPartwayThroughAChain(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch calls.Add(1) {
		case 1:
			return response(request, http.StatusFound, http.Header{
				"Location":   {"https://steamcommunity.com/id/vanity/tradeoffers/privacy"},
				"Set-Cookie": {"sessionid=fresh; Path=/", "steamCountry=NL; Path=/"},
			}, nil), nil
		case 2:
			cookie := request.Header.Get("Cookie")
			// Replaced in place, not appended: two sessionid pairs is a different
			// request from the one Steam asked for.
			if cookie != "steamLoginSecure=token; sessionid=fresh; steamCountry=NL" {
				t.Fatalf("cookie = %q", cookie)
			}
			return response(request, http.StatusNoContent, nil, nil), nil
		default:
			return nil, errors.New("too many requests")
		}
	})})

	if _, err := client.Do(context.Background(), Request{
		Method:                    http.MethodGet,
		Endpoint:                  "https://steamcommunity.com/profiles/1/tradeoffers/privacy",
		Route:                     RouteRequest,
		Header:                    http.Header{"Cookie": {"steamLoginSecure=token; sessionid=stale"}},
		Timeout:                   time.Second,
		AllowRedirects:            true,
		PreserveHeadersOnRedirect: true,
	}); err != nil {
		t.Fatal(err)
	}
}

// A cookie picked up mid-chain is still the origin's, so it goes no further than
// the origin does.
func TestDoDoesNotCarryACollectedCookieToAnotherHost(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch calls.Add(1) {
		case 1:
			return response(request, http.StatusFound, http.Header{
				"Location":   {"https://store.steampowered.com/login/"},
				"Set-Cookie": {"sessionid=fresh; Path=/"},
			}, nil), nil
		case 2:
			if request.Header.Get("Cookie") != "" {
				t.Fatalf("cookie replayed offsite: %#v", request.Header)
			}
			return response(request, http.StatusNoContent, nil, nil), nil
		default:
			return nil, errors.New("too many requests")
		}
	})})

	if _, err := client.Do(context.Background(), Request{
		Method:                    http.MethodGet,
		Endpoint:                  "https://steamcommunity.com/profiles/1/tradeoffers/privacy",
		Route:                     RouteRequest,
		Header:                    http.Header{"Cookie": {"steamLoginSecure=token"}},
		Timeout:                   time.Second,
		AllowRedirects:            true,
		PreserveHeadersOnRedirect: true,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestDoHonoursARaisedRedirectBudget(t *testing.T) {
	t.Parallel()

	const hops = 5
	var calls atomic.Int32
	client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if hop := int(calls.Add(1)); hop <= hops {
			return response(request, http.StatusFound, http.Header{
				"Location": {"https://steamcommunity.com/id/vanity/" + strconv.Itoa(hop)},
			}, nil), nil
		}
		return response(request, http.StatusNoContent, nil, nil), nil
	})})

	if _, err := client.Do(context.Background(), Request{
		Method:                    http.MethodGet,
		Endpoint:                  "https://steamcommunity.com/profiles/1/tradeoffers/privacy",
		Route:                     RouteRequest,
		Timeout:                   time.Second,
		AllowRedirects:            true,
		PreserveHeadersOnRedirect: true,
		MaxRedirects:              MaxRedirectBudget,
	}); err != nil {
		t.Fatal(err)
	}
}

// The budget is a ceiling a caller may raise to, never past. A request asking
// for more is refused before the transport runs rather than quietly clamped.
func TestDoRejectsARedirectBudgetBeyondTheCeiling(t *testing.T) {
	t.Parallel()

	for _, budget := range []int{-1, MaxRedirectBudget + 1} {
		t.Run(strconv.Itoa(budget), func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			client := NewClient(Options{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				return nil, errors.New("transport must not run")
			})})
			_, err := client.Do(context.Background(), Request{
				Method:         http.MethodGet,
				Endpoint:       "https://steamcommunity.com/profiles/1/tradeoffers/privacy",
				Route:          RouteRequest,
				Timeout:        time.Second,
				AllowRedirects: true,
				MaxRedirects:   budget,
			})
			assertProtocolCode(t, err, CodeInvalidRequest)
			if calls.Load() != 0 {
				t.Fatalf("transport called %d times", calls.Load())
			}
		})
	}
}

// The whole point of the callback is a log line, so it must carry no query: a
// signed Steam request keeps its signature there.
func TestDoReportsFollowedHopsWithoutTheirQuery(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch calls.Add(1) {
		case 1:
			return response(request, http.StatusFound, http.Header{
				"Location": {"https://steamcommunity.com/login/home/?goto=privacy&k=signature"},
			}, nil), nil
		default:
			return response(request, http.StatusNoContent, nil, nil), nil
		}
	})})

	var hops []string
	if _, err := client.Do(context.Background(), Request{
		Method:                    http.MethodGet,
		Endpoint:                  "https://steamcommunity.com/profiles/1/tradeoffers/privacy",
		Route:                     RouteRequest,
		Timeout:                   time.Second,
		AllowRedirects:            true,
		PreserveHeadersOnRedirect: true,
		OnRedirect:                func(hop int, target string) { hops = append(hops, strconv.Itoa(hop)+" "+target) },
	}); err != nil {
		t.Fatal(err)
	}
	if len(hops) != 1 || hops[0] != "1 https://steamcommunity.com/login/home/" {
		t.Fatalf("hops = %#v", hops)
	}
}

// A refused hop is named by the error, not handed to a logger: its destination
// is whatever the response asked for, and this is the one case where that is
// somewhere policy said no to.
func TestDoDoesNotReportARefusedHop(t *testing.T) {
	t.Parallel()

	client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, http.StatusFound, http.Header{
			"Location": {"https://evil.example/collect?token=secret"},
		}, nil), nil
	})})

	var hops int
	_, err := client.Do(context.Background(), Request{
		Method:         http.MethodGet,
		Endpoint:       "https://steamcommunity.com/profiles/1/tradeoffers/privacy",
		Route:          RouteRequest,
		Timeout:        time.Second,
		AllowRedirects: true,
		OnRedirect:     func(int, string) { hops++ },
	})
	assertProtocolCode(t, err, CodeRedirectDenied)
	if hops != 0 {
		t.Fatalf("reported %d refused hops", hops)
	}
}

func TestDoRejectsOversizedResponse(t *testing.T) {
	t.Parallel()

	client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		result := response(request, http.StatusOK, nil, bytes.Repeat([]byte{'x'}, 33))
		result.ContentLength = -1
		return result, nil
	})})
	_, err := client.Do(context.Background(), Request{
		Method:           http.MethodGet,
		Endpoint:         "https://api.steampowered.com/test",
		Route:            RouteRequest,
		Timeout:          time.Second,
		MaxResponseBytes: 32,
	})
	assertProtocolCode(t, err, CodeResponseTooLarge)
}

func TestDoCancellationIsTypedAndSecretFree(t *testing.T) {
	t.Parallel()

	const secret = "cancel-secret"
	started := make(chan struct{})
	client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		close(started)
		<-request.Context().Done()
		return nil, fmt.Errorf("%s: %w", secret, request.Context().Err())
	})})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := client.Do(ctx, Request{
			Method:   http.MethodGet,
			Endpoint: "https://api.steampowered.com/test?access_token=" + secret,
			Route:    RouteRequest,
			Header:   http.Header{"Authorization": {"Bearer " + secret}},
			Timeout:  time.Second,
		})
		done <- err
	}()
	<-started
	cancel()
	err := <-done
	assertProtocolCode(t, err, CodeCanceled)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("errors.Is(context.Canceled) = false: %v", err)
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "access_token") {
		t.Fatalf("cancellation error leaked request data: %q", err)
	}
}

func TestDoAddsExplicitRequestDeadline(t *testing.T) {
	t.Parallel()

	const timeout = 5 * time.Second
	client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		deadline, ok := request.Context().Deadline()
		if !ok {
			t.Fatal("request context has no deadline")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 || remaining > timeout {
			t.Fatalf("deadline remaining = %s", remaining)
		}
		return response(request, http.StatusNoContent, nil, nil), nil
	})})
	_, err := client.Do(context.Background(), Request{
		Method:   http.MethodGet,
		Endpoint: "https://api.steampowered.com/test",
		Route:    RouteRequest,
		Timeout:  timeout,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDoParsesRetryAfterWithoutReturningBody(t *testing.T) {
	t.Parallel()

	const bodySecret = "server-body-secret"
	client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, http.StatusTooManyRequests, http.Header{
			"Retry-After": {"17"},
		}, []byte(bodySecret)), nil
	})})
	_, err := client.Do(context.Background(), Request{
		Method:   http.MethodPost,
		Endpoint: "https://api.steampowered.com/auth",
		Route:    RouteRequest,
		Body:     []byte("request-secret"),
		Timeout:  time.Second,
	})
	protocolErr := assertProtocolCode(t, err, CodeRateLimited)
	if protocolErr.State != StateRetryable || !protocolErr.HasRetryAfter || protocolErr.RetryAfter != 17*time.Second {
		t.Fatalf("rate limit metadata = %#v", protocolErr)
	}
	if strings.Contains(err.Error(), bodySecret) {
		t.Fatalf("HTTP error leaked response body: %q", err)
	}
}

func TestDoCapturesCanonicalEResult(t *testing.T) {
	t.Parallel()

	client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, http.StatusOK, http.Header{"x-eresult": {"15"}}, nil), nil
	})})
	result, err := client.Do(context.Background(), Request{
		Method:   http.MethodPost,
		Endpoint: "https://api.steampowered.com/auth",
		Route:    RouteRequest,
		Timeout:  time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasEResult || result.EResult != 15 {
		t.Fatalf("Steam result metadata = %#v", result)
	}
}

func TestDoRejectsMalformedEResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header http.Header
	}{
		{name: "empty", header: http.Header{"X-EResult": {""}}},
		{name: "whitespace", header: http.Header{"X-EResult": {" 1"}}},
		{name: "leading zero", header: http.Header{"X-EResult": {"01"}}},
		{name: "zero", header: http.Header{"X-EResult": {"0"}}},
		{name: "text", header: http.Header{"X-EResult": {"AccessDenied"}}},
		{name: "multiple", header: http.Header{"X-EResult": {"1", "15"}}},
		{name: "case duplicated", header: http.Header{"X-EResult": {"1"}, "x-eresult": {"15"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := NewClient(Options{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return response(request, http.StatusOK, test.header, []byte("response-secret")), nil
			})})
			_, err := client.Do(context.Background(), Request{
				Method:   http.MethodPost,
				Endpoint: "https://api.steampowered.com/auth",
				Route:    RouteRequest,
				Timeout:  time.Second,
			})
			assertProtocolCode(t, err, CodeInvalidResponse)
			if strings.Contains(err.Error(), "response-secret") || strings.Contains(err.Error(), test.header.Get("X-EResult")) && test.header.Get("X-EResult") != "" {
				t.Fatalf("EResult parse error leaked response data: %q", err)
			}
		})
	}
}

func TestDoSanitizesTransportError(t *testing.T) {
	t.Parallel()

	const secret = "transport-secret"
	client := NewClient(Options{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial failed with " + secret)
	})})
	_, err := client.Do(context.Background(), Request{
		Method:   http.MethodGet,
		Endpoint: "https://api.steampowered.com/test?token=" + secret,
		Route:    RouteRequest,
		Timeout:  time.Second,
	})
	assertProtocolCode(t, err, CodeTransport)
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "token") {
		t.Fatalf("transport error leaked data: %q", err)
	}
}

func assertProtocolCode(t *testing.T, err error, code Code) *Error {
	t.Helper()
	var protocolErr *Error
	if !errors.As(err, &protocolErr) {
		t.Fatalf("error type = %T, want *protocol.Error: %v", err, err)
	}
	if protocolErr.Code != code {
		t.Fatalf("error code = %q, want %q: %v", protocolErr.Code, code, err)
	}
	return protocolErr
}

func response(request *http.Request, status int, header http.Header, body []byte) *http.Response {
	if header == nil {
		header = make(http.Header)
	}
	return &http.Response{
		StatusCode:    status,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       request,
	}
}
