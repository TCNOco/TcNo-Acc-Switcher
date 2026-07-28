package protocol

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
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
