//go:build windows

// steambench measures where wall-clock time goes when the switcher closes Steam.
//
//	go run ./tools/steambench -mode micro      # no side effects: primitive costs only
//	go run ./tools/steambench -mode inventory  # no side effects: what is actually running
//	go run ./tools/steambench -mode current    # CLOSES STEAM using the shipping algorithm, timed
//	go run ./tools/steambench -mode fast       # CLOSES STEAM using the proposed algorithm, timed
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"TcNo-Acc-Switcher/internal/winutil"
)

var steamKillNames = []string{
	"steam.exe",
	"SERVICE:Steam Client Service",
	"steamwebhelper.exe",
	"GameOverlayUI.exe",
}

// ---------- timing ----------

type span struct {
	Name     string  `json:"name"`
	Depth    int     `json:"depth"`
	StartMs  float64 `json:"startMs"`
	Duration float64 `json:"durationMs"`
	Note     string  `json:"note,omitempty"`
}

type recorder struct {
	mu    sync.Mutex
	t0    time.Time
	spans []span
}

func newRecorder() *recorder { return &recorder{t0: time.Now()} }

func (r *recorder) mark(name string, depth int, start time.Time, note string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.spans = append(r.spans, span{
		Name:     name,
		Depth:    depth,
		StartMs:  float64(start.Sub(r.t0).Microseconds()) / 1000,
		Duration: float64(time.Since(start).Microseconds()) / 1000,
		Note:     note,
	})
}

func (r *recorder) time(name string, depth int, note string, fn func() string) {
	start := time.Now()
	extra := fn()
	if extra != "" {
		note = strings.TrimSpace(note + " " + extra)
	}
	r.mark(name, depth, start, note)
}

func (r *recorder) report(title string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sort.SliceStable(r.spans, func(i, j int) bool { return r.spans[i].StartMs < r.spans[j].StartMs })
	total := 0.0
	for _, s := range r.spans {
		if end := s.StartMs + s.Duration; end > total {
			total = end
		}
	}
	fmt.Printf("\n=== %s ===\n", title)
	fmt.Printf("%10s %10s  %s\n", "start", "dur", "phase")
	fmt.Println(strings.Repeat("-", 100))
	for _, s := range r.spans {
		indent := strings.Repeat("  ", s.Depth)
		line := fmt.Sprintf("%8.1fms %8.1fms  %s%s", s.StartMs, s.Duration, indent, s.Name)
		if s.Note != "" {
			line += "   " + s.Note
		}
		fmt.Println(line)
	}
	fmt.Println(strings.Repeat("-", 100))
	fmt.Printf("WALL CLOCK: %.1f ms\n", total)
	if *jsonOut != "" {
		b, _ := json.MarshalIndent(map[string]any{"title": title, "wallMs": total, "spans": r.spans}, "", "  ")
		_ = os.WriteFile(*jsonOut, b, 0o644)
		fmt.Printf("json written: %s\n", *jsonOut)
	}
}

// ---------- process table ----------

type procLite struct {
	PID       uint32
	ParentPID uint32
	ExeBase   string
}

func snapshotProcesses() ([]procLite, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		return nil, err
	}
	var out []procLite
	for {
		out = append(out, procLite{PID: pe.ProcessID, ParentPID: pe.ParentProcessID, ExeBase: utf16FixedToString(pe.ExeFile[:])})
		if err := windows.Process32Next(snap, &pe); err != nil {
			return out, nil
		}
	}
}

func utf16FixedToString(b []uint16) string {
	n := 0
	for n < len(b) && b[n] != 0 {
		n++
	}
	return windows.UTF16ToString(b[:n])
}

func normalizeExeBase(s string) string {
	s = strings.TrimSpace(filepath.Base(s))
	if s == "" {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(s), ".exe") {
		s += ".exe"
	}
	return s
}

func allPIDsForImageName(want string) ([]uint32, error) {
	want = normalizeExeBase(want)
	all, err := snapshotProcesses()
	if err != nil {
		return nil, err
	}
	var out []uint32
	for _, p := range all {
		if strings.EqualFold(p.ExeBase, want) {
			out = append(out, p.PID)
		}
	}
	return out, nil
}

// ---------- win32 ----------

var (
	modUser32                    = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows              = modUser32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = modUser32.NewProc("GetWindowThreadProcessId")
	procGetWindow                = modUser32.NewProc("GetWindow")
	procPostMessageW             = modUser32.NewProc("PostMessageW")
)

