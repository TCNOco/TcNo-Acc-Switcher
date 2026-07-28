package dependencypolicy

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/mod/semver"
)

const (
	textModule        = "golang.org/x/text"
	minimumTextModule = "v0.39.0"
)

type moduleInfo struct {
	Path    string      `json:"Path"`
	Version string      `json:"Version"`
	Replace *moduleInfo `json:"Replace"`
}

func TestSelectedTextModuleMeetsMinimum(t *testing.T) {
	selected := loadSelectedModule(t, textModule)
	if err := requireMinimumVersion(selected, textModule, minimumTextModule); err != nil {
		t.Fatal(err)
	}
}

func TestRequireMinimumVersion(t *testing.T) {
	tests := []struct {
		name    string
		module  *moduleInfo
		wantErr bool
	}{
		{name: "minimum", module: &moduleInfo{Path: textModule, Version: "v0.39.0"}},
		{name: "higher", module: &moduleInfo{Path: textModule, Version: "v0.40.0"}},
		{name: "prerelease above minimum", module: &moduleInfo{Path: textModule, Version: "v0.40.0-rc.1"}},
		{name: "same module replacement above minimum", module: &moduleInfo{
			Path: textModule, Version: "v0.38.0", Replace: &moduleInfo{Path: textModule, Version: "v0.40.0"},
		}},
		{name: "below minimum", module: &moduleInfo{Path: textModule, Version: "v0.38.0"}, wantErr: true},
		{name: "prerelease below stable minimum", module: &moduleInfo{Path: textModule, Version: "v0.39.0-rc.1"}, wantErr: true},
		{name: "missing", module: nil, wantErr: true},
		{name: "wrong module", module: &moduleInfo{Path: "example.com/text", Version: "v9.0.0"}, wantErr: true},
		{name: "invalid version", module: &moduleInfo{Path: textModule, Version: "0.39.0"}, wantErr: true},
		{name: "unversioned replacement", module: &moduleInfo{
			Path: textModule, Version: "v0.39.0", Replace: &moduleInfo{Path: textModule},
		}, wantErr: true},
		{name: "different module replacement", module: &moduleInfo{
			Path: textModule, Version: "v0.39.0", Replace: &moduleInfo{Path: "example.com/fork", Version: "v1.0.0"},
		}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := requireMinimumVersion(test.module, textModule, minimumTextModule)
			if (err != nil) != test.wantErr {
				t.Fatalf("requireMinimumVersion() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func loadSelectedModule(t *testing.T, modulePath string) *moduleInfo {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve dependency policy test path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	command := exec.Command("go", "list", "-mod=readonly", "-m", "-json", modulePath)
	command.Dir = repositoryRoot
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("resolve selected module %s: %v: %s", modulePath, err, strings.TrimSpace(string(output)))
	}

	var selected moduleInfo
	if err := json.Unmarshal(output, &selected); err != nil {
		t.Fatalf("decode selected module %s: %v", modulePath, err)
	}
	return &selected
}

func requireMinimumVersion(selected *moduleInfo, modulePath, minimum string) error {
	if selected == nil {
		return fmt.Errorf("required module %s is absent", modulePath)
	}
	if selected.Path != modulePath {
		return fmt.Errorf("resolved module %q, expected %q", selected.Path, modulePath)
	}
	if !semver.IsValid(minimum) {
		return fmt.Errorf("invalid policy minimum %q", minimum)
	}

	effective := selected
	if selected.Replace != nil {
		if selected.Replace.Path != modulePath || selected.Replace.Version == "" {
			return fmt.Errorf("module %s has an unverifiable replacement", modulePath)
		}
		effective = selected.Replace
	}
	if !semver.IsValid(effective.Version) {
		return fmt.Errorf("module %s selected invalid version %q", modulePath, effective.Version)
	}
	if semver.Compare(effective.Version, minimum) < 0 {
		return fmt.Errorf("module %s selected %s, require %s or newer", modulePath, effective.Version, minimum)
	}
	return nil
}
