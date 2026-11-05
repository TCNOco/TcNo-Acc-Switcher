package basic

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/platform"
)

type sessionMemoryInfo struct {
	name string
	mode os.FileMode
}

func (i sessionMemoryInfo) Name() string       { return i.name }
func (i sessionMemoryInfo) Size() int64        { return 0 }
func (i sessionMemoryInfo) Mode() os.FileMode  { return i.mode }
func (i sessionMemoryInfo) ModTime() time.Time { return time.Time{} }
func (i sessionMemoryInfo) IsDir() bool        { return i.mode.IsDir() }
func (i sessionMemoryInfo) Sys() any           { return nil }
func (i sessionMemoryInfo) Type() os.FileMode  { return i.mode.Type() }
func (i sessionMemoryInfo) Info() (os.FileInfo, error) {
	return i, nil
}

type sessionMemoryFS struct {
	nodes      map[string]os.FileMode
	reparses   map[string]bool
	holders    []string
	moves      [][2]string
	removed    []string
	failRename string
}

func newSessionMemoryFS() *sessionMemoryFS {
	return &sessionMemoryFS{nodes: map[string]os.FileMode{}, reparses: map[string]bool{}}
}

func (m *sessionMemoryFS) directory(path string) {
	for {
		m.nodes[path] = os.ModeDir | 0o700
		parent := filepath.Dir(path)
		if parent == path {
			return
		}
		path = parent
	}
}

func (m *sessionMemoryFS) ops() sessionPathOps {
	return sessionPathOps{
		lstat: func(path string) (os.FileInfo, error) {
			mode, ok := m.nodes[path]
			if !ok {
				return nil, os.ErrNotExist
			}
			return sessionMemoryInfo{name: filepath.Base(path), mode: mode}, nil
		},
		reparse: func(path string) (bool, error) { return m.reparses[path], nil },
		mkdirTemp: func(parent, prefix string) (string, error) {
			path := filepath.Join(parent, fmt.Sprintf("%s%d", prefix, len(m.holders)+1))
			m.directory(path)
			m.holders = append(m.holders, path)
			return path, nil
		},
		rename: func(from, to string) error {
			if from == m.failRename {
				return errors.New("rename failed")
			}
			if _, ok := m.nodes[from]; !ok {
				return os.ErrNotExist
			}
			if _, ok := m.nodes[to]; ok {
				return os.ErrExist
			}
			m.moves = append(m.moves, [2]string{from, to})
			changed := map[string]os.FileMode{}
			for path, mode := range m.nodes {
				if path == from || strings.HasPrefix(path, from+string(filepath.Separator)) {
					changed[to+strings.TrimPrefix(path, from)] = mode
					delete(m.nodes, path)
				}
			}
			for path, mode := range changed {
				m.nodes[path] = mode
			}
			return nil
		},
		readDir: func(path string) ([]os.DirEntry, error) {
			if _, ok := m.nodes[path]; !ok {
				return nil, os.ErrNotExist
			}
			var entries []os.DirEntry
			for child, mode := range m.nodes {
				if child != path && filepath.Dir(child) == path {
					entries = append(entries, sessionMemoryInfo{name: filepath.Base(child), mode: mode})
				}
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
			return entries, nil
		},
		removeAll: func(path string) error {
			m.removed = append(m.removed, path)
			for child := range m.nodes {
				if child == path || strings.HasPrefix(child, path+string(filepath.Separator)) {
					delete(m.nodes, child)
				}
			}
			return nil
		},
	}
}

func sessionMemoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("testdata", "session-path-fixture"))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func sessionMemoryFlow(paths ...string) FlowContext {
	return FlowContext{Descriptor: platform.Descriptor{
		PathListToClear: paths,
		Extras:          platform.DescriptorExtras{CachePaths: paths},
	}}
}

