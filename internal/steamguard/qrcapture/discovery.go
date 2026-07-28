package qrcapture

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var steamExecutables = map[string]struct{}{
	"steam.exe":          {},
	"steamwebhelper.exe": {},
}

// Scanner applies identity and geometry policy around a native Backend.
type Scanner struct {
	backend        Backend
	applicationPID uint32
}

// New returns the platform scanner and excludes the current process's windows.
func New() *Scanner {
	return NewWithBackend(newBackend(), uint32(os.Getpid()))
}

// NewWithBackend constructs an injectable scanner.
func NewWithBackend(backend Backend, applicationPID uint32) *Scanner {
	return &Scanner{backend: backend, applicationPID: applicationPID}
}

// Discover resolves trusted Steam roots and returns every eligible Steam window.
func (s *Scanner) Discover(configuredPath string) (Discovery, error) {
	processes, err := s.backend.RunningProcesses()
	if err != nil {
		return classifyDiscoveryError(err)
	}
	registryPaths, registryErr := s.backend.RegistrySteamPaths()
	if registryErr != nil && len(strings.TrimSpace(configuredPath)) == 0 && len(processes) == 0 {
		return classifyDiscoveryError(registryErr)
	}

	hints := make([]string, 0, 1+len(registryPaths)+len(processes))
	if strings.TrimSpace(configuredPath) != "" {
		hints = append(hints, configuredPath)
	}
	hints = append(hints, registryPaths...)
	for _, process := range processes {
		if isSteamExecutable(process.ExecutablePath) {
			hints = append(hints, process.ExecutablePath)
		}
	}
	roots := s.resolveRoots(hints)
	if len(roots) == 0 {
		return Discovery{State: DiscoverySteamNotFound}, nil
	}

	verifiedProcesses := s.verifyProcesses(processes, roots)
	windows, err := s.backend.TopLevelWindows()
	if err != nil {
		return classifyDiscoveryError(err)
	}
	monitors, err := s.backend.MonitorBounds()
	if err != nil {
		return classifyDiscoveryError(err)
	}
	candidates := filterWindows(windows, verifiedProcesses, monitors, s.applicationPID)
	state := DiscoveryReady
	if len(candidates) == 0 {
		state = DiscoveryNoWindow
	}
	return Discovery{State: state, Roots: roots, Windows: candidates}, nil
}

// CaptureWindow revalidates process identity and current window geometry before
// reading pixels, preventing stale HWND reuse from crossing the trust boundary.
func (s *Scanner) CaptureWindow(candidate Candidate) (Capture, error) {
	canonicalRoot, err := s.backend.CanonicalSteamRoot(candidate.SteamRoot)
	if err != nil || !samePath(canonicalRoot, candidate.SteamRoot) {
		return Capture{State: CaptureUnavailable}, nil
	}
	processes, err := s.backend.RunningProcesses()
	if err != nil {
		return classifyCaptureError(err)
	}
	verified := s.verifyProcesses(processes, []string{candidate.SteamRoot})
	identity, ok := verified[candidate.PID]
	if !ok || !samePath(identity.executablePath, candidate.ExecutablePath) {
		return Capture{State: CaptureUnavailable}, nil
	}
	windows, err := s.backend.TopLevelWindows()
	if err != nil {
		return classifyCaptureError(err)
	}
	monitors, err := s.backend.MonitorBounds()
	if err != nil {
		return classifyCaptureError(err)
	}
	var current *WindowInfo
	for i := range windows {
		if windows[i].Handle == candidate.Handle && windows[i].PID == candidate.PID {
			current = &windows[i]
			break
		}
	}
	if current == nil {
		return Capture{State: CaptureUnavailable}, nil
	}
	filtered := filterWindows([]WindowInfo{*current}, verified, monitors, s.applicationPID)
	if len(filtered) != 1 {
		return Capture{State: CaptureUnavailable}, nil
	}
	revalidated := filtered[0]
	frames := make([]Frame, 0, len(revalidated.Regions))
	for _, region := range revalidated.Regions {
		frame, err := s.backend.CaptureRegion(region)
		if err != nil {
			wipeFrames(frames)
			return Capture{State: CaptureFailed, Window: revalidated}, errors.Join(ErrCapture, err)
		}
		frames = append(frames, frame)
	}
	after, err := s.backend.TopLevelWindows()
	if err != nil {
		wipeFrames(frames)
		return classifyCaptureError(err)
	}
	stable := false
	for _, window := range after {
		if window.Handle == revalidated.Handle && window.PID == revalidated.PID && window.Visible && !window.Cloaked && window.Bounds == revalidated.Bounds {
			stable = true
			break
		}
	}
	if !stable {
		wipeFrames(frames)
		return Capture{State: CaptureUnavailable}, nil
	}
	return Capture{State: CaptureReady, Window: revalidated, Frames: frames}, nil
}

