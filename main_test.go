package main

import (
	"os"
	"strings"
	"testing"
)

// The updater's swap helper is this exe re-executed; helper mode is entered
// via updater.HandleHelperMode, which must run before the singleton check or
// the helper sees the still-running parent and exits without applying the
// update.
func TestHelperModeRunsBeforeSingletonCheck(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)

	helper := strings.Index(body, "updater.HandleHelperMode()")
	singleton := strings.Index(body, "winutil.TryAcquireSingleton()")
	if helper == -1 {
		t.Fatal("main.go no longer calls updater.HandleHelperMode(); the update helper will die at the singleton check")
	}
	if singleton == -1 {
		t.Fatal("singleton acquisition not found; update this test for the new startup shape")
	}
	if helper > singleton {
		t.Fatal("updater.HandleHelperMode() must run before winutil.TryAcquireSingleton(), or update applies break")
	}
}
