package updatecheck

import (
	"errors"
	"testing"
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
