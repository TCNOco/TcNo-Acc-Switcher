package steamguard

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/registry"
	"TcNo-Acc-Switcher/internal/steamguard/securefile"
	"TcNo-Acc-Switcher/internal/steamguard/sessionrefresh"
	"TcNo-Acc-Switcher/internal/steamguard/vault"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const restoreStagePrefix = "SteamGuard-RestoreStage-"

var (
	ErrRestoreMergeRequiresVault = errors.New("Steam Guard restore merge requires a configured vault")
	ErrRestoreMergeNoPlan        = errors.New("Steam Guard restore merge has no prepared plan")
)

// RestoreMergeAccount is one account found in a backup, with enough context to
// decide whether to bring it across: whether the live vault already holds the
// SteamID64, and when each copy's session token lapses (0 = no token).
type RestoreMergeAccount struct {
	SteamID64          string `json:"steamId64"`
	AccountName        string `json:"accountName"`
	Exists             bool   `json:"exists"`
	BackupTokenExpiry  int64  `json:"backupTokenExpiry,omitempty"`
	CurrentTokenExpiry int64  `json:"currentTokenExpiry,omitempty"`
}

// RestoreMergePlan is the outcome of staging a backup for a merge.
// State "canceled" is the folder picker being dismissed; "backup_password"
// means the staged copy would not unlock with the supplied passwords — the
// stage is kept, so a retry with the backup's own password skips the picker.
type RestoreMergePlan struct {
	State    string                `json:"state"`
	Accounts []RestoreMergeAccount `json:"accounts,omitempty"`
}

type RestoreMergeResult struct {
	Added            int    `json:"added"`
	Replaced         int    `json:"replaced"`
	SafetyBackupPath string `json:"safetyBackupPath"`
	// The merge rotates the vault generation, so any capability the UI holds
	// is invalid and must be re-acquired.
	CapabilityRefreshRequired bool `json:"capabilityRefreshRequired"`
}

// PlanRestoreMerge stages an encrypted backup next to the vault and reports
// its accounts against the live vault, without writing to either. A stage kept
// by an earlier "backup_password" outcome is reused instead of asking for the
// folder again.
func (s *Service) PlanRestoreMerge(password, backupPassword, backupAppPassword string) (RestoreMergePlan, error) {
	s.mu.Lock()
	stage := s.restoreMergeStage
	s.mu.Unlock()
	if stage == "" {
		app := application.Get()
		if app == nil {
			return RestoreMergePlan{}, errors.New("application not initialised")
		}
		dialog := app.Dialog.OpenFile().
			SetTitle("Choose an encrypted Steam Guard backup folder").
			CanChooseDirectories(true).
			CanChooseFiles(false)
		if owner := dialogOwnerWindow(); owner != nil {
			dialog = dialog.AttachToWindow(owner)
		}
		source, err := dialog.PromptForSingleSelection()
		logDialogOutcome("steam-guard-restore-merge-folder", strings.TrimSpace(source) != "", err)
		if err != nil {
			if dialogCancelled(err) {
				return RestoreMergePlan{State: "canceled"}, nil
			}
			return RestoreMergePlan{}, err
		}
		if strings.TrimSpace(source) == "" {
			return RestoreMergePlan{State: "canceled"}, nil
		}
		if stage, err = s.stageRestoreMerge(source); err != nil {
			return RestoreMergePlan{}, err
		}
	}
	return s.planStagedRestoreMerge(stage, password, backupPassword, backupAppPassword)
}

// stageRestoreMerge copies the chosen backup into a protected staging folder
// beside the vault. The copy exists so that opening and unlocking — which can
// write (journal recovery) — never touches the user's own backup.
func (s *Service) stageRestoreMerge(source string) (string, error) {
	source = filepath.Clean(strings.TrimSpace(source))
	if !filepath.IsAbs(source) {
		return "", ErrInvalidBackupDestination
	}
	canonicalSource, err := canonicalSafeDirectory(source)
	if err != nil {
		return "", errors.Join(ErrInvalidBackupDestination, err)
	}
	vaultRoot, err := VaultFolderPath()
	if err != nil {
		return "", err
	}
	if withinPath(canonicalSource, vaultRoot) || withinPath(vaultRoot, canonicalSource) {
		return "", ErrInvalidBackupDestination
	}
	parent := filepath.Dir(vaultRoot)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discardRestoreMergeStageLocked()
	sweepRestoreMergeStages(parent)
	stage := filepath.Join(parent, restoreStagePrefix+strconv.FormatInt(time.Now().UTC().UnixNano(), 10))
	if err := securefile.CreateDirectoryNew(stage); err != nil {
		return "", err
	}
	if err := copyBackupTree(canonicalSource, stage); err != nil {
		removeCreatedBackup(parent, stage)
		return "", err
	}
	s.restoreMergeStage = stage
	s.restoreMergeSource = filepath.Dir(canonicalSource)
	return stage, nil
}

