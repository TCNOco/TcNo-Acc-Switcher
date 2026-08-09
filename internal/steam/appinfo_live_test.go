package steam

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestParseAppInfoAgainstLocalSteam runs the parser over whatever appinfo.vdf this
// machine has. It is skipped where Steam is not installed, so it is a developer
// check rather than a gate - the format is Valve's and undocumented, and this is the
// only way to notice it has moved.
func TestParseAppInfoAgainstLocalSteam(t *testing.T) {
	// installRoot needs Platforms.json and app settings next to the built exe, which
	// a test binary has no business assembling. The default install locations are
	// enough for a check that skips when it finds nothing.
	var raw []byte
	var err error
	found := ""
	for _, base := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), `C:\Program Files (x86)`} {
		if base == "" {
			continue
		}
		path := filepath.Join(base, "Steam", "appcache", "appinfo.vdf")
		if raw, err = os.ReadFile(path); err == nil {
			found = path
			break
		}
	}
	if found == "" {
		t.Skip("no readable appinfo.vdf in the default Steam locations")
	}
	t.Logf("reading %s", found)

	start := time.Now()
	names, err := parseAppInfo(raw)
	if err != nil {
		t.Fatalf("live appinfo.vdf did not parse (%d bytes): %v", len(raw), err)
	}
	elapsed := time.Since(start)
	t.Logf("parsed %d bytes into %d names in %s", len(raw), len(names), elapsed)

	if len(names) < 100 {
		t.Fatalf("only %d names parsed from %d bytes; the format has probably moved", len(names), len(raw))
	}
	// A publisher leaking in as an app name is the failure mode that looks like
	// success, so pin a value that would change if the depth scoping broke.
	if got := names["730"]; got != "" && got != "Counter-Strike 2" {
		t.Fatalf("appid 730 resolved to %q, want \"Counter-Strike 2\"", got)
	}
}
