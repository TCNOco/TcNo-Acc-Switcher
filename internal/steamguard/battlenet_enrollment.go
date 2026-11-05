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
	vault   *vault.Vault
	status  BattleNetEnrollmentStatus
	token   string
	expires time.Time
	cancel  context.CancelFunc
}

type battleNetRuntime struct {
	pending   securemem.Handle
	browser   *steambrowser.Service
	operation *battleNetOperation
	client    *battlenetauth.Client
}

func ConfigureBattleNetBrowser(s *Service, browser *steambrowser.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.battleNet = &battleNetRuntime{browser: browser, client: battlenetauth.NewClient()}
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
	if s.battleNet.pending != nil {
		s.mu.Unlock()
		return BattleNetEnrollmentStatus{}, errors.New("recover the previous enrollment result before enrolling again")
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
	defer s.mu.Unlock()
	if s.battleNet == nil || s.battleNet.operation != op || op.status.State != "waiting" || !time.Now().Before(op.expires) {
		return
	}
	v, err := s.battleNetOperationVaultLocked(op)
	if err != nil {
		op.status.State = "failed"
		op.status.Error = "Unlock the vault and start enrollment again"
		return
	}
	op.status.State = "saving"
	probe, probeErr := securemem.New().Store([]byte("enrollment"))
	if probeErr != nil {
		op.status.State = "failed"
		op.status.Error = "Protected memory is unavailable; enrollment was not started"
		return
	}
	_ = probe.Destroy()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	op.cancel = cancel
	err = v.CaptureProviderRecord(vault.ProviderBattleNet, func() (string, []byte, error) {
		a, err := s.battleNet.client.Enroll(ctx, sessionToken)
		sessionToken = ""
		if err != nil {
			return "", nil, err
		}
		op.status.Serial = a.Serial
		a.BrowserProfile = "battlenet-" + strings.ToLower(op.status.Handle)
		raw, err := json.Marshal(a)
		if err == nil {
			s.battleNet.pending, err = securemem.New().Store(raw)
		}
		return a.Serial, raw, err
	})
	if err != nil {
		op.status.State = "failed"
		op.status.Error = err.Error()
		if s.battleNet.pending != nil {
			op.status.Error = "Setup completed but could not be saved. Use Recover unsaved setup before trying again. " + op.status.Error
		}
		return
	}
	if s.battleNet.pending != nil {
		_ = s.battleNet.pending.Destroy()
		s.battleNet.pending = nil
	}
	op.status.State = "pending-backup"
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
	if op.status.State == "waiting" {
		op.status.State = "canceled"
	}
	if op.cancel != nil {
		op.cancel()
	}
	return nil
}

func (s *Service) RestoreBattleNetAuthenticator(accountIdentifier, serial, restoreCode, token string) error {
	serial = strings.ToUpper(strings.TrimSpace(serial))
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
	err = v.CaptureProviderRecord(vault.ProviderBattleNet, func() (string, []byte, error) {
		a, err := client.Restore(ctx, accountIdentifier, strings.ToUpper(serial), strings.ToUpper(restoreCode))
		if err != nil {
			return "", nil, err
		}
		raw, err := json.Marshal(a)
		if err == nil {
			s.battleNet.pending, err = securemem.New().Store(raw)
		}
		return a.Serial, raw, err
	})
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
	profile := a.BrowserProfile
	if profile == "" {
		sum := sha256.Sum256([]byte(a.Serial))
		profile = fmt.Sprintf("battlenet-%x", sum[:16])
	}
	s.mu.Unlock()
	return steambrowser.OpenBattleNetAccount(browser, profile)
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

func (s *Service) RecoverBattleNetEnrollments(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.battleNetVaultLocked(token)
	if err != nil {
		return err
	}
	if s.battleNet != nil && s.battleNet.pending != nil {
		err := s.battleNet.pending.With(func(raw []byte) error {
			a, err := battlenetauth.ParseAccount(raw)
			if err != nil {
				return err
			}
			if _, err := battleNetRecord(v, a.Serial); err == nil {
				return nil
			} else if !errors.Is(err, ErrAccountNotFound) {
				return err
			}
			_, err = v.PutProviderRecord(vault.ProviderBattleNet, a.Serial, raw)
			return err
		})
		if err != nil {
			return err
		}
		_ = s.battleNet.pending.Destroy()
		s.battleNet.pending = nil
	}
	payloads, err := v.BattleNetRecoveryPayloads()
	if err != nil {
		return err
	}
	defer func() {
		for _, raw := range payloads {
			wipe(raw)
		}
	}()
	updates := []vault.RecordUpdate{}
	for _, raw := range payloads {
		a, err := battlenetauth.ParseAccount(raw)
		if err != nil {
			return err
		}
		if _, err := battleNetRecord(v, a.Serial); err == nil {
			continue
		} else if !errors.Is(err, ErrAccountNotFound) {
			return err
		}
		identity, _ := vault.ProviderIdentity(vault.ProviderBattleNet, a.Serial)
		updates = append(updates, vault.RecordUpdate{SteamID64: identity, Plaintext: raw})
	}
	return v.PutRecords(updates)
}
