package basic

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"TcNo-Acc-Switcher/internal/winutil"
)

func TestCredentialSessionChangeOrderingAndErrors(t *testing.T) {
	resetErr := errors.New("browser state could not be moved")
	installErr := errors.New("credential write failed")
	rollbackErr := errors.New("previous session could not be restored")
	for _, tc := range []struct {
		name        string
		resetErr    error
		installErr  error
		rollbackErr error
		wantCalls   []string
		wantErrors  []error
	}{
		{name: "complete", wantCalls: []string{"reset", "install"}},
		{name: "reset failure", resetErr: resetErr, wantCalls: []string{"reset", "rollback"}, wantErrors: []error{resetErr}},
		{name: "reset and rollback failure", resetErr: resetErr, rollbackErr: rollbackErr, wantCalls: []string{"reset", "rollback"}, wantErrors: []error{resetErr, rollbackErr}},
		{name: "install failure", installErr: installErr, wantCalls: []string{"reset", "install", "rollback"}, wantErrors: []error{installErr}},
		{name: "install and rollback failure", installErr: installErr, rollbackErr: rollbackErr, wantCalls: []string{"reset", "install", "rollback"}, wantErrors: []error{installErr, rollbackErr}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			err := runSessionChange(func() error {
				calls = append(calls, "reset")
				return tc.resetErr
			}, func() error {
				calls = append(calls, "install")
				return tc.installErr
			}, func() error {
				calls = append(calls, "rollback")
				return tc.rollbackErr
			})
			if !reflect.DeepEqual(calls, tc.wantCalls) {
				t.Fatalf("operations = %v, want %v", calls, tc.wantCalls)
			}
			if len(tc.wantErrors) == 0 && err != nil {
				t.Fatalf("completed replacement returned %v", err)
			}
			for _, want := range tc.wantErrors {
				if !errors.Is(err, want) {
					t.Errorf("error %v lost failure %v", err, want)
				}
			}
		})
	}
}

func TestCredentialSessionChangeRestoresPartialReplacement(t *testing.T) {
	type sessionState struct {
		files, credential, browser string
	}
	previous := sessionState{"old-files", "old-credential", "old-browser"}
	current := previous
	wantErr := errors.New("new credential rejected")
	rollbacks := 0
	err := runSessionChange(func() error {
		current.browser = ""
		return nil
	}, func() error {
		current.files = "new-files"
		return wantErr
	}, func() error {
		rollbacks++
		current = previous
		return nil
	})
	if !errors.Is(err, wantErr) || current != previous || rollbacks != 1 {
		t.Fatalf("error=%v state=%#v rollbacks=%d", err, current, rollbacks)
	}
}

func TestCredentialSessionChangeRestoresPartialReset(t *testing.T) {
	current := map[string]string{"browser-a": "old-a", "browser-b": "old-b"}
	wantErr := errors.New("second browser directory is locked")
	installs, rollbacks := 0, 0
	err := runSessionChange(func() error {
		delete(current, "browser-a")
		return wantErr
	}, func() error {
		installs++
		return nil
	}, func() error {
		rollbacks++
		current["browser-a"] = "old-a"
		return nil
	})
	if !errors.Is(err, wantErr) || installs != 0 || rollbacks != 1 || current["browser-a"] != "old-a" || current["browser-b"] != "old-b" {
		t.Fatalf("error=%v state=%v installs=%d rollbacks=%d", err, current, installs, rollbacks)
	}
}

func TestCredentialSessionCredentialValidation(t *testing.T) {
	const target = "credentials.example.test"
	valid := func() *winutil.Credential {
		return &winutil.Credential{TargetName: target, Persist: 2, Blob: []byte{1, 2, 3}}
	}
	if err := validateSessionCredential(target, nil); err != nil {
		t.Fatalf("clear-session credential rejected: %v", err)
	}
	for _, persist := range []uint32{1, 2, 3} {
		credential := valid()
		credential.Persist = persist
		credential.Blob = make([]byte, 2560)
		if err := validateSessionCredential(target, credential); err != nil {
			t.Fatalf("valid credential with persistence %d rejected: %v", persist, err)
		}
	}
	alias := valid()
	alias.TargetName = "LegacyGeneric:target=" + target
	if err := validateSessionCredential(target, alias); err != nil {
		t.Fatalf("equivalent target spelling rejected: %v", err)
	}
	for _, tc := range []struct {
		name   string
		change func(*winutil.Credential)
	}{
		{name: "redirected target", change: func(c *winutil.Credential) { c.TargetName = "credentials.other.test" }},
		{name: "empty target", change: func(c *winutil.Credential) { c.TargetName = "" }},
		{name: "empty blob", change: func(c *winutil.Credential) { c.Blob = nil }},
		{name: "oversized blob", change: func(c *winutil.Credential) { c.Blob = make([]byte, 2561) }},
		{name: "missing persistence", change: func(c *winutil.Credential) { c.Persist = 0 }},
		{name: "unsupported persistence", change: func(c *winutil.Credential) { c.Persist = 4 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			credential := valid()
			tc.change(credential)
			if err := validateSessionCredential(target, credential); err == nil {
				t.Fatal("invalid credential accepted")
			}
		})
	}
}