const (
	winWMClose      = 0x0010
	winWMSysCommand = 0x0112
	winSCClose      = 0xF060
	winGWOwner      = 4
)

var gracefulQuitCb uintptr
var postedWindows int

func init() {
	gracefulQuitCb = syscall.NewCallback(func(hwnd, lParam uintptr) uintptr {
		targetPID := uint32(lParam)
		var windowPID uint32
		r0, _, _ := procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
		if r0 == 0 || windowPID != targetPID {
			return 1
		}
		if owner, _, _ := procGetWindow.Call(hwnd, uintptr(winGWOwner)); owner != 0 {
			return 1
		}
		postedWindows++
		procPostMessageW.Call(hwnd, uintptr(winWMSysCommand), uintptr(winSCClose), 0)
		procPostMessageW.Call(hwnd, uintptr(winWMClose), 0, 0)
		return 1
	})
}

func postGracefulQuitPass(pid uint32) {
	if err := procEnumWindows.Find(); err != nil {
		return
	}
	_, _, _ = procEnumWindows.Call(gracefulQuitCb, uintptr(pid))
}

func taskKillViaCmd(name string, force bool) error {
	args := []string{"/C", "taskkill"}
	if force {
		args = append(args, "/F")
	}
	args = append(args, "/T", "/IM", name)
	cmd := exec.Command("cmd.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_, err := cmd.CombinedOutput()
	return err
}

func taskKillDirect(name string, force bool) error {
	args := []string{}
	if force {
		args = append(args, "/F")
	}
	args = append(args, "/T", "/IM", name)
	cmd := exec.Command("taskkill.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_, err := cmd.CombinedOutput()
	return err
}

// terminateByPIDs is the no-subprocess equivalent of taskkill /F /IM.
func terminateByPIDs(pids []uint32) (killed int) {
	for _, pid := range pids {
		h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
		if err != nil {
			continue
		}
		if windows.TerminateProcess(h, 1) == nil {
			killed++
		}
		windows.CloseHandle(h)
	}
	return
}

// waitPIDsExit blocks on the real process handles: returns the instant the last one dies.
func waitPIDsExit(pids []uint32, maxWait time.Duration) (alive int) {
	var handles []windows.Handle
	for _, pid := range pids {
		h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
		if err == nil {
			handles = append(handles, h)
		}
	}
	defer func() {
		for _, h := range handles {
			windows.CloseHandle(h)
		}
	}()
	deadline := time.Now().Add(maxWait)
	for len(handles) > 0 {
		remain := time.Until(deadline)
		if remain <= 0 {
			return len(handles)
		}
		batch := handles
		if len(batch) > 64 {
			batch = batch[:64]
		}
		r, err := windows.WaitForMultipleObjects(batch, true, uint32(remain/time.Millisecond))
		if err != nil || r == uint32(windows.WAIT_TIMEOUT) {
			return len(handles)
		}
		handles = handles[len(batch):]
	}
	return 0
}

// ---------- current algorithm, instrumented ----------

const gracefulCombinedExitMaxWait = 5 * time.Second

func runCurrent(r *recorder) {
	// preflight, exactly as internal/steam/switcher.go does it
	r.time("ErrIfCannotKill (preflight)", 0, "", func() string {
		n := 0
		for _, raw := range steamKillNames {
			if strings.HasPrefix(strings.ToUpper(raw), "SERVICE:") {
				continue
			}
			_, _ = allPIDsForImageName(normalizeExeBase(raw))
			n++
		}
		return fmt.Sprintf("(%d full process-table snapshots)", n)
	})

	killStart := time.Now()
	var wg sync.WaitGroup
	for _, name := range steamKillNames {
		wg.Add(1)
		go func(raw string) {
			defer wg.Done()
			if strings.HasPrefix(strings.ToUpper(raw), "SERVICE:") {
				svcName := strings.TrimSpace(raw[len("SERVICE:"):])
				r.time("service stop: "+svcName, 1, "", func() string {
					if err := stopWindowsService(svcName); err != nil {
						_ = taskKillViaCmd(svcName+".exe", true)
						return "(SCM failed -> taskkill fallback: " + err.Error() + ")"
					}
					return "(SCM Stop ok)"
				})
				return
			}
			base := normalizeExeBase(raw)
			branchStart := time.Now()

			var pids []uint32
			r.time("["+base+"] enum PIDs", 2, "", func() string {
				pids, _ = allPIDsForImageName(base)
				return fmt.Sprintf("(%d pids)", len(pids))
			})
			r.time("["+base+"] WM_CLOSE pass + 200ms + pass, per PID", 2, "", func() string {
				for _, pid := range pids {
					postGracefulQuitPass(pid)
					time.Sleep(200 * time.Millisecond)
					postGracefulQuitPass(pid)
				}
				return fmt.Sprintf("(%d pids x 200ms = %dms of pure sleep)", len(pids), len(pids)*200)
			})
			r.time("["+base+"] taskkill soft (cmd.exe + taskkill.exe)", 2, "", func() string {
				_ = taskKillViaCmd(base, false)
				return ""
			})
			r.time("["+base+"] waitForImageExit (5s cap, 100ms poll)", 2, "", func() string {
				return waitForImageExitInstrumented(base, gracefulCombinedExitMaxWait, 100*time.Millisecond)
			})
			r.time("["+base+"] taskkill force (cmd.exe + taskkill.exe)", 2, "", func() string {
				_ = taskKillViaCmd(base, true)
				return ""
			})
			r.mark("["+base+"] branch total", 1, branchStart, "")
		}(name)
	}
	wg.Wait()
	r.mark("KillByName (all branches, parallel)", 0, killStart, "")
}

func waitForImageExitInstrumented(exeImage string, maxWait, poll time.Duration) string {
	deadline := time.Now().Add(maxWait)
	var cachedPIDs []uint32
	polls := 0
	for time.Now().Before(deadline) {
		if len(cachedPIDs) == 0 {
			var err error
			cachedPIDs, err = allPIDsForImageName(exeImage)
			if err != nil {
				time.Sleep(300 * time.Millisecond)
				return "(snapshot error -> early return)"
			}
			if len(cachedPIDs) == 0 {
				return fmt.Sprintf("(gone after %d polls)", polls)
			}
		}
		stillRunning := false
		remaining := cachedPIDs[:0]
		for _, pid := range cachedPIDs {
			h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
			if err != nil {
				if err == windows.ERROR_ACCESS_DENIED {
					stillRunning = true
					remaining = append(remaining, pid)
				}
				continue
			}
			rr, _ := windows.WaitForSingleObject(h, 0)
			windows.CloseHandle(h)
			if rr == windows.WAIT_OBJECT_0 {
				continue
			}
			stillRunning = true
			remaining = append(remaining, pid)
		}
		cachedPIDs = remaining
		if !stillRunning {
			return fmt.Sprintf("(gone after %d polls)", polls)
		}
		polls++
		time.Sleep(poll)
	}
	return fmt.Sprintf("(TIMED OUT after %d polls - burned the full %s)", polls, maxWait)
}

func stopWindowsService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return err
	}
	defer s.Close()
	_, err = s.Control(svc.Stop)
	return err
}

