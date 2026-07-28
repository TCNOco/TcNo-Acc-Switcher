package timesync

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/otp"
)

type sequenceClock struct {
	mu    sync.Mutex
	times []time.Time
	last  time.Time
}

func (c *sequenceClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.times) > 0 {
		c.last = c.times[0]
		c.times = c.times[1:]
	}
	return c.last
}

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestSyncAcceptsStrictResponseAndUsesMidpoint(t *testing.T) {
	start := time.Unix(1_700_000_000, 0).UTC()
	finish := start.Add(200 * time.Millisecond)
	clock := &sequenceClock{times: []time.Time{start, finish}, last: finish}
	doer := doerFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.String() != Endpoint || req.URL.Hostname() != "api.steampowered.com" {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
		if deadline, ok := req.Context().Deadline(); !ok || time.Until(deadline) > RequestTimeout {
			t.Fatalf("request deadline = %s, %v", deadline, ok)
		}
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != "steamid=0" || req.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Fatalf("request body/header = %q, %q", raw, req.Header.Get("Content-Type"))
		}
		return response(http.StatusOK, `{"response":{"server_time":"1700000010"}}`), nil
	})
	state := otp.NewTimeStateWithLimits(clock, time.Minute, time.Minute)
	result, err := NewClientWithDependencies(doer, clock).Sync(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if result.RoundTrip != 200*time.Millisecond || !result.SampledAt.Equal(start.Add(100*time.Millisecond)) {
		t.Fatalf("result = %+v", result)
	}
	corrected, freshness := state.Now()
	if freshness != otp.FreshnessFresh || !corrected.Equal(time.Unix(1_700_000_010, 0).Add(100*time.Millisecond)) {
		t.Fatalf("corrected = %s, %v", corrected, freshness)
	}
}

func TestDefaultClientRejectsRedirectsAndPinsTransportLimits(t *testing.T) {
	client := NewClient()
	httpClient, ok := client.doer.(*http.Client)
	if !ok {
		t.Fatalf("doer type = %T", client.doer)
	}
	redirectErr := httpClient.CheckRedirect(&http.Request{}, []*http.Request{{}})
	if !errors.Is(redirectErr, http.ErrUseLastResponse) {
		t.Fatalf("redirect error = %v", redirectErr)
	}
	transport, ok := httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T", httpClient.Transport)
	}
	if httpClient.Timeout != RequestTimeout || transport.TLSClientConfig.MinVersion != tls.VersionTLS12 ||
		transport.ResponseHeaderTimeout <= 0 || transport.TLSHandshakeTimeout <= 0 {
		t.Fatalf("network limits are not configured")
	}
}

