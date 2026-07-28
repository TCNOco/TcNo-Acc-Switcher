package confirmationicon

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/image/webp"
)

const (
	defaultInputBytes  = int64(1 << 20)
	defaultOutputBytes = 4 << 20
	defaultDimension   = 1024
	defaultPixels      = 1 << 20
	defaultTimeout     = 10 * time.Second

	hardMaxInputBytes  = int64(4 << 20)
	hardMaxOutputBytes = 8 << 20
	hardMaxDimension   = 4096
	hardMaxPixels      = 4 << 20
	hardMaxTimeout     = 15 * time.Second
	maxAllowedHosts    = 16
)

// State tells callers whether sanitized bytes or a local placeholder should be
// rendered. A placeholder never contains or discloses the rejected remote URL.
type State string

const (
	StateReady       State = "ready"
	StatePlaceholder State = "placeholder"
)

// FailureKind is safe to expose to application code and logs.
type FailureKind string

const (
	FailureInvalidURL  FailureKind = "invalid-url"
	FailureBlocked     FailureKind = "blocked"
	FailureUnavailable FailureKind = "unavailable"
	FailureTooLarge    FailureKind = "too-large"
	FailureUnsupported FailureKind = "unsupported-image"
	FailureCanceled    FailureKind = "canceled"
)

// Error deliberately omits URLs, hostnames, response bodies, and network
// errors. It is safe to log and compare by Kind.
type Error struct {
	Kind FailureKind
}

func (e *Error) Error() string {
	if e == nil {
		return "Steam confirmation image failed"
	}
	switch e.Kind {
	case FailureInvalidURL:
		return "Steam confirmation image URL is invalid"
	case FailureBlocked:
		return "Steam confirmation image source is blocked"
	case FailureUnavailable:
		return "Steam confirmation image is unavailable"
	case FailureTooLarge:
		return "Steam confirmation image exceeds safety limits"
	case FailureUnsupported:
		return "Steam confirmation image format is unsupported"
	case FailureCanceled:
		return "Steam confirmation image request was canceled"
	default:
		return "Steam confirmation image failed"
	}
}

func (e *Error) Is(target error) bool {
	return e != nil && e.Kind == FailureCanceled && target == context.Canceled
}

// Result contains only a sanitized, single-frame image. Data is re-encoded to
// remove metadata, trailing payloads, and unrecognized chunks.
type Result struct {
	State        State
	Reason       FailureKind
	MIME         string
	Data         []byte
	Width        int
	Height       int
	CacheControl string
}

// Policy contains bounded security limits and an exact hostname allowlist.
type Policy struct {
	AllowedHosts   []string
	MaxInputBytes  int64
	MaxOutputBytes int
	MaxWidth       int
	MaxHeight      int
	MaxPixels      int
	Timeout        time.Duration
}

type Fetcher struct {
	allowed        map[string]struct{}
	client         *http.Client
	maxInputBytes  int64
	maxOutputBytes int
	maxWidth       int
	maxHeight      int
	maxPixels      int
}

// New constructs a fetcher with a DNS-pinning transport. Every hostname is
// resolved and checked before the connection is made directly to that address.
func New(policy Policy) (*Fetcher, error) {
	return newFetcher(policy, nil)
}

func newFetcher(policy Policy, transport http.RoundTripper) (*Fetcher, error) {
	normalized, err := normalizePolicy(policy)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(normalized.AllowedHosts))
	for _, host := range normalized.AllowedHosts {
		allowed[host] = struct{}{}
	}
	fetcher := &Fetcher{
		allowed: allowed, maxInputBytes: normalized.MaxInputBytes,
		maxOutputBytes: normalized.MaxOutputBytes, maxWidth: normalized.MaxWidth,
		maxHeight: normalized.MaxHeight, maxPixels: normalized.MaxPixels,
	}
	if transport == nil {
		transport = secureTransport()
	}
	fetcher.client = &http.Client{
		Transport: transport,
		Timeout:   normalized.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return &Error{Kind: FailureBlocked}
			}
			if _, err := fetcher.validateURL(req.URL); err != nil {
				return err
			}
			setRequestHeaders(req)
			return nil
		},
	}
	return fetcher, nil
}