// ---------- proposed fast algorithm, instrumented ----------

// watchDeaths spawns one goroutine per PID that blocks on the real handle and records
// the exact moment that process died, so we can see who the straggler is.
func watchDeaths(r *recorder, pids map[uint32]string, maxWait time.Duration) {
	var wg sync.WaitGroup
	start := time.Now()
	for pid, image := range pids {
		wg.Add(1)
		go func(pid uint32, image string) {
			defer wg.Done()
			h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
			if err != nil {
				r.mark(fmt.Sprintf("  died: %s pid=%d", image, pid), 1, start, "(handle not openable)")
				return
			}
			defer windows.CloseHandle(h)
			res, _ := windows.WaitForSingleObject(h, uint32(maxWait/time.Millisecond))
			note := ""
			if res != windows.WAIT_OBJECT_0 {
				note = "(STILL ALIVE at cutoff)"
			}
			r.mark(fmt.Sprintf("  died: %s pid=%d", image, pid), 1, start, note)
		}(pid, image)
	}
	wg.Wait()
	r.mark("all target processes gone", 0, start, "")
}

func steamTargets() (map[uint32]string, map[string][]uint32) {
	all, _ := snapshotProcesses()
	byImage := map[string][]uint32{}
	for _, p := range all {
		byImage[strings.ToLower(normalizeExeBase(p.ExeBase))] = append(byImage[strings.ToLower(normalizeExeBase(p.ExeBase))], p.PID)
	}
	targets := map[uint32]string{}
	for _, n := range steamKillNames {
		if strings.HasPrefix(strings.ToUpper(n), "SERVICE:") {
			continue
		}
		b := strings.ToLower(normalizeExeBase(n))
		for _, pid := range byImage[b] {
			targets[pid] = b
		}
	}
	return targets, byImage
}

