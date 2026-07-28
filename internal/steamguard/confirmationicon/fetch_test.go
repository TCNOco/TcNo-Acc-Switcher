package confirmationicon

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const allowedHost = "cdn.example.com"

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func testPolicy() Policy {
	return Policy{AllowedHosts: []string{allowedHost}}
}

func testFetcher(t *testing.T, policy Policy, transport http.RoundTripper) *Fetcher {
	t.Helper()
	fetcher, err := newFetcher(policy, transport)
	if err != nil {
		t.Fatalf("newFetcher: %v", err)
	}
	return fetcher
}

func response(status int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

func encodedPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			canvas.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 0x88, A: 0xff})
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return output.Bytes()
}

func encodedJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			canvas.SetRGBA(x, y, color.RGBA{R: 0x41, G: uint8(x), B: uint8(y), A: 0xff})
		}
	}
	var output bytes.Buffer
	if err := jpeg.Encode(&output, canvas, nil); err != nil {
		t.Fatalf("encode JPEG: %v", err)
	}
	return output.Bytes()
}

func TestFetchSanitizesImageAndRequest(t *testing.T) {
	marker := []byte("remote-metadata-and-trailing-payload")
	remote := append(encodedPNG(t, 3, 2), marker...)
	fetcher := testFetcher(t, testPolicy(), roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != allowedHost || request.URL.Scheme != "https" {
			t.Fatalf("unexpected destination: %s", request.URL.Redacted())
		}
		for _, header := range []string{"Authorization", "Cookie", "Referer", "Origin"} {
			if value := request.Header.Get(header); value != "" {
				t.Fatalf("sensitive header %s was sent", header)
			}
		}
		if got := request.Header.Get("Accept"); !strings.Contains(got, "image/png") {
			t.Fatalf("unexpected Accept: %q", got)
		}
		result := response(http.StatusOK, remote)
		result.Header.Set("Content-Type", "text/html") // MIME headers are not trusted.
		return result, nil
	}))

	result, err := fetcher.Fetch(context.Background(), "https://cdn.example.com/icon?id=public")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if result.State != StateReady || result.MIME != "image/png" || result.Width != 3 || result.Height != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.CacheControl != "no-store" {
		t.Fatalf("cache metadata = %q", result.CacheControl)
	}
	if bytes.Contains(result.Data, marker) {
		t.Fatal("sanitized image retained trailing remote payload")
	}
	if _, err := png.Decode(bytes.NewReader(result.Data)); err != nil {
		t.Fatalf("sanitized PNG does not decode: %v", err)
	}
}

