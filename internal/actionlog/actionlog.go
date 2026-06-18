package actionlog

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	maxLines          = 10_000
	DefaultPruneFirst = 100
	DefaultPruneLast  = 300
)

var (
	mu    sync.Mutex
	lines []string
	ready bool
)

// Init resets the session action log. Call once at startup.
func Init() {
	mu.Lock()
	defer mu.Unlock()
	lines = nil
	ready = true
}

// Record appends a single-line action entry. value may be empty for operations without payload.
func Record(op, target, value string, err error) {
	if !ready {
		return
	}
	op = strings.TrimSpace(op)
	target = strings.TrimSpace(target)
	if op == "" {
		op = "unknown"
	}
	outcome := "ok"
	errMsg := ""
	if err != nil {
		outcome = "fail"
		errMsg = strings.TrimSpace(err.Error())
	}
	line := fmt.Sprintf("%s op=%s target=%q outcome=%s",
		time.Now().UTC().Format(time.RFC3339Nano), op, target, outcome)
	if value != "" {
		line += fmt.Sprintf(" value=%q", value)
	}
	if errMsg != "" {
		line += fmt.Sprintf(" err=%q", errMsg)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(lines) >= maxLines {
		lines = append(lines[1:], line)
		return
	}
	lines = append(lines, line)
}

// SnapshotPruned returns the buffered log, keeping the first firstN and last lastN lines when truncated.
func SnapshotPruned(firstN, lastN int) string {
	mu.Lock()
	defer mu.Unlock()
	if len(lines) == 0 {
		return ""
	}
	if firstN < 0 {
		firstN = 0
	}
	if lastN < 0 {
		lastN = 0
	}
	if firstN+lastN >= len(lines) {
		return strings.Join(lines, "\n")
	}
	head := lines[:firstN]
	tail := lines[len(lines)-lastN:]
	omitted := len(lines) - firstN - lastN
	var b strings.Builder
	for i, ln := range head {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(ln)
	}
	if b.Len() > 0 {
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "--- … %d lines omitted … ---", omitted)
	for _, ln := range tail {
		b.WriteByte('\n')
		b.WriteString(ln)
	}
	return b.String()
}
