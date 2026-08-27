package steamguard

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
	"TcNo-Acc-Switcher/internal/steamguard/gcpd"
	"TcNo-Acc-Switcher/internal/steamguard/vault"

	"golang.org/x/sys/windows"
)

// This measures the authenticated GCPD endpoint, so that the sweep's pacing can
// be set from a number rather than from taste. Unlike the community probe next
// door it needs the vault open, and being turned away here is attached to an
// account rather than to an address - so it is built to be run deliberately,
// once, against an account chosen on purpose.
//
//	$env:TCNO_CS2_PROBE_EXE_DIR = "C:\...\bin\TcNo Account Switcher"
//	$env:TCNO_CS2_PROBE_ID = "76561199..."
//	go test ./internal/steamguard -run CS2RateLimitProbe -v -timeout 15m
//
// Close the app first. A sweep running alongside this adds its own requests to
// the count and makes the result meaningless.
//
// The password is read from stdin, never from a flag or an environment variable,
// so it stays out of the command line, the shell history and the process table.
// Echo is turned off when stdin is a console; piping in works too.
//
// It is strictly read-only. It calls FetchCS2GCPD and parses the answer, and
// writes to no store and no tag - so it cannot rotate the vault generation, and
// a run that goes wrong leaves nothing behind to undo.

const (
	cs2ProbeMaxRequests = 120
	cs2ProbeMaxDuration = 5 * time.Minute
	// cs2ProbeStepRequests is per rung. A rate limit is usually a count inside a
	// window, so a rung has to be long enough to fill one.
	cs2ProbeStepRequests = 10
)

// cs2ProbeGaps is the ladder, and it is a ladder of gaps rather than of
// concurrency: what a limiter needs to know is how many requests per unit time
// are tolerated, not how many may be in flight. It starts at the sweep's current
// stagger, so the first rung is a control - it is exactly what the app does
// today, and if that rung refuses then the app is already over the line.
var cs2ProbeGaps = []time.Duration{
	2 * time.Second,
	time.Second,
	500 * time.Millisecond,
	250 * time.Millisecond,
	0,
}

type cs2ProbeRung struct {
	gap       time.Duration
	requests  int
	parsed    int
	latencies []time.Duration
	stopped   string
}

func (r cs2ProbeRung) rate() float64 {
	total := time.Duration(0)
	for _, l := range r.latencies {
		total += l
	}
	spent := total + time.Duration(r.requests)*r.gap
	if spent <= 0 {
		return 0
	}
	return float64(r.requests) / spent.Seconds()
}

func (r cs2ProbeRung) percentile(p float64) time.Duration {
	if len(r.latencies) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), r.latencies...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[int(float64(len(sorted)-1)*p)]
}

// secretInput picks where a typed secret is read from, and returns a closer.
//
// go test hands the test binary a stdin that is already at EOF, so prompting on
// stdin is answered instantly with nothing. The three cases are told apart
// rather than guessed at:
//
//   - stdin is a pipe or a file: the caller redirected it on purpose, so honour
//     it and a piped password still works.
//   - stdin is itself the console: use it.
//   - anything else, which is go test's NUL: open the console's own input, which
//     is unaffected by whatever stdin was wired to.
func secretInput() (*os.File, func()) {
	none := func() {}
	handle := windows.Handle(os.Stdin.Fd())
	if kind, err := windows.GetFileType(handle); err == nil && kind != windows.FILE_TYPE_CHAR {
		return os.Stdin, none
	}
	var mode uint32
	if windows.GetConsoleMode(handle, &mode) == nil {
		return os.Stdin, none
	}
	console, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		return os.Stdin, none
	}
	return console, func() { _ = console.Close() }
}

