package steamguard

import (
	"os"
	"testing"
	"time"
)

// tempDir is t.TempDir with a cleanup that tolerates a delete still pending.
//
// On Windows a file another process opened without FILE_SHARE_DELETE keeps its
// directory entry until that handle closes, even though the delete itself
// succeeded. RemoveAll then removes the file, and the rmdir behind it sees a
// directory that is not empty. t.TempDir reports that as a failure of whichever
// test happened to own the directory - which is why the failures land on
// unrelated tests, name a different one each run, and never reproduce alone.
//
// The production code already works around the same agents: see the comment on
// renameWithRetry in internal/fsutil, which retries for exactly this reason.
//
// Retrying costs a few milliseconds when it happens and nothing when it does
// not. A directory that still will not go is left for the OS to reclaim rather
// than failing a test that had already passed.
func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "steamguard-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { removeAllWithRetry(t, dir) })
	return dir
}

func removeAllWithRetry(t *testing.T, dir string) {
	t.Helper()
	backoff := 10 * time.Millisecond
	var err error
	for attempt := 0; attempt < 6; attempt++ {
		if err = os.RemoveAll(dir); err == nil {
			return
		}
		time.Sleep(backoff)
		backoff *= 2
	}
	t.Logf("temp dir %s could not be removed: %v", dir, err)
}