// runReal drives the actual shipping winutil code path, so the before/after number
// reflects internal/winutil rather than a replica of it.
func runReal(r *recorder, steamExe string, useNative bool) {
	targets, _ := steamTargets()
	opts := winutil.KillOpts{}
	if useNative && steamExe != "" {
		root := filepath.Dir(steamExe)
		opts.NativeQuit = func() error {
			return winutil.Start(steamExe, []string{"-shutdown"}, winutil.StartOpts{
				HideWindow: true,
				WorkingDir: root,
			})
		}
	}
	go watchDeaths(r, targets, 30*time.Second)
	r.time("winutil.KillByNameWithOpts (Combined)", 0, "", func() string {
		if err := winutil.KillByNameWithOpts(steamKillNames, winutil.ClosingCombined, opts); err != nil {
			return "(err: " + err.Error() + ")"
		}
		return ""
	})
}

// runNative sends only Steam's real quit signal and records when each process dies,
// so we can see when steam.exe itself (the process that owns loginusers.vdf and the
// AutoLoginUser registry value) is gone versus when the CEF helpers finally drain.
func runNative(r *recorder, steamExe string) {
	var targets map[uint32]string
	var byImage map[string][]uint32
	r.time("snapshot + build target set", 0, "", func() string {
		targets, byImage = steamTargets()
		return fmt.Sprintf("(%d target pids)", len(targets))
	})
	if len(targets) == 0 {
		fmt.Println("(no Steam processes running)")
		return
	}
	r.time("steam.exe -shutdown", 0, "", func() string {
		c := exec.Command(steamExe, "-shutdown")
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		if err := c.Start(); err != nil {
			return "(err: " + err.Error() + ")"
		}
		go func() { _ = c.Wait() }()
		return "(fire-and-forget)"
	})
	// The one process the switch actually has to outlive.
	steamPIDs := byImage["steam.exe"]
	go func() {
		start := time.Now()
		for _, pid := range steamPIDs {
			h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
			if err != nil {
				continue
			}
			windows.WaitForSingleObject(h, 30000)
			windows.CloseHandle(h)
			r.mark(">>> steam.exe GONE (switch could proceed here)", 0, start, "")
		}
	}()
	watchDeaths(r, targets, 30*time.Second)
}

// runFloor is the theoretical minimum: no graceful signal at all, just TerminateProcess.
func runFloor(r *recorder) {
	var targets map[uint32]string
	r.time("snapshot + build target set", 0, "", func() string {
		targets, _ = steamTargets()
		return fmt.Sprintf("(%d target pids)", len(targets))
	})
	r.time("TerminateProcess on every target", 0, "", func() string {
		var pids []uint32
		for pid := range targets {
			pids = append(pids, pid)
		}
		return fmt.Sprintf("(terminated %d)", terminateByPIDs(pids))
	})
	watchDeaths(r, targets, 10*time.Second)
}

// runHybrid sends Steam's real quit signal, waits event-driven, and force-terminates
// whatever has not exited by the grace deadline.
func runHybrid(r *recorder, steamExe string, grace time.Duration) {
	var targets map[uint32]string
	var byImage map[string][]uint32
	r.time("snapshot + build target set", 0, "", func() string {
		targets, byImage = steamTargets()
		return fmt.Sprintf("(%d target pids)", len(targets))
	})
	if len(targets) == 0 {
		fmt.Println("(no Steam processes running)")
		return
	}
	r.time("steam.exe -shutdown", 0, "", func() string {
		c := exec.Command(steamExe, "-shutdown")
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		if err := c.Start(); err != nil {
			return "(err: " + err.Error() + ")"
		}
		go func() { _ = c.Wait() }()
		return "(fire-and-forget)"
	})
	r.time("WM_CLOSE to steam.exe windows", 0, "", func() string {
		postedWindows = 0
		for _, pid := range byImage["steam.exe"] {
			postGracefulQuitPass(pid)
		}
		return fmt.Sprintf("(%d windows)", postedWindows)
	})

	graceStart := time.Now()
	r.time(fmt.Sprintf("event-driven wait, %s grace", grace), 0, "", func() string {
		var pids []uint32
		for pid := range targets {
			pids = append(pids, pid)
		}
		return fmt.Sprintf("(%d still alive at grace deadline)", waitPIDsExit(pids, grace))
	})
	_ = graceStart

	r.time("TerminateProcess sweep on stragglers", 0, "", func() string {
		var still []uint32
		for pid := range targets {
			h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
			if err != nil {
				continue
			}
			res, _ := windows.WaitForSingleObject(h, 0)
			windows.CloseHandle(h)
			if res != windows.WAIT_OBJECT_0 {
				still = append(still, pid)
			}
		}
		return fmt.Sprintf("(%d stragglers, terminated %d)", len(still), terminateByPIDs(still))
	})
	watchDeaths(r, targets, 5*time.Second)
}

