//go:build windows

// closebench measures where wall-clock time goes when the switcher closes a platform,
// and in particular whether a platform actually exits on WM_CLOSE or whether it ignores
// the message and burns the whole graceful window before being force-killed.
//
// Side-effect free:
//
//	go run ./tools/closebench -mode inventory -platform "Epic Games"
//	go run ./tools/closebench -mode spawn
//
// These CLOSE the platform:
//
//	go run ./tools/closebench -mode wmclose -platform "Epic Games"   # isolates: does WM_CLOSE work?
//	go run ./tools/closebench -mode current -platform "Epic Games"   # full shipping path, timed
//	go run ./tools/closebench -mode native  -platform "Steam" -native "C:\...\steam.exe -shutdown"
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

// steamKillNames mirrors internal/steam.steamKillNames, which is deliberately wider than
// the Steam descriptor's ExesToEnd.
var steamKillNames = []string{
	"steam.exe",
	"SERVICE:Steam Client Service",
	"steamwebhelper.exe",
	"GameOverlayUI.exe",
}

// resolveTargets returns the kill list for a platform key, read from Platforms.json the way
// the switcher does, so the bench measures the same set the product does.
func resolveTargets(platformsPath, platformKey string) ([]string, error) {
	if strings.EqualFold(strings.TrimSpace(platformKey), "Steam") {
		return steamKillNames, nil
	}
	b, err := os.ReadFile(platformsPath)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Platforms map[string]struct {
			ExesToEnd []string `json:"ExesToEnd"`
			Extras    struct {
				ClosingMethod      string `json:"ClosingMethod"`
				ForceClosingMethod bool   `json:"ForceClosingMethod"`
			} `json:"Extras"`
		} `json:"Platforms"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	for k, v := range doc.Platforms {
		if strings.EqualFold(k, platformKey) {
			if m := strings.TrimSpace(v.Extras.ClosingMethod); m != "" {
				fmt.Printf("note: %s overrides ClosingMethod=%s (forced=%t)\n", k, m, v.Extras.ForceClosingMethod)
			}
			return v.ExesToEnd, nil
		}
	}
	return nil, fmt.Errorf("platform %q not found in %s", platformKey, platformsPath)
}

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
		line := fmt.Sprintf("%8.1fms %8.1fms  %s%s", s.StartMs, s.Duration, strings.Repeat("  ", s.Depth), s.Name)
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

func isService(raw string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(raw)), "SERVICE:")
}

// targetPIDs maps each running target PID to its image name.
func targetPIDs(names []string) (map[uint32]string, map[string][]uint32) {
	all, _ := snapshotProcesses()
	byImage := map[string][]uint32{}
	for _, p := range all {
		b := strings.ToLower(normalizeExeBase(p.ExeBase))
		byImage[b] = append(byImage[b], p.PID)
	}
	out := map[uint32]string{}
	for _, n := range names {
		if isService(n) {
			continue
		}
		b := strings.ToLower(normalizeExeBase(n))
		for _, pid := range byImage[b] {
			out[pid] = b
		}
	}
	return out, byImage
}

var (
	modUser32                    = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows              = modUser32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = modUser32.NewProc("GetWindowThreadProcessId")
	procGetWindow                = modUser32.NewProc("GetWindow")
	procPostMessageW             = modUser32.NewProc("PostMessageW")
	procIsWindowVisible          = modUser32.NewProc("IsWindowVisible")
	procGetClassNameW            = modUser32.NewProc("GetClassNameW")
)

const (
	winWMClose      = 0x0010
	winWMSysCommand = 0x0112
	winSCClose      = 0xF060
	winGWOwner      = 4
)

type postedWindow struct {
	class   string
	visible bool
}

var (
	gracefulQuitCb uintptr
	postedMu       sync.Mutex
	postedList     []postedWindow
)

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
		buf := make([]uint16, 256)
		n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		vis, _, _ := procIsWindowVisible.Call(hwnd)
		postedMu.Lock()
		postedList = append(postedList, postedWindow{class: windows.UTF16ToString(buf[:n]), visible: vis != 0})
		postedMu.Unlock()
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

// watchDeaths records the exact moment each target process dies. A process whose handle we
// cannot open (a service running as SYSTEM, say) is polled through the process table instead:
// "cannot open" means unknown, never dead, or the verdict reads as success when nothing happened.
func watchDeaths(r *recorder, pids map[uint32]string, maxWait time.Duration) (survivors int) {
	var wg sync.WaitGroup
	start := time.Now()
	for pid, image := range pids {
		wg.Add(1)
		go func(pid uint32, image string) {
			defer wg.Done()
			note := ""
			if h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid); err == nil {
				res, _ := windows.WaitForSingleObject(h, uint32(maxWait/time.Millisecond))
				windows.CloseHandle(h)
				if res != windows.WAIT_OBJECT_0 {
					note = "(STILL ALIVE at cutoff)"
				}
			} else {
				note = "(no handle; polled)"
				deadline := time.Now().Add(maxWait)
				for time.Now().Before(deadline) {
					if !pidAlive(pid) {
						break
					}
					time.Sleep(200 * time.Millisecond)
				}
				if pidAlive(pid) {
					note = "(STILL ALIVE at cutoff, no handle)"
				}
			}
			r.mark(fmt.Sprintf("died: %s pid=%d", image, pid), 1, start, note)
		}(pid, image)
	}
	wg.Wait()
	// Ground truth: whatever the handles said, ask the process table what is still there.
	live, _ := targetPIDs(imageNames(pids))
	return len(live)
}

func pidAlive(pid uint32) bool {
	all, err := snapshotProcesses()
	if err != nil {
		return true
	}
	for _, p := range all {
		if p.PID == pid {
			return true
		}
	}
	return false
}

func imageNames(pids map[uint32]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, img := range pids {
		if !seen[img] {
			seen[img] = true
			out = append(out, img)
		}
	}
	return out
}

func runInventory(names []string) map[uint32]string {
	targets, _ := targetPIDs(names)
	all, _ := snapshotProcesses()
	fmt.Printf("\n=== target inventory (%d processes on the system) ===\n", len(all))
	counts := map[string]int{}
	for _, img := range targets {
		counts[img]++
	}
	for _, n := range names {
		if isService(n) {
			svcName := strings.TrimSpace(n[len("SERVICE:"):])
			state := "unknown"
			if m, err := mgr.Connect(); err == nil {
				if s, err := m.OpenService(svcName); err == nil {
					if st, err := s.Query(); err == nil {
						state = fmt.Sprintf("state=%d", st.State)
					}
					s.Close()
				} else {
					state = "not openable: " + err.Error()
				}
				m.Disconnect()
			}
			fmt.Printf("  %-34s SERVICE   %s\n", svcName, state)
			continue
		}
		b := strings.ToLower(normalizeExeBase(n))
		fmt.Printf("  %-34s %d process(es)\n", b, counts[b])
	}
	fmt.Printf("  -> %d target processes running\n", len(targets))
	return targets
}

// runWMClose is the isolation experiment: send exactly the graceful signal the generic
// Combined path sends, then do nothing else. If the platform is still alive after the
// graceful window, WM_CLOSE does not close it and the shipping path is guaranteed to
// burn its whole deadline and then force-kill.
func runWMClose(r *recorder, names []string, watch time.Duration) {
	targets, _ := targetPIDs(names)
	if len(targets) == 0 {
		fmt.Println("(nothing running)")
		return
	}
	r.time("WM_CLOSE + WM_SYSCOMMAND/SC_CLOSE to every top-level window (2 passes)", 0, "", func() string {
		postedMu.Lock()
		postedList = nil
		postedMu.Unlock()
		for pid := range targets {
			postGracefulQuitPass(pid)
		}
		time.Sleep(200 * time.Millisecond)
		for pid := range targets {
			postGracefulQuitPass(pid)
		}
		postedMu.Lock()
		defer postedMu.Unlock()
		vis := 0
		classes := map[string]int{}
		for _, w := range postedList {
			if w.visible {
				vis++
			}
			classes[w.class]++
		}
		var cs []string
		for c, n := range classes {
			cs = append(cs, fmt.Sprintf("%s x%d", c, n))
		}
		sort.Strings(cs)
		return fmt.Sprintf("(%d windows, %d visible: %s)", len(postedList), vis, strings.Join(cs, ", "))
	})
	survivors := watchDeaths(r, targets, watch)
	r.mark(fmt.Sprintf("VERDICT: %d/%d still alive after %s", survivors, len(targets), watch), 0, time.Now(), "")
	fmt.Println()
	if survivors == len(targets) {
		fmt.Printf(">>> WM_CLOSE DOES NOTHING: all %d processes ignored it.\n", survivors)
		fmt.Println(">>> The shipping Combined path will burn its full 5s window, then force-kill.")
	} else if survivors > 0 {
		fmt.Printf(">>> PARTIAL: %d of %d exited on WM_CLOSE; the rest hold the window open.\n", len(targets)-survivors, len(targets))
	} else {
		fmt.Println(">>> WM_CLOSE WORKS: every process exited without a force kill.")
	}
}

// runEndSession tests the Windows session-end protocol as a generic alternative to WM_CLOSE.
// A tray app that swallows WM_CLOSE (treating it as "minimise") still has to honour
// WM_QUERYENDSESSION/WM_ENDSESSION or it would block logoff, so this is the signal that
// actually means "you are going away" rather than "hide yourself".
func runEndSession(r *recorder, names []string, watch time.Duration) {
	targets, _ := targetPIDs(names)
	if len(targets) == 0 {
		fmt.Println("(nothing running)")
		return
	}
	r.time("WM_QUERYENDSESSION + WM_ENDSESSION to every top-level window", 0, "", func() string {
		posted := 0
		for pid := range targets {
			posted += postEndSessionPass(pid)
		}
		return fmt.Sprintf("(%d windows)", posted)
	})
	survivors := watchDeaths(r, targets, watch)
	r.mark(fmt.Sprintf("VERDICT: %d/%d still alive after %s", survivors, len(targets), watch), 0, time.Now(), "")
	fmt.Println()
	if survivors == 0 {
		fmt.Println(">>> SESSION-END WORKS: every process exited without a force kill.")
	} else if survivors < len(targets) {
		fmt.Printf(">>> PARTIAL: %d of %d exited.\n", len(targets)-survivors, len(targets))
	} else {
		fmt.Printf(">>> SESSION-END DOES NOTHING: all %d ignored it.\n", survivors)
	}
}

const (
	winWMQueryEndSession = 0x0011
	winWMEndSession      = 0x0016
	endsessionCloseApp   = 0x00000001
	smtoAbortIfHung      = 0x0002
)

var (
	procSendMessageTimeoutW = modUser32.NewProc("SendMessageTimeoutW")
	endSessionCb            uintptr
	endSessionCount         int
)

func init() {
	endSessionCb = syscall.NewCallback(func(hwnd, lParam uintptr) uintptr {
		targetPID := uint32(lParam)
		var windowPID uint32
		r0, _, _ := procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
		if r0 == 0 || windowPID != targetPID {
			return 1
		}
		if owner, _, _ := procGetWindow.Call(hwnd, uintptr(winGWOwner)); owner != 0 {
			return 1
		}
		endSessionCount++
		var res uintptr
		procSendMessageTimeoutW.Call(hwnd, uintptr(winWMQueryEndSession), 0, uintptr(endsessionCloseApp),
			uintptr(smtoAbortIfHung), 3000, uintptr(unsafe.Pointer(&res)))
		procSendMessageTimeoutW.Call(hwnd, uintptr(winWMEndSession), 1, uintptr(endsessionCloseApp),
			uintptr(smtoAbortIfHung), 3000, uintptr(unsafe.Pointer(&res)))
		return 1
	})
}

func postEndSessionPass(pid uint32) int {
	if err := procEnumWindows.Find(); err != nil {
		return 0
	}
	endSessionCount = 0
	_, _, _ = procEnumWindows.Call(endSessionCb, uintptr(pid))
	return endSessionCount
}

// runReal drives the shipping winutil path so the number reflects the product.
func runReal(r *recorder, names []string, nativeCmd string, method winutil.ClosingMethod) {
	targets, _ := targetPIDs(names)
	opts := winutil.KillOpts{}
	if strings.TrimSpace(nativeCmd) != "" {
		exe, args := splitCommand(nativeCmd)
		opts.NativeQuit = func() error {
			return winutil.Start(exe, args, winutil.StartOpts{HideWindow: true, WorkingDir: filepath.Dir(exe)})
		}
	}
	go watchDeaths(r, targets, 40*time.Second)
	r.time(fmt.Sprintf("winutil.KillByNameWithOpts (%s)", method), 0, "", func() string {
		if err := winutil.KillByNameWithOpts(names, method, opts); err != nil {
			return "(err: " + err.Error() + ")"
		}
		return ""
	})
}

// splitCommand splits `C:\path with spaces\app.exe -flag` on the .exe boundary so the
// caller does not have to quote.
func splitCommand(s string) (exe string, args []string) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, `"`) {
		if end := strings.Index(s[1:], `"`); end >= 0 {
			return s[1 : end+1], strings.Fields(strings.TrimSpace(s[end+2:]))
		}
	}
	lower := strings.ToLower(s)
	if i := strings.Index(lower, ".exe"); i >= 0 {
		return s[:i+4], strings.Fields(strings.TrimSpace(s[i+4:]))
	}
	f := strings.Fields(s)
	if len(f) == 0 {
		return "", nil
	}
	return f[0], f[1:]
}

