package updatecheck

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

type signedGitHubProvider struct {
	inner     *github.Provider
	owner     string
	repo      string
	sigSuffix string
}

func NewSignedGitHubProvider(cfg github.Config, sigSuffix string) (updater.Provider, error) {
	inner, err := github.New(cfg)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(cfg.Repository, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository: %s", cfg.Repository)
	}
	return &signedGitHubProvider{
		inner:     inner,
		owner:     parts[0],
		repo:      parts[1],
		sigSuffix: sigSuffix,
	}, nil
}

func (p *signedGitHubProvider) Name() string { return p.inner.Name() }

func (p *signedGitHubProvider) Check(ctx context.Context, req updater.CheckRequest) (*updater.Release, error) {
	r, err := p.inner.Check(ctx, req)
	if err != nil || r == nil {
		return r, err
	}

	sig, err := p.fetchSignature(ctx, r.Version)
	if err != nil {
		return r, nil
	}

	if r.Verification == nil {
		r.Verification = &updater.Verification{}
	}
	r.Verification.SignatureAlgo = "ed25519"
	r.Verification.Signature = sig

	return r, nil
}

func (p *signedGitHubProvider) Download(ctx context.Context, r *updater.Release, dst io.Writer, onProgress func(written, total int64)) error {
	return p.inner.Download(ctx, r, dst, onProgress)
}

func (p *signedGitHubProvider) fetchSignature(ctx context.Context, version string) ([]byte, error) {
	tag := "v" + version
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", p.owner, p.repo, tag)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "TcNo-Acc-Switcher-Updater")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api: %s", resp.Status)
	}

	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	for _, a := range release.Assets {
		if strings.HasSuffix(a.Name, p.sigSuffix) {
			return p.downloadSig(ctx, a.BrowserDownloadURL)
		}
	}
	return nil, fmt.Errorf("signature file %q not found in release", p.sigSuffix)
}

func (p *signedGitHubProvider) downloadSig(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TcNo-Acc-Switcher-Updater")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download sig: %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return nil, err
	}

	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(body)))
	if err != nil {
		return nil, fmt.Errorf("decoding signature: %w", err)
	}
	return sig, nil
}
