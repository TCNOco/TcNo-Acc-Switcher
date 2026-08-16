// Package filelog provides the app's on-disk log sink.
//
// Release builds link with -H windowsgui and so have no console attached at
// all: anything written to os.Stderr is discarded, which left a shipped build
// with no record of a failed start, a failed swap or an unresolvable path. This
// gives those writes somewhere to land.
package filelog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DefaultMaxBytes caps one log file. One rotated generation is kept, so the
// sink costs at most twice this on disk.
const DefaultMaxBytes int64 = 4 << 20

// Writer is a size-capped log file that keeps a single previous generation.
// It is safe for concurrent use; slog handlers write from any goroutine.
type Writer struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	file     *os.File
	size     int64
}

// Open prepares dir and opens dir/name for appending. An existing file is
// carried over rather than truncated, so a log survives a restart, and is
// rotated immediately if it is already at the cap.
func Open(dir, name string, maxBytes int64) (*Writer, error) {
	if dir == "" {
		return nil, errors.New("filelog: empty dir")
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("filelog: mkdir %s: %w", dir, err)
	}
	w := &Writer{path: filepath.Join(dir, name), maxBytes: maxBytes}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Writer) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("filelog: open %s: %w", w.path, err)
	}
	size := int64(0)
	if st, err := f.Stat(); err == nil {
		size = st.Size()
	}
	w.file, w.size = f, size
	return nil
}

// rotate replaces the previous generation with the current file and starts a
// new one. The caller holds w.mu.
func (w *Writer) rotate() error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	// os.Rename replaces an existing destination on Unix but not on Windows.
	prev := w.path + ".1"
	_ = os.Remove(prev)
	if err := os.Rename(w.path, prev); err != nil && !errors.Is(err, os.ErrNotExist) {
		// A locked or vanished file must not take the app down: reopen and
		// keep appending rather than losing the sink entirely.
		return w.open()
	}
	return w.open()
}

func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return 0, errors.New("filelog: closed")
	}
	// Rotate before the write that would cross the cap, so a single record is
	// never split across two generations.
	if w.size > 0 && w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}