// runWindows counts the top-level windows each target owns. A target with none can never
// answer WM_CLOSE, so any graceful window spent waiting on it is spent waiting for nothing.
func runWindows(names []string) {
	targets, _ := targetPIDs(names)
	fmt.Println("\n=== TOP-LEVEL WINDOWS PER TARGET ===")
	if len(targets) == 0 {
		fmt.Println("  (nothing running)")
		return
	}
	byImage := map[string]int{}
	for pid, image := range targets {
		postedMu.Lock()
		postedList = nil
		postedMu.Unlock()
		enumTopLevelForPID(pid)
		postedMu.Lock()
		n := len(postedList)
		var cs []string
		for _, w := range postedList {
			vis := ""
			if w.visible {
				vis = " (visible)"
			}
			cs = append(cs, w.class+vis)
		}
		postedMu.Unlock()
		byImage[image] += n
		sort.Strings(cs)
		fmt.Printf("  %-30s pid=%-7d %d window(s)  %s\n", image, pid, n, strings.Join(cs, ", "))
	}
	fmt.Println()
	for img, n := range byImage {
		if n == 0 {
			fmt.Printf("  >>> %s has NO top-level windows: WM_CLOSE cannot reach it.\n", img)
		}
	}
}

var enumOnlyCb uintptr

