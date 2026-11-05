package steamguard

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/battlenetauth"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/steambrowser"
	"TcNo-Acc-Switcher/internal/steamguard/securemem"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

type BattleNetEnrollmentStatus struct {
	Handle string `json:"handle"`
	State  string `json:"state"`
	Error  string `json:"error"`
	Serial string `json:"serial"`
}

type battleNetOperation struct {
	vault    *vault.Vault
	status   BattleNetEnrollmentStatus
	token    string
	expires  time.Time
	cancel   context.CancelFunc
	identity battlenetauth.EnrollmentIdentity
}

var ErrBattleNetResponseRecovery = errors.New("Recover the saved Battle.net response before starting setup again")

type battleNetRuntime struct {
	pending                securemem.Handle
	browser                *steambrowser.Service
	operation              *battleNetOperation
	client                 battleNetEnrollmentClient
	readAccountName        func(context.Context, *steambrowser.Service, string) (steambrowser.BattleNetAccountIdentity, error)
	readLoginIdentity      func(context.Context, *steambrowser.Service, string) (steambrowser.BattleNetAccountIdentity, error)
	closeEnrollmentBrowser func(*steambrowser.Service, string)
	identityStatus         map[string]string
}

type battleNetEnrollmentClient interface {
	EnrollWithRecovery(context.Context, string, func([]byte) error, ...func(battlenetauth.EnrollmentIdentity) error) (battlenetauth.Account, error)
}

func ConfigureBattleNetBrowser(s *Service, browser *steambrowser.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.battleNet = &battleNetRuntime{browser: browser, client: battlenetauth.NewClient(), readAccountName: steambrowser.ReadBattleNetAccountIdentity, readLoginIdentity: steambrowser.ReadBattleNetLoginIdentity, closeEnrollmentBrowser: steambrowser.CloseBattleNetEnrollmentBrowser}
}

func (s *Service) BeginBattleNetEnrollment(token string) (BattleNetEnrollmentStatus, error) {
	s.mu.Lock()
	v, err := s.battleNetVaultLocked(token)
	if err != nil {
		s.mu.Unlock()
		return BattleNetEnrollmentStatus{}, err
	}
	if v.IsLocked() {
		s.mu.Unlock()
		return BattleNetEnrollmentStatus{}, vault.ErrLocked
	}
	if s.battleNet == nil {
		s.mu.Unlock()
		return BattleNetEnrollmentStatus{}, ErrUnsupportedInput
	}
	if s.battleNet.operation != nil && s.battleNet.operation.status.State == "recovery-required" {
		s.mu.Unlock()
		return BattleNetEnrollmentStatus{}, ErrBattleNetResponseRecovery
	}
	if s.battleNet.pending != nil {
		s.mu.Unlock()
		return BattleNetEnrollmentStatus{}, errors.New("recover the previous enrollment result before enrolling again")
	}
	if pending, err := battleNetRecoveryPending(v); err != nil {
		s.mu.Unlock()
		return BattleNetEnrollmentStatus{}, err
	} else if pending {
		s.mu.Unlock()
		return BattleNetEnrollmentStatus{}, ErrBattleNetResponseRecovery
	}
	if previous := s.battleNet.operation; previous != nil && time.Now().Before(previous.expires) && (previous.status.State == "waiting" || previous.status.State == "saving") {
		if _, err := s.capabilities.Resolve(previous.token); err == nil {
			s.mu.Unlock()
			return BattleNetEnrollmentStatus{}, ErrSteamOperationBusy
		}
		previous.status.State = "canceled"
	}
	id := rand.Text()
	op := &battleNetOperation{vault: v, status: BattleNetEnrollmentStatus{Handle: id, State: "waiting"}, token: token, expires: time.Now().Add(5 * time.Minute)}
	s.battleNet.operation = op
	browser := s.battleNet.browser
	s.mu.Unlock()
	err = steambrowser.OpenBattleNetLogin(browser, "battlenet-"+strings.ToLower(id), battleNetEnrollmentNavigation(func(sessionToken string) {
		go s.completeBattleNetEnrollment(op, sessionToken)
	}))
	if err != nil {
		s.mu.Lock()
		op.status.State = "failed"
		op.status.Error = "Battle.net login window could not be opened"
		s.mu.Unlock()
		return BattleNetEnrollmentStatus{}, err
	}
	return BattleNetEnrollmentStatus{Handle: id, State: "waiting"}, nil
}

