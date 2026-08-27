package steamguard

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/steamguard/enrollmentapi"
	"TcNo-Acc-Switcher/internal/steamguard/enrollmentflow"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
	"TcNo-Acc-Switcher/internal/steamguard/registry"
	"TcNo-Acc-Switcher/internal/steamguard/sessionrefresh"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

const authServiceVaultPassword = "correct horse battery staple"

// writingEnrollmentManager mirrors the real manager where it matters for this
// test: acknowledgment and finalization both rewrite the account record, which
// rotates the vault generation and invalidates live capabilities.
type writingEnrollmentManager struct {
	service        *Service
	accountID      string
	revealCode     string
	finalizeStatus enrollmentflow.Status
}

func (m *writingEnrollmentManager) rewriteRecord() error {
	records, err := m.service.vault.ListRecords()
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return errors.New("no records to rewrite")
	}
	raw, err := m.service.vault.GetRecord(records[0].ID)
	if err != nil {
		return err
	}
	defer wipe(raw)
	_, err = m.service.vault.PutRecord(m.accountID, raw)
	return err
}

func (m *writingEnrollmentManager) Start(context.Context, enrollmentflow.StartRequest) (enrollmentflow.Status, error) {
	return enrollmentflow.Status{}, errors.New("not used")
}

func (m *writingEnrollmentManager) Resume(uint64) (enrollmentflow.Status, error) {
	return enrollmentflow.Status{
		State: enrollmentapi.StateAwaitingSMS, Confirmation: enrollmentapi.ConfirmationSMS,
		Pending: true, Resumed: true, RevocationViewAvailable: true,
	}, nil
}

func (m *writingEnrollmentManager) RevealRevocationCode(uint64) (enrollmentflow.RevocationView, error) {
	return enrollmentflow.RevocationView{Code: m.revealCode}, nil
}

func (m *writingEnrollmentManager) AcknowledgeRevocationCode(uint64, []byte) (enrollmentflow.Status, error) {
	if err := m.rewriteRecord(); err != nil {
		return enrollmentflow.Status{}, err
	}
	return enrollmentflow.Status{
		State: enrollmentapi.StateAwaitingSMS, Confirmation: enrollmentapi.ConfirmationSMS, Pending: true,
	}, nil
}

func (m *writingEnrollmentManager) Finalize(context.Context, enrollmentflow.FinalizeRequest) (enrollmentflow.Status, error) {
	if err := m.rewriteRecord(); err != nil {
		return enrollmentflow.Status{}, err
	}
	return m.finalizeStatus, nil
}

func (m *writingEnrollmentManager) Cancel(uint64) error { return nil }

// TestVaultWritingModalOperationsSignalCapabilityRefresh locks the contract the
// modal depends on: any bound operation that writes to the vault rotates the
// generation, so its result must tell the UI to re-acquire its capability.
func TestVaultWritingModalOperationsSignalCapabilityRefresh(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T, service *Service, accountID string, grant SensitiveViewGrant) bool
	}{
		{
			name: "LoginAgain",
			run: func(t *testing.T, service *Service, accountID string, grant SensitiveViewGrant) bool {
				client := &authServiceTokenClient{result: protocol.TokenResult{
					State:        protocol.AuthResultTokenIssued,
					AccessToken:  "refreshed-access-token",
					RefreshToken: "refreshed-refresh-token",
				}}
				service.newSessionRefresher = func(v *vault.Vault) steamSessionRefresher {
					return sessionrefresh.New(client, v)
				}
				result, err := service.LoginAgain(accountID, grant.Capability)
				if err != nil || result.State != "refreshed" {
					t.Fatalf("login again = %#v, err = %v", result, err)
				}
				return result.CapabilityRefreshRequired
			},
		},
		{
			name: "AcknowledgeSteamGuardRevocationCode",
			run: func(t *testing.T, service *Service, accountID string, grant SensitiveViewGrant) bool {
				service.enrollmentManager = &writingEnrollmentManager{
					service: service, accountID: accountID, revealCode: "R12345",
				}
				if _, err := service.RevealSteamGuardRevocationCode(accountID, grant.Capability); err != nil {
					t.Fatal(err)
				}
				result, err := service.AcknowledgeSteamGuardRevocationCode(accountID, grant.Capability, "R12345")
				if err != nil {
					t.Fatal(err)
				}
				return result.CapabilityRefreshRequired
			},
		},
		{
			name: "FinalizeSteamGuardEnrollment",
			run: func(t *testing.T, service *Service, accountID string, grant SensitiveViewGrant) bool {
				service.enrollmentManager = &writingEnrollmentManager{
					service: service, accountID: accountID,
					finalizeStatus: enrollmentflow.Status{State: enrollmentapi.StateComplete},
				}
				result, err := service.FinalizeSteamGuardEnrollment(accountID, grant.Capability, "12345")
				if err != nil {
					t.Fatal(err)
				}
				return result.CapabilityRefreshRequired
			},
		},
		{
			name: "ImportPlaintext",
			run: func(t *testing.T, service *Service, accountID string, _ SensitiveViewGrant) bool {
				source := writeImportableMaFile(t, 76561198000000123)
				results, err := service.ImportPlaintext([]string{source}, authServiceVaultPassword, false)
				if err != nil || len(results) != 1 || !results[0].Imported {
					t.Fatalf("import = %#v, err = %v", results, err)
				}
				return results[0].CapabilityRefreshRequired
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service, accountID, grant := newAuthServiceFixture(t)
			service.registryUpsertFn = func(string, registry.State) error { return nil }
			before := service.vault.Generation()
			signalled := testCase.run(t, service, accountID, grant)
			after := service.vault.Generation()
			if before == after {
				t.Fatalf("%s did not write to the vault; this test no longer covers what it claims", testCase.name)
			}
			if !signalled {
				t.Fatalf("%s wrote to the vault but did not set capabilityRefreshRequired", testCase.name)
			}
		})
	}
}