func runFast(r *recorder, steamExe string) {
	var all []procLite
	r.time("single process-table snapshot", 0, "", func() string {
		all, _ = snapshotProcesses()
		return fmt.Sprintf("(%d processes)", len(all))
	})

	byImage := map[string][]uint32{}
	for _, p := range all {
		b := strings.ToLower(normalizeExeBase(p.ExeBase))
		byImage[b] = append(byImage[b], p.PID)
	}
	var targets []uint32
	counts := []string{}
	for _, n := range steamKillNames {
		if strings.HasPrefix(strings.ToUpper(n), "SERVICE:") {
			continue
		}
		b := strings.ToLower(normalizeExeBase(n))
		targets = append(targets, byImage[b]...)
		counts = append(counts, fmt.Sprintf("%s=%d", b, len(byImage[b])))
	}
	fmt.Println("target inventory:", strings.Join(counts, " "))

	if len(targets) == 0 {
		fmt.Println("(no Steam processes running - nothing to measure)")
		return
	}

	r.time("steam.exe -shutdown (native)", 0, "", func() string {
		if steamExe == "" {
			return "(SKIPPED: -steamexe not provided)"
		}
		cmd := exec.Command(steamExe, "-shutdown")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := cmd.Start(); err != nil {
			return "(start err: " + err.Error() + ")"
		}
		go func() { _ = cmd.Wait() }()
		return "(fire-and-forget)"
	})

	r.time("WM_CLOSE to steam.exe windows (no sleeps)", 0, "", func() string {
		postedWindows = 0
		for _, pid := range byImage["steam.exe"] {
			postGracefulQuitPass(pid)
		}
		return fmt.Sprintf("(%d top-level windows posted)", postedWindows)
	})

	alive := 0
	r.time("WaitForMultipleObjects on real handles", 0, "", func() string {
		alive = waitPIDsExit(targets, 5*time.Second)
		return fmt.Sprintf("(%d/%d still alive at cutoff)", alive, len(targets))
	})

	if alive > 0 {
		r.time("TerminateProcess sweep (no subprocess)", 0, "", func() string {
			var still []uint32
			for _, pid := range targets {
				h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
				if err == nil {
					rr, _ := windows.WaitForSingleObject(h, 0)
					windows.CloseHandle(h)
					if rr == windows.WAIT_OBJECT_0 {
						continue
					}
				}
				still = append(still, pid)
			}
			return fmt.Sprintf("(terminated %d)", terminateByPIDs(still))
		})
	}

	r.time("service stop: Steam Client Service", 0, "", func() string {
		if err := stopWindowsService("Steam Client Service"); err != nil {
			return "(" + err.Error() + ")"
		}
		return "(ok)"
	})
}

// ---------- microbenchmarks (no side effects) ----------