func wipeFrames(frames []Frame) {
	for i := range frames {
		for j := range frames[i].BGRA {
			frames[i].BGRA[j] = 0
		}
		frames[i].BGRA = nil
	}
}

type verifiedProcess struct {
	executablePath string
	steamRoot      string
}

func (s *Scanner) resolveRoots(hints []string) []string {
	seen := make(map[string]struct{})
	var roots []string
	for _, hint := range hints {
		root, err := s.backend.CanonicalSteamRoot(hint)
		if err != nil || root == "" {
			continue
		}
		key := strings.ToLower(filepath.Clean(root))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		roots = append(roots, root)
	}
	sort.Slice(roots, func(i, j int) bool { return strings.ToLower(roots[i]) < strings.ToLower(roots[j]) })
	return roots
}

func (s *Scanner) verifyProcesses(processes []ProcessInfo, roots []string) map[uint32]verifiedProcess {
	verified := make(map[uint32]verifiedProcess)
	for _, process := range processes {
		if process.PID == 0 || process.PID == s.applicationPID || !isSteamExecutable(process.ExecutablePath) {
			continue
		}
		executable, err := s.backend.CanonicalExecutable(process.ExecutablePath)
		if err != nil {
			continue
		}
		for _, root := range roots {
			if pathWithinRoot(root, executable) {
				verified[process.PID] = verifiedProcess{executablePath: executable, steamRoot: root}
				break
			}
		}
	}
	return verified
}

func filterWindows(windows []WindowInfo, processes map[uint32]verifiedProcess, monitors []Rect, applicationPID uint32) []Candidate {
	seen := make(map[uintptr]struct{})
	candidates := make([]Candidate, 0, len(windows))
	for _, window := range windows {
		identity, trusted := processes[window.PID]
		if !trusted || window.Handle == 0 || window.PID == 0 || window.PID == applicationPID || !window.Visible || window.Cloaked {
			continue
		}
		if isTcNoWindow(window) {
			continue
		}
		regions := visibleRegions(window.Bounds, monitors)
		if len(regions) == 0 {
			continue
		}
		if _, duplicate := seen[window.Handle]; duplicate {
			continue
		}
		seen[window.Handle] = struct{}{}
		candidates = append(candidates, Candidate{
			Handle: window.Handle, Owner: window.Owner, PID: window.PID,
			Title: window.Title, ClassName: window.ClassName,
			ExecutablePath: identity.executablePath, SteamRoot: identity.steamRoot,
			Bounds: window.Bounds, Regions: regions,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Owner != candidates[j].Owner {
			return candidates[i].Owner != 0
		}
		return candidates[i].Handle < candidates[j].Handle
	})
	return candidates
}

func isTcNoWindow(window WindowInfo) bool {
	title := strings.ToLower(strings.TrimSpace(window.Title))
	className := strings.ToLower(strings.TrimSpace(window.ClassName))
	return strings.Contains(title, "tcno account switcher") || strings.Contains(className, "tcnoaccswitcher")
}

func isSteamExecutable(path string) bool {
	_, ok := steamExecutables[strings.ToLower(filepath.Base(filepath.Clean(path)))]
	return ok
}

func pathWithinRoot(root, path string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil || relative == "." || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func samePath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func classifyDiscoveryError(err error) (Discovery, error) {
	if errors.Is(err, ErrUnsupported) {
		return Discovery{State: DiscoveryUnsupported}, err
	}
	return Discovery{State: DiscoveryFailed}, errors.Join(ErrPlatform, err)
}

func classifyCaptureError(err error) (Capture, error) {
	if errors.Is(err, ErrUnsupported) {
		return Capture{State: CaptureUnsupported}, err
	}
	return Capture{State: CaptureFailed}, errors.Join(ErrPlatform, err)
}