// TestLoginAgainReauthenticationRequiredIsNotAnError pins the other half of the
// contract: the re-authentication outcome is a state, not a failure, and it
// leaves the caller's capability valid because nothing was written.
func TestLoginAgainReauthenticationRequiredIsNotAnError(t *testing.T) {
	service, accountID, grant := newAuthServiceFixture(t)
	service.registryUpsertFn = func(string, registry.State) error { return nil }
	client := &authServiceTokenClient{err: errors.New("remote rejection")}
	service.newSessionRefresher = func(v *vault.Vault) steamSessionRefresher {
		return sessionrefresh.New(client, v)
	}
	before := service.vault.Generation()
	result, err := service.LoginAgain(accountID, grant.Capability)
	if err != nil {
		t.Fatalf("re-authentication surfaced as an error: %v", err)
	}
	if result.State != "reauthentication_required" {
		t.Fatalf("state = %q", result.State)
	}
	if result.CapabilityRefreshRequired {
		t.Fatal("no vault write happened, so the capability must stay valid")
	}
	if after := service.vault.Generation(); after != before {
		t.Fatal("failed refresh unexpectedly rotated the vault generation")
	}
	if _, err := service.LoginAgain(accountID, grant.Capability); err != nil {
		t.Fatalf("capability was invalidated by a non-writing failure: %v", err)
	}
}

func TestDialogCancelledDistinguishesCancelFromFailure(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{err: nil, want: false},
		{err: errors.New("cancelled by user"), want: true},
		{err: errors.New("canceled by user"), want: true},
		{err: fmt.Errorf("open file dialog: %w", errors.New("cancelled by user")), want: true},
		{err: errors.New("Cancelled by user"), want: true},
		{err: errors.New("unable to release dialog"), want: false},
		{err: os.ErrPermission, want: false},
	}
	for _, testCase := range cases {
		if got := dialogCancelled(testCase.err); got != testCase.want {
			t.Fatalf("dialogCancelled(%v) = %v, want %v", testCase.err, got, testCase.want)
		}
	}
}

func writeImportableMaFile(t *testing.T, steamID uint64) string {
	t.Helper()
	account := mafile.Account{
		SharedSecret:   base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x43}, 20)),
		IdentitySecret: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x44}, 20)),
		DeviceID:       "android:0a1b2c3d-4e5f-6071-8293-a4b5c6d7e8f9",
		AccountName:    "imported_account",
		FullyEnrolled:  true,
		Session:        &mafile.SessionData{SteamID: steamID, AccessToken: "a", RefreshToken: "r"},
	}
	plain, err := mafile.ExportPlaintext(account, mafile.ExportOptions{IncludeTokens: true})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tempDir(t), fmt.Sprintf("%d.maFile", steamID))
	if err := os.WriteFile(path, plain, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