func TestSessionPathsPrepareDefersSourceMoves(t *testing.T) {
	root := sessionMemoryRoot(t)
	auth, browser := filepath.Join(root, "auth"), filepath.Join(root, "browser")
	memory := newSessionMemoryFS()
	memory.directory(auth)
	memory.directory(browser)
	backups, err := prepareSessionPathsWithOps(sessionMemoryFlow("", auth, browser, filepath.Join(root, "absent")), []string{auth}, memory.ops())
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 || backups[0].Path != browser || filepath.Base(backups[0].Backup) != "data" {
		t.Fatalf("unexpected backups: %+v", backups)
	}
	if len(memory.moves) != 0 || len(memory.removed) != 0 || len(memory.holders) != 1 {
		t.Fatalf("moves=%v removed=%v holders=%v", memory.moves, memory.removed, memory.holders)
	}
	if _, ok := memory.nodes[browser]; !ok {
		t.Fatal("preparation changed the live directory")
	}
}

func TestSessionPathsPrepareRejectsUnsafePlansBeforeAllocation(t *testing.T) {
	root := sessionMemoryRoot(t)
	auth, browser := filepath.Join(root, "auth"), filepath.Join(root, "browser")
	volumeRoot := filepath.VolumeName(root) + string(filepath.Separator)
	for _, tc := range []struct {
		name      string
		paths     []string
		protected []string
		reparse   string
	}{
		{name: "registry", paths: []string{"REG:HKCU\\Software\\Example:Value"}},
		{name: "json", paths: []string{"JSON_EMPTY_VALUE::file.json::token"}},
		{name: "glob", paths: []string{filepath.Join(root, "*")}},
		{name: "root", paths: []string{volumeRoot}},
		{name: "relative", paths: []string{"relative"}},
		{name: "traversal", paths: []string{root + string(filepath.Separator) + ".." + string(filepath.Separator) + "other"}},
		{name: "protected ancestor", paths: []string{root}, protected: []string{auth}},
		{name: "protected child", paths: []string{filepath.Join(auth, "child")}, protected: []string{auth}},
		{name: "overlap", paths: []string{browser, filepath.Join(browser, "child")}},
		{name: "ancestor reparse", paths: []string{browser}, reparse: root},
	} {
		t.Run(tc.name, func(t *testing.T) {
			memory := newSessionMemoryFS()
			memory.directory(filepath.Join(auth, "child"))
			memory.directory(filepath.Join(browser, "child"))
			memory.reparses[tc.reparse] = true
			if _, err := prepareSessionPathsWithOps(sessionMemoryFlow(tc.paths...), tc.protected, memory.ops()); err == nil {
				t.Fatal("unsafe plan accepted")
			}
			if len(memory.holders) != 0 || len(memory.moves) != 0 || len(memory.removed) != 0 {
				t.Fatalf("unsafe plan mutated state: %+v", memory)
			}
		})
	}
}

func TestSessionPathsMoveFailureCanRestoreEarlierPaths(t *testing.T) {
	root := sessionMemoryRoot(t)
	first, second := filepath.Join(root, "first"), filepath.Join(root, "second")
	memory := newSessionMemoryFS()
	memory.directory(first)
	memory.directory(second)
	backups, err := prepareSessionPathsWithOps(sessionMemoryFlow(first, second), nil, memory.ops())
	if err != nil {
		t.Fatal(err)
	}
	memory.failRename = second
	if err := moveSessionPathsWithOps(backups, memory.ops()); err == nil {
		t.Fatal("expected move failure")
	}
	if err := restoreSessionPathsWithOps(backups, memory.ops()); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{first, second} {
		if _, exists := memory.nodes[path]; !exists {
			t.Fatalf("live directory not restored: %s", path)
		}
	}
	if len(memory.removed) != 0 || len(memory.moves) != 2 {
		t.Fatalf("moves=%v removed=%v", memory.moves, memory.removed)
	}
}

func TestSessionPathsRestorePreservesConflictingCurrentDirectory(t *testing.T) {
	root := sessionMemoryRoot(t)
	browser := filepath.Join(root, "browser")
	memory := newSessionMemoryFS()
	memory.directory(browser)
	backups, err := prepareSessionPathsWithOps(sessionMemoryFlow(browser), nil, memory.ops())
	if err != nil {
		t.Fatal(err)
	}
	if err := moveSessionPathsWithOps(backups, memory.ops()); err != nil {
		t.Fatal(err)
	}
	memory.directory(browser)
	if err := restoreSessionPathsWithOps(backups, memory.ops()); err == nil {
		t.Fatal("restore overwrote the conflicting current directory")
	}
	if _, exists := memory.nodes[backups[0].Backup]; !exists {
		t.Fatal("conflicting backup was lost")
	}
	if len(memory.moves) != 1 || len(memory.removed) != 0 {
		t.Fatalf("moves=%v removed=%v", memory.moves, memory.removed)
	}
}

