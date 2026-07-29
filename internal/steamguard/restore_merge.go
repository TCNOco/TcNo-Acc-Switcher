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
// State "canceled" is an empty source, meaning no folder was chosen;
// "backup_password" means the staged copy would not unlock with the supplied
// passwords — the stage is kept, so a retry reuses it without a second copy.
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

// PlanRestoreMerge stages the encrypted backup at source next to the vault and
// reports its accounts against the live vault, without writing to either. A
// stage kept by an earlier "backup_password" outcome is reused, so a retry
// neither re-copies the backup nor asks for the folder again.
func (s *Service) PlanRestoreMerge(source, password, backupPassword, backupAppPassword string) (RestoreMergePlan, error) {
	return s.PlanRestoreMergeWithFactors(source, password, backupPassword, backupAppPassword, "", "")
}

// PlanRestoreMergeWithFactors plans a merge where either vault needs more than a
// password. The same keyfile and backup key are offered to both: the live vault
// and the backup are the same vault at two points in time, so the factors
// enrolled in one almost always open the other.
func (s *Service) PlanRestoreMergeWithFactors(source, password, backupPassword, backupAppPassword, keyfilePath, backupKey string) (RestoreMergePlan, error) {
	live, backup, err := mergeCredentials(password, backupPassword, keyfilePath, backupKey)
	if err != nil {
		return RestoreMergePlan{}, err
	}
	defer wipeMergeCredentials(live)
	s.mu.Lock()
	stage := s.restoreMergeStage
	s.mu.Unlock()
	if stage == "" {
		if strings.TrimSpace(source) == "" {
			return RestoreMergePlan{State: "canceled"}, nil
		}
		if stage, err = s.stageRestoreMerge(source); err != nil {
			return RestoreMergePlan{}, err
		}
	}
	return s.planStagedRestoreMerge(stage, live, backup, backupAppPassword)
}

// mergeCredentials builds the live and backup factor sets from one set of files.
// They share the keyfile and backup-key material, so only the live set is wiped.
func mergeCredentials(password, backupPassword, keyfilePath, backupKey string) (vault.Credentials, vault.Credentials, error) {
	live, err := buildVaultCredentials(password, keyfilePath, backupKey)
	if err != nil {
		return vault.Credentials{}, vault.Credentials{}, err
	}
	backup := live
	if strings.TrimSpace(backupPassword) != "" {
		backup.Password = backupPassword
	}
	return live, backup, nil
}

func wipeMergeCredentials(creds vault.Credentials) {
	wipe(creds.Keyfile)
	wipe(creds.RecoveryCode)
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

func (s *Service) planStagedRestoreMerge(stage string, live, backup vault.Credentials, backupAppPassword string) (RestoreMergePlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	staged, err := s.openStagedRestoreMergeLocked(stage, backup, backupAppPassword)
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
		if err := s.unlockWithSecurityKeyFallbackLocked(v, &live); err != nil {
			return RestoreMergePlan{}, err
		}
		defer wipe(live.SecurityKey)
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
	return s.CommitRestoreMergeWithFactors(password, backupPassword, backupAppPassword, currentAppPassword, "", "", steamIDs)
}

// CommitRestoreMergeWithFactors commits a merge where either vault needs more
// than a password.
func (s *Service) CommitRestoreMergeWithFactors(password, backupPassword, backupAppPassword, currentAppPassword, keyfilePath, backupKey string, steamIDs []string) (RestoreMergeResult, error) {
	live, backup, err := mergeCredentials(password, backupPassword, keyfilePath, backupKey)
	if err != nil {
		return RestoreMergeResult{}, err
	}
	defer wipeMergeCredentials(live)
	s.mu.Lock()
	stage, source := s.restoreMergeStage, s.restoreMergeSource
	s.mu.Unlock()
	if stage == "" {
		return RestoreMergeResult{}, ErrRestoreMergeNoPlan
	}
	if len(steamIDs) == 0 {
		return RestoreMergeResult{}, ErrRestoreMergeNoPlan
	}
	// The safety copy happens before anything is written; the backup manages its
	// own locking, so it runs outside the merge's critical section.
	safetyPath, err := s.createVerifiedBackupAtWith(source, live, currentAppPassword, time.Now().UTC())
	if err != nil {
		return RestoreMergeResult{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	staged, err := s.openStagedRestoreMergeLocked(stage, backup, backupAppPassword)
	if err != nil {
		return RestoreMergeResult{}, err
	}
	defer func() { _ = staged.Lock() }()
	v, err := s.requireVaultLocked()
	if err != nil {
		return RestoreMergeResult{}, errors.Join(ErrRestoreMergeRequiresVault, err)
	}
	if v.IsLocked() {
		if err := s.unlockWithSecurityKeyFallbackLocked(v, &live); err != nil {
			return RestoreMergeResult{}, err
		}
		defer wipe(live.SecurityKey)
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

// openStagedRestoreMergeLocked opens and unlocks the staged copy with the
// backup's own factor set, which mergeCredentials derives from the live one when
// the caller supplied no separate backup password.
func (s *Service) openStagedRestoreMergeLocked(stage string, creds vault.Credentials, backupAppPassword string) (*vault.Vault, error) {
	if strings.TrimSpace(creds.Password) == "" && len(creds.Keyfile) == 0 && len(creds.RecoveryCode) == 0 {
		return nil, vault.ErrInvalidPassword
	}
	staged, err := vault.Open(stage, s.vaultOptions...)
	if err != nil {
		return nil, err
	}
	if staged.HasRecoveryWrapper() {
		err = staged.UnlockWithFactorsAndRecovery(creds, backupAppPassword, vault.FixedLease)
	} else {
		err = staged.UnlockWith(creds, vault.FixedLease)
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
