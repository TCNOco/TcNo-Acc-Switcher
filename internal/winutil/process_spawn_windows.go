//go:build windows

package winutil

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procIsProcessInJob = modKernel32.NewProc("IsProcessInJob")

// Anything we launch must outlive us: a crash, a restart or a taskkill of the switcher
// must never take Steam - or a game Steam started - down with it. Four things tie a
// launched program to us on Windows, and all four are cut here:
//   - The console. Every process attached to a console gets CTRL_CLOSE_EVENT when that
//     console goes away and is terminated shortly after, so a child must get its own.
//   - The job object. Children join the parent's job by default, and a job with
//     JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE kills all of its processes when it closes.
//   - Handles. Inherited handles keep our files, pipes and sockets alive in the child,
//     and a retained process handle invites lifetime coupling on our side.
//   - The process tree, via the creator PID Windows records on every process. See
//     [launchSpec.reparented].
const detachedFlagsBase = windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_DEFAULT_ERROR_MODE

// detachedCreationFlags builds the CreateProcess flags for a launch that must survive us.
// CREATE_NEW_CONSOLE / CREATE_NO_WINDOW both hand the child its own console; DETACHED_PROCESS
// would also decouple it but leaves console-mode children with no stdio at all.
func detachedCreationFlags(hideWindow bool) uint32 {
	flags := uint32(detachedFlagsBase)
	if hideWindow {
		flags |= windows.CREATE_NO_WINDOW
	} else {
		flags |= windows.CREATE_NEW_CONSOLE
	}
	if jobBreakawayAllowed() {
		flags |= windows.CREATE_BREAKAWAY_FROM_JOB
	}
	return flags
}

var breakawayProbe struct {
	sync.Once
	allowed bool
}

// jobBreakawayAllowed reports whether CREATE_BREAKAWAY_FROM_JOB will be accepted. Passing it
// while inside a job that forbids breakaway fails the whole CreateProcess call with
// ERROR_ACCESS_DENIED, so probe the job once instead of launching and retrying every time.
func jobBreakawayAllowed() bool {
	breakawayProbe.Do(func() {
		breakawayProbe.allowed = probeJobBreakaway()
		slogWin().Debug("job breakaway probe", "allowed", breakawayProbe.allowed)
	})
	return breakawayProbe.allowed
}

func probeJobBreakaway() bool {
	if err := procIsProcessInJob.Find(); err != nil {
		return false
	}
	var inJob int32
	r1, _, _ := procIsProcessInJob.Call(uintptr(windows.CurrentProcess()), 0, uintptr(unsafe.Pointer(&inJob)))
	if r1 == 0 {
		return false
	}
	if inJob == 0 {
		return true // No job to break out of; the flag is then a harmless no-op.
	}
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	var retLen uint32
	// A NULL job handle queries the job the calling process is assigned to.
	if err := windows.QueryInformationJobObject(0, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), &retLen); err != nil {
		slogWin().Debug("query job limits failed", "err", err)
		return false
	}
	const breakawayMask = windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK | windows.JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK
	return info.BasicLimitInformation.LimitFlags&breakawayMask != 0
}

// launchSpec is one resolved CreateProcess call, minus the choice of how to parent it.
type launchSpec struct {
	exe     string
	app     *uint16
	cmdLine *uint16
	wd      *uint16
	si      windows.StartupInfo
	flags   uint32
}

func newLaunchSpec(exe string, args []string, workingDir string, hideWindow bool) (*launchSpec, error) {
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return nil, fmt.Errorf("empty executable")
	}
	// CreateProcess never searches PATH for lpApplicationName the way exec.Command does.
	resolved, err := resolveExecutablePath(exe)
	if err != nil {
		return nil, err
	}
	spec := &launchSpec{exe: resolved, flags: detachedCreationFlags(hideWindow)}
	if spec.app, err = windows.UTF16PtrFromString(resolved); err != nil {
		return nil, err
	}
	if spec.cmdLine, err = windows.UTF16PtrFromString(buildDetachedCommandLine(resolved, args)); err != nil {
		return nil, err
	}

	wd := strings.TrimSpace(workingDir)
	if wd == "" && filepath.IsAbs(resolved) {
		// Never hand the child our working directory: it would hold a lock on that folder.
		wd = filepath.Dir(resolved)
	}
	if wd != "" {
		if spec.wd, err = windows.UTF16PtrFromString(wd); err != nil {
			return nil, err
		}
	}

	spec.si.Cb = uint32(unsafe.Sizeof(spec.si))
	if hideWindow {
		spec.si.Flags |= windows.STARTF_USESHOWWINDOW
		spec.si.ShowWindow = windows.SW_HIDE
	}
	return spec, nil
}

// spawnDetached starts exe with args as an independent process and returns its PID. The child
// inherits no handles, owns its console and process group, stays out of our job object wherever
// that job permits breakaway, and - when a shell is reachable - is not in our process tree.
func spawnDetached(exe string, args []string, workingDir string, hideWindow bool) (uint32, error) {
	spec, err := newLaunchSpec(exe, args, workingDir, hideWindow)
	if err != nil {
		return 0, err
	}
	pid, err := spec.reparented()
	if err == nil {
		return pid, nil
	}
	slogWin().Debug("shell reparent unavailable; launching in our tree", "exe", spec.exe, "err", err)
	return spec.create(spec.flags, 0)
}