func (s *Service) planStagedRestoreMerge(stage, password, backupPassword, backupAppPassword string) (RestoreMergePlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	staged, err := s.openStagedRestoreMergeLocked(stage, password, backupPassword, backupAppPassword)
	if errors.Is(err, vault.ErrInvalidPassword) {
		return RestoreMergePlan{State: "backup_password"}, nil
	}
	if err != nil {
		return RestoreMergePlan{}, err
	}
	defer func() { _ = staged.Lock() }()
	v, err := s.requireVaultLocked()
	if err != nil {
		return RestoreMergePlan{}, errors.Join(ErrRestoreMergeRequiresVault, err)
	}
	if v.IsLocked() {
		if err := s.unlockVaultLocked(v, password, false); err != nil {
			return RestoreMergePlan{}, err
		}
	}
	liveExpiry := make(map[string]int64)
	liveRecords, err := v.List()
	if err != nil {
		return RestoreMergePlan{}, err
	}
	for _, record := range liveRecords {
		expiry, getErr := recordTokenExpiry(v, record.ID)
		if getErr != nil {
			return RestoreMergePlan{}, getErr
		}
		liveExpiry[record.SteamID64] = expiry
	}
	stagedRecords, err := staged.List()
	if err != nil {
		return RestoreMergePlan{}, err
	}
	accounts := make([]RestoreMergeAccount, 0, len(stagedRecords))
	for _, record := range stagedRecords {
		plaintext, getErr := staged.Get(record.ID)
		if getErr != nil {
			return RestoreMergePlan{}, getErr
		}
		parsed, parseErr := mafile.ParsePlaintext(plaintext)
		wipe(plaintext)
		if parseErr != nil {
			return RestoreMergePlan{}, parseErr
		}
		currentExpiry, exists := liveExpiry[record.SteamID64]
		accounts = append(accounts, RestoreMergeAccount{
			SteamID64:          record.SteamID64,
			AccountName:        parsed.Account.AccountName,
			Exists:             exists,
			BackupTokenExpiry:  sessionTokenExpiry(parsed.Account.Session),
			CurrentTokenExpiry: currentExpiry,
		})
	}
	serviceLogger().Info("Steam Guard restore merge planned", "accounts", len(accounts))
	return RestoreMergePlan{State: "ok", Accounts: accounts}, nil
}

