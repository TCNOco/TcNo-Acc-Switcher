package basic

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/platform"

	"go.yaml.in/yaml/v3"
)

func TestPatchConfigValueClosePreference(t *testing.T) {
	for _, format := range []string{"yaml", "json"} {
		t.Run(format, func(t *testing.T) {
			data := []byte("# application settings\nuser:\n  closebehavior: CloseBehavior_Ask # close choice\n  offline: false\nuntouched: 42\n")
			if format == "json" {
				data = []byte(`{"user":{"closebehavior":"CloseBehavior_Ask","offline":false},"untouched":42}`)
			}
			patched, changed, err := patchConfigValue(data, format, "user.closebehavior", "CloseBehavior_Close")
			if err != nil || !changed {
				t.Fatalf("changed=%t err=%v", changed, err)
			}
			var got map[string]any
			if format == "json" {
				err = json.Unmarshal(patched, &got)
			} else {
				err = yaml.Unmarshal(patched, &got)
				if !strings.Contains(string(patched), "# application settings") || !strings.Contains(string(patched), "# close choice") {
					t.Fatal("YAML comments were lost")
				}
			}
			if err != nil || got["user"].(map[string]any)["closebehavior"] != "CloseBehavior_Close" || got["user"].(map[string]any)["offline"] != false || got["untouched"] == nil {
				t.Fatalf("other settings changed: %v, %v", got, err)
			}
			again, changed, err := patchConfigValue(patched, format, "user.closebehavior", "CloseBehavior_Close")
			if err != nil || changed || !bytes.Equal(again, patched) {
				t.Fatalf("already-configured file should not be rewritten: changed=%t err=%v", changed, err)
			}
		})
	}
}

func TestPatchConfigValueGenericScalarTypesAndMissingKeys(t *testing.T) {
	for _, format := range []string{"yaml", "json"} {
		data := []byte("user: {}\n")
		if format == "json" {
			data = []byte(`{"user":{}}`)
		}
		for _, value := range []any{true, 2.5, "Close", nil} {
			patched, changed, err := patchConfigValue(data, format, "user.preference", value)
			if err != nil || !changed {
				t.Fatalf("%s value %v: changed=%t err=%v", format, value, changed, err)
			}
			var got map[string]any
			if format == "json" {
				err = json.Unmarshal(patched, &got)
			} else {
				err = yaml.Unmarshal(patched, &got)
			}
			if err != nil || !reflect.DeepEqual(got["user"].(map[string]any)["preference"], value) {
				t.Fatalf("%s value changed type or content: %v, %v", format, got, err)
			}
		}
	}
}

func TestPatchConfigValueRejectsInvalidOrBroadChanges(t *testing.T) {
	for _, tc := range []struct {
		format, data, path string
		value              any
	}{
		{"ini", "user=ask", "user", "close"},
		{"json", "broken", "user.close", "close"},
		{"yaml", "user: [broken", "user.close", "close"},
		{"yaml", "user: {}\n---\nother: 1", "user.close", "close"},
		{"yaml", "user: ask", "user.close", "close"},
		{"json", `{"user":"ask"}`, "user.close", "close"},
		{"json", `{"user":{}}`, "user", "close"},
		{"yaml", "user: {}", "user", "close"},
		{"yaml", "user: {}", "user..close", "close"},
		{"yaml", "user: {}", "user.close", map[string]any{"other": true}},
	} {
		if _, _, err := patchConfigValue([]byte(tc.data), tc.format, tc.path, tc.value); err == nil {
			t.Errorf("accepted invalid override: %+v", tc)
		}
	}
}

func TestApplyConfigOverridesLeavesConfiguredFileUntouched(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("testdata", "config_overrides", "settings.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fc := FlowContext{Descriptor: platform.Descriptor{LoginFiles: map[string]string{path: "settings.yaml"}}}
	fc.Descriptor.Extras.ConfigOverrides = []platform.ConfigOverride{{File: path, Format: "yaml", Path: "user.closebehavior", Value: "CloseBehavior_Close"}}
	if err := applyConfigOverrides(fc); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatal("unchanged config was modified")
	}
	fc.Descriptor.LoginFiles = nil
	if err := applyConfigOverrides(fc); err == nil {
		t.Fatal("accepted a file outside the launcher's login files")
	}
}

func TestUbisoftCatalogClosesAndSavesCurrentData(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "Platforms.json"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := platform.ParseDescriptor(raw, "Ubisoft")
	if err != nil {
		t.Fatal(err)
	}
	want := platform.ConfigOverride{File: `%LocalAppData%\Ubisoft Game Launcher\settings.yaml`, Format: "yaml", Path: "user.closebehavior", Value: "CloseBehavior_Close"}
	if len(d.Extras.ConfigOverrides) != 1 || !reflect.DeepEqual(d.Extras.ConfigOverrides[0], want) {
		t.Fatalf("Ubisoft close override = %+v", d.Extras.ConfigOverrides)
	}
	if d.LoginFiles[`%LocalAppData%\Ubisoft Game Launcher\user.vdat`] != "user.vdat" || d.LoginFiles[want.File] != "settings.yaml" {
		t.Fatal("Ubisoft must save its modern user data and owned settings file")
	}
}