func TestCredentialSessionSnapshotVersionOneCompatibility(t *testing.T) {
	data := []byte(`{"version":1,"uniqueId":"abc123-456","credential":{"targetName":"credentials.example.test","userName":"credentials","targetAlias":"alias","comment":"comment","persist":2,"blob":"AQID"},"authFiles":{"access-token-prod.dpapi":"AQ==","auth.json":"e30="}}`)
	var snapshot credentialSessionSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	cfg := credentialSessionTestConfig()
	cfg.target = "credentials.example.test"
	reads := 0
	err := validateCredentialSessionSnapshot(cfg, &snapshot, "ABC123-456", func(selected credentialSessionConfig, files map[string][]byte) (string, error) {
		reads++
		if selected.idFile != cfg.idFile || selected.idPath != cfg.idPath || !bytes.Equal(files[cfg.idFile], []byte{1}) {
			t.Fatalf("incorrect identity reader inputs: %#v, %v", selected, files)
		}
		return "abc123-456", nil
	})
	if err != nil || reads != 1 {
		t.Fatalf("version-one validation error=%v identity reads=%d", err, reads)
	}
	if snapshot.Credential.UserName != "credentials" || snapshot.Credential.TargetAlias != "alias" || snapshot.Credential.Comment != "comment" || !bytes.Equal(snapshot.Credential.Blob, []byte{1, 2, 3}) {
		t.Fatalf("existing saved credential changed during preflight: %#v", snapshot.Credential)
	}
}

func credentialSessionTestSnapshot() *credentialSessionSnapshot {
	return &credentialSessionSnapshot{
		Version: credentialSessionVersion, UniqueID: "account-one",
		Credential: credentialSessionCredential{TargetName: "credentials.example.test", Persist: 2, Blob: []byte{1}},
		AuthFiles:  credentialSessionTestFiles(),
	}
}

func TestCredentialSessionSnapshotRejectsBeforeIdentityRead(t *testing.T) {
	for _, tc := range []struct {
		name       string
		change     func(*credentialSessionSnapshot)
		expectedID string
		nilValue   bool
	}{
		{name: "nil snapshot", nilValue: true, expectedID: "account-one"},
		{name: "no selected account", expectedID: ""},
		{name: "unsupported version", expectedID: "account-one", change: func(s *credentialSessionSnapshot) { s.Version++ }},
		{name: "wrong profile", expectedID: "account-two"},
		{name: "no stored identity", expectedID: "account-one", change: func(s *credentialSessionSnapshot) { s.UniqueID = "" }},
		{name: "different credential target", expectedID: "account-one", change: func(s *credentialSessionSnapshot) { s.Credential.TargetName = "credentials.other.test" }},
		{name: "missing credential blob", expectedID: "account-one", change: func(s *credentialSessionSnapshot) { s.Credential.Blob = nil }},
		{name: "oversized credential blob", expectedID: "account-one", change: func(s *credentialSessionSnapshot) { s.Credential.Blob = make([]byte, 2561) }},
		{name: "invalid persistence", expectedID: "account-one", change: func(s *credentialSessionSnapshot) { s.Credential.Persist = 0 }},
		{name: "missing identity file", expectedID: "account-one", change: func(s *credentialSessionSnapshot) { delete(s.AuthFiles, "access-token-prod.dpapi") }},
		{name: "invalid auth JSON", expectedID: "account-one", change: func(s *credentialSessionSnapshot) { s.AuthFiles["auth.json"] = []byte("{") }},
		{name: "traversing filename", expectedID: "account-one", change: func(s *credentialSessionSnapshot) { s.AuthFiles["../outside"] = []byte{1} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := credentialSessionTestConfig()
			cfg.target = "credentials.example.test"
			snapshot := credentialSessionTestSnapshot()
			if tc.nilValue {
				snapshot = nil
			} else if tc.change != nil {
				tc.change(snapshot)
			}
			reads := 0
			err := validateCredentialSessionSnapshot(cfg, snapshot, tc.expectedID, func(credentialSessionConfig, map[string][]byte) (string, error) {
				reads++
				return "account-one", nil
			})
			if err == nil || reads != 0 {
				t.Fatalf("invalid snapshot error=%v identity reads=%d", err, reads)
			}
		})
	}
}

