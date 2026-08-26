package discordrpc

import (
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
)

// slowConnect stands in for a Discord that is not running. It fails rather than
// succeeding, so the manager never reaches richgo's logout path.
func slowConnect(tb testing.TB, d time.Duration, calls *atomic.Int32) {
	tb.Helper()
	previous := discordLogin
	discordLogin = func(string) error {
		calls.Add(1)
		time.Sleep(d)
		return errors.New("discord not running")
	}
	tb.Cleanup(func() { discordLogin = previous })
}

// newManagerTestEnv points the settings loader at a temp dir, where DiscordRpc
// defaults to true.
func newManagerTestEnv(tb testing.TB) {
	tb.Helper()
	exeDir := tb.TempDir()
	platform.ResetPathSingletonsForTest(exeDir)
	paths.ResetForTest(filepath.Join(exeDir, "TcNo Account Switcher"))
}

// Start runs before the window exists, so it must not wait on the connect.
func TestStartDoesNotBlockOnTheDiscordConnect(t *testing.T) {
	newManagerTestEnv(t)
	var calls atomic.Int32
	slowConnect(t, 2*time.Second, &calls)

	m := NewManager()
	t.Cleanup(m.Stop)

	begin := time.Now()
	m.Start()
	elapsed := time.Since(begin)
	t.Logf("Start() returned in %s with a 2s connect stubbed in", elapsed.Round(time.Millisecond))

	if elapsed > 500*time.Millisecond {
		t.Fatalf("Start blocked for %s; it must not wait on the Discord connect", elapsed)
	}
}

// A Discord that is not running must not cost a two-second dial on every
// refresh for the rest of the session.
func TestConnectBacksOffAfterAFailure(t *testing.T) {
	newManagerTestEnv(t)
	var calls atomic.Int32
	slowConnect(t, 0, &calls)

	m := NewManager()
	m.Refresh()
	if got := calls.Load(); got != 1 {
		t.Fatalf("first refresh made %d connect attempts, want 1", got)
	}

	m.Refresh()
	m.Refresh()
	if got := calls.Load(); got != 1 {
		t.Fatalf("made %d connect attempts across three refreshes, want 1 - the failure should back off", got)
	}

	m.mu.Lock()
	m.connectBackoffUntil = time.Now().Add(-time.Second)
	m.mu.Unlock()
	m.Refresh()
	if got := calls.Load(); got != 2 {
		t.Fatalf("after the backoff expired there were %d attempts, want 2", got)
	}
}

func TestConnectBackoffGrowsAndIsCapped(t *testing.T) {
	m := NewManager()
	m.mu.Lock()
	defer m.mu.Unlock()

	m.noteConnectFailedLocked()
	if m.connectBackoff != connectRetryMin {
		t.Fatalf("first backoff = %s, want %s", m.connectBackoff, connectRetryMin)
	}
	m.noteConnectFailedLocked()
	if m.connectBackoff != 2*connectRetryMin {
		t.Fatalf("second backoff = %s, want %s", m.connectBackoff, 2*connectRetryMin)
	}
	for range 10 {
		m.noteConnectFailedLocked()
	}
	if m.connectBackoff != connectRetryMax {
		t.Fatalf("backoff settled at %s, want the %s cap", m.connectBackoff, connectRetryMax)
	}
}

// Quitting shortly after launch must not wait out an in-flight connect that has
// not started dialling yet.
func TestStopIsNotBlockedByAQueuedRefresh(t *testing.T) {
	newManagerTestEnv(t)
	var calls atomic.Int32
	slowConnect(t, 0, &calls)

	m := NewManager()
	m.Start()

	begin := time.Now()
	m.Stop()
	if elapsed := time.Since(begin); elapsed > time.Second {
		t.Fatalf("Stop took %s", elapsed)
	}

	before := calls.Load()
	m.Refresh()
	if got := calls.Load(); got != before {
		t.Fatalf("a refresh after Stop attempted a connect (%d -> %d)", before, got)
	}
}
