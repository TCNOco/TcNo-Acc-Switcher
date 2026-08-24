package app_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Package-level vars capture whatever slog.Default() is at package-init time,
// which runs before main configures the real handler. Records from such a
// logger reach the configured handler only via the log package bridge: the
// level collapses to INFO, attributes arrive flattened into the message, and
// anything below INFO is dropped. Nothing fails visibly, so only a scan catches
// a reintroduction. Resolve the default inside a function instead.
var earlyDefaultLogger = regexp.MustCompile(`(?m)^var\s+\w+\s*=\s*slog\.Default\(\)`)

func TestNoPackageLevelSlogDefault(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if earlyDefaultLogger.Match(src) {
			rel, _ := filepath.Rel(root, path)
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Errorf("package-level slog.Default() capture in: %s", strings.Join(offenders, ", "))
	}
}
