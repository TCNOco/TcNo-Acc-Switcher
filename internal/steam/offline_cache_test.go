package steam

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/profileimage"
)

type offlineCacheTransport func(*http.Request) (*http.Response, error)

func (f offlineCacheTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestExpiredSteamMetadataSurvivesOfflineRefresh(t *testing.T) {
	previousLimiter := miniprofileLimiter
	miniprofileLimiter = newRequestWindow(miniprofileBudget, miniprofileWindow)
	t.Cleanup(func() { miniprofileLimiter = previousLimiter })
	paths.ResetForTest(t.TempDir())
	const id = "76561198000000000"
	xmlPath, _ := xmlCachePath(id)
	miniPath, _ := miniprofileCachePath(id)
	oldXML := `<profile><steamID64>` + id + `</steamID64><steamID>Cached Name</steamID><vacBanned>1</vacBanned><avatarFull>https://example.invalid/avatar.jpg</avatarFull></profile>`
	oldMini := `<div class="miniprofile_container"><span class="persona online">Cached Mini Name</span></div>`
	writeTestFile(t, xmlPath, oldXML)
	writeTestFile(t, miniPath, oldMini)
	if err := markCacheFileStale(xmlPath); err != nil {
		t.Fatal(err)
	}
	if err := markCacheFileStale(miniPath); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		status int
		body   string
		err    error
	}{
		{name: "DNS unavailable", err: &net.DNSError{Err: "no such host"}},
		{name: "network timeout", err: context.DeadlineExceeded},
		{name: "server unavailable", status: 503},
		{name: "captive portal", status: 200, body: "<html>sign in to Wi-Fi</html>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: offlineCacheTransport(func(req *http.Request) (*http.Response, error) {
				if tc.err != nil {
					return nil, tc.err
				}
				return &http.Response{StatusCode: tc.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.body)), Request: req}, nil
			})}
			fields, err := FetchProfileXML(context.Background(), client, id)
			if err != nil || !fields.Stale || fields.CommunityDisplayName != "Cached Name" || !fields.VacBanned {
				t.Fatalf("cached profile lost: fields=%#v err=%v", fields, err)
			}
			miniCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			_, mini, stale, err := fetchMiniprofile(miniCtx, client, id, 7)
			cancel()
			if err != nil || !stale || ExtractMiniprofileDisplayName(mini) != "Cached Mini Name" {
				t.Fatalf("cached miniprofile lost: %v", err)
			}
			for path, want := range map[string]string{xmlPath: oldXML, miniPath: oldMini} {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != want {
					t.Fatalf("cache changed on failure: %s err=%v", path, err)
				}
				assertStale(t, path)
			}
		})
	}
	client := &http.Client{Transport: offlineCacheTransport(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(strings.ReplaceAll(oldXML, "Cached Name", "Updated Name"))), Request: req}, nil
	})}
	fields, err := FetchProfileXML(context.Background(), client, id)
	if err != nil || fields.Stale || fields.CommunityDisplayName != "Updated Name" {
		t.Fatalf("profile did not recover: %v", err)
	}
}

func TestMiniprofileAssetFailureKeepsPreviousFragment(t *testing.T) {
	previousLimiter := miniprofileLimiter
	miniprofileLimiter = newRequestWindow(miniprofileBudget, miniprofileWindow)
	t.Cleanup(func() { miniprofileLimiter = previousLimiter })
	paths.ResetForTest(t.TempDir())
	const id = "76561198000000000"
	path, _ := miniprofileCachePath(id)
	old := `<div class="miniprofile_container"><span class="persona">Cached Name</span></div>`
	writeTestFile(t, path, old)
	if err := markCacheFileStale(path); err != nil {
		t.Fatal(err)
	}
	calls := 0
	client := &http.Client{Transport: offlineCacheTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Hostname() != "steamcommunity.com" {
			return nil, &net.DNSError{Err: "network unavailable"}
		}
		body := `<div class="miniprofile_container"><span class="persona">New Name</span><div class="playersection_avatar_frame"><img src="https://shared.akamai.steamstatic.com/community_assets/images/items/1/frame.png"></div></div>`
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})}
	_, mini, stale, err := fetchMiniprofile(context.Background(), client, id, 7)
	if err != nil || !stale || calls != 2 || ExtractMiniprofileDisplayName(mini) != "Cached Name" {
		t.Fatalf("partial download displaced cached miniprofile: stale=%t calls=%d err=%v", stale, calls, err)
	}
	if got, _ := os.ReadFile(path); string(got) != old {
		t.Fatal("partial download overwrote the cached fragment")
	}
}

func TestExpiredSteamAnimationSurvivesDisconnectedRefresh(t *testing.T) {
	paths.ResetForTest(t.TempDir())
	const id = "76561198000000000"
	dir, _ := profileimage.ProfileDir(PlatformKey)
	main := dir + string(os.PathSeparator) + id + ".webm"
	static := dir + string(os.PathSeparator) + steamStaticAvatarID(id) + ".jpg"
	writeTestFile(t, main, "cached-animation")
	writeTestFile(t, static, "cached-static")
	if err := markCacheFileStale(main); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: offlineCacheTransport(func(*http.Request) (*http.Response, error) {
		return nil, &net.DNSError{Err: "network unavailable"}
	})}
	remote := `<div class="miniprofile_container"><div class="playerAvatarAutoSizeInner"><video><source src="https://cdn.akamai.steamstatic.com/steamcommunity/public/images/items/1/avatar.webm"></video></div></div>`
	imageURL, staticURL, stale, err := downloadSteamAccountAvatars(context.Background(), client, id, "https://example.invalid/avatar.jpg", remote, true, 7)
	if err != nil || !stale || !strings.HasSuffix(imageURL, ".webm") || !strings.HasSuffix(staticURL, ".jpg") {
		t.Fatalf("animation lost on failed refresh: image=%q static=%q stale=%t err=%v", imageURL, staticURL, stale, err)
	}
	local := strings.ReplaceAll(remote, "https://cdn.akamai.steamstatic.com/steamcommunity/public/images/items/1/avatar.webm", imageURL)
	imageURL, _, _, err = downloadSteamAccountAvatars(context.Background(), client, id, "https://example.invalid/avatar.jpg", local, true, 7)
	if err != nil || !strings.HasSuffix(imageURL, ".webm") {
		t.Fatalf("localized animation was treated as a missing animation: %q %v", imageURL, err)
	}
	if got, _ := os.ReadFile(main); string(got) != "cached-animation" {
		t.Fatal("failed animation refresh changed the cached file")
	}
}

func TestExpiredVACCacheRemainsReadable(t *testing.T) {
	paths.ResetForTest(t.TempDir())
	rows := []VacEntry{{SteamID: "76561198000000000", Vac: true, Ltd: true}}
	if err := SaveVacCache(rows); err != nil {
		t.Fatal(err)
	}
	path, _ := vacCachePath()
	old := time.Now().Add(-365 * 24 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	got, err := LoadVacCache(1)
	if err != nil || len(got) != 1 || got[0] != rows[0] {
		t.Fatalf("expired flags disappeared: %v %v", got, err)
	}
}