// CommitRestoreMerge writes the selected accounts from the staged backup into
// the live vault. The vault as it stands is first copied into a fresh verified
// backup beside the source backup, so the pre-merge state stays recoverable.
func (s *Service) CommitRestoreMerge(password, backupPassword, backupAppPassword, currentAppPassword string, steamIDs []string) (RestoreMergeResult, error) {
	s.mu.Lock()
	stage, source := s.restoreMergeStage, s.restoreMergeSource
	s.mu.Unlock()
	if stage == "" {
		return RestoreMergeResult{}, ErrRestoreMergeNoPlan
	}
	if len(steamIDs) == 0 {
		return RestoreMergeResult{}, ErrRestoreMergeNoPlan
	}
	// The safety copy happens before anything is written; createVerifiedBackupAt
	// manages its own locking, so it runs outside the merge's critical section.
	safetyPath, err := s.createVerifiedBackupAt(source, password, currentAppPassword, time.Now().UTC())
	if err != nil {
		return RestoreMergeResult{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	staged, err := s.openStagedRestoreMergeLocked(stage, password, backupPassword, backupAppPassword)
	if err != nil {
		return RestoreMergeResult{}, err
	}
	defer func() { _ = staged.Lock() }()
	v, err := s.requireVaultLocked()
	if err != nil {
		return RestoreMergeResult{}, errors.Join(ErrRestoreMergeRequiresVault, err)
	}
	if v.IsLocked() {
		if err := s.unlockVaultLocked(v, password, false); err != nil {
			return RestoreMergeResult{}, err
		}
	}
	liveRecords, err := v.List()
	if err != nil {
		return RestoreMergeResult{}, err
	}
	existing := make(map[string]struct{}, len(liveRecords))
	for _, record := range liveRecords {
		existing[record.SteamID64] = struct{}{}
	}
	selected := make(map[string]struct{}, len(steamIDs))
	for _, id := range steamIDs {
		selected[strings.TrimSpace(id)] = struct{}{}
	}
	stagedRecords, err := staged.List()
	if err != nil {
		return RestoreMergeResult{}, err
	}
	result := RestoreMergeResult{SafetyBackupPath: safetyPath}
	for _, record := range stagedRecords {
		if _, wanted := selected[record.SteamID64]; !wanted {
			continue
		}
		plaintext, getErr := staged.Get(record.ID)
		if getErr != nil {
			return result, getErr
		}
		parsed, parseErr := mafile.ParsePlaintext(plaintext)
		wipe(plaintext)
		if parseErr != nil {
			return result, parseErr
		}
		steamID64, _, _, commitErr := commitImportedAccount(v, parsed)
		if commitErr != nil {
			return result, commitErr
		}
		if err := registry.Upsert(steamID64, registry.StateActive); err != nil {
			return result, err
		}
		if _, was := existing[steamID64]; was {
			result.Replaced++
		} else {
			result.Added++
		}
		result.CapabilityRefreshRequired = true
	}
	s.discardRestoreMergeStageLocked()
	serviceLogger().Info("Steam Guard restore merge committed",
		"added", result.Added, "replaced", result.Replaced)
	return result, nil
}

// CancelRestoreMerge discards any staged backup copy.
func (s *Service) CancelRestoreMerge() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discardRestoreMergeStageLocked()
	return nil
}

func (s *Service) discardRestoreMergeStageLocked() {
	if s.restoreMergeStage == "" {
		return
	}
	removeCreatedBackup(filepath.Dir(s.restoreMergeStage), s.restoreMergeStage)
	s.restoreMergeStage = ""
	s.restoreMergeSource = ""
}

// sweepRestoreMergeStages removes staging folders a crash left behind. They
// hold a copy of an encrypted backup, so they are junk, not data.
func sweepRestoreMergeStages(parent string) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), restoreStagePrefix) {
			removeCreatedBackup(parent, filepath.Join(parent, entry.Name()))
		}
	}
}

// openStagedRestoreMergeLocked opens and unlocks the staged copy. The vault
// password defaults to the live one — a backup made before a password change
// needs its own, which the caller passes as backupPassword.
func (s *Service) openStagedRestoreMergeLocked(stage, password, backupPassword, backupAppPassword string) (*vault.Vault, error) {
	vaultPassword := backupPassword
	if strings.TrimSpace(vaultPassword) == "" {
		vaultPassword = password
	}
	if strings.TrimSpace(vaultPassword) == "" {
		return nil, vault.ErrInvalidPassword
	}
	staged, err := vault.Open(stage, s.vaultOptions...)
	if err != nil {
		return nil, err
	}
	if staged.HasRecoveryWrapper() {
		err = staged.UnlockWithRecovery(vaultPassword, backupAppPassword, vault.FixedLease)
	} else {
		err = staged.Unlock(vaultPassword, vault.FixedLease)
	}
	if err != nil {
		return nil, err
	}
	return staged, nil
}

func recordTokenExpiry(v *vault.Vault, recordID string) (int64, error) {
	plaintext, err := v.Get(recordID)
	if err != nil {
		return 0, err
	}
	parsed, parseErr := mafile.ParsePlaintext(plaintext)
	wipe(plaintext)
	if parseErr != nil {
		return 0, parseErr
	}
	return sessionTokenExpiry(parsed.Account.Session), nil
}

// sessionTokenExpiry reads the access token's expiry offline; 0 when the copy
// carries no readable token. The token itself never leaves this function.
func sessionTokenExpiry(session *mafile.SessionData) int64 {
	if session == nil {
		return 0
	}
	expiry, ok := sessionrefresh.AccessTokenExpiry(session.AccessToken)
	if !ok {
		return 0
	}
	return expiry.Unix()
}