func normalizePolicy(policy Policy) (Policy, error) {
	if len(policy.AllowedHosts) == 0 || len(policy.AllowedHosts) > maxAllowedHosts {
		return Policy{}, &Error{Kind: FailureBlocked}
	}
	policy.AllowedHosts = append([]string(nil), policy.AllowedHosts...)
	if policy.MaxInputBytes == 0 {
		policy.MaxInputBytes = defaultInputBytes
	}
	if policy.MaxOutputBytes == 0 {
		policy.MaxOutputBytes = defaultOutputBytes
	}
	if policy.MaxWidth == 0 {
		policy.MaxWidth = defaultDimension
	}
	if policy.MaxHeight == 0 {
		policy.MaxHeight = defaultDimension
	}
	if policy.MaxPixels == 0 {
		policy.MaxPixels = defaultPixels
	}
	if policy.Timeout == 0 {
		policy.Timeout = defaultTimeout
	}
	if policy.MaxInputBytes < 1 || policy.MaxInputBytes > hardMaxInputBytes ||
		policy.MaxOutputBytes < 1 || policy.MaxOutputBytes > hardMaxOutputBytes ||
		policy.MaxWidth < 1 || policy.MaxWidth > hardMaxDimension ||
		policy.MaxHeight < 1 || policy.MaxHeight > hardMaxDimension ||
		policy.MaxPixels < 1 || policy.MaxPixels > hardMaxPixels ||
		policy.Timeout < time.Millisecond || policy.Timeout > hardMaxTimeout {
		return Policy{}, &Error{Kind: FailureBlocked}
	}
	seen := make(map[string]struct{}, len(policy.AllowedHosts))
	for i, raw := range policy.AllowedHosts {
		host := strings.ToLower(raw)
		if raw != host || !validDNSName(host) || net.ParseIP(host) != nil {
			return Policy{}, &Error{Kind: FailureBlocked}
		}
		if _, duplicate := seen[host]; duplicate {
			return Policy{}, &Error{Kind: FailureBlocked}
		}
		seen[host] = struct{}{}
		policy.AllowedHosts[i] = host
	}
	return policy, nil
}

// Fetch downloads, validates, decodes, and re-encodes one image. On failure it
// returns a typed placeholder alongside a secret-free error.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (Result, error) {
	placeholder := Result{State: StatePlaceholder, CacheControl: "no-store"}
	parsed, err := url.Parse(rawURL)
	if err != nil || strings.TrimSpace(rawURL) != rawURL {
		return failed(placeholder, FailureInvalidURL)
	}
	parsed, err = f.validateURL(parsed)
	if err != nil {
		var safe *Error
		if errors.As(err, &safe) {
			return failed(placeholder, safe.Kind)
		}
		return failed(placeholder, FailureInvalidURL)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return failed(placeholder, FailureInvalidURL)
	}
	setRequestHeaders(request)
	response, err := f.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return failed(placeholder, FailureCanceled)
		}
		var safe *Error
		if errors.As(err, &safe) {
			return failed(placeholder, safe.Kind)
		}
		return failed(placeholder, FailureUnavailable)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return failed(placeholder, FailureUnavailable)
	}
	if encoding := response.Header.Get("Content-Encoding"); encoding != "" && !strings.EqualFold(encoding, "identity") {
		return failed(placeholder, FailureUnsupported)
	}
	if response.ContentLength > f.maxInputBytes {
		return failed(placeholder, FailureTooLarge)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, f.maxInputBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return failed(placeholder, FailureCanceled)
		}
		return failed(placeholder, FailureUnavailable)
	}
	if int64(len(raw)) > f.maxInputBytes {
		return failed(placeholder, FailureTooLarge)
	}
	data, mime, width, height, kind := f.sanitize(raw)
	if kind != "" {
		return failed(placeholder, kind)
	}
	return Result{
		State: StateReady, MIME: mime, Data: data, Width: width, Height: height,
		CacheControl: "no-store",
	}, nil
}

func (f *Fetcher) validateURL(candidate *url.URL) (*url.URL, error) {
	if candidate == nil || candidate.Scheme != "https" || candidate.Opaque != "" ||
		candidate.User != nil || candidate.Host == "" || candidate.Fragment != "" {
		return nil, &Error{Kind: FailureInvalidURL}
	}
	host := strings.ToLower(candidate.Hostname())
	if host != candidate.Hostname() || net.ParseIP(host) != nil || !validDNSName(host) {
		return nil, &Error{Kind: FailureBlocked}
	}
	if port := candidate.Port(); port != "" && port != "443" {
		return nil, &Error{Kind: FailureBlocked}
	}
	if _, ok := f.allowed[host]; !ok {
		return nil, &Error{Kind: FailureBlocked}
	}
	copy := *candidate
	copy.Scheme = "https"
	return &copy, nil
}