func runMicro(r *recorder) {
	const reps = 5
	ghost := "tcno_nonexistent_probe_target.exe"

	r.time(fmt.Sprintf("CreateToolhelp32Snapshot full walk x%d", reps), 0, "", func() string {
		n := 0
		for i := 0; i < reps; i++ {
			all, _ := snapshotProcesses()
			n = len(all)
		}
		return fmt.Sprintf("(%d processes each)", n)
	})

	r.time(fmt.Sprintf("cmd.exe /C taskkill (miss) x%d", reps), 0, "<- what taskKillIM does today", func() string {
		for i := 0; i < reps; i++ {
			_ = taskKillViaCmd(ghost, true)
		}
		return ""
	})

	r.time(fmt.Sprintf("taskkill.exe direct (miss) x%d", reps), 0, "<- same work, one less spawn", func() string {
		for i := 0; i < reps; i++ {
			_ = taskKillDirect(ghost, true)
		}
		return ""
	})

	r.time("mgr.Connect + OpenService + Query", 0, "", func() string {
		m, err := mgr.Connect()
		if err != nil {
			return "(" + err.Error() + ")"
		}
		defer m.Disconnect()
		s, err := m.OpenService("Steam Client Service")
		if err != nil {
			return "(not openable: " + err.Error() + ")"
		}
		defer s.Close()
		st, err := s.Query()
		if err != nil {
			return "(query err)"
		}
		return fmt.Sprintf("(state=%d)", st.State)
	})

	r.time("OpenProcess+WaitForSingleObject over all PIDs", 0, "", func() string {
		all, _ := snapshotProcesses()
		ok := 0
		for _, p := range all {
			h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, p.PID)
			if err == nil {
				windows.WaitForSingleObject(h, 0)
				windows.CloseHandle(h)
				ok++
			}
		}
		return fmt.Sprintf("(%d/%d openable)", ok, len(all))
	})
}

// runSpawn isolates why one taskkill call costs what it costs: cmd.exe vs direct,
// pipes vs no pipes, /T vs no /T, hidden window vs CREATE_NO_WINDOW.
func runSpawn() {
	const ghost = "tcno_ghost_probe_target.exe"
	const reps = 5
	bench := func(name string, fn func()) {
		fn() // warm
		s := time.Now()
		for i := 0; i < reps; i++ {
			fn()
		}
		fmt.Printf("%9.1f ms/call   %s\n", float64(time.Since(s).Microseconds())/1000/float64(reps), name)
	}
	fmt.Printf("\n=== SPAWN COST BREAKDOWN (%d reps each, image that matches nothing) ===\n", reps)

	bench("cmd.exe /C taskkill /F /T /IM + CombinedOutput + HideWindow  <-- SHIPPING", func() {
		c := exec.Command("cmd.exe", "/C", "taskkill", "/F", "/T", "/IM", ghost)
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_, _ = c.CombinedOutput()
	})
	bench("cmd.exe /C taskkill /F /T /IM + CombinedOutput, no HideWindow", func() {
		c := exec.Command("cmd.exe", "/C", "taskkill", "/F", "/T", "/IM", ghost)
		_, _ = c.CombinedOutput()
	})
	bench("taskkill.exe /F /T /IM + CombinedOutput + HideWindow", func() {
		c := exec.Command("taskkill.exe", "/F", "/T", "/IM", ghost)
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_, _ = c.CombinedOutput()
	})
	bench("taskkill.exe /F /IM (no /T) + CombinedOutput + HideWindow", func() {
		c := exec.Command("taskkill.exe", "/F", "/IM", ghost)
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_, _ = c.CombinedOutput()
	})
	bench("taskkill.exe /F /T /IM + stdout/stderr to NUL (no pipes)", func() {
		devnull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		c := exec.Command("taskkill.exe", "/F", "/T", "/IM", ghost)
		c.Stdout, c.Stderr = devnull, devnull
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = c.Run()
		devnull.Close()
	})
	bench("taskkill.exe /F /T /IM + CREATE_NO_WINDOW + pipes", func() {
		c := exec.Command("taskkill.exe", "/F", "/T", "/IM", ghost)
		c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
		_, _ = c.CombinedOutput()
	})
	bench("baseline: cmd.exe /C rem   (pure spawn cost)", func() {
		c := exec.Command("cmd.exe", "/C", "rem")
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_, _ = c.CombinedOutput()
	})
	bench("baseline: taskkill.exe /?   (spawn + load, no query)", func() {
		c := exec.Command("taskkill.exe", "/?")
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_, _ = c.CombinedOutput()
	})
	bench("in-process equivalent: snapshot + OpenProcess (no subprocess)", func() {
		all, _ := snapshotProcesses()
		for _, p := range all {
			if strings.EqualFold(p.ExeBase, ghost) {
				if h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, p.PID); err == nil {
					windows.CloseHandle(h)
				}
			}
		}
	})
}

