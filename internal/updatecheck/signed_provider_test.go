package updatecheck

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

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

func newTestProvider(release *updater.Release, apiBase string, client ...*http.Client) *signedGitHubProvider {
	var httpClient *http.Client
	if len(client) > 0 {
		httpClient = client[0]
	}
	return &signedGitHubProvider{
		inner:     &stubProvider{release: release},
		owner:     "owner",
		repo:      "repo",
		sigSuffix: ".exe.sig",
		apiBase:   apiBase,
		client:    httpClient,
	}
}

func TestSignedProviderAttachesSignature(t *testing.T) {
	sig := []byte("signature-bytes-0123456789012345678901234567890123456789012345678901234567890123")
	var srv *httptest.Server
	srv = httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases/tags/v1.2.3":
			fmt.Fprintf(w, `{"assets":[{"name":"TcNo-Acc-Switcher.exe.sig","browser_download_url":"%s/sig"}]}`, srv.URL)
		case "/sig":
			fmt.Fprintln(w, base64.StdEncoding.EncodeToString(sig))
		default:
			http.NotFound(w, r)
		}
	}))
	client := srv.Client()

	p := newTestProvider(&updater.Release{Version: "1.2.3"}, srv.URL, client)
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
			srv := httptest.NewTestServer(t, handler)
			client := srv.Client()

			p := newTestProvider(&updater.Release{Version: "1.2.3"}, srv.URL, client)
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

// A release carries one signature per artifact, so matching by suffix alone can
// pick another platform's.
func TestSignedProviderPicksTheSignatureForItsOwnArtifact(t *testing.T) {
	linuxSig := []byte("linux-signature-01234567890123456789012345678901234567890123456789012345678")
	var srv *httptest.Server
	srv = httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases/tags/v1.2.3":
			fmt.Fprintf(w, `{"assets":[
				{"name":"TcNo-Acc-Switcher.exe.sig","browser_download_url":"%s/windows"},
				{"name":"TcNo-Acc-Switcher-linux-amd64.tar.gz.sig","browser_download_url":"%s/linux"}
			]}`, srv.URL, srv.URL)
		case "/linux":
			fmt.Fprintln(w, base64.StdEncoding.EncodeToString(linuxSig))
		default:
			http.Error(w, "wrong signature fetched", http.StatusNotFound)
		}
	}))
	client := srv.Client()

	release := &updater.Release{Version: "1.2.3"}
	release.Artifact.Filename = "TcNo-Acc-Switcher-linux-amd64.tar.gz"

	got, err := newTestProvider(release, srv.URL, client).Check(context.Background(), updater.CheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Verification == nil || string(got.Verification.Signature) != string(linuxSig) {
		t.Errorf("attached the wrong signature: %q", got.Verification.Signature)
	}
}

func TestSignatureCheckCancelsInFlightRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := httptest.NewTestServer(t, http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
			<-request.Context().Done()
		}))
		client := srv.Client()
		provider := newTestProvider(&updater.Release{Version: "1.2.3"}, srv.URL, client)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		started := time.Now()
		if release, err := provider.Check(ctx, updater.CheckRequest{}); release != nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("incomplete signature check accepted: release=%v err=%v", release, err)
		}
		if elapsed := time.Since(started); elapsed != 5*time.Second {
			t.Fatalf("request did not honor its deadline: %v", elapsed)
		}
	})
}
