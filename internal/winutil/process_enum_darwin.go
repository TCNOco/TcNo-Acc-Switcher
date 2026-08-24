//go:build darwin

package winutil

import (
	"bytes"

	"golang.org/x/sys/unix"
)

// procNameMaxLen is MAXCOMLEN: the kernel keeps 16 characters of a process name
// in kinfo_proc.p_comm (the field is 17 bytes, the last one a terminator).
const procNameMaxLen = 16

// forEachProcess walks the process table.
//
// macOS has no /proc, so the list comes from the kern.proc.all sysctl. That
// yields the accounting name only - the full executable path would need
// proc_pidpath from libproc, which means cgo for a name we do not need: the
// clients this matches (steam_osx, Discord, OBS) are all inside the 16
// characters the kernel keeps, and unixNameMatches compares targets against
// their own truncation for anything longer.
func forEachProcess(fn func(pid int, names []string)) {
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		slogWin().Warn("read kern.proc.all", "err", err)
		return
	}
	for i := range procs {
		p := &procs[i]
		pid := int(p.Proc.P_pid)
		if pid <= 0 {
			continue
		}
		comm := p.Proc.P_comm[:]
		if end := bytes.IndexByte(comm, 0); end >= 0 {
			comm = comm[:end]
		}
		if len(comm) == 0 {
			continue
		}
		fn(pid, []string{string(comm)})
	}
}
