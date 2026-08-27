package steam

import (
	"context"
	"encoding/xml"
	"errors"
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

type profileXMLHTTPError struct {
	StatusCode int
}

func (e *profileXMLHTTPError) Error() string {
	return fmt.Sprintf("profile XML HTTP %d", e.StatusCode)
}

// profileXMLBodyError marks a 200 response whose body is not a profile document.
//
// A captive portal, an ISP interception page or a truncated read all land here,
// and none of them say anything about the account - so it is a distinct type
// from a real HTTP status, and the refresh treats it as worth retrying.
type profileXMLBodyError struct {
	err error
}

func (e *profileXMLBodyError) Error() string { return "profile XML unreadable: " + e.err.Error() }

func (e *profileXMLBodyError) Unwrap() error { return e.err }

// parseProfileXMLDoc reads a body as a community profile, reporting anything
// that is not one as a body error rather than a fact about the account.
func parseProfileXMLDoc(data []byte) (xmlProfileDoc, error) {
	var doc xmlProfileDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return xmlProfileDoc{}, &profileXMLBodyError{err: err}
	}
	// A profile always identifies itself; a private one says so instead. A body
	// with neither parsed as XML by accident and carries nothing usable.
	if strings.TrimSpace(doc.SteamID64) == "" && len(doc.PrivacyMessage) == 0 {
		return xmlProfileDoc{}, &profileXMLBodyError{err: errors.New("no steamID64 in profile document")}
	}
	return doc, nil
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
	cached := false
	if st, err := os.Stat(cache); err == nil && !st.IsDir() && time.Since(st.ModTime()) < xmlCacheTTL {
		data, err = os.ReadFile(cache)
		if err != nil {
			data = nil
		}
		cached = len(data) > 0
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
			return ProfileXMLFields{}, &profileXMLHTTPError{StatusCode: resp.StatusCode}
		}
		data, err = io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if err != nil {
			return ProfileXMLFields{}, err
		}
	}

	// Parsed before it is cached, and the cache is dropped when it stops
	// parsing: caching first pins an interception page over the real profile for
	// the whole 24h lifetime, and every retry re-reads it instead of asking
	// Steam again.
	doc, err := parseProfileXMLDoc(data)
	if err != nil {
		if cached {
			_ = os.Remove(cache)
		}
		return ProfileXMLFields{}, err
	}
	if !cached {
		_ = os.MkdirAll(filepath.Dir(cache), 0o755)
		_ = fsutil.WriteFileAtomic(cache, data, 0o644)
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

// CachedCommunityDisplayName returns the freshest known public persona label for an account.
// Miniprofile HTML is preferred over profile XML because it updates more often than loginusers.vdf.
func CachedCommunityDisplayName(steamID64 string) string {
	return communityDisplayNameFrom(ReadCachedMiniprofileHTML(steamID64), steamID64)
}

// communityDisplayNameFrom is CachedCommunityDisplayName for a caller that has
// already read the account's miniprofile HTML. Reading it costs a file read plus
// a full HTML parse and sanitise pass, so the account list passes in the copy it
// already holds rather than paying for a second identical one per account.
func communityDisplayNameFrom(miniProfileHTML, steamID64 string) string {
	if n := ExtractMiniprofileDisplayName(miniProfileHTML); n != "" {
		return n
	}
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
