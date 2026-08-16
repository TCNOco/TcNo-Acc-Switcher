package app

import (
	"io"
	"os"

	"TcNo-Acc-Switcher/internal/filelog"
	"TcNo-Acc-Switcher/internal/paths"
)

// LogFileName is the current log inside [paths.LogDir]; the rotated generation
// sits beside it with a .1 suffix.
const LogFileName = "TcNo-Acc-Switcher.log"

// OpenLogSink opens the on-disk log under the resolved data root. Call it after
// platform.InitDataPaths.
//
// A failure here is never fatal — the app still runs, it just logs to stderr
// alone, which on a release build means nowhere.
func OpenLogSink() (*filelog.Writer, error) {
	dir, err := paths.LogDir()
	if err != nil {
		return nil, err
	}
	return filelog.Open(dir, LogFileName, filelog.DefaultMaxBytes)
}

// LogWriter returns the writer slog handlers should target: stderr plus the
// on-disk log when one is available.
//
// Takes the concrete type rather than an io.Writer so a nil *filelog.Writer
// from a failed Open does not arrive as a non-nil interface holding a nil
// pointer, which would panic on the first record.
func LogWriter(sink *filelog.Writer) io.Writer {
	if sink == nil {
		return os.Stderr
	}
	return io.MultiWriter(os.Stderr, sink)
}