// spawnReparented is spawnDetached without the in-tree fallback. Callers reach for it when the
// launch must also drop our elevation: a plain CreateProcess would hand the child our admin
// token, so failing here and letting the caller try something else is the only safe answer.
func spawnReparented(exe string, args []string, workingDir string, hideWindow bool) (uint32, error) {
	spec, err := newLaunchSpec(exe, args, workingDir, hideWindow)
	if err != nil {
		return 0, err
	}
	return spec.reparented()
}

// spawnWithToken launches under an explicit primary token via CreateProcessWithTokenW.
func spawnWithToken(exe string, args []string, workingDir string, hideWindow bool, token windows.Token) (uint32, error) {
	spec, err := newLaunchSpec(exe, args, workingDir, hideWindow)
	if err != nil {
		return 0, err
	}
	return spec.create(spec.flags, token)
}

// reparented records explorer.exe, not us, as the new process's parent. Windows only ever
// stores a creator PID, but the tools that walk it are exactly the ones that hurt: Task
// Manager's "End task" and `taskkill /T` kill a whole tree, so a force-closed switcher would
// otherwise still take Steam - and every game Steam started - with it. The child also picks up
// the shell's job (normally none) and its non-elevated token.
func (s *launchSpec) reparented() (uint32, error) {
	shell, err := openShellProcess()
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(shell)

	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return 0, err
	}
	defer attrs.Delete()
	if err := attrs.Update(windows.PROC_THREAD_ATTRIBUTE_PARENT_PROCESS, unsafe.Pointer(&shell), unsafe.Sizeof(shell)); err != nil {
		return 0, err
	}

	siex := windows.StartupInfoEx{StartupInfo: s.si, ProcThreadAttributeList: attrs.List()}
	siex.Cb = uint32(unsafe.Sizeof(siex))
	// StartupInfo is StartupInfoEx's first field, so this is the STARTUPINFOEX the flag promises.
	return createProcessAllowingJob(s.app, s.cmdLine, s.wd, &siex.StartupInfo, s.flags|windows.EXTENDED_STARTUPINFO_PRESENT, 0)
}

func (s *launchSpec) create(flags uint32, token windows.Token) (uint32, error) {
	si := s.si
	return createProcessAllowingJob(s.app, s.cmdLine, s.wd, &si, flags, token)
}

func openShellProcess() (windows.Handle, error) {
	hwnd := windows.GetShellWindow()
	if hwnd == 0 {
		return 0, fmt.Errorf("no shell window")
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &pid); err != nil {
		return 0, err
	}
	if pid == 0 {
		return 0, fmt.Errorf("shell pid is 0")
	}
	return windows.OpenProcess(windows.PROCESS_CREATE_PROCESS, false, pid)
}

// createProcessAllowingJob retries without CREATE_BREAKAWAY_FROM_JOB when the job rejects it:
// staying inside the job beats not launching at all.
func createProcessAllowingJob(app, cmdLine, wd *uint16, si *windows.StartupInfo, flags uint32, token windows.Token) (uint32, error) {
	pid, err := createProcessDetached(app, cmdLine, wd, si, flags, token)
	if err != nil && flags&windows.CREATE_BREAKAWAY_FROM_JOB != 0 && breakawayRejected(err) {
		slogWin().Warn("job breakaway rejected; launching inside job", "err", err)
		pid, err = createProcessDetached(app, cmdLine, wd, si, flags&^windows.CREATE_BREAKAWAY_FROM_JOB, token)
	}
	return pid, err
}

// createProcessDetached closes both returned handles immediately. Keeping the process handle
// is what would let anything on our side wait on, or be tied to, the child's lifetime.
func createProcessDetached(app, cmdLine, wd *uint16, si *windows.StartupInfo, flags uint32, token windows.Token) (uint32, error) {
	var pi windows.ProcessInformation
	var err error
	if token == 0 {
		err = windows.CreateProcess(app, cmdLine, nil, nil, false, flags, nil, wd, si, &pi)
	} else {
		err = createProcessWithToken(token, app, cmdLine, flags, wd, si, &pi)
	}
	if err != nil {
		return 0, err
	}
	if pi.Thread != 0 {
		_ = windows.CloseHandle(pi.Thread)
	}
	if pi.Process != 0 {
		_ = windows.CloseHandle(pi.Process)
	}
	return pi.ProcessId, nil
}

func breakawayRejected(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	return errno == windows.ERROR_ACCESS_DENIED || errno == windows.ERROR_INVALID_PARAMETER
}

// resolveExecutablePath expands a bare name such as "cmd.exe" to a full path.
func resolveExecutablePath(exe string) (string, error) {
	if strings.ContainsAny(exe, `\/`) {
		return exe, nil
	}
	return exec.LookPath(exe)
}

// buildDetachedCommandLine produces a lpCommandLine whose argv[0] is the executable, matching
// what os/exec passes; CreateProcess still takes the image from lpApplicationName.
func buildDetachedCommandLine(exe string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, syscall.EscapeArg(exe))
	for _, a := range args {
		parts = append(parts, syscall.EscapeArg(a))
	}
	return strings.Join(parts, " ")
}

// helperCreationFlags decouples the short-lived helpers we do wait on (they keep their
// inherited stdio pipes, so handle inheritance stays on) from our console and job.
func helperCreationFlags() uint32 {
	flags := uint32(windows.CREATE_NO_WINDOW)
	if jobBreakawayAllowed() {
		flags |= windows.CREATE_BREAKAWAY_FROM_JOB
	}
	return flags
}
