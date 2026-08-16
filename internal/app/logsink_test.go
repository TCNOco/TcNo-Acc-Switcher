package app

import (
	"testing"

	"TcNo-Acc-Switcher/internal/filelog"
)

// OpenLogSink returns a typed nil when the data root is not ready. Widening
// LogWriter's parameter to io.Writer would turn that into a non-nil interface
// holding a nil pointer, and the first log record would panic instead of
// falling back to stderr.
func TestLogWriterFallsBackToStderrForAFailedSink(t *testing.T) {
	var failed *filelog.Writer
	w := LogWriter(failed)
	if w == nil {
		t.Fatal("LogWriter returned nil; slog would panic on the first record")
	}
	if _, err := w.Write([]byte("startup\n")); err != nil {
		t.Fatalf("write to the fallback writer failed: %v", err)
	}
}
