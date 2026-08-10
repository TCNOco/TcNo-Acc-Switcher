package updatecheck

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

type stubProvider struct {
	release *updater.Release
}

func (s *stubProvider) Name() string { return "stub" }

func (s *stubProvider) Check(ctx context.Context, req updater.CheckRequest) (*updater.Release, error) {
	return s.release, nil
}

func (s *stubProvider) Download(ctx context.Context, r *updater.Release, dst io.Writer, onProgress func(written, total int64)) error {
	return nil
}

func newTestProvider(release *updater.Release, apiBase string) *signedGitHubProvider {
	return &signedGitHubProvider{
		inner:     &stubProvider{release: release},
		owner:     "owner",
		repo:      "repo",
		sigSuffix: ".exe.sig",
		apiBase:   apiBase,
	}
}

func TestSignedProviderAttachesSignature(t *testing.T) {
	sig := []byte("signature-bytes-0123456789012345678901234567890123456789012345678901234567890123")
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases/tags/v1.2.3":
			fmt.Fprintf(w, `{"assets":[{"name":"TcNo-Acc-Switcher.exe.sig","browser_download_url":"%s/sig"}]}`, srv.URL)
		case "/sig":
			fmt.Fprintln(w, base64.StdEncoding.EncodeToString(sig))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := newTestProvider(&updater.Release{Version: "1.2.3"}, srv.URL)
	r, err := p.Check(context.Background(), updater.CheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Verification == nil || r.Verification.SignatureAlgo != "ed25519" {
		t.Fatalf("Verification = %+v, want ed25519 signature attached", r.Verification)
	}
	if string(r.Verification.Signature) != string(sig) {
		t.Fatalf("Signature = %q, want %q", r.Verification.Signature, sig)
	}
}

// A release whose signature cannot be fetched must not pass through
// unverified — Check has to fail closed, not fall back to digest-only.
func TestSignedProviderFailsClosedWithoutSignature(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"api error": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "rate limited", http.StatusForbidden)
		},
		"no sig asset": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"assets":[{"name":"TcNo-Acc-Switcher.exe","browser_download_url":"https://example.invalid/exe"}]}`)
		},
	}
	for name, handler := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(handler)
			defer srv.Close()

			p := newTestProvider(&updater.Release{Version: "1.2.3"}, srv.URL)
			r, err := p.Check(context.Background(), updater.CheckRequest{})
			if err == nil {
				t.Fatalf("Check = %+v, nil; want error when signature is unavailable", r)
			}
		})
	}
}

func TestSignedProviderPassesThroughUpToDate(t *testing.T) {
	p := newTestProvider(nil, "http://unused.invalid")
	r, err := p.Check(context.Background(), updater.CheckRequest{})
	if err != nil || r != nil {
		t.Fatalf("Check = %v, %v; want nil, nil for up to date", r, err)
	}
}