func TestSessionPathsMoveValidatesEveryPlanBeforeMutation(t *testing.T) {
	root := sessionMemoryRoot(t)
	browser := filepath.Join(root, "browser")
	memory := newSessionMemoryFS()
	memory.directory(browser)
	backups, err := prepareSessionPathsWithOps(sessionMemoryFlow(browser), nil, memory.ops())
	if err != nil {
		t.Fatal(err)
	}
	backups = append(backups, sessionPathBackup{Path: filepath.Join(root, "other"), Backup: filepath.Join(root, "unowned", "data")})
	if err := moveSessionPathsWithOps(backups, memory.ops()); err == nil {
		t.Fatal("invalid backup plan accepted")
	}
	if len(memory.moves) != 0 {
		t.Fatalf("earlier source moved before invalid plan was rejected: %v", memory.moves)
	}
}

func TestSessionPathsDiscardOnlyRemovesOwnedHolder(t *testing.T) {
	root := sessionMemoryRoot(t)
	browser := filepath.Join(root, "browser")
	memory := newSessionMemoryFS()
	memory.directory(browser)
	backups, err := prepareSessionPathsWithOps(sessionMemoryFlow(browser), nil, memory.ops())
	if err != nil {
		t.Fatal(err)
	}
	if err := moveSessionPathsWithOps(backups, memory.ops()); err != nil {
		t.Fatal(err)
	}
	memory.directory(browser)
	if err := discardSessionPathsWithOps(backups, memory.ops()); err != nil {
		t.Fatal(err)
	}
	if len(memory.removed) != 1 || memory.removed[0] != filepath.Dir(backups[0].Backup) {
		t.Fatalf("removed=%v", memory.removed)
	}
	if _, exists := memory.nodes[browser]; !exists {
		t.Fatal("discard removed the current live directory")
	}
}

func TestSessionPathsDiscardRetainsUnexpectedOrReparseContents(t *testing.T) {
	for _, scenario := range []string{"unexpected holder entry", "nested reparse", "unowned holder"} {
		t.Run(scenario, func(t *testing.T) {
			root := sessionMemoryRoot(t)
			browser := filepath.Join(root, "browser")
			memory := newSessionMemoryFS()
			memory.directory(browser)
			backups, err := prepareSessionPathsWithOps(sessionMemoryFlow(browser), nil, memory.ops())
			if err != nil {
				t.Fatal(err)
			}
			holder := filepath.Dir(backups[0].Backup)
			switch scenario {
			case "unexpected holder entry":
				memory.nodes[filepath.Join(holder, "unexpected")] = 0o600
			case "nested reparse":
				memory.directory(backups[0].Backup)
				child := filepath.Join(backups[0].Backup, "junction")
				memory.directory(child)
				memory.reparses[child] = true
			case "unowned holder":
				backups[0].Backup = filepath.Join(root, "unowned", "data")
			}
			if err := discardSessionPathsWithOps(backups, memory.ops()); err == nil {
				t.Fatal("unsafe discard was accepted")
			}
			if len(memory.removed) != 0 {
				t.Fatalf("unsafe discard removed %v", memory.removed)
			}
		})
	}
}