func init() {
	enumOnlyCb = syscall.NewCallback(func(hwnd, lParam uintptr) uintptr {
		targetPID := uint32(lParam)
		var windowPID uint32
		r0, _, _ := procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
		if r0 == 0 || windowPID != targetPID {
			return 1
		}
		if owner, _, _ := procGetWindow.Call(hwnd, uintptr(winGWOwner)); owner != 0 {
			return 1
		}
		buf := make([]uint16, 256)
		n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		vis, _, _ := procIsWindowVisible.Call(hwnd)
		postedMu.Lock()
		postedList = append(postedList, postedWindow{class: windows.UTF16ToString(buf[:n]), visible: vis != 0})
		postedMu.Unlock()
		return 1
	})
}

func enumTopLevelForPID(pid uint32) {
	if err := procEnumWindows.Find(); err != nil {
		return
	}
	_, _, _ = procEnumWindows.Call(enumOnlyCb, uintptr(pid))
}

// runPreflight reports what the switcher's own elevation check makes of this target list,
// without touching anything. A target the current token cannot open forces the whole switch
// to demand admin, so a misdeclared entry is visible here before any process is closed.
func runPreflight(names []string) {
	fmt.Printf("\n=== PREFLIGHT (winutil.CanKillProcesses, elevated=%t) ===\n", winutil.IsProcessElevated())
	blocker, ok := winutil.CanKillProcesses(names, winutil.ClosingCombined)
	if ok {
		fmt.Println("  Combined: OK, no elevation required")
	} else {
		fmt.Printf("  Combined: NEEDS ADMIN, blocked by %q\n", blocker)
	}
	blocker, ok = winutil.CanKillProcesses(names, winutil.ClosingTaskKill)
	if ok {
		fmt.Println("  TaskKill: OK, no elevation required")
	} else {
		fmt.Printf("  TaskKill: NEEDS ADMIN, blocked by %q\n", blocker)
	}
	for _, n := range names {
		if isService(n) {
			continue
		}
		img := normalizeExeBase(n)
		one, ok := winutil.CanKillProcesses([]string{img}, winutil.ClosingCombined)
		status := "killable"
		if !ok {
			status = "BLOCKS (needs admin): " + one
		}
		fmt.Printf("    %-34s %s\n", img, status)
	}
}

