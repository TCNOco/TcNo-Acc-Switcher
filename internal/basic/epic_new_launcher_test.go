package basic

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/platform"
)

func credentialSessionTestConfig() credentialSessionConfig {
	return credentialSessionConfig{
		idFile: "access-token-prod.dpapi", idPath: "account_id", jsonFiles: []string{"auth.json"},
	}
}

func credentialSessionTestFiles() map[string][]byte {
	return map[string][]byte{"access-token-prod.dpapi": {1}, "auth.json": []byte("{}")}
}

func TestValidateCredentialSessionAccountID(t *testing.T) {
	for _, tc := range []struct {
		name, id, want string
	}{
		{name: "account ID", id: "abc123-456", want: "abc123-456"},
		{name: "trimmed", id: "  abc123-456\t", want: "abc123-456"},
		{name: "Unicode", id: "账户", want: "账户"},
		{name: "maximum length", id: strings.Repeat("a", 128), want: strings.Repeat("a", 128)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateCredentialSessionAccountID(tc.id)
			if err != nil || got != tc.want {
				t.Fatalf("account ID = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	for _, id := range []string{"", "  ", "abc 123", "abc\t123", "abc\x00123", "abc\u2003123", strings.Repeat("a", 129)} {
		if got, err := validateCredentialSessionAccountID(id); err == nil || got != "" {
			t.Errorf("invalid account ID %q returned %q, %v", id, got, err)
		}
	}
}

func TestCredentialSessionFileNameValidation(t *testing.T) {
	for _, name := range []string{"auth.json", "token-prod.dpapi", ".state", "_state", "state__v2", "state_", "玩家.json", "file & name.bin", strings.Repeat("a", 255), strings.Repeat("\U00010400", 127)} {
		if !validCredentialSessionFileName(name) {
			t.Errorf("valid session filename %q rejected", name)
		}
	}
	for _, name := range []string{"", ".", "..", "../outside", `..\outside`, `C:outside`, `C:\outside`, "/outside", "name\x00", "name\n", "name.", "name ", "CON", "con.json", "NUL.bin", "COM1", "LPT9.txt", "CONIN$", "CONOUT$", "name?", "name*", "name|", `name"`, "name<", "name>", strings.Repeat("a", 256), strings.Repeat("\U00010400", 128)} {
		if validCredentialSessionFileName(name) {
			t.Errorf("invalid session filename %q accepted", name)
		}
	}
}

func TestValidateCredentialSessionFiles(t *testing.T) {
	cfg := credentialSessionTestConfig()
	if err := validateCredentialSessionFiles(cfg, credentialSessionTestFiles(), true); err != nil {
		t.Fatalf("valid files rejected: %v", err)
	}
	for _, tc := range []struct {
		name  string
		files map[string][]byte
	}{
		{name: "empty", files: map[string][]byte{}},
		{name: "missing identity", files: map[string][]byte{"auth.json": []byte("{}")}},
		{name: "empty identity", files: map[string][]byte{"access-token-prod.dpapi": {}, "auth.json": []byte("{}")}},
		{name: "missing JSON", files: map[string][]byte{"access-token-prod.dpapi": {1}}},
		{name: "invalid JSON", files: map[string][]byte{"access-token-prod.dpapi": {1}, "auth.json": []byte("{")}},
		{name: "trailing JSON", files: map[string][]byte{"access-token-prod.dpapi": {1}, "auth.json": []byte("{} {}")}},
		{name: "path traversal", files: map[string][]byte{"access-token-prod.dpapi": {1}, "auth.json": []byte("{}"), "../outside": {1}}},
		{name: "duplicate Windows name", files: map[string][]byte{"access-token-prod.dpapi": {1}, "auth.json": []byte("{}"), "AUTH.JSON": []byte("{}")}},
		{name: "reserved name", files: map[string][]byte{"access-token-prod.dpapi": {1}, "auth.json": []byte("{}"), "NUL.dat": {1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateCredentialSessionFiles(cfg, tc.files, true); err == nil {
				t.Fatal("invalid saved session accepted")
			}
		})
	}
	t.Run("BOM", func(t *testing.T) {
		files := credentialSessionTestFiles()
		files["auth.json"] = append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"prod":{}}`)...)
		if err := validateCredentialSessionFiles(cfg, files, true); err != nil {
			t.Fatalf("JSON BOM rejected: %v", err)
		}
	})
}

func TestCredentialSessionFileLimits(t *testing.T) {
	cfg := credentialSessionTestConfig()
	t.Run("file size", func(t *testing.T) {
		files := credentialSessionTestFiles()
		files["large.bin"] = make([]byte, credentialSessionMaxFileSize)
		if err := validateCredentialSessionFiles(cfg, files, true); err != nil {
			t.Fatalf("maximum file size rejected: %v", err)
		}
		files["large.bin"] = append(files["large.bin"], 0)
		if err := validateCredentialSessionFiles(cfg, files, true); err == nil {
			t.Fatal("oversized file accepted")
		}
	})
	t.Run("total size", func(t *testing.T) {
		files := credentialSessionTestFiles()
		for i := 0; i < 3; i++ {
			files[fmt.Sprintf("part-%d.bin", i)] = make([]byte, credentialSessionMaxFileSize)
		}
		files["last.bin"] = make([]byte, credentialSessionMaxTotalSize-3*credentialSessionMaxFileSize-3)
		if err := validateCredentialSessionFiles(cfg, files, true); err != nil {
			t.Fatalf("maximum total size rejected: %v", err)
		}
		files["last.bin"] = append(files["last.bin"], 0)
		if err := validateCredentialSessionFiles(cfg, files, true); err == nil {
			t.Fatal("oversized total accepted")
		}
	})
	t.Run("file count", func(t *testing.T) {
		files := credentialSessionTestFiles()
		for len(files) < credentialSessionMaxFiles {
			files[fmt.Sprintf("part-%d.bin", len(files))] = nil
		}
		if err := validateCredentialSessionFiles(cfg, files, true); err != nil {
			t.Fatalf("maximum file count rejected: %v", err)
		}
		files["extra.bin"] = nil
		if err := validateCredentialSessionFiles(cfg, files, true); err == nil {
			t.Fatal("excessive file count accepted")
		}
	})
}

func TestCredentialSessionRecoveryAllowsIncompleteFiles(t *testing.T) {
	cfg := credentialSessionTestConfig()
	for _, files := range []map[string][]byte{nil, {}, {"auth.json": []byte("unfinished JSON")}, {"other.bin": {1}}} {
		if err := validateCredentialSessionFiles(cfg, files, false); err != nil {
			t.Fatalf("original session state rejected for recovery: %v", err)
		}
	}
	if err := validateCredentialSessionFiles(cfg, map[string][]byte{"../outside": {1}}, false); err == nil {
		t.Fatal("recovery bypassed filename validation")
	}
}

func TestCredentialSessionFlowUsesDescriptor(t *testing.T) {
	fc := FlowContext{PlatformKey: "Example", Folder: "unrelated-installation"}
	if isCredentialSessionFlow(fc) {
		t.Fatal("flow enabled without descriptor capability")
	}
	fc.Descriptor.Extras.CredentialSession = &platform.CredentialSessionDescriptor{}
	if !isCredentialSessionFlow(fc) {
		t.Fatal("descriptor capability did not select credential flow")
	}
}

func credentialSessionTestFlow() FlowContext {
	directory := filepath.Join(filepath.VolumeName(os.TempDir())+string(filepath.Separator), "tcno-session-policy", "auth")
	return FlowContext{
		PlatformKey: "Example",
		Descriptor: platform.Descriptor{
			LoginFiles:     map[string]string{directory: "auth"},
			UniqueIdMethod: "DPAPI_JSON_SELECT",
			UniqueIdFile:   "JSON_SELECT::" + filepath.Join(directory, "access-token-prod.dpapi") + "::account_id",
			Extras: platform.DescriptorExtras{CredentialSession: &platform.CredentialSessionDescriptor{
				Target: "credentials.example.test", SnapshotFile: "saved-session.dpapi", JSONFiles: []string{"auth.json"}, RecoveryCopies: 3,
			}},
		},
	}
}

func TestCredentialSessionConfiguration(t *testing.T) {
	fc := credentialSessionTestFlow()
	cfg, err := credentialSessionConfiguration(fc)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.idFile != "access-token-prod.dpapi" || cfg.idPath != "account_id" || cfg.snapshotFile != "saved-session.dpapi" || cfg.target != "credentials.example.test" || cfg.recoveryCopies != 3 || cfg.platformKey != "Example" {
		t.Fatalf("configuration = %#v", cfg)
	}
	fc.Descriptor.Extras.CredentialSession.JSONFiles[0] = "changed.json"
	if cfg.jsonFiles[0] != "auth.json" {
		t.Fatal("configuration aliases the descriptor JSON file slice")
	}
}

func TestCredentialSessionConfigurationRejectsInvalidDescriptors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*FlowContext)
	}{
		{name: "missing capability", change: func(fc *FlowContext) { fc.Descriptor.Extras.CredentialSession = nil }},
		{name: "empty target", change: func(fc *FlowContext) { fc.Descriptor.Extras.CredentialSession.Target = " " }},
		{name: "no login directory", change: func(fc *FlowContext) { fc.Descriptor.LoginFiles = nil }},
		{name: "multiple directories", change: func(fc *FlowContext) { fc.Descriptor.LoginFiles["other"] = "other" }},
		{name: "registry login source", change: func(fc *FlowContext) { fc.Descriptor.LoginFiles = map[string]string{"REG:HKCU\\Example:Token": "auth"} }},
		{name: "glob directory", change: func(fc *FlowContext) { fc.Descriptor.LoginFiles = map[string]string{"session*": "auth"} }},
		{name: "wrong identity method", change: func(fc *FlowContext) { fc.Descriptor.UniqueIdMethod = "REGKEY" }},
		{name: "malformed selector", change: func(fc *FlowContext) { fc.Descriptor.UniqueIdFile = "JSON_SELECT::file" }},
		{name: "identity outside directory", change: func(fc *FlowContext) { fc.Descriptor.UniqueIdFile = "JSON_SELECT::other.dpapi::account_id" }},
		{name: "unsafe snapshot filename", change: func(fc *FlowContext) { fc.Descriptor.Extras.CredentialSession.SnapshotFile = "../snapshot" }},
		{name: "unsafe required JSON filename", change: func(fc *FlowContext) { fc.Descriptor.Extras.CredentialSession.JSONFiles = []string{"NUL.json"} }},
		{name: "excessive recovery copies", change: func(fc *FlowContext) { fc.Descriptor.Extras.CredentialSession.RecoveryCopies = 21 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fc := credentialSessionTestFlow()
			tc.change(&fc)
			if _, err := credentialSessionConfiguration(fc); err == nil {
				t.Fatal("invalid descriptor accepted")
			}
		})
	}
}