func validDNSName(host string) bool {
	if host == "" || len(host) > 253 || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	return true
}

func setRequestHeaders(request *http.Request) {
	request.Header = make(http.Header)
	request.Header.Set("Accept", "image/png, image/jpeg, image/webp")
	request.Header.Set("Cache-Control", "no-store")
	request.Header["User-Agent"] = []string{""}
	request.Host = ""
}

func failed(result Result, kind FailureKind) (Result, error) {
	result.Reason = kind
	return result, &Error{Kind: kind}
}

func (f *Fetcher) sanitize(raw []byte) ([]byte, string, int, int, FailureKind) {
	format, animated := sniffFormat(raw)
	if format == "" || animated {
		return nil, "", 0, 0, FailureUnsupported
	}
	var (
		config  image.Config
		decoded image.Image
		err     error
	)
	switch format {
	case "png":
		config, err = png.DecodeConfig(bytes.NewReader(raw))
	case "jpeg":
		config, err = jpeg.DecodeConfig(bytes.NewReader(raw))
	case "webp":
		config, err = webp.DecodeConfig(bytes.NewReader(raw))
	}
	if err != nil {
		return nil, "", 0, 0, FailureUnsupported
	}
	if config.Width < 1 || config.Height < 1 || config.Width > f.maxWidth || config.Height > f.maxHeight ||
		config.Width > f.maxPixels/config.Height {
		return nil, "", 0, 0, FailureTooLarge
	}
	switch format {
	case "png":
		decoded, err = png.Decode(bytes.NewReader(raw))
	case "jpeg":
		decoded, err = jpeg.Decode(bytes.NewReader(raw))
	case "webp":
		decoded, err = webp.Decode(bytes.NewReader(raw))
	}
	if err != nil {
		return nil, "", 0, 0, FailureUnsupported
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != config.Width || bounds.Dy() != config.Height {
		return nil, "", 0, 0, FailureUnsupported
	}
	output := &boundedBuffer{limit: f.maxOutputBytes}
	mime := "image/png"
	if format == "jpeg" {
		mime = "image/jpeg"
		err = jpeg.Encode(output, decoded, &jpeg.Options{Quality: 90})
	} else {
		err = png.Encode(output, decoded)
	}
	if err != nil {
		if errors.Is(err, errOutputLimit) {
			return nil, "", 0, 0, FailureTooLarge
		}
		return nil, "", 0, 0, FailureUnsupported
	}
	return append([]byte(nil), output.Bytes()...), mime, config.Width, config.Height, ""
}

func sniffFormat(raw []byte) (format string, animated bool) {
	if len(raw) >= 8 && bytes.Equal(raw[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return "png", pngAnimated(raw)
	}
	if len(raw) >= 3 && raw[0] == 0xff && raw[1] == 0xd8 && raw[2] == 0xff {
		return "jpeg", false
	}
	if len(raw) >= 12 && string(raw[:4]) == "RIFF" && string(raw[8:12]) == "WEBP" {
		return "webp", webpAnimated(raw)
	}
	return "", false
}

func pngAnimated(raw []byte) bool {
	for offset := 8; offset+12 <= len(raw); {
		length := int(binary.BigEndian.Uint32(raw[offset : offset+4]))
		if length < 0 || length > len(raw)-offset-12 {
			return true
		}
		chunk := string(raw[offset+4 : offset+8])
		if chunk == "acTL" || chunk == "fcTL" || chunk == "fdAT" {
			return true
		}
		offset += 12 + length
		if chunk == "IEND" {
			return false
		}
	}
	return true
}

func webpAnimated(raw []byte) bool {
	declared := int64(binary.LittleEndian.Uint32(raw[4:8])) + 8
	if declared != int64(len(raw)) {
		return true
	}
	for offset := 12; offset+8 <= len(raw); {
		length := int(binary.LittleEndian.Uint32(raw[offset+4 : offset+8]))
		padded := length + length%2
		if length < 0 || padded > len(raw)-offset-8 {
			return true
		}
		chunk := string(raw[offset : offset+4])
		if chunk == "ANIM" || chunk == "ANMF" || (chunk == "VP8X" && length > 0 && raw[offset+8]&0x02 != 0) {
			return true
		}
		offset += 8 + padded
	}
	return false
}

var errOutputLimit = errors.New("sanitized image exceeds output limit")

type boundedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(data []byte) (int, error) {
	if len(data) > b.limit-b.Len() {
		return 0, errOutputLimit
	}
	return b.Buffer.Write(data)
}