func TestSessionPathsRecoveryPlansMustMatchConfiguredResetPaths(t *testing.T) {
	root := sessionMemoryRoot(t)
	auth, browser := filepath.Join(root, "auth"), filepath.Join(root, "browser")
	memory := newSessionMemoryFS()
	memory.directory(auth)
	memory.directory(browser)
	flow := sessionMemoryFlow(auth, browser)
	backups, err := prepareSessionPathsWithOps(flow, []string{auth}, memory.ops())
	if err != nil {
		t.Fatal(err)
	}
	if err := validateConfiguredSessionBackupsWithOps(flow, []string{auth}, backups, memory.ops()); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"unconfigured", "protected", "duplicate", "unowned holder"} {
		t.Run(scenario, func(t *testing.T) {
			plans := append([]sessionPathBackup(nil), backups...)
			switch scenario {
			case "unconfigured":
				plans[0].Path = filepath.Join(root, "other")
			case "protected":
				plans[0].Path = auth
			case "duplicate":
				plans = append(plans, plans[0])
			case "unowned holder":
				plans[0].Backup = filepath.Join(root, "unowned", "data")
			}
			if err := validateConfiguredSessionBackupsWithOps(flow, []string{auth}, plans, memory.ops()); err == nil {
				t.Fatal("unsafe recovery plan accepted")
			}
		})
	}
	if len(memory.moves) != 0 || len(memory.removed) != 0 || len(memory.holders) != 1 {
		t.Fatalf("validation changed state: %+v", memory)
	}
}

func TestSessionPathsRevertRestoredDataPreservesBackupForRetry(t *testing.T) {
	root := sessionMemoryRoot(t)
	browser := filepath.Join(root, "browser")
	memory := newSessionMemoryFS()
	memory.directory(browser)
	backups, err := prepareSessionPathsWithOps(sessionMemoryFlow(browser), nil, memory.ops())
	if err != nil {
		t.Fatal(err)
	}
	if err := moveSessionPathsWithOps(backups, memory.ops()); err != nil {
		t.Fatal(err)
	}
	if err := restoreSessionPathsWithOps(backups, memory.ops()); err != nil {
		t.Fatal(err)
	}
	if err := revertRestoredSessionPathsWithOps(backups, memory.ops()); err != nil {
		t.Fatal(err)
	}
	if _, exists := memory.nodes[browser]; exists {
		t.Fatal("restored directory was not moved back to recovery storage")
	}
	if _, exists := memory.nodes[backups[0].Backup]; !exists {
		t.Fatal("backup was not retained for retry")
	}
	if len(memory.moves) != 3 || len(memory.removed) != 0 {
		t.Fatalf("moves=%v removed=%v", memory.moves, memory.removed)
	}
}

func TestSessionPathsRevertSkipsUntouchedAndAbsentPlans(t *testing.T) {
	for _, scenario := range []string{"untouched", "both present", "both absent"} {
		t.Run(scenario, func(t *testing.T) {
			root := sessionMemoryRoot(t)
			browser := filepath.Join(root, "browser")
			memory := newSessionMemoryFS()
			memory.directory(browser)
			backups, err := prepareSessionPathsWithOps(sessionMemoryFlow(browser), nil, memory.ops())
			if err != nil {
				t.Fatal(err)
			}
			if scenario != "both absent" {
				if err := moveSessionPathsWithOps(backups, memory.ops()); err != nil {
					t.Fatal(err)
				}
				if scenario == "both present" {
					memory.directory(browser)
				}
			} else {
				delete(memory.nodes, browser)
			}
			moves := len(memory.moves)
			if err := revertRestoredSessionPathsWithOps(backups, memory.ops()); err != nil {
				t.Fatal(err)
			}
			if len(memory.moves) != moves || len(memory.removed) != 0 {
				t.Fatalf("unchanged plan mutated state: %+v", memory)
			}
		})
	}
}

func TestSessionPathsRevertFailurePreservesRestoredData(t *testing.T) {
	root := sessionMemoryRoot(t)
	browser := filepath.Join(root, "browser")
	memory := newSessionMemoryFS()
	memory.directory(browser)
	backups, err := prepareSessionPathsWithOps(sessionMemoryFlow(browser), nil, memory.ops())
	if err != nil {
		t.Fatal(err)
	}
	memory.failRename = browser
	if err := revertRestoredSessionPathsWithOps(backups, memory.ops()); err == nil {
		t.Fatal("rename failure was not returned")
	}
	if _, exists := memory.nodes[browser]; !exists {
		t.Fatal("failed revert lost the restored directory")
	}
	if len(memory.moves) != 0 || len(memory.removed) != 0 {
		t.Fatalf("failed revert mutated state: %+v", memory)
	}
}

