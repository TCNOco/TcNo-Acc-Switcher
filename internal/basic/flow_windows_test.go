//go:build windows

package basic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/winutil"
)

const flowTestRegistryRoot = `HKCU\Software\TcNo-Acc-Switcher\FlowTest`

func flowTestRegKey(sub string) string {
	return flowTestRegistryRoot + `\` + sub
}

func setTestRegValue(t *testing.T, keyPath, valueName, data string) {
	t.Helper()
	if err := winutil.RegistryWrite(flowTestRegKey(keyPath)+":"+valueName, data); err != nil {
		t.Fatalf("RegistryWrite %s:%s: %v", keyPath, valueName, err)
	}
	t.Cleanup(func() {
		_ = winutil.RegistryDelete(flowTestRegKey(keyPath) + ":" + valueName)
	})
}

func TestFlow_RegistryInLoginFiles(t *testing.T) {
	env := newFlowTestEnv(t)

	regKeyPath := `TestValues`
	regFullKey := flowTestRegKey(regKeyPath)
	regFullValue := regFullKey + ":StringVal"
	setTestRegValue(t, regKeyPath, "StringVal", "hello-from-reg")

	fc := env.flowContext()
	fc.Descriptor.LoginFiles = map[string]string{
		"REG:" + regFullValue: "reg.json",
	}
	fc.Descriptor.AllFilesRequired = false

	// Also add a file so we can verify the save succeeded
	mustWrite(t, filepath.Join(env.instDir, "%Platform_Folder%\\config.cfg"), "file-data")

	fc.Descriptor.LoginFiles["%Platform_Folder%\\config.cfg"] = "Saved/config.cfg"

	if err := saveCurrentAfterKill(FlowDeps{}, "RegAccount", fc); err != nil {
		t.Fatalf("save: %v", err)
	}

	cacheDir := env.cacheDir("RegAccount")
	regJSONPath := filepath.Join(cacheDir, "reg.json")
	if !pathExists(t, regJSONPath) {
		t.Fatal("reg.json was not created in cache directory")
	}

	data, err := os.ReadFile(regJSONPath)
	if err != nil {
		t.Fatalf("read reg.json: %v", err)
	}
	var regDump map[string]regDumpEntry
	if err := json.Unmarshal(data, &regDump); err != nil {
		t.Fatalf("parse reg.json: %v", err)
	}

	found := false
	for k, v := range regDump {
		t.Logf("reg.json entry: %s -> v=%q t=%d", k, v.V, v.T)
		if k == "REG:"+regFullValue || k == regFullValue {
			if v.V != "hello-from-reg" {
				t.Errorf("reg.json has %q, want hello-from-reg", v.V)
			}
			found = true
		}
	}
	if !found {
		t.Errorf("reg.json does not contain REG entry for %s", regFullValue)
	}

	t.Cleanup(func() { _ = winutil.RegistryDelete(regFullValue) })
	if err := winutil.RegistryDelete(regFullValue); err != nil {
		t.Fatalf("delete reg value: %v", err)
	}

	if err := Login(FlowDeps{}, fc, "RegAccount"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	restored, _, err := winutil.RegistryRead(regFullKey + ":StringVal")
	if err != nil {
		t.Fatalf("RegistryRead after Login: %v", err)
	}
	restoredStr, ok := restored.(string)
	if !ok {
		t.Fatalf("registry value is not a string: %T", restored)
	}
	if restoredStr != "hello-from-reg" {
		t.Errorf("restored registry value = %q, want hello-from-reg", restoredStr)
	}
}

func TestFlow_RegistryAllValuesInKey(t *testing.T) {
	env := newFlowTestEnv(t)

	keyPath := `AllValues`
	setTestRegValue(t, keyPath, "Val1", "first")
	setTestRegValue(t, keyPath, "Val2", "second")
	t.Cleanup(func() {
		_ = winutil.RegistryDelete(flowTestRegKey(keyPath) + ":Val1")
		_ = winutil.RegistryDelete(flowTestRegKey(keyPath) + ":Val2")
	})

	fc := env.flowContext()
	fc.Descriptor.LoginFiles = map[string]string{
		"REG:" + flowTestRegKey(keyPath) + ":*": "reg.json",
	}
	fc.Descriptor.AllFilesRequired = false

	mustWrite(t, filepath.Join(env.instDir, "%Platform_Folder%\\config.cfg"), "x")
	fc.Descriptor.LoginFiles["%Platform_Folder%\\config.cfg"] = "Saved/config.cfg"

	if err := saveCurrentAfterKill(FlowDeps{}, "RegAllAcct", fc); err != nil {
		t.Fatalf("save: %v", err)
	}

	regJSONPath := filepath.Join(env.cacheDir("RegAllAcct"), "reg.json")
	data, _ := os.ReadFile(regJSONPath)
	var regDump map[string]regDumpEntry
	json.Unmarshal(data, &regDump)

	for k, v := range regDump {
		if len(v.Values) >= 2 {
			t.Logf("REG:* bundle found: %s with %d values", k, len(v.Values))
		}
	}

	_ = winutil.RegistryDelete(flowTestRegKey(keyPath) + ":Val1")
	_ = winutil.RegistryDelete(flowTestRegKey(keyPath) + ":Val2")

	if err := Login(FlowDeps{}, fc, "RegAllAcct"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	v1, _, _ := winutil.RegistryRead(flowTestRegKey(keyPath) + ":Val1")
	if v1 != "first" {
		t.Errorf("Val1 = %v, want first", v1)
	}

	v2, _, _ := winutil.RegistryRead(flowTestRegKey(keyPath) + ":Val2")
	if v2 != "second" {
		t.Errorf("Val2 = %v, want second", v2)
	}
}

func TestFlow_RegistryGlobValues(t *testing.T) {
	env := newFlowTestEnv(t)

	keyPath := `GlobVals`
	setTestRegValue(t, keyPath, "Settings_json", `{"lang":"en"}`)
	setTestRegValue(t, keyPath, "Settings_bak", `{"lang":"de"}`)
	setTestRegValue(t, keyPath, "OtherVal", "keep-me")
	t.Cleanup(func() {
		_ = winutil.RegistryDelete(flowTestRegKey(keyPath) + ":Settings_json")
		_ = winutil.RegistryDelete(flowTestRegKey(keyPath) + ":Settings_bak")
		_ = winutil.RegistryDelete(flowTestRegKey(keyPath) + ":OtherVal")
	})

	fc := env.flowContext()
	fc.Descriptor.LoginFiles = map[string]string{
		"REG:" + flowTestRegKey(keyPath) + ":Settings_*": "reg.json",
	}
	fc.Descriptor.AllFilesRequired = false

	mustWrite(t, filepath.Join(env.instDir, "%Platform_Folder%\\config.cfg"), "x")
	fc.Descriptor.LoginFiles["%Platform_Folder%\\config.cfg"] = "Saved/config.cfg"

	if err := saveCurrentAfterKill(FlowDeps{}, "RegGlobAcct", fc); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Verify only Settings_* values were captured, not OtherVal
	regJSONPath := filepath.Join(env.cacheDir("RegGlobAcct"), "reg.json")
	data, _ := os.ReadFile(regJSONPath)
	var regDump map[string]regDumpEntry
	json.Unmarshal(data, &regDump)

	for k, v := range regDump {
		if len(v.Values) > 0 {
			if _, hasOther := v.Values["OtherVal"]; hasOther {
				t.Errorf("OtherVal should not be in glob capture for Settings_*")
			}
			if _, hasJSON := v.Values["Settings_json"]; !hasJSON {
				t.Errorf("Settings_json should be in glob capture")
			}
			if _, hasBak := v.Values["Settings_bak"]; !hasBak {
				t.Errorf("Settings_bak should be in glob capture")
			}
			t.Logf("Glob bundle: %s with %d values", k, len(v.Values))
		}
	}

	_ = winutil.RegistryDelete(flowTestRegKey(keyPath) + ":Settings_json")
	_ = winutil.RegistryDelete(flowTestRegKey(keyPath) + ":Settings_bak")

	if err := Login(FlowDeps{}, fc, "RegGlobAcct"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	sj, _, _ := winutil.RegistryRead(flowTestRegKey(keyPath) + ":Settings_json")
	if sj != `{"lang":"en"}` {
		t.Errorf("Settings_json = %v", sj)
	}
	sb, _, _ := winutil.RegistryRead(flowTestRegKey(keyPath) + ":Settings_bak")
	if sb != `{"lang":"de"}` {
		t.Errorf("Settings_bak = %v", sb)
	}
}

func TestFlow_RegistryOnClear(t *testing.T) {
	env := newFlowTestEnv(t)

	keyPath := `ClearTest`
	regFullValue := flowTestRegKey(keyPath) + ":ToBeCleared"
	if err := winutil.RegistryWrite(regFullValue, "data-to-delete"); err != nil {
		t.Fatalf("RegistryWrite: %v", err)
	}
	t.Cleanup(func() { _ = winutil.RegistryDelete(regFullValue) })

	fc := env.flowContext()
	fc.Descriptor.PathListToClear = []string{
		"REG:" + regFullValue,
	}
	fc.Descriptor.RegDeleteOnClear = true

	if err := ClearCurrentLogin(FlowDeps{}, fc); err != nil {
		t.Fatalf("ClearCurrentLogin: %v", err)
	}

	_, _, err := winutil.RegistryRead(regFullValue)
	if err == nil {
		t.Error("registry value should have been deleted on clear")
	}
}

func TestFlow_RegistryOnClear_Empty(t *testing.T) {
	env := newFlowTestEnv(t)

	keyPath := `ClearEmptyTest`
	regFullValue := flowTestRegKey(keyPath) + ":ToBeEmptied"
	if err := winutil.RegistryWrite(regFullValue, "original-data"); err != nil {
		t.Fatalf("RegistryWrite: %v", err)
	}
	t.Cleanup(func() { _ = winutil.RegistryDelete(regFullValue) })

	fc := env.flowContext()
	fc.Descriptor.PathListToClear = []string{
		"REG:" + regFullValue,
	}
	fc.Descriptor.RegDeleteOnClear = false

	if err := ClearCurrentLogin(FlowDeps{}, fc); err != nil {
		t.Fatalf("ClearCurrentLogin: %v", err)
	}

	// RegistryWrite("") calls deleteRegistryValueIfPresent — the value is removed.
	// This matches RegDeleteOnClear=false behaviour: the value is cleared.
	_, _, err := winutil.RegistryRead(regFullValue)
	if err == nil {
		t.Error("registry value should have been removed (empty string write = delete on Windows)")
	}
}