// runSpawn isolates the per-call cost of the subprocess helpers the kill path uses.
func runSpawn() {
	const ghost = "tcno_ghost_probe_target.exe"
	const reps = 5
	bench := func(name string, fn func()) {
		fn()
		s := time.Now()
		for i := 0; i < reps; i++ {
			fn()
		}
		fmt.Printf("%9.1f ms/call   %s\n", float64(time.Since(s).Microseconds())/1000/float64(reps), name)
	}
	fmt.Printf("\n=== SPAWN COST BREAKDOWN (%d reps, image that matches nothing) ===\n", reps)
	bench("cmd.exe /C taskkill /F /T /IM + CombinedOutput + HideWindow", func() {
		_ = taskKillViaCmd(ghost, true)
	})
	bench("taskkill.exe /F /T /IM direct", func() {
		_ = taskKillDirect(ghost, true)
	})
	bench("baseline: cmd.exe /C rem   (pure spawn cost)", func() {
		c := exec.Command("cmd.exe", "/C", "rem")
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_, _ = c.CombinedOutput()
	})
	bench("in-process: Toolhelp snapshot + match", func() {
		all, _ := snapshotProcesses()
		for _, p := range all {
			_ = strings.EqualFold(p.ExeBase, ghost)
		}
	})
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

var jsonOut = flag.String("json", "", "write span data to this JSON file")

func main() {
	mode := flag.String("mode", "inventory", "inventory | windows | preflight | spawn | wmclose | endsession | current | native")
	platform := flag.String("platform", "", `platform key from Platforms.json, e.g. "Epic Games"`)
	exes := flag.String("exes", "", "comma-separated image names, overrides -platform")
	platformsPath := flag.String("platforms", "Platforms.json", "path to Platforms.json")
	nativeCmd := flag.String("native", "", `native quit command, e.g. "C:\...\steam.exe -shutdown"`)
	methodFlag := flag.String("method", "Combined", "ClosingMethod: Combined | Close | TaskKill | Electron")
	watchSec := flag.Int("watch", 15, "seconds to watch for exit in -mode wmclose")
	flag.Parse()

	var names []string
	switch {
	case strings.TrimSpace(*exes) != "":
		for _, e := range strings.Split(*exes, ",") {
			if e = strings.TrimSpace(e); e != "" {
				names = append(names, e)
			}
		}
	case strings.TrimSpace(*platform) != "":
		var err error
		names, err = resolveTargets(*platformsPath, *platform)
		if err != nil {
			fmt.Println("error:", err)
			os.Exit(2)
		}
	default:
		if *mode != "spawn" {
			fmt.Println("need -platform or -exes")
			os.Exit(2)
		}
	}
	if len(names) > 0 {
		fmt.Printf("targets: %s\n", strings.Join(names, ", "))
	}

	switch *mode {
	case "inventory":
		runInventory(names)
	case "windows":
		runInventory(names)
		runWindows(names)
	case "preflight":
		runInventory(names)
		runPreflight(names)
	case "spawn":
		runSpawn()
	case "wmclose":
		runInventory(names)
		fmt.Println("\n>>> sending WM_CLOSE only, no taskkill, no force...")
		r := newRecorder()
		runWMClose(r, names, time.Duration(*watchSec)*time.Second)
		r.report("WM_CLOSE ISOLATION")
	case "endsession":
		runInventory(names)
		fmt.Println(">>> sending WM_QUERYENDSESSION/WM_ENDSESSION only...")
		r := newRecorder()
		runEndSession(r, names, time.Duration(*watchSec)*time.Second)
		r.report("SESSION-END ISOLATION")
	case "current":
		runInventory(names)
		fmt.Println("\n>>> closing via the shipping winutil path (no native quit)...")
		r := newRecorder()
		runReal(r, names, "", winutil.ClosingMethod(*methodFlag))
		r.report("SHIPPING PATH, GENERIC (no native quit)")
	case "native":
		runInventory(names)
		fmt.Println("\n>>> closing via the shipping winutil path (native quit ON)...")
		r := newRecorder()
		runReal(r, names, *nativeCmd, winutil.ClosingMethod(*methodFlag))
		r.report("SHIPPING PATH + NATIVE QUIT")
	default:
		fmt.Println("unknown mode")
		os.Exit(2)
	}
	_ = stopWindowsService
}
