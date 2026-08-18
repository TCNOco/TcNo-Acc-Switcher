package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// clearTargetPath returns the filesystem path a PathListToClear entry acts on.
// Entries are either a bare path or DIRECTIVE::path[::selector] (the JSON_*
// forms). Registry entries have no filesystem target.
func clearTargetPath(entry string) (string, bool) {
	e := strings.TrimSpace(entry)
	if e == "" || strings.HasPrefix(e, "REG:") {
		return "", false
	}
	i := strings.Index(e, "::")
	if i < 0 {
		return e, true
	}
	rest := e[i+2:]
	if j := strings.Index(rest, "::"); j >= 0 {
		return rest[:j], true
	}
	return rest, true
}

// TestPlatformsClearOnlyOwnPaths pins the invariant that a platform may only
// clear files it also saves and restores. Clearing is a recursive delete or an
// in-place JSON edit, so a descriptor that names another platform's path
// destroys that platform's data on every switch and never clears its own
// session.
//
// This is a regression pin: fa0b05f0 gave the Epic Games entry the BattleNet
// entry's PathListToClear verbatim, which blanked Client.SavedAccountNames in
// Battle.net.config on every Epic switch and Add New.
func TestPlatformsClearOnlyOwnPaths(t *testing.T) {
	t.Parallel()

	var catalog struct {
		Platforms map[string]struct {
			PathListToClear []string          `json:"PathListToClear"`
			LoginFiles      map[string]string `json:"LoginFiles"`
		} `json:"Platforms"`
	}
	if err := json.Unmarshal(embeddedPlatformsJSON, &catalog); err != nil {
		t.Fatalf("parse embedded Platforms.json: %v", err)
	}
	if len(catalog.Platforms) == 0 {
		t.Fatal("embedded Platforms.json has no platforms")
	}

	for name, p := range catalog.Platforms {
		owned := make(map[string]struct{}, len(p.LoginFiles))
		for key := range p.LoginFiles {
			if path, ok := clearTargetPath(key); ok {
				owned[strings.ToLower(path)] = struct{}{}
			}
		}
		for _, entry := range p.PathListToClear {
			if strings.TrimSpace(entry) == "SAME_AS_LOGIN_FILES" {
				continue
			}
			path, ok := clearTargetPath(entry)
			if !ok {
				continue
			}
			if _, ok := owned[strings.ToLower(path)]; !ok {
				t.Errorf("%s clears %q, which none of its own LoginFiles reference; "+
					"a platform must only clear what it saves", name, path)
			}
		}
	}
}

// TestSteamDeclaresQuitArgs pins the catalog half of the native-quit wiring. Steam ignores
// WM_CLOSE - its main window close means minimise to tray - so without this argument the
// graceful window expires on every switch and the client is force-killed ~5s later. The code
// falls back to the same value, so a missing field costs speed silently rather than failing.
func TestSteamDeclaresQuitArgs(t *testing.T) {
	t.Parallel()

	var catalog struct {
		Platforms map[string]struct {
			Extras struct {
				QuitArgs string `json:"QuitArgs"`
			} `json:"Extras"`
		} `json:"Platforms"`
	}
	if err := json.Unmarshal(embeddedPlatformsJSON, &catalog); err != nil {
		t.Fatalf("parse catalog: %v", err)
	}
	if got := strings.TrimSpace(catalog.Platforms["Steam"].Extras.QuitArgs); got != "-shutdown" {
		t.Fatalf("Steam Extras.QuitArgs = %q, want %q", got, "-shutdown")
	}
}
