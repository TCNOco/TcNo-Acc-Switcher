package steam

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
)

const xmlCacheTTL = 24 * time.Hour

// ProfileXMLFields are extracted from community profile ?xml=1.
type ProfileXMLFields struct {
	SteamID64 string
	// CommunityDisplayName is the public profile title from <steamID> (not the login name).
	CommunityDisplayName string
	VacBanned            bool
	Limited              bool
	AvatarFullURL        string
	Private              bool
}

type xmlProfileDoc struct {
	XMLName             xml.Name `xml:"profile"`
	PrivacyMessage      []string `xml:"privacyMessage"`
	SteamID64           string   `xml:"steamID64"`
	SteamCommunityTitle string   `xml:"steamID"`
	VacBanned           string   `xml:"vacBanned"`
	IsLimited           string   `xml:"isLimitedAccount"`
	AvatarFull          string   `xml:"avatarFull"`
}

func xmlCachePath(steamID64 string) (string, error) {
	r, err := paths.LoginCacheDir("Steam")
	if err != nil {
		return "", err
	}
	return filepath.Join(r, "VACCache", steamID64+".xml"), nil
}

// FetchProfileXML downloads or loads cached profile XML and parses ban/avatar fields.
func FetchProfileXML(ctx context.Context, client *http.Client, steamID64 string) (ProfileXMLFields, error) {
	cache, err := xmlCachePath(steamID64)
	if err != nil {
		return ProfileXMLFields{}, err
	}
	url := fmt.Sprintf("https://steamcommunity.com/profiles/%s?xml=1", steamID64)

	var data []byte
	if st, err := os.Stat(cache); err == nil && !st.IsDir() && time.Since(st.ModTime()) < xmlCacheTTL {
		data, err = os.ReadFile(cache)
		if err != nil {
			data = nil
		}
	}
	if len(data) == 0 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return ProfileXMLFields{}, err
		}
		req.Header.Set("User-Agent", "TcNo Account Switcher")
		resp, err := client.Do(req)
		if err != nil {
			return ProfileXMLFields{}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return ProfileXMLFields{}, fmt.Errorf("profile XML HTTP %d", resp.StatusCode)
		}
		data, err = io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if err != nil {
			return ProfileXMLFields{}, err
		}
		_ = os.MkdirAll(filepath.Dir(cache), 0o755)
		_ = fsutil.WriteFileAtomic(cache, data, 0o644)
	}

	var doc xmlProfileDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return ProfileXMLFields{}, err
	}
	if len(doc.PrivacyMessage) > 0 && strings.TrimSpace(doc.PrivacyMessage[0]) != "" {
		return ProfileXMLFields{SteamID64: doc.SteamID64, Private: true}, nil
	}
	return ProfileXMLFields{
		SteamID64:            strings.TrimSpace(doc.SteamID64),
		CommunityDisplayName: strings.TrimSpace(doc.SteamCommunityTitle),
		VacBanned:            strings.TrimSpace(doc.VacBanned) == "1",
		Limited:              strings.TrimSpace(doc.IsLimited) == "1",
		AvatarFullURL:        strings.TrimSpace(doc.AvatarFull),
	}, nil
}

// CachedCommunityDisplayName returns <steamID> from on-disk profile XML cache if present, ignoring TTL.
func CachedCommunityDisplayName(steamID64 string) string {
	p, err := xmlCachePath(steamID64)
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(p)
	if err != nil || len(data) == 0 {
		return ""
	}
	var doc xmlProfileDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return ""
	}
	return strings.TrimSpace(doc.SteamCommunityTitle)
}