func TestFetchReencodesJPEGAndWebP(t *testing.T) {
	webpBytes, err := base64.StdEncoding.DecodeString("UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA==")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		input    []byte
		wantMIME string
	}{
		{name: "jpeg", input: encodedJPEG(t, 2, 2), wantMIME: "image/jpeg"},
		{name: "webp", input: webpBytes, wantMIME: "image/png"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fetcher := testFetcher(t, testPolicy(), roundTripFunc(func(*http.Request) (*http.Response, error) {
				return response(http.StatusOK, test.input), nil
			}))
			result, err := fetcher.Fetch(context.Background(), "https://cdn.example.com/icon")
			if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if result.MIME != test.wantMIME || result.Width < 1 || result.Height < 1 {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func TestFetchRejectsHostileURLsWithoutNetwork(t *testing.T) {
	var calls atomic.Int32
	fetcher := testFetcher(t, testPolicy(), roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("must not be called")
	}))
	tests := []struct {
		name string
		url  string
	}{
		{name: "http", url: "http://cdn.example.com/icon"},
		{name: "userinfo", url: "https://user:password@cdn.example.com/icon"},
		{name: "ipv4", url: "https://127.0.0.1/icon"},
		{name: "ipv6", url: "https://[::1]/icon"},
		{name: "port", url: "https://cdn.example.com:8443/icon"},
		{name: "unlisted", url: "https://evil.example/icon"},
		{name: "fragment", url: "https://cdn.example.com/icon#hidden"},
		{name: "trailing-dot", url: "https://cdn.example.com./icon"},
		{name: "uppercase", url: "https://CDN.example.com/icon"},
		{name: "whitespace", url: " https://cdn.example.com/icon"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := fetcher.Fetch(context.Background(), test.url)
			if err == nil || result.State != StatePlaceholder || result.Reason == "" || result.Data != nil {
				t.Fatalf("expected typed placeholder, got result=%+v err=%v", result, err)
			}
			if strings.Contains(err.Error(), test.url) || strings.Contains(err.Error(), "password") {
				t.Fatalf("error disclosed URL data: %q", err)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("network called %d times", calls.Load())
	}
}

func TestRedirectsStayInsideAllowlistAndDropHeaders(t *testing.T) {
	fetcher := testFetcher(t, Policy{AllowedHosts: []string{allowedHost, "images.example.com"}}, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("not used")
	}))
	redirect, _ := http.NewRequest(http.MethodGet, "https://images.example.com/next", nil)
	redirect.Header.Set("Authorization", "secret")
	redirect.Header.Set("Cookie", "secret")
	redirect.Header.Set("Referer", "https://secret.example/")
	if err := fetcher.client.CheckRedirect(redirect, []*http.Request{{}, {}}); err != nil {
		t.Fatalf("allowed redirect rejected: %v", err)
	}
	for _, name := range []string{"Authorization", "Cookie", "Referer"} {
		if redirect.Header.Get(name) != "" {
			t.Fatalf("redirect retained %s", name)
		}
	}

	blocked, _ := http.NewRequest(http.MethodGet, "https://evil.example/next", nil)
	if err := fetcher.client.CheckRedirect(blocked, []*http.Request{{}}); err == nil {
		t.Fatal("redirect outside allowlist accepted")
	}
	tooMany, _ := http.NewRequest(http.MethodGet, "https://cdn.example.com/next", nil)
	if err := fetcher.client.CheckRedirect(tooMany, []*http.Request{{}, {}, {}}); err == nil {
		t.Fatal("redirect limit not enforced")
	}
}

func TestFetchRejectsUnsafeResponses(t *testing.T) {
	valid := encodedPNG(t, 2, 2)
	animatedPNG := append([]byte(nil), valid[:8]...)
	animatedPNG = append(animatedPNG,
		0, 0, 0, 8, 'a', 'c', 'T', 'L', 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0)
	animatedPNG = append(animatedPNG, valid[8:]...)
	animatedWebP := []byte{
		'R', 'I', 'F', 'F', 22, 0, 0, 0, 'W', 'E', 'B', 'P',
		'V', 'P', '8', 'X', 10, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}
	tests := []struct {
		name      string
		status    int
		body      []byte
		encoding  string
		wantKind  FailureKind
		configure func(*Policy)
	}{
		{name: "svg", status: http.StatusOK, body: []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), wantKind: FailureUnsupported},
		{name: "animated-png", status: http.StatusOK, body: animatedPNG, wantKind: FailureUnsupported},
		{name: "animated-webp", status: http.StatusOK, body: animatedWebP, wantKind: FailureUnsupported},
		{name: "compressed", status: http.StatusOK, body: valid, encoding: "gzip", wantKind: FailureUnsupported},
		{name: "status", status: http.StatusNotFound, body: valid, wantKind: FailureUnavailable},
		{name: "input-limit", status: http.StatusOK, body: valid, wantKind: FailureTooLarge, configure: func(p *Policy) { p.MaxInputBytes = int64(len(valid) - 1) }},
		{name: "output-limit", status: http.StatusOK, body: valid, wantKind: FailureTooLarge, configure: func(p *Policy) { p.MaxOutputBytes = 10 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := testPolicy()
			if test.configure != nil {
				test.configure(&policy)
			}
			fetcher := testFetcher(t, policy, roundTripFunc(func(*http.Request) (*http.Response, error) {
				result := response(test.status, test.body)
				result.Header.Set("Content-Encoding", test.encoding)
				return result, nil
			}))
			result, err := fetcher.Fetch(context.Background(), "https://cdn.example.com/icon")
			var safe *Error
			if !errors.As(err, &safe) || safe.Kind != test.wantKind || result.Reason != test.wantKind {
				t.Fatalf("got result=%+v err=%v, want %s", result, err, test.wantKind)
			}
		})
	}
}

func TestFetchRejectsExcessiveDimensionsBeforeDecode(t *testing.T) {
	input := encodedPNG(t, 5, 2)
	policy := testPolicy()
	policy.MaxWidth = 4
	fetcher := testFetcher(t, policy, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, input), nil
	}))
	result, err := fetcher.Fetch(context.Background(), "https://cdn.example.com/icon")
	var safe *Error
	if !errors.As(err, &safe) || safe.Kind != FailureTooLarge || result.Reason != FailureTooLarge {
		t.Fatalf("got result=%+v err=%v", result, err)
	}
}

func TestFetchCancellationIsTypedAndSecretFree(t *testing.T) {
	secret := "do-not-disclose-this-host-error"
	fetcher := testFetcher(t, testPolicy(), roundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, errors.New(secret)
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := fetcher.Fetch(ctx, "https://cdn.example.com/icon?token=not-a-real-token")
	var safe *Error
	if !errors.As(err, &safe) || safe.Kind != FailureCanceled || result.Reason != FailureCanceled {
		t.Fatalf("got result=%+v err=%v", result, err)
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "token") {
		t.Fatalf("error disclosed private data: %q", err)
	}
}

func TestPolicyRejectsUnsafeOrUnboundedConfiguration(t *testing.T) {
	tests := []Policy{
		{},
		{AllowedHosts: []string{"*.example.com"}},
		{AllowedHosts: []string{"EXAMPLE.com"}},
		{AllowedHosts: []string{"127.0.0.1"}},
		{AllowedHosts: []string{allowedHost, allowedHost}},
		{AllowedHosts: []string{allowedHost}, MaxInputBytes: hardMaxInputBytes + 1},
		{AllowedHosts: []string{allowedHost}, MaxOutputBytes: hardMaxOutputBytes + 1},
		{AllowedHosts: []string{allowedHost}, MaxPixels: hardMaxPixels + 1},
		{AllowedHosts: []string{allowedHost}, Timeout: hardMaxTimeout + time.Millisecond},
	}
	for index, policy := range tests {
		if _, err := newFetcher(policy, roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, nil })); err == nil {
			t.Fatalf("unsafe policy %d accepted: %+v", index, policy)
		}
	}
}
