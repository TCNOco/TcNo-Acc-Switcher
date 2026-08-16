package serverpicker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"

	buildinfo "TcNo-Acc-Switcher/build"
	"TcNo-Acc-Switcher/internal/api"
	"TcNo-Acc-Switcher/internal/appclient"
)

const (
	sdrURLTemplate  = "https://api.steampowered.com/ISteamApps/GetSDRConfig/v1/?appid=%d"
	maxSDRBodyBytes = 4 << 20
)

// POP is one Steam Datagram Relay point of presence that has relays we can act on.
type POP struct {
	ID    string
	Desc  string
	Lon   float64
	Lat   float64
	Relay []string
}

// SDRConfig is the subset of GetSDRConfig we use.
type SDRConfig struct {
	Revision int64
	POPs     []POP
}

// parseSDRConfig decodes a GetSDRConfig response. POPs without relays are
// dropped: they expose no IPs, so there is nothing to block. Result is sorted
// by POP id so callers get a stable order without re-sorting a map.
func parseSDRConfig(raw []byte) (SDRConfig, error) {
	var payload struct {
		Revision int64 `json:"revision"`
		POPs     map[string]struct {
			Desc   string    `json:"desc"`
			Geo    []float64 `json:"geo"`
			Relays []struct {
				IPv4 string `json:"ipv4"`
			} `json:"relays"`
		} `json:"pops"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return SDRConfig{}, fmt.Errorf("server picker: decode SDR config: %w", err)
	}
	if len(payload.POPs) == 0 {
		return SDRConfig{}, fmt.Errorf("server picker: SDR config has no relay data")
	}

	cfg := SDRConfig{Revision: payload.Revision}
	for id, p := range payload.POPs {
		ips := make([]string, 0, len(p.Relays))
		seen := make(map[string]struct{}, len(p.Relays))
		for _, r := range p.Relays {
			ip := strings.TrimSpace(r.IPv4)
			if ip == "" || net.ParseIP(ip) == nil {
				continue
			}
			if _, dup := seen[ip]; dup {
				continue
			}
			seen[ip] = struct{}{}
			ips = append(ips, ip)
		}
		if len(ips) == 0 {
			continue
		}
		pop := POP{
			ID:    strings.TrimSpace(id),
			Desc:  strings.TrimSpace(p.Desc),
			Relay: ips,
		}
		if len(p.Geo) >= 2 {
			pop.Lon, pop.Lat = p.Geo[0], p.Geo[1]
		}
		if pop.Desc == "" {
			pop.Desc = pop.ID
		}
		cfg.POPs = append(cfg.POPs, pop)
	}
	if len(cfg.POPs) == 0 {
		return SDRConfig{}, fmt.Errorf("server picker: SDR config has no POPs with relays")
	}
	sort.Slice(cfg.POPs, func(i, j int) bool { return cfg.POPs[i].ID < cfg.POPs[j].ID })
	return cfg, nil
}

func fetchSDRConfig(ctx context.Context, appID int) (SDRConfig, error) {
	if appclient.IsOfflineMode() {
		return SDRConfig{}, appclient.ErrOfflineMode
	}
	url := fmt.Sprintf(sdrURLTemplate, appID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return SDRConfig{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", api.UserAgent(buildinfo.Version()))

	resp, err := appclient.Shared.Do(req)
	if err != nil {
		return SDRConfig{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SDRConfig{}, fmt.Errorf("server picker: GET SDR config: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSDRBodyBytes))
	if err != nil {
		return SDRConfig{}, err
	}
	return parseSDRConfig(body)
}
