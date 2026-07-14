package steam

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/profileimage"
)

func TestFetchProfileXMLWithRetryMarksTransientFailurePendingThenSucceeds(t *testing.T) {
	timeoutErr := &url.Error{
		Op:  "Get",
		URL: "https://steamcommunity.com/profiles/76561198000000000?xml=1",
		Err: context.DeadlineExceeded,
	}
	want := ProfileXMLFields{SteamID64: "76561198000000000", CommunityDisplayName: "Recovered"}
	attempts := 0
	retryStates := make([]AccountPatch, 0, 1)

	got, err := fetchProfileXMLWithRetry(
		context.Background(),
		profileXMLRetryPolicy{MaxAttempts: 2, AttemptTimeout: time.Second},
		func(context.Context) (ProfileXMLFields, error) {
			attempts++
			if attempts == 1 {
				return ProfileXMLFields{}, timeoutErr
			}
			return want, nil
		},
		func(err error) {
			message, pending := profileRefreshErrorState(err, true)
			retryStates = append(retryStates, AccountPatch{Error: message, MetaPending: pending})
		},
	)
	if err != nil {
		t.Fatalf("fetchProfileXMLWithRetry: %v", err)
	}
	if got != want {
		t.Fatalf("fields = %#v, want %#v", got, want)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if len(retryStates) != 1 {
		t.Fatalf("retry states = %d, want 1", len(retryStates))
	}
	if retryStates[0].Error != "" || !retryStates[0].MetaPending {
		t.Fatalf("retry state = %#v, want pending without a raw error", retryStates[0])
	}
}

func TestProfileRefreshErrorStateSanitizesExhaustedTransientError(t *testing.T) {
	raw := &url.Error{
		Op:  "Get",
		URL: "https://steamcommunity.com/profiles/76561198000000000?xml=1",
		Err: context.DeadlineExceeded,
	}

	message, pending := profileRefreshErrorState(raw, false)
	if pending {
		t.Fatal("exhausted retry must not remain pending")
	}
	if message != temporaryProfileRefreshMessage {
		t.Fatalf("message = %q, want %q", message, temporaryProfileRefreshMessage)
	}
}

func TestProfileRefreshErrorStateNeverExposesRawTransportError(t *testing.T) {
	raw := &url.Error{
		Op:  "Get",
		URL: "https://steamcommunity.com/profiles/76561198000000000?xml=1",
		Err: errors.New("certificate verification failed"),
	}

	message, pending := profileRefreshErrorState(raw, false)
	if pending || message != temporaryProfileRefreshMessage {
		t.Fatalf("transport state = %q/%t, want %q/false", message, pending, temporaryProfileRefreshMessage)
	}
}

func TestFetchProfileXMLWithRetryPreservesPermanentError(t *testing.T) {
	wantErr := &profileXMLHTTPError{StatusCode: 404}
	attempts := 0
	retries := 0

	_, err := fetchProfileXMLWithRetry(
		context.Background(),
		profileXMLRetryPolicy{MaxAttempts: 3, AttemptTimeout: time.Second},
		func(context.Context) (ProfileXMLFields, error) {
			attempts++
			return ProfileXMLFields{}, wantErr
		},
		func(error) { retries++ },
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if attempts != 1 || retries != 0 {
		t.Fatalf("attempts/retries = %d/%d, want 1/0", attempts, retries)
	}
	message, pending := profileRefreshErrorState(err, false)
	if message != wantErr.Error() || pending {
		t.Fatalf("permanent state = %q/%t, want %q/false", message, pending, wantErr)
	}
}

func TestRefreshAllSteamImagesClearsProfileMetadataCaches(t *testing.T) {
	paths.ResetForTest(t.TempDir())

	const steamID = "76561198000000000"

	xmlPath, err := xmlCachePath(steamID)
	if err != nil {
		t.Fatalf("xmlCachePath: %v", err)
	}
	miniPath, err := miniprofileCachePath(steamID)
	if err != nil {
		t.Fatalf("miniprofileCachePath: %v", err)
	}
	vacPath, err := vacCachePath()
	if err != nil {
		t.Fatalf("vacCachePath: %v", err)
	}
	profileDir, err := profileimage.ProfileDir(PlatformKey)
	if err != nil {
		t.Fatalf("ProfileDir: %v", err)
	}
	imagePath := filepath.Join(profileDir, steamID+".webp")

	writeTestFile(t, xmlPath, "<profile><steamID>Old Name</steamID></profile>")
	writeTestFile(t, miniPath, `<span class="persona">Old Mini</span>`)
	writeTestFile(t, vacPath, `[{"SteamID":"`+steamID+`","Vac":false,"Ltd":false}]`)
	writeTestFile(t, imagePath, "old image")

	svc := NewSteamService()
	if err := svc.RefreshAllSteamImages(); err != nil {
		t.Fatalf("RefreshAllSteamImages: %v", err)
	}
	stopRefreshTimer(svc)

	assertMissing(t, xmlPath)
	assertMissing(t, miniPath)
	assertMissing(t, vacPath)
	assertMissing(t, imagePath)
}

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, stat err=%v", path, err)
	}
}

func stopRefreshTimer(svc *SteamService) {
	svc.refreshMu.Lock()
	defer svc.refreshMu.Unlock()
	if svc.refreshTimer != nil {
		svc.refreshTimer.Stop()
		svc.refreshTimer = nil
	}
}