func (s *Service) completeBattleNetEnrollment(op *battleNetOperation, sessionToken string) {
	s.mu.Lock()
	if s.battleNet == nil || s.battleNet.operation != op || op.status.State != "waiting" || !time.Now().Before(op.expires) {
		s.mu.Unlock()
		return
	}
	v, err := s.battleNetOperationVaultLocked(op)
	if err != nil {
		op.status.State = "failed"
		op.status.Error = "Unlock the vault and start enrollment again"
		s.mu.Unlock()
		return
	}
	op.status.State = "saving"
	probe, probeErr := securemem.New().Store([]byte("enrollment"))
	if probeErr != nil {
		op.status.State = "failed"
		op.status.Error = "Protected memory is unavailable; enrollment was not started"
		s.mu.Unlock()
		return
	}
	_ = probe.Destroy()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	op.cancel = cancel
	runtime := s.battleNet
	profile := "battlenet-" + strings.ToLower(op.status.Handle)
	s.mu.Unlock()
	var loginIdentity steambrowser.BattleNetAccountIdentity
	if runtime.readLoginIdentity != nil {
		readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
		loginIdentity, _ = runtime.readLoginIdentity(readCtx, runtime.browser, profile)
		readCancel()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.battleNet != runtime || runtime.operation != op || op.status.State != "saving" {
		return
	}
	if _, err = s.battleNetOperationVaultLocked(op); err != nil || ctx.Err() != nil {
		op.status.State = "failed"
		op.status.Error = "Unlock the vault and start enrollment again"
		return
	}
	receiptID, err := v.CaptureProviderRecord(vault.ProviderBattleNet, func(save func([]byte) error) (string, []byte, error) {
		a, err := runtime.client.EnrollWithRecovery(ctx, sessionToken, s.preserveBattleNetResponse(profile, "", "", save, func() battlenetauth.EnrollmentIdentity { return op.identity }), func(identity battlenetauth.EnrollmentIdentity) error {
			if identity.Email == "" {
				identity.Email = battlenetauth.NormalizeAccountEmail(loginIdentity.Email)
			}
			if err := identity.Validate(); err != nil {
				return err
			}
			op.identity = identity
			return nil
		})
		sessionToken = ""
		if err != nil {
			return "", nil, err
		}
		op.status.Serial = a.Serial
		a.BrowserProfile = profile
		a.AccountName = op.identity.BattleTag
		a.AccountLink = op.identity.Email
		raw, err := json.Marshal(a)
		return a.Serial, raw, err
	})
	if err != nil {
		if errors.Is(err, battlenetauth.ErrAccountVerification) {
			if receiptID != "" {
				if completeErr := v.CompleteBattleNetRecovery(receiptID); completeErr != nil {
					op.status.State = "recovery-required"
					op.status.Error = ErrBattleNetResponseRecovery.Error()
					return
				}
			}
			if s.battleNet.pending != nil {
				_ = s.battleNet.pending.Destroy()
				s.battleNet.pending = nil
			}
			op.status.State = "verification-required"
			op.status.Error = err.Error()
			return
		}
		op.status.State = "failed"
		op.status.Error = err.Error()
		if receiptID != "" || s.battleNet.pending != nil || errors.Is(err, ErrBattleNetResponseRecovery) {
			op.status.State = "recovery-required"
			op.status.Error = ErrBattleNetResponseRecovery.Error()
		}
		return
	}
	if s.battleNet.pending != nil {
		_ = s.battleNet.pending.Destroy()
		s.battleNet.pending = nil
	}
	op.status.State = "pending-backup"
	if runtime.closeEnrollmentBrowser != nil {
		runtime.closeEnrollmentBrowser(runtime.browser, profile)
	}
}

func (s *Service) OpenBattleNetEnrollmentAccount(handle, token string) error {
	s.mu.Lock()
	if s.battleNet == nil || s.battleNet.operation == nil {
		s.mu.Unlock()
		return ErrAccountNotFound
	}
	op := s.battleNet.operation
	if op.status.Handle != handle || subtle.ConstantTimeCompare([]byte(op.token), []byte(token)) != 1 || (op.status.State != "verification-required" && op.status.State != "recovery-required") {
		s.mu.Unlock()
		return ErrSensitiveView
	}
	v, err := s.battleNetVaultLocked(token)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	if v != op.vault || v.IsLocked() {
		s.mu.Unlock()
		return ErrSensitiveView
	}
	browser := s.battleNet.browser
	profile := "battlenet-" + strings.ToLower(op.status.Handle)
	s.mu.Unlock()
	return steambrowser.OpenBattleNetAccount(browser, profile)
}

func (s *Service) battleNetOperationVaultLocked(op *battleNetOperation) (*vault.Vault, error) {
	if err := security.RequireUnlocked(); err != nil {
		return nil, err
	}
	v, exists, err := s.openVaultLocked()
	if err != nil {
		return nil, err
	}
	if !exists || v != op.vault || v.IsLocked() {
		return nil, vault.ErrLocked
	}
	binding, err := s.capabilities.Resolve(op.token)
	if err != nil {
		return nil, err
	}
	if binding.AccountID != battleNetView || binding.WindowName != mainWindowName || binding.Scope != modalCapabilityScope {
		return nil, ErrSensitiveView
	}
	s.contentProtectionMu.Lock()
	lease, ok := s.contentProtectionLeases[binding.LeaseID]
	s.contentProtectionMu.Unlock()
	if !ok || lease.binding != binding {
		return nil, ErrSensitiveView
	}
	return v, nil
}

func (s *Service) GetBattleNetEnrollmentStatus(handle, token string) (BattleNetEnrollmentStatus, error) {
	if err := security.RequireUnlocked(); err != nil {
		return BattleNetEnrollmentStatus{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.battleNet == nil || s.battleNet.operation == nil {
		return BattleNetEnrollmentStatus{}, ErrAccountNotFound
	}
	op := s.battleNet.operation
	if _, err := s.capabilities.Resolve(token); err != nil {
		return BattleNetEnrollmentStatus{}, err
	}
	if op.status.Handle != handle || subtle.ConstantTimeCompare([]byte(op.token), []byte(token)) != 1 {
		return BattleNetEnrollmentStatus{}, ErrSensitiveView
	}
	if op.status.State == "waiting" && !time.Now().Before(op.expires) {
		op.status.State = "expired"
	}
	return op.status, nil
}

func (s *Service) CancelBattleNetEnrollment(handle, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.battleNet == nil || s.battleNet.operation == nil {
		return ErrAccountNotFound
	}
	op := s.battleNet.operation
	if op.status.Handle != handle || subtle.ConstantTimeCompare([]byte(op.token), []byte(token)) != 1 {
		return ErrSensitiveView
	}
	if op.status.State == "waiting" || op.status.State == "verification-required" {
		op.status.State = "canceled"
	}
	if op.cancel != nil {
		op.cancel()
	}
	return nil
}

func (s *Service) RestoreBattleNetAuthenticator(accountIdentifier, serial, restoreCode, token string) error {
	var err error
	serial, err = battlenetauth.NormalizeSerial(serial)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.battleNetVaultLocked(token)
	if err != nil {
		return err
	}
	if _, err := battleNetRecord(v, serial); err == nil {
		return vault.ErrAlreadyExists
	} else if !errors.Is(err, ErrAccountNotFound) {
		return err
	}
	client := battlenetauth.NewClient()
	if s.battleNet == nil {
		return ErrUnsupportedInput
	}
	if s.battleNet.pending != nil {
		return errors.New("recover the previous enrollment result first")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if pending, err := battleNetRecoveryPending(v); err != nil {
		return err
	} else if pending {
		return ErrBattleNetResponseRecovery
	}
	receiptID, err := v.CaptureProviderRecord(vault.ProviderBattleNet, func(save func([]byte) error) (string, []byte, error) {
		a, err := client.RestoreWithRecovery(ctx, accountIdentifier, serial, strings.ToUpper(restoreCode), s.preserveBattleNetResponse("", serial, strings.ToUpper(restoreCode), save))
		if err != nil {
			return "", nil, err
		}
		raw, err := json.Marshal(a)
		return a.Serial, raw, err
	})
	if errors.Is(err, battlenetauth.ErrAccountVerification) && receiptID != "" {
		if completeErr := v.CompleteBattleNetRecovery(receiptID); completeErr != nil {
			return ErrBattleNetResponseRecovery
		}
		if s.battleNet.pending != nil {
			_ = s.battleNet.pending.Destroy()
			s.battleNet.pending = nil
		}
	} else if err != nil && (receiptID != "" || s.battleNet.pending != nil) {
		return ErrBattleNetResponseRecovery
	}
	if err == nil && s.battleNet.pending != nil {
		_ = s.battleNet.pending.Destroy()
		s.battleNet.pending = nil
	}
	return err
}

func (s *Service) OpenBattleNetAccount(serial, token string) error {
	s.mu.Lock()
	v, err := s.battleNetVaultLocked(token)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	a, err := battleNetRecord(v, serial)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	if s.battleNet == nil {
		s.mu.Unlock()
		return ErrUnsupportedInput
	}
	browser := s.battleNet.browser
	profile := battleNetBrowserProfile(a)
	s.mu.Unlock()
	return steambrowser.OpenBattleNetAccount(browser, profile)
}

func battleNetBrowserProfile(a battlenetauth.Account) string {
	if a.BrowserProfile != "" {
		return a.BrowserProfile
	}
	sum := sha256.Sum256([]byte(a.Serial))
	return fmt.Sprintf("battlenet-%x", sum[:16])
}

func (s *Service) AcknowledgeBattleNetBackup(serial, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.battleNetVaultLocked(token)
	if err != nil {
		return err
	}
	a, err := battleNetRecord(v, serial)
	if err != nil {
		return err
	}
	a.Pending = false
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	defer wipe(raw)
	_, err = v.PutProviderRecord(vault.ProviderBattleNet, a.Serial, raw)
	return err
}

func battleNetRecoveryPending(v *vault.Vault) (bool, error) {
	payloads, err := v.BattleNetRecoveryPayloads()
	for _, payload := range payloads {
		wipe(payload.Plaintext)
	}
	return len(payloads) > 0, err
}

func (s *Service) preserveBattleNetResponse(profile, serial, restoreCode string, save func([]byte) error, identity ...func() battlenetauth.EnrollmentIdentity) func([]byte) error {
	return func(response []byte) error {
		if s.battleNet.pending != nil {
			return ErrBattleNetResponseRecovery
		}
		var metadata []battlenetauth.EnrollmentIdentity
		if len(identity) > 0 && identity[0] != nil {
			metadata = append(metadata, identity[0]())
		}
		raw, err := battlenetauth.EncodeRecovery(response, profile, serial, restoreCode, metadata...)
		if err != nil {
			return err
		}
		defer wipe(raw)
		handle, memoryErr := securemem.New().Store(raw)
		if memoryErr == nil {
			s.battleNet.pending = handle
		}
		if err := save(raw); err != nil {
			return ErrBattleNetResponseRecovery
		}
		return nil
	}
}

func (s *Service) RecoverBattleNetEnrollments(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.battleNetVaultLocked(token)
	if err != nil {
		return err
	}
	payloads, err := v.BattleNetRecoveryPayloads()
	if err != nil {
		return err
	}
	defer func() {
		for _, payload := range payloads {
			wipe(payload.Plaintext)
		}
	}()
	if s.battleNet != nil && s.battleNet.pending != nil {
		if err := s.battleNet.pending.With(func(raw []byte) error {
			payloads = append(payloads, vault.BattleNetRecoveryPayload{Plaintext: append([]byte(nil), raw...)})
			return nil
		}); err != nil {
			return err
		}
	}
	if len(payloads) == 0 && s.battleNet != nil && s.battleNet.operation != nil && s.battleNet.operation.status.State == "recovery-required" {
		return ErrBattleNetResponseRecovery
	}
	updates := []vault.RecordUpdate{}
	recovered := map[string]string{}
	for _, payload := range payloads {
		a, err := battlenetauth.RecoverAccount(payload.Plaintext)
		if errors.Is(err, battlenetauth.ErrAccountVerification) {
			continue
		}
		if err != nil {
			return ErrBattleNetResponseRecovery
		}
		if secret, seen := recovered[a.Serial]; seen {
			if secret != a.DeviceSecret {
				return vault.ErrAlreadyExists
			}
			continue
		}
		recovered[a.Serial] = a.DeviceSecret
		if existing, err := battleNetRecord(v, a.Serial); err == nil {
			if existing.DeviceSecret != a.DeviceSecret {
				return vault.ErrAlreadyExists
			}
			continue
		} else if !errors.Is(err, ErrAccountNotFound) {
			return err
		}
		raw, err := json.Marshal(a)
		if err != nil {
			return err
		}
		defer wipe(raw)
		identity, _ := vault.ProviderIdentity(vault.ProviderBattleNet, a.Serial)
		updates = append(updates, vault.RecordUpdate{SteamID64: identity, Plaintext: raw})
	}
	if err := v.PutRecords(updates); err != nil {
		return err
	}
	for _, payload := range payloads {
		if payload.ID != "" {
			if err := v.CompleteBattleNetRecovery(payload.ID); err != nil {
				return err
			}
		}
	}
	if s.battleNet != nil && s.battleNet.pending != nil {
		_ = s.battleNet.pending.Destroy()
		s.battleNet.pending = nil
	}
	if s.battleNet != nil && s.battleNet.operation != nil && s.battleNet.operation.status.State == "recovery-required" {
		s.battleNet.operation.status.State = "canceled"
	}
	return nil
}