func runInventory() {
	all, err := snapshotProcesses()
	if err != nil {
		fmt.Println("snapshot err:", err)
		return
	}
	fmt.Printf("\n=== Steam process inventory (%d total processes) ===\n", len(all))
	byImage := map[string][]procLite{}
	for _, p := range all {
		byImage[strings.ToLower(p.ExeBase)] = append(byImage[strings.ToLower(p.ExeBase)], p)
	}
	sleepTotal := 0
	for _, n := range steamKillNames {
		if strings.HasPrefix(strings.ToUpper(n), "SERVICE:") {
			continue
		}
		b := strings.ToLower(normalizeExeBase(n))
		list := byImage[b]
		fmt.Printf("  %-24s %d process(es)", b, len(list))
		if len(list) > 0 {
			pids := []string{}
			for _, p := range list {
				pids = append(pids, fmt.Sprint(p.PID))
			}
			fmt.Printf("  pids=%s", strings.Join(pids, ","))
		}
		fmt.Println()
		sleepTotal += len(list) * 200
	}
	fmt.Printf("\n  Fixed sleep the current algorithm pays: %d ms (200ms per PID, serialized within each image)\n", sleepTotal)
	fmt.Println("  Subprocess spawns the current algorithm pays: 6 cmd.exe + 6 taskkill.exe (2 per non-service image)")

	m, err := mgr.Connect()
	if err == nil {
		defer m.Disconnect()
		if s, err := m.OpenService("Steam Client Service"); err == nil {
			defer s.Close()
			if st, err := s.Query(); err == nil {
				fmt.Printf("  Steam Client Service state: %d (1=stopped, 4=running)\n", st.State)
			}
		} else {
			fmt.Println("  Steam Client Service: not openable:", err)
		}
	}
}

var jsonOut = flag.String("json", "", "write span data to this JSON file")

func main() {
	mode := flag.String("mode", "micro", "micro | spawn | inventory | current | fast")
	steamExe := flag.String("steamexe", "", "path to steam.exe (for -mode fast/hybrid native shutdown)")
	graceMs := flag.Int("grace", 1500, "grace window in ms before force-terminating (hybrid mode)")
	flag.Parse()

	switch *mode {
	case "inventory":
		runInventory()
	case "real":
		runInventory()
		fmt.Println(">>> CLOSING STEAM via the real winutil path (native quit ON)...")
		r := newRecorder()
		runReal(r, *steamExe, true)
		r.report("SHIPPING CODE + NATIVE QUIT (winutil.KillByNameWithOpts)")
	case "generic":
		runInventory()
		fmt.Println(">>> CLOSING STEAM via the real winutil path (native quit OFF)...")
		r := newRecorder()
		runReal(r, *steamExe, false)
		r.report("SHIPPING CODE, GENERIC PATH ONLY (no native quit)")
	case "native":
		runInventory()
		fmt.Println(">>> CLOSING STEAM: -shutdown only, per-process death times...")
		r := newRecorder()
		runNative(r, *steamExe)
		r.report("NATIVE (steam.exe -shutdown, no force kill at all)")
	case "floor":
		runInventory()
		fmt.Println(">>> CLOSING STEAM: force-terminate floor...")
		r := newRecorder()
		runFloor(r)
		r.report("FLOOR (TerminateProcess immediately, no graceful signal)")
	case "hybrid":
		runInventory()
		fmt.Println(">>> CLOSING STEAM: -shutdown + event wait + force sweep...")
		r := newRecorder()
		runHybrid(r, *steamExe, time.Duration(*graceMs)*time.Millisecond)
		r.report(fmt.Sprintf("HYBRID (steam -shutdown, %dms grace, then TerminateProcess)", *graceMs))
	case "spawn":
		runSpawn()
	case "micro":
		r := newRecorder()
		runMicro(r)
		r.report("MICROBENCHMARK: primitive costs (no side effects)")
	case "current":
		runInventory()
		fmt.Println("\n>>> CLOSING STEAM with the shipping algorithm...")
		r := newRecorder()
		runCurrent(r)
		r.report("CURRENT ALGORITHM (winutil.KillByName, ClosingMethod=Combined)")
	case "fast":
		runInventory()
		fmt.Println("\n>>> CLOSING STEAM with the proposed algorithm...")
		r := newRecorder()
		runFast(r, *steamExe)
		r.report("PROPOSED ALGORITHM (native shutdown + handle waits)")
	default:
		fmt.Println("unknown mode")
		os.Exit(2)
	}
}