func TestSessionPathsRevertValidatesEveryPlanBeforeMutation(t *testing.T) {
	root := sessionMemoryRoot(t)
	browser := filepath.Join(root, "browser")
	memory := newSessionMemoryFS()
	memory.directory(browser)
	backups, err := prepareSessionPathsWithOps(sessionMemoryFlow(browser), nil, memory.ops())
	if err != nil {
		t.Fatal(err)
	}
	backups = append(backups, sessionPathBackup{Path: filepath.Join(root, "other"), Backup: filepath.Join(root, "unowned", "data")})
	if err := revertRestoredSessionPathsWithOps(backups, memory.ops()); err == nil {
		t.Fatal("invalid later plan was accepted")
	}
	if len(memory.moves) != 0 || len(memory.removed) != 0 {
		t.Fatalf("earlier restored directory moved before invalid plan was rejected: %+v", memory)
	}
}

func TestSessionPathsTrackedRestoreReturnsOnlySuccessfulMovesOnFailure(t *testing.T) {
	root := sessionMemoryRoot(t)
	first, second := filepath.Join(root, "first"), filepath.Join(root, "second")
	memory := newSessionMemoryFS()
	memory.directory(first)
	memory.directory(second)
	backups, err := prepareSessionPathsWithOps(sessionMemoryFlow(first, second), nil, memory.ops())
	if err != nil {
		t.Fatal(err)
	}
	if err := moveSessionPathsWithOps(backups, memory.ops()); err != nil {
		t.Fatal(err)
	}
	memory.failRename = backups[0].Backup
	restored, err := restoreSessionPathsTrackedWithOps(backups, memory.ops())
	if err == nil || len(restored) != 1 || restored[0] != backups[1] {
		t.Fatalf("restored=%+v err=%v", restored, err)
	}
	if _, exists := memory.nodes[first]; exists {
		t.Fatal("failed restore changed its source path")
	}
	if _, exists := memory.nodes[second]; !exists {
		t.Fatal("successful restore was not recorded")
	}
	if err := revertRestoredSessionPathsWithOps(restored, memory.ops()); err != nil {
		t.Fatal(err)
	}
	for _, backup := range backups {
		if _, exists := memory.nodes[backup.Backup]; !exists {
			t.Fatalf("backup was not preserved for retry: %s", backup.Backup)
		}
		if _, exists := memory.nodes[backup.Path]; exists {
			t.Fatalf("restored path was not reverted: %s", backup.Path)
		}
	}
}

func TestSessionPathsTrackedRestoreDoesNotReportNoOps(t *testing.T) {
	for _, present := range []bool{false, true} {
		t.Run(fmt.Sprint(present), func(t *testing.T) {
			root := sessionMemoryRoot(t)
			browser := filepath.Join(root, "browser")
			memory := newSessionMemoryFS()
			memory.directory(browser)
			backups, err := prepareSessionPathsWithOps(sessionMemoryFlow(browser), nil, memory.ops())
			if err != nil {
				t.Fatal(err)
			}
			if !present {
				delete(memory.nodes, browser)
			}
			restored, err := restoreSessionPathsTrackedWithOps(backups, memory.ops())
			if err != nil || len(restored) != 0 || len(memory.moves) != 0 {
				t.Fatalf("restored=%+v moves=%v err=%v", restored, memory.moves, err)
			}
		})
	}
}

func TestSessionPathsRequireExplicitCacheDeclarationBeforeAllocation(t *testing.T) {
	root := sessionMemoryRoot(t)
	browser := filepath.Join(root, "browser")
	memory := newSessionMemoryFS()
	memory.directory(browser)
	flow := sessionMemoryFlow(browser)
	flow.Descriptor.Extras.CachePaths = nil
	if _, err := prepareSessionPathsWithOps(flow, nil, memory.ops()); err == nil {
		t.Fatal("undeclared reset cache was accepted")
	}
	plan := sessionPathBackup{Path: browser, Backup: filepath.Join(root, ".tcno-session-reset-fixture", "data")}
	if err := validateConfiguredSessionBackupsWithOps(flow, nil, []sessionPathBackup{plan}, memory.ops()); err == nil {
		t.Fatal("recovery accepted an undeclared reset cache")
	}
	if len(memory.holders) != 0 || len(memory.moves) != 0 || len(memory.removed) != 0 {
		t.Fatalf("undeclared cache changed state: %+v", memory)
	}
}
