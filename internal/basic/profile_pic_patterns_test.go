package basic

import (
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/platform"
)

func TestBuiltInPatternFallbacksSavedPaths(t *testing.T) {
	// Pure path resolution: no files or settings are created.
	live, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(live, "saved-account")
	d := platform.Descriptor{LoginFiles: map[string]string{live: "LocalAppData"}}
	patterns := []string{"", filepath.Join(live, "cache", "*", "%blob%"), filepath.Join(live, "legacy", "%blob%")}
	vars := map[string]string{"blob": "data_*"}
	livePatterns := builtInPatternFallbacks(d, "", platform.PathTokenContext{}, patterns, vars, "", false)
	if len(livePatterns) != 2 || livePatterns[0] != filepath.Join(live, "cache", "*", "data_*") || livePatterns[1] != filepath.Join(live, "legacy", "data_*") {
		t.Fatalf("live patterns = %v", livePatterns)
	}
	savedPatterns := builtInPatternFallbacks(d, "", platform.PathTokenContext{}, patterns, vars, saved, true)
	want := filepath.Join(saved, "LocalAppData", "cache", "*", "data_*")
	if len(savedPatterns) != 2 || savedPatterns[0] != want || savedPatterns[1] != filepath.Join(saved, "LocalAppData", "legacy", "data_*") {
		t.Fatalf("saved patterns = %v; want %q", savedPatterns, want)
	}
}