func TestSyncAcceptsNumericServerTime(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	clock := &sequenceClock{times: []time.Time{now, now}, last: now}
	state := otp.NewTimeState(clock)
	client := NewClientWithDependencies(doerFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"response":{"server_time":1700000001}}`), nil
	}), clock)
	if _, err := client.Sync(context.Background(), state); err != nil {
		t.Fatal(err)
	}
}

// liveQueryTimeBody mirrors the real ITwoFactorService/QueryTime payload, whose
// tuning fields must not fail the decode.
const liveQueryTimeBody = `{"response":{"server_time":"1700000010","skew_tolerance_seconds":"60",` +
	`"large_time_jink":"86400","probe_frequency_seconds":3600,"adjusted_time_probe_frequency_seconds":300,` +
	`"hint_probe_frequency_seconds":60,"sync_timeout":60,"try_again_seconds":900,"max_attempts":3}}`

func TestSyncAcceptsLiveResponseShape(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	clock := &sequenceClock{times: []time.Time{now, now}, last: now}
	state := otp.NewTimeStateWithLimits(clock, time.Minute, time.Minute)
	client := NewClientWithDependencies(doerFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, liveQueryTimeBody), nil
	}), clock)
	result, err := client.Sync(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ServerTime.Equal(time.Unix(1_700_000_010, 0).UTC()) {
		t.Fatalf("server time = %s", result.ServerTime)
	}
	if got := state.Freshness(); got != otp.FreshnessFresh {
		t.Fatalf("freshness = %v", got)
	}
}

func TestInvalidResponsesBecomeUntrusted(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	tests := []string{
		``,
		`{"response":{}}`,
		`{"response":{"server_time":0}}`,
		`{"response":{"server_time":"01700000000"}}`,
		`{"response":{"server_time":1700000000}} trailing`,
		strings.Repeat("x", MaxResponseBytes+1),
	}
	for _, body := range tests {
		t.Run(bodyName(body), func(t *testing.T) {
			clock := &sequenceClock{times: []time.Time{now, now}, last: now}
			state := otp.NewTimeState(clock)
			client := NewClientWithDependencies(doerFunc(func(*http.Request) (*http.Response, error) {
				return response(http.StatusOK, body), nil
			}), clock)
			if _, err := client.Sync(context.Background(), state); !errors.Is(err, ErrInvalidResponse) {
				t.Fatalf("error = %v", err)
			}
			if got := state.Freshness(); got != otp.FreshnessUntrusted {
				t.Fatalf("freshness = %v", got)
			}
		})
	}
}

func TestOutOfRangeAndSlowSamplesBecomeUntrusted(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	for name, finish := range map[string]time.Time{
		"slow":     now.Add(MaxRoundTrip + time.Nanosecond),
		"rollback": now.Add(-time.Nanosecond),
	} {
		t.Run(name, func(t *testing.T) {
			clock := &sequenceClock{times: []time.Time{now, finish}, last: finish}
			state := otp.NewTimeState(clock)
			client := NewClientWithDependencies(doerFunc(func(*http.Request) (*http.Response, error) {
				return response(http.StatusOK, `{"response":{"server_time":1700000000}}`), nil
			}), clock)
			if _, err := client.Sync(context.Background(), state); !errors.Is(err, ErrSlowResponse) {
				t.Fatalf("error = %v", err)
			}
			if got := state.Freshness(); got != otp.FreshnessUntrusted {
				t.Fatalf("freshness = %v", got)
			}
		})
	}

	clock := &sequenceClock{times: []time.Time{now, now}, last: now}
	state := otp.NewTimeStateWithLimits(clock, time.Minute, time.Minute)
	client := NewClientWithDependencies(doerFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"response":{"server_time":1700003600}}`), nil
	}), clock)
	if _, err := client.Sync(context.Background(), state); !errors.Is(err, otp.ErrTimeSampleOutOfRange) {
		t.Fatalf("error = %v", err)
	}
	if got := state.Freshness(); got != otp.FreshnessUntrusted {
		t.Fatalf("freshness = %v", got)
	}
}

func TestTransportFailuresAgeExistingSample(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	clock := &sequenceClock{times: []time.Time{now, now}, last: now}
	state := otp.NewTimeStateWithLimits(clock, time.Minute, time.Minute)
	if err := state.AcceptSample(now.Add(10*time.Second).Unix(), now); err != nil {
		t.Fatal(err)
	}
	client := NewClientWithDependencies(doerFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	}), clock)
	if _, err := client.Sync(context.Background(), state); !errors.Is(err, ErrRequestFailed) {
		t.Fatalf("error = %v", err)
	}
	if got := state.Freshness(); got != otp.FreshnessFresh {
		t.Fatalf("freshness after transport error = %v", got)
	}
	clock.last = now.Add(time.Minute + time.Nanosecond)
	if got := state.Freshness(); got != otp.FreshnessStale {
		t.Fatalf("aged freshness = %v", got)
	}
}

func TestUnexpectedStatusDoesNotDiscardSample(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	clock := &sequenceClock{times: []time.Time{now, now}, last: now}
	state := otp.NewTimeState(clock)
	if err := state.AcceptSample(now.Unix(), now); err != nil {
		t.Fatal(err)
	}
	client := NewClientWithDependencies(doerFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusServiceUnavailable, "unavailable"), nil
	}), clock)
	if _, err := client.Sync(context.Background(), state); !errors.Is(err, ErrUnexpectedStatus) {
		t.Fatalf("error = %v", err)
	}
	if got := state.Freshness(); got != otp.FreshnessFresh {
		t.Fatalf("freshness = %v", got)
	}
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}

func bodyName(body string) string {
	if len(body) > 40 {
		return body[:40]
	}
	if body == "" {
		return "empty"
	}
	return body
}
