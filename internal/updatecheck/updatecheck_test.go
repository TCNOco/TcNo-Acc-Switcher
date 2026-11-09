package updatecheck

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/appclient"
)

func TestHandleLaunchAPICheckResult_UpdateAvailable(t *testing.T) {
	t.Parallel()
	var got string
	HandleLaunchAPICheckResult(LaunchAPICheckResult{
		Latest:  "4.1.2",
		Message: "update ready",
	}, "", "4.1.1", func(message string) {
		got = message
	}, func() {
		t.Fatal("did not expect failure callback")
	})
	if got != "update ready" {
		t.Fatalf("got update message %q, want %q", got, "update ready")
	}
}

func TestLaunchUpdateCheckIsQuietWhenNetworkIsUnavailable(t *testing.T) {
	t.Parallel()
	for _, err := range []error{appclient.ErrOfflineMode, context.DeadlineExceeded, &net.DNSError{Err: "no such host"}, errInvalidVersionResponse} {
		HandleLaunchAPICheckResult(LaunchAPICheckResult{Err: err}, "", "4.0.8", func(string) {
			t.Fatal("offline check announced an update")
		}, func() { t.Fatal("offline startup interrupted the user with a failure toast") })
	}
}

type updateTestTransport func(*http.Request) (*http.Response, error)

func (f updateTestTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestUpdateCheckRejectsCaptivePortalAndEmptyResponse(t *testing.T) {
	for _, body := range []string{"", "<html>Sign in to Wi-Fi</html>", "not a version"} {
		client := &http.Client{Transport: updateTestTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		latest, _, err := FetchLatestVersion(context.Background(), client, "4.0.8")
		if !errors.Is(err, errInvalidVersionResponse) || latest != "" {
			t.Fatalf("untrusted response announced a version: latest=%q err=%v", latest, err)
		}
	}
}

func TestManualUpdateCheckReportsDisconnectedNetwork(t *testing.T) {
	previous := appclient.Shared
	t.Cleanup(func() { appclient.Shared = previous })
	appclient.Shared = &http.Client{Transport: updateTestTransport(func(*http.Request) (*http.Response, error) {
		return nil, &net.DNSError{Err: "network unavailable"}
	})}
	if got := RunManualCheck(context.Background(), "4.0.8", func(string) {
		t.Fatal("offline check announced an update")
	}); got != "offline" {
		t.Fatalf("manual disconnected check returned %q", got)
	}
}

func TestHandleLaunchAPICheckResult_UpToDate(t *testing.T) {
	t.Parallel()
	HandleLaunchAPICheckResult(LaunchAPICheckResult{
		Latest:  "4.1.1",
		Message: "ignored",
	}, "", "4.1.2", func(string) {
		t.Fatal("did not expect update callback")
	}, func() {
		t.Fatal("did not expect failure callback")
	})
}

func TestHandleLaunchAPICheckResult_ThrottlesFailureCallback(t *testing.T) {
	exeDir := t.TempDir()
	calls := 0
	onFailed := func() {
		calls++
	}

	result := LaunchAPICheckResult{Err: errors.New("network down")}
	HandleLaunchAPICheckResult(result, exeDir, "4.1.1", nil, onFailed)
	HandleLaunchAPICheckResult(result, exeDir, "4.1.1", nil, onFailed)

	if calls != 1 {
		t.Fatalf("failure callback calls = %d, want 1", calls)
	}
}