// readSecret takes one line from the console without echoing it.
func readSecret(prompt string) (string, error) {
	source, closeSource := secretInput()
	defer closeSource()
	fmt.Fprint(os.Stderr, prompt)
	defer fmt.Fprintln(os.Stderr)
	// Echo off, but line input left on so backspace still works and the read
	// returns on Enter.
	handle := windows.Handle(source.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err == nil {
		if windows.SetConsoleMode(handle, mode&^windows.ENABLE_ECHO_INPUT) == nil {
			defer func() { _ = windows.SetConsoleMode(handle, mode) }()
		}
	}
	line, err := bufio.NewReader(source).ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		return "", fmt.Errorf("%w (no console and nothing on stdin; pipe the password in, "+
			"or build the binary with `go test -c` and run it directly)", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// openProbeVault points the process at the real install's data directory and
// unlocks the vault.
//
// It unlocks the vault object directly rather than going through
// unlockVaultWithLocked, which would signal a whole-platform data refresh on the
// way out - a bulk stats and profile download is the last thing a run that is
// counting requests to Steam wants starting underneath it.
func openProbeVault(t *testing.T) *Service {
	t.Helper()
	exeDir := strings.TrimSpace(os.Getenv("TCNO_CS2_PROBE_EXE_DIR"))
	if exeDir == "" {
		t.Skip("set TCNO_CS2_PROBE_EXE_DIR to the app's install directory to run the CS2 rate limit probe")
	}
	if err := platform.InitDataPaths(exeDir); err != nil {
		t.Fatalf("resolve the app's data directory from %q: %v", exeDir, err)
	}
	root, err := paths.DataRoot()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("data root: %s", root)

	password, err := readSecret("Steam Guard vault password: ")
	if err != nil {
		t.Fatalf("read vault password: %v", err)
	}
	if password == "" {
		t.Fatal("no vault password given")
	}

	service := NewService()
	service.mu.Lock()
	defer service.mu.Unlock()
	v, err := service.requireVaultLocked()
	if err != nil {
		t.Fatalf("open vault: %v", err)
	}

	status, err := security.GetStatus()
	if err != nil {
		t.Fatalf("read security status: %v", err)
	}
	if !status.SavedAccountDataEncrypted {
		if err := v.UnlockWith(vault.PasswordOnly(password), vault.FixedLease); err != nil {
			t.Fatalf("unlock vault: %v", err)
		}
		return service
	}
	// The vault sits under the app password as well, so that has to be verified
	// before the outer key it guards can be derived.
	appPassword, err := readSecret("App password (this vault is also app-encrypted): ")
	if err != nil {
		t.Fatalf("read app password: %v", err)
	}
	if err := security.VerifyAppPassword(appPassword); err != nil {
		t.Fatalf("verify app password: %v", err)
	}
	key, err := security.DeriveSteamGuardOuterKey()
	if err != nil {
		t.Fatalf("derive outer key: %v", err)
	}
	defer security.WipeSecret(key)
	if err := v.UnlockWithFactorsAndOuter(vault.PasswordOnly(password), key, vault.FixedLease); err != nil {
		t.Fatalf("unlock vault: %v", err)
	}
	return service
}

// classifyCS2Probe decides whether one answer ends the run, and why.
//
// Kind alone is not enough. FailureRateLimit is only ever set for HTTP 429, and
// the community endpoints next door state their limits as a bare 500 and as a
// 403 from the edge - so any of the three ends the run here, whatever the
// classifier made of it. A sign-in page ends it too and matters more than a
// refusal: it means the session was spent rather than throttled.
func classifyCS2Probe(body []byte, err error) string {
	if err == nil {
		if outcome := gcpd.Parse(body, time.Now()).Outcome; outcome == gcpd.OutcomeNotSignedIn {
			return "SESSION REJECTED (sign-in page) - stop and re-authenticate this account"
		}
		return ""
	}
	var apiErr *confirmationapi.Error
	if !errors.As(err, &apiErr) {
		return fmt.Sprintf("transport failure: %v", err)
	}
	detail := fmt.Sprintf("kind=%s status=%d", apiErr.Kind, apiErr.StatusCode)
	if apiErr.HasRetryAfter {
		detail += fmt.Sprintf(" retryAfter=%s", apiErr.RetryAfter)
	}
	if apiErr.Detail != "" {
		detail += fmt.Sprintf(" detail=%q", apiErr.Detail)
	}
	switch {
	case apiErr.Kind == confirmationapi.FailureRateLimit,
		apiErr.StatusCode >= 500,
		apiErr.StatusCode == 403,
		apiErr.Kind == confirmationapi.FailureReauth:
		return detail
	}
	return ""
}

func TestCS2RateLimitProbe(t *testing.T) {
	wanted := strings.TrimSpace(os.Getenv("TCNO_CS2_PROBE_ID"))
	if wanted == "" {
		t.Skip("set TCNO_CS2_PROBE_ID to the one SteamID64 this probe may use")
	}
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("TCNO_CS2_PROBE_MODE")))
	if mode == "" {
		mode = "latency"
	}
	if mode != "latency" && mode != "ramp" {
		t.Fatalf(`TCNO_CS2_PROBE_MODE must be "latency" or "ramp", got %q`, mode)
	}
	budget := cs2ProbeMaxRequests
	if raw := strings.TrimSpace(os.Getenv("TCNO_CS2_PROBE_MAX")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			t.Fatalf("TCNO_CS2_PROBE_MAX must be a positive integer, got %q", raw)
		}
		budget = parsed
	}

	service := openProbeVault(t)
	var target *cooldownTarget
	for _, candidate := range service.collectCooldownTargets() {
		if candidate.steamID64 == wanted {
			found := candidate
			target = &found
			break
		}
	}
	if target == nil {
		t.Fatalf("no vault record with a usable session for %s", wanted)
	}

	// Only the ladder's first rung in latency mode: that rung is exactly the
	// cadence the sweep uses today, so it costs Steam no more than an ordinary
	// sweep of the same length and answers how long a GCPD read really takes.
	gaps := cs2ProbeGaps
	if mode == "latency" {
		gaps = cs2ProbeGaps[:1]
	}

	ctx, cancel := context.WithTimeout(context.Background(), cs2ProbeMaxDuration)
	defer cancel()
	credentials := confirmationapi.Credentials{
		SteamID: target.steamID64, AccessToken: target.accessToken, SessionID: target.sessionID,
	}
	t.Logf("mode=%s account=%s budget=%d requests over %s", mode, wanted, budget, cs2ProbeMaxDuration)

	spent := 0
	var lastClean *cs2ProbeRung
	for _, gap := range gaps {
		if spent >= budget || ctx.Err() != nil {
			t.Logf("budget spent after %d requests", spent)
			break
		}
		rung := cs2ProbeRung{gap: gap}
		for i := 0; i < cs2ProbeStepRequests && spent < budget; i++ {
			if i > 0 && gap > 0 {
				select {
				case <-ctx.Done():
				case <-time.After(gap):
				}
			}
			if ctx.Err() != nil {
				rung.stopped = "time budget spent"
				break
			}
			requestCtx, requestCancel := context.WithTimeout(ctx, cooldownRequestTimeout)
			started := time.Now()
			body, err := service.confirmationClient.FetchCS2GCPD(requestCtx, credentials)
			latency := time.Since(started)
			requestCancel()

			rung.requests++
			spent++
			rung.latencies = append(rung.latencies, latency)
			if stop := classifyCS2Probe(body, err); stop != "" {
				rung.stopped = stop
				break
			}
			if err == nil {
				rung.parsed++
			}
		}

		t.Logf("gap=%-6s requests=%-3d parsed=%-3d rate=%4.2f/s p50=%-8s max=%s",
			rung.gap, rung.requests, rung.parsed, rung.rate(),
			rung.percentile(0.50).Round(time.Millisecond),
			rung.percentile(1).Round(time.Millisecond))

		if rung.stopped != "" {
			t.Logf("STOPPED at gap=%s after %d total requests: %s", rung.gap, spent, rung.stopped)
			if lastClean != nil {
				t.Logf("last clean rung: gap=%s at %.2f req/s - pace the sweep no faster than this",
					lastClean.gap, lastClean.rate())
			} else {
				t.Log("the very first rung refused, and that rung is the cadence the sweep " +
					"uses today - the app is already over the line")
			}
			return
		}
		clean := rung
		lastClean = &clean
	}

	if lastClean == nil {
		return
	}
	if mode == "latency" {
		t.Logf("a GCPD read costs p50=%s max=%s; at the sweep's %s stagger that is %.2f req/s",
			lastClean.percentile(0.50).Round(time.Millisecond),
			lastClean.percentile(1).Round(time.Millisecond),
			cooldownAccountStagger, lastClean.rate())
		t.Log("run again with TCNO_CS2_PROBE_MODE=ramp to find where it refuses")
		return
	}
	t.Logf("no refusal within budget; cleared gap=%s at %.2f req/s over %d requests",
		lastClean.gap, lastClean.rate(), spent)
	t.Log("this is 'not today', not 'no limit': one account over a few minutes is a " +
		"narrow sample, and a limit may be per-account, per-IP, or windowed longer than this run")
}
