// Package timesync obtains the clock correction used for Steam Guard codes.
package timesync

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/otp"
)

const (
	Endpoint         = "https://api.steampowered.com/ITwoFactorService/QueryTime/v0001"
	RequestTimeout   = 5 * time.Second
	MaxRoundTrip     = 5 * time.Second
	MaxResponseBytes = 4 << 10
)

var (
	ErrRequestFailed    = errors.New("Steam time request failed")
	ErrUnexpectedStatus = errors.New("Steam time endpoint returned an unexpected status")
	ErrInvalidResponse  = errors.New("invalid Steam time response")
	ErrSlowResponse     = errors.New("Steam time response exceeded the round-trip limit")
)

// HTTPDoer is the subset of http.Client needed by Client.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Clock supplies local timestamps around the request.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Result describes an accepted Steam time sample.
type Result struct {
	ServerTime time.Time
	SampledAt  time.Time
	RoundTrip  time.Duration
	Offset     time.Duration
}

// Client queries the fixed Steam time endpoint.
type Client struct {
	doer  HTTPDoer
	clock Clock
}

// NewClient creates a client with bounded network timeouts and no redirects.
func NewClient() *Client {
	dialer := &net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: 3 * time.Second,
		ExpectContinueTimeout: time.Second,
		DisableCompression:    true,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   RequestTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return NewClientWithDependencies(httpClient, systemClock{})
}

// NewClientWithDependencies creates a client around injected testable dependencies.
// Production code should use NewClient.
func NewClientWithDependencies(doer HTTPDoer, clock Clock) *Client {
	if doer == nil {
		return NewClient()
	}
	if clock == nil {
		clock = systemClock{}
	}
	return &Client{doer: doer, clock: clock}
}

// Sync obtains one sample and records it in state. Transport failures leave the
// prior correction to age naturally. Invalid samples clear it as untrusted.
func (c *Client) Sync(ctx context.Context, state *otp.TimeState) (Result, error) {
	if state == nil {
		return Result{}, ErrInvalidResponse
	}
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()

	body := url.Values{"steamid": {"0"}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, Endpoint, bytes.NewBufferString(body))
	if err != nil {
		return Result{}, errors.Join(ErrRequestFailed, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	startedAt := c.clock.Now()
	response, err := c.doer.Do(req)
	finishedAt := c.clock.Now()
	if err != nil {
		return Result{}, errors.Join(ErrRequestFailed, err)
	}
	if response == nil || response.Body == nil {
		state.MarkUntrusted()
		return Result{}, ErrInvalidResponse
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("%w: %d", ErrUnexpectedStatus, response.StatusCode)
	}

	roundTrip := finishedAt.Sub(startedAt)
	if roundTrip < 0 || roundTrip > MaxRoundTrip {
		state.MarkUntrusted()
		return Result{}, ErrSlowResponse
	}
	serverUnix, err := decodeResponse(response.Body, response.ContentLength)
	if err != nil {
		state.MarkUntrusted()
		return Result{}, err
	}
	sampledAt := startedAt.Add(roundTrip / 2)
	if err := state.AcceptSample(serverUnix, sampledAt); err != nil {
		return Result{}, err
	}
	serverTime := time.Unix(serverUnix, 0).UTC()
	return Result{
		ServerTime: serverTime,
		SampledAt:  sampledAt,
		RoundTrip:  roundTrip,
		Offset:     serverTime.Sub(sampledAt),
	}, nil
}

func decodeResponse(body io.Reader, contentLength int64) (int64, error) {
	if contentLength > MaxResponseBytes {
		return 0, ErrInvalidResponse
	}
	limited := io.LimitReader(body, MaxResponseBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil || len(raw) == 0 || len(raw) > MaxResponseBytes {
		return 0, errors.Join(ErrInvalidResponse, err)
	}
	var envelope struct {
		Response struct {
			ServerTime json.RawMessage `json:"server_time"`
		} `json:"response"`
	}
	// Steam ships tuning fields (skew_tolerance_seconds, probe frequencies, ...)
	// alongside server_time, so unknown fields are tolerated. Only server_time
	// has to be present and parseable.
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&envelope); err != nil {
		return 0, errors.Join(ErrInvalidResponse, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return 0, ErrInvalidResponse
	}
	serverUnix, err := parseUnix(envelope.Response.ServerTime)
	if err != nil {
		return 0, errors.Join(ErrInvalidResponse, err)
	}
	return serverUnix, nil
}

func parseUnix(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 {
		return 0, ErrInvalidResponse
	}
	value := string(raw)
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, err
		}
		value = text
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != value {
		return 0, ErrInvalidResponse
	}
	return parsed, nil
}