func TestCredentialSessionSnapshotRejectsTokenMismatchOrReadFailure(t *testing.T) {
	readErr := errors.New("token cannot be decrypted")
	for _, tc := range []struct {
		name string
		id   string
		err  error
	}{
		{name: "different token account", id: "account-two"},
		{name: "no token account", id: ""},
		{name: "identity read failed", err: readErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := credentialSessionTestConfig()
			cfg.target = "credentials.example.test"
			reads := 0
			err := validateCredentialSessionSnapshot(cfg, credentialSessionTestSnapshot(), "account-one", func(credentialSessionConfig, map[string][]byte) (string, error) {
				reads++
				return tc.id, tc.err
			})
			if err == nil || reads != 1 || (tc.err != nil && !errors.Is(err, tc.err)) {
				t.Fatalf("validation error=%v identity reads=%d", err, reads)
			}
		})
	}
}

func TestCredentialSessionCredentialAliasesRemainDistinct(t *testing.T) {
	const target = "credentials.example.test"
	credentials := []winutil.Credential{
		{TargetName: target, Persist: 2, Blob: []byte{1}},
		{TargetName: "LegacyGeneric:target=" + target, Persist: 3, Blob: []byte{2}},
	}
	if err := validateSessionCredentials(target, credentials); err != nil {
		t.Fatalf("two exact target aliases rejected: %v", err)
	}
	credentials[1].TargetName = "CREDENTIALS.EXAMPLE.TEST"
	if err := validateSessionCredentials(target, credentials); err == nil {
		t.Fatal("case-insensitive duplicate credential targets accepted")
	}
	credentials = append(credentials, winutil.Credential{TargetName: target, Persist: 2, Blob: []byte{3}})
	if err := validateSessionCredentials(target, credentials); err == nil {
		t.Fatal("excessive credential targets accepted")
	}
}

func TestCredentialSessionCredentialCopiesMetadataAndBuffers(t *testing.T) {
	original := winutil.Credential{
		TargetName: "credentials.example.test", UserName: "test-user", TargetAlias: "test-alias", Comment: "test-comment", Persist: 3, Blob: []byte{1, 2, 3},
	}
	snapshot := snapshotCredential(original)
	clear(original.Blob)
	if !bytes.Equal(snapshot.Blob, []byte{1, 2, 3}) {
		t.Fatal("snapshot shares the original credential buffer")
	}
	restored := snapshot.native()
	if restored.TargetName != original.TargetName || restored.UserName != original.UserName || restored.TargetAlias != original.TargetAlias || restored.Comment != original.Comment || restored.Persist != original.Persist {
		t.Fatalf("credential metadata changed: %#v", restored)
	}
	restored.Blob[0] = 9
	if snapshot.Blob[0] != 1 {
		t.Fatal("restored credential shares the encrypted snapshot's decoded buffer")
	}
}

func TestCredentialSessionSnapshotAndRecoveryWipeBuffers(t *testing.T) {
	blob, file := []byte{1, 2, 3}, []byte{4, 5, 6}
	wipeCredentialSessionSnapshot(&credentialSessionSnapshot{
		Credential: credentialSessionCredential{Blob: blob}, AuthFiles: map[string][]byte{"state.bin": file},
	})
	if !bytes.Equal(blob, []byte{0, 0, 0}) || !bytes.Equal(file, []byte{0, 0, 0}) {
		t.Fatal("snapshot buffers were not cleared")
	}
	blob, file = []byte{1, 2, 3}, []byte{4, 5, 6}
	wipeCredentialSessionRecovery(&credentialSessionRecovery{
		Credential: &winutil.Credential{Blob: blob}, AuthFiles: map[string][]byte{"state.bin": file},
	})
	if !bytes.Equal(blob, []byte{0, 0, 0}) || !bytes.Equal(file, []byte{0, 0, 0}) {
		t.Fatal("recovery buffers were not cleared")
	}
	first, second := []byte{1, 2}, []byte{3, 4}
	wipeCredentialSessionRecovery(&credentialSessionRecovery{Credentials: []winutil.Credential{{Blob: first}, {Blob: second}}})
	if !bytes.Equal(first, []byte{0, 0}) || !bytes.Equal(second, []byte{0, 0}) {
		t.Fatal("recovery alias credential buffers were not cleared")
	}
	wipeCredentialSessionRecovery(&credentialSessionRecovery{})
}
