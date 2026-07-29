package hwkey

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These two modules are pre-v1 and single-maintainer, and they document that
// minor releases may break the API. Nothing here can prove an upgrade still
// works, because the only real proof is enrolling and unlocking with a physical
// key, which no test can do.
//
// So the versions are pinned deliberately and asserted here. An upgrade fails
// this test, which is the point: it forces someone to re-test against hardware
// and update the pin on purpose, instead of a routine `go get -u` silently
// changing how a vault's key material is derived.
var pinnedModules = map[string]string{
	"github.com/go-ctap/winhello": "v0.1.0",
	"github.com/go-ctap/ctaphid":  "v0.7.0",
}

func TestSecurityKeyDependenciesArePinned(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}

	found := make(map[string]string, len(pinnedModules))
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		if _, watched := pinnedModules[fields[0]]; watched {
			found[fields[0]] = fields[1]
		}
	}

	for module, want := range pinnedModules {
		got, ok := found[module]
		if !ok {
			t.Fatalf("%s is no longer in go.mod; if security-key support was removed, remove this pin too", module)
		}
		if got != want {
			t.Fatalf("%s is %s, pinned at %s.\n"+
				"Upgrading it changes code that derives vault key material, and no test can verify that.\n"+
				"Enrol and unlock with a real security key, then update the pin in this test.",
				module, got, want)
		}
	}
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
