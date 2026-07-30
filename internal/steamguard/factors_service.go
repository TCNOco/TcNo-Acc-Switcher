package steamguard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"TcNo-Acc-Switcher/internal/passwordpolicy"
	"TcNo-Acc-Switcher/internal/steamguard/hwkey"
	"TcNo-Acc-Switcher/internal/steamguard/vault"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	ErrFactorCancelled         = errors.New("enrolment cancelled")
	ErrBackupKeyMissing        = errors.New("a backup key must be saved before enrolling this factor")
	ErrPasswordAlreadyEnrolled = errors.New("the password already opens this Steam Guard vault on its own")
	ErrKeyfileAlreadyEnrolled  = errors.New("a keyfile is already enrolled; remove it before adding another")
	ErrFactorNotRemovable      = errors.New("this way into the Steam Guard vault cannot be removed")
	ErrManagementAuthRequired  = errors.New("changing Steam Guard factors needs the vault password or another way in")
)

// VaultFactor is one enrolled way to open the vault, for display. Requires
// lists the factor kinds that must be presented together; Kind is the one the
// way in is named after, which is the thing the user holds rather than the
// password that may accompany it.
type VaultFactor struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	Kind             string   `json:"kind"`
	Requires         []string `json:"requires"`
	RequiresPassword bool     `json:"requiresPassword"`
	// Removable is false when taking this way in away would leave the user
	// unable to open the vault in practice, and Blocks says why so the screen
	// can explain instead of just greying out a button.
	Removable bool   `json:"removable"`
	Blocks    string `json:"blocks"`
	// LastPasswordWayIn marks the only way in that still uses the password.
	// Removing it leaves a vault with no password at all, which is allowed but
	// is not what a row labelled "Keyfile" looks like it would do.
	LastPasswordWayIn bool `json:"lastPasswordWayIn"`
}

// Reasons a way in cannot be removed. Sent to the UI as a stable token rather
// than a sentence, so the wording lives with the rest of the translations.
const (
	blockLastWayIn       = "last"
	blockLastInteractive = "lastInteractive"
	blockBackupNeeded    = "backupNeeded"
)

// VaultFactorStatus is what the settings screen needs to state plainly which
// combinations open the vault, and whether a backup key exists.
type VaultFactorStatus struct {
	Factors      []VaultFactor `json:"factors"`
	HasBackupKey bool          `json:"hasBackupKey"`
	PasswordOnly bool          `json:"passwordOnly"`
	// PasswordOpens reports whether some slot needs nothing but the password.
	// Unlike PasswordOnly this stays true once other factors are enrolled
	// alongside it, which is what tells a caller whether asking for a password
	// alone is enough to reopen the vault.
	PasswordOpens bool `json:"passwordOpens"`
	// KeyfileCount and SecurityKeyCount let the settings screen offer "add" or
	// "remove" per factor kind without re-deriving it from Factors. Keyfiles are
	// counted rather than flagged because an older build could enrol two, and a
	// single "Remove keyfile" button would silently leave one of them working.
	HasKeyfile       bool `json:"hasKeyfile"`
	KeyfileCount     int  `json:"keyfileCount"`
	SecurityKeyCount int  `json:"securityKeyCount"`
	CanRemoveAFactor bool `json:"canRemoveAFactor"`
}

func (s *Service) ListVaultFactors() (VaultFactorStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, exists, err := s.openVaultLocked()
	if err != nil {
		return VaultFactorStatus{}, err
	}
	if !exists {
		return VaultFactorStatus{}, vault.ErrNotFound
	}
	return summariseFactors(v.ListSlots()), nil
}

// primaryKind names a way in after the thing the user holds. A slot needing a
// password and a keyfile is a keyfile you also need the password for, not a
// password: the password is the accompaniment, and calling it anything else
// would put two rows in the settings list that mean the same slot.
func primaryKind(factors []string) string {
	for _, kind := range factors {
		if kind != vault.FactorPassword {
			return kind
		}
	}
	return vault.FactorPassword
}

func summariseFactors(slots []vault.SlotInfo) VaultFactorStatus {
	status := VaultFactorStatus{Factors: make([]VaultFactor, 0, len(slots))}
	// Counted first: whether a way in can go depends on what the others are.
	var withPasswordCount, interactiveCount, losableCount int
	for _, slot := range slots {
		if slices.Contains(slot.Factors, vault.FactorPassword) {
			withPasswordCount++
		}
		switch primaryKind(slot.Factors) {
		case vault.FactorRecoveryCode:
			status.HasBackupKey = true
		case vault.FactorKeyfile:
			status.KeyfileCount++
			interactiveCount++
			losableCount++
		case vault.FactorSecurityKey:
			status.SecurityKeyCount++
			interactiveCount++
			losableCount++
		case vault.FactorPassword:
			status.PasswordOpens = true
			interactiveCount++
		}
	}
	status.HasKeyfile = status.KeyfileCount > 0

	for _, slot := range slots {
		kind := primaryKind(slot.Factors)
		usesPassword := slices.Contains(slot.Factors, vault.FactorPassword)
		factor := VaultFactor{
			ID:               slot.ID,
			Label:            slot.Label,
			Kind:             kind,
			Requires:         slot.Factors,
			RequiresPassword: kind != vault.FactorPassword && usesPassword,
			Removable:        true,
		}
		switch {
		case len(slots) <= 1:
			factor.Removable, factor.Blocks = false, blockLastWayIn
		// A backup key is written on paper, not something the user can open the
		// app with. Leaving it as the only way in strands them until they find it.
		case kind != vault.FactorRecoveryCode && interactiveCount <= 1:
			factor.Removable, factor.Blocks = false, blockLastInteractive
		// The rule that a losable factor needs a backup key does not stop
		// applying the moment it is enrolled.
		case kind == vault.FactorRecoveryCode && losableCount > 0:
			factor.Removable, factor.Blocks = false, blockBackupNeeded
		}
		// Not a block: a vault opened only by a security key is a setup someone
		// may well want. But a row labelled "keyfile" does not say it is also
		// carrying the password, so removing it has to be confirmed as such.
		factor.LastPasswordWayIn = usesPassword && withPasswordCount <= 1
		status.Factors = append(status.Factors, factor)
	}
	status.PasswordOnly = len(slots) == 1 && status.PasswordOpens
	status.CanRemoveAFactor = false
	for _, factor := range status.Factors {
		if factor.Removable {
			status.CanRemoveAFactor = true
		}
	}
	return status
}

// CreateVaultBackupKey enrols a new backup key and returns it once. The caller
// must show it and confirm the user saved it; it cannot be retrieved later.
// Any existing backup key is replaced, so a key the user has lost stops working.
func (s *Service) CreateVaultBackupKey(password string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.unlockedVaultLocked(password)
	if err != nil {
		return "", err
	}
	code, err := vault.NewRecoveryCode()
	if err != nil {
		return "", err
	}
	raw, err := vault.ParseRecoveryCode(code)
	if err != nil {
		return "", err
	}
	defer wipe(raw)

	replaces := backupKeySlotIDs(v.ListSlots())
	if _, err := v.AddSlot(vault.SlotSpec{
		Label: "Backup key",
		Kinds: []string{vault.FactorRecoveryCode},
		Creds: vault.Credentials{RecoveryCode: raw},
	}, replaces); err != nil {
		return "", err
	}
	serviceLogger().Info("Steam Guard backup key issued", "replaced", len(replaces))
	return code, nil
}

// securityKeyTimeout bounds a single authenticator prompt. The user has to
// touch the key, so this is generous, but it must end rather than hang the
// operation lock forever if the prompt is never answered.
const securityKeyTimeout = 2 * time.Minute

// authenticator is a field so tests substitute a deterministic fake. Nil means
// the platform driver.
func (s *Service) securityKeyAuthenticator() hwkey.Authenticator {
	if s.authenticator != nil {
		return s.authenticator
	}
	return hwkey.New()
}

// SecurityKeySupport reports whether this system can use security keys, and why
// not when it cannot, so the UI can explain rather than offer an action that
// always fails. A struct rather than two values, because the bound surface pairs
// a result with an error.
type SecurityKeySupport struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}

func (s *Service) SecurityKeyAvailable() (SecurityKeySupport, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	available, reason := s.securityKeyAuthenticator().Available(ctx)
	return SecurityKeySupport{Available: available, Reason: reason}, nil
}

// EnrollVaultSecurityKey enrols a FIDO2 security key as a new way into the
// vault. name is what the settings list calls it, so the user can tell one key
// from another. keyPassword is optional: left empty the key opens the vault on
// its own, and set it becomes a password this key alone is used with.
//
// Nothing already enrolled is removed. Several keys can be enrolled, and each
// opens the vault independently, which is how a spare key kept elsewhere works.
//
// A backup key must already exist. A security key can be lost or reset, and the
// secret it derives cannot be extracted or copied, so without another way in
// that would make the vault permanently unreadable.
func (s *Service) EnrollVaultSecurityKey(password, name, keyPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.unlockedVaultLocked(password)
	if err != nil {
		return err
	}
	slots := v.ListSlots()
	if !summariseFactors(slots).HasBackupKey {
		return ErrBackupKeyMissing
	}
	// Validated before the device is touched, so a typo does not cost the user a
	// key ceremony and leave a slot nothing can open. The label counts too: it is
	// bounded in bytes by the header, and a name in a non-Latin script used to
	// pass here and be refused as a corrupt vault after two touches.
	if keyPassword != "" {
		if err := passwordpolicy.ValidateNew(keyPassword); err != nil {
			return err
		}
	}
	label := uniqueFactorName(name, "Security key", slots)
	if len(label) > vault.MaxSlotLabelBytes {
		return vault.ErrInvalidFormat
	}

	// The ceremony waits on the user twice. Held under the service lock it
	// blocked Lock Now, app-lock and every other Steam Guard call for as long as
	// nobody touched the key.
	var cred hwkey.Credential
	var secret []byte
	s.promptWithoutServiceLock(func() {
		device := s.securityKeyAuthenticator()
		// A deadline each. Sharing one meant a slow first ceremony - a new key
		// being given a PIN - spent the budget the second one needs, and the
		// credential is already written to the hardware by then: the enrolment
		// fails leaving an orphaned credential on the key.
		enrolCtx, cancelEnrol := context.WithTimeout(context.Background(), securityKeyTimeout)
		defer cancelEnrol()
		if cred, err = device.Enroll(enrolCtx, hwkey.RPID, "Steam Guard vault"); err != nil {
			return
		}
		// Derived immediately, so a key that enrols but cannot produce a secret
		// fails before a slot exists that nothing can open.
		evalCtx, cancelEval := context.WithTimeout(context.Background(), securityKeyTimeout)
		defer cancelEval()
		_, secret, err = device.Evaluate(evalCtx, []hwkey.Credential{cred})
	})
	if err != nil {
		return err
	}
	defer wipe(secret)
	if len(secret) != hwkey.SecretLength {
		return fmt.Errorf("%w: security key returned no usable key material", hwkey.ErrUnavailable)
	}
	// Re-read: the vault can have been locked while the ceremony was up.
	if v, err = s.reopenedVaultLocked(); err != nil {
		return err
	}

	kinds := []string{vault.FactorSecurityKey}
	creds := vault.Credentials{SecurityKey: secret}
	if keyPassword != "" {
		kinds = []string{vault.FactorPassword, vault.FactorSecurityKey}
		creds.Password = keyPassword
	}
	if _, err := v.AddSlot(vault.SlotSpec{
		Label:        label,
		Kinds:        kinds,
		Creds:        creds,
		CredentialID: cred.ID,
		RPID:         cred.RPID,
		UV:           cred.UV,
	}, nil); err != nil {
		return err
	}
	serviceLogger().Info("Steam Guard security key enrolled", "requiresPassword", keyPassword != "")
	return nil
}

// factorName keeps a user-chosen label short and free of the control characters
// and newlines a display list would render badly.
func factorName(name, fallback string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, strings.TrimSpace(name))
	if cleaned == "" {
		return fallback
	}
	return truncateToBytes(cleaned, maxFactorNameBytes)
}

// maxFactorNameBytes leaves room inside the vault's own label limit for the
// " (12)" a duplicate name picks up. Bytes, not runes: the vault counts bytes,
// so a rune cap let a name in Japanese through here and had it rejected as a
// corrupt vault - after the user had already touched their security key twice.
const maxFactorNameBytes = vault.MaxSlotLabelBytes - 8

// truncateToBytes cuts a string to a byte budget without splitting a rune.
func truncateToBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// uniqueFactorName keeps two keys from sharing a row label. Two rows reading
// "Security key" with a Remove link each is a coin flip the user only discovers
// by finding out which device stopped working.
func uniqueFactorName(name, fallback string, existing []vault.SlotInfo) string {
	base := factorName(name, fallback)
	taken := make(map[string]bool, len(existing))
	for _, slot := range existing {
		taken[slot.Label] = true
	}
	if !taken[base] {
		return base
	}
	for n := 2; n < 100; n++ {
		candidate := fmt.Sprintf("%s (%d)", base, n)
		if !taken[candidate] {
			return candidate
		}
	}
	return base
}

// evaluateSecurityKeyLocked offers every enrolled credential at once and returns
// the secret of whichever key is actually attached, along with the handle that
// answered. Several keys can be enrolled - a spare kept elsewhere is the point -
// and the user is holding one of them, so the driver is asked to match rather
// than being interrogated per credential.
//
// Callers hold s.mu. It is released for the ceremony itself: the prompt waits on
// a person, and holding the service lock across it made Lock Now, app-lock and
// every other Steam Guard call wait with them - so the vault stayed open on an
// unattended machine for exactly as long as nobody answered. Vault state can
// change while it is released, so callers re-read it afterwards rather than
// trusting anything they read before.
func (s *Service) evaluateSecurityKeyLocked(v *vault.Vault) (string, []byte, error) {
	refs := v.SecurityKeys()
	if len(refs) == 0 {
		return "", nil, nil
	}
	device := s.securityKeyAuthenticator()
	creds := make([]hwkey.Credential, 0, len(refs))
	for _, ref := range refs {
		creds = append(creds, hwkey.Credential{ID: ref.CredentialID, RPID: ref.RPID, UV: ref.UV})
	}

	s.mu.Unlock()
	defer s.mu.Lock()
	if ok, reason := device.Available(context.Background()); !ok {
		return "", nil, fmt.Errorf("%w: %s", hwkey.ErrUnavailable, reason)
	}
	ctx, cancel := context.WithTimeout(context.Background(), securityKeyTimeout)
	defer cancel()
	answered, secret, err := device.Evaluate(ctx, creds)
	if err != nil {
		return "", nil, err
	}
	return answered.ID, secret, nil
}

// PickVaultKeyfile asks the user for their keyfile and returns its path. Only
// the path crosses to the frontend; the contents are read by Go at unlock.
// Returns an empty string when the dialog is cancelled, so a cancel is not an
// error the caller has to special-case.
func (s *Service) PickVaultKeyfile() (string, error) {
	app := application.Get()
	if app == nil {
		return "", ErrFactorCancelled
	}
	dialog := app.Dialog.OpenFile()
	dialog.SetMessage("Select your Steam Guard keyfile")
	dialog.AddFilter("Text file", "*.txt")
	dialog.CanChooseFiles(true)
	if owner := dialogOwnerWindow(); owner != nil {
		dialog.AttachToWindow(owner)
	}
	path, err := dialog.PromptForSingleSelection()
	logDialogOutcome("pick-keyfile", path != "", err)
	if err != nil {
		if dialogCancelled(err) {
			return "", nil
		}
		return "", err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if !filepath.IsAbs(path) {
		return "", vault.ErrInvalidKeyfile
	}
	// Parsed now so a wrong file is reported while the user is still looking at
	// the picker, rather than as a failed unlock later.
	raw, readErr := os.ReadFile(path)
	if readErr != nil {
		return "", errors.Join(vault.ErrInvalidKeyfile, readErr)
	}
	defer wipe(raw)
	keyfile, parseErr := vault.ParseKeyfile(raw)
	if parseErr != nil {
		return "", parseErr
	}
	wipe(keyfile.Secret)
	return path, nil
}

// SaveVaultBackupKey writes a backup key to a file the user picks. Go writes it
// rather than the webview offering a download, because the navigation guard
// blocks the anchor a browser-style download needs, and a native save dialog is
// what a desktop app should be doing anyway.
func (s *Service) SaveVaultBackupKey(code string) (string, error) {
	// Parsed, not just non-empty: this refuses to write a file that would not
	// open the vault, which is worse than writing nothing.
	raw, err := vault.ParseRecoveryCode(code)
	if err != nil {
		return "", err
	}
	wipe(raw)

	app := application.Get()
	if app == nil {
		return "", ErrFactorCancelled
	}
	dialog := app.Dialog.SaveFile()
	dialog.SetMessage("Save Steam Guard backup key")
	dialog.SetFilename("steam-guard-backup-key.txt")
	dialog.AddFilter("Text file", "*.txt")
	if owner := dialogOwnerWindow(); owner != nil {
		dialog.AttachToWindow(owner)
	}
	path, dialogErr := dialog.PromptForSingleSelection()
	logDialogOutcome("save-backup-key", path != "", dialogErr)
	if dialogErr != nil {
		if dialogCancelled(dialogErr) {
			return "", ErrFactorCancelled
		}
		return "", dialogErr
	}
	if strings.TrimSpace(path) == "" {
		return "", ErrFactorCancelled
	}
	if !filepath.IsAbs(path) {
		return "", ErrInvalidBackupDestination
	}
	if err := os.WriteFile(path, backupKeyFileContents(code), 0o600); err != nil {
		return "", err
	}
	serviceLogger().Info("Steam Guard backup key saved to a file")
	return path, nil
}

// backupKeyFileContents keeps the warning with the key, because the file will
// outlive whatever the screen said when it was written.
func backupKeyFileContents(code string) []byte {
	var b strings.Builder
	b.WriteString("TcNo Account Switcher Steam Guard backup key\n\n")
	b.WriteString("Anyone holding this key can open the Steam Guard vault it\n")
	b.WriteString("belongs to. Store it away from this PC. Issuing a new backup\n")
	b.WriteString("key replaces this one.\n\n")
	b.WriteString(code)
	b.WriteString("\n")
	return []byte(b.String())
}

func backupKeySlotIDs(slots []vault.SlotInfo) []string {
	var ids []string
	for _, slot := range slots {
		if len(slot.Factors) == 1 && slot.Factors[0] == vault.FactorRecoveryCode {
			ids = append(ids, slot.ID)
		}
	}
	return ids
}

// EnrollVaultKeyfile generates a keyfile, saves it where the user chooses, and
// enrols it as a new way into the vault. keyfilePassword is optional: left empty
// the keyfile opens the vault on its own, and set it becomes a password this
// keyfile alone is used with.
//
// Nothing already enrolled is removed, so the password keeps working exactly as
// it did. Taking the password away is a separate, deliberate act.
//
// A backup key must already exist. Losing the only keyfile would otherwise make
// the vault unreadable with no way back.
func (s *Service) EnrollVaultKeyfile(password, keyfilePassword string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.unlockedVaultLocked(password)
	if err != nil {
		return "", err
	}
	summary := summariseFactors(v.ListSlots())
	if !summary.HasBackupKey {
		return "", ErrBackupKeyMissing
	}
	// One keyfile, so that "Remove keyfile" can mean what it says. Two would
	// leave a file on disk still opening the vault after the user believed they
	// had revoked it.
	if summary.HasKeyfile {
		return "", ErrKeyfileAlreadyEnrolled
	}
	if keyfilePassword != "" {
		if err := passwordpolicy.ValidateNew(keyfilePassword); err != nil {
			return "", err
		}
	}
	keyfile, err := vault.NewKeyfile()
	if err != nil {
		return "", err
	}
	defer wipe(keyfile.Secret)
	save := s.saveKeyfile
	if save == nil {
		save = saveKeyfileDialog
	}
	// The save dialog waits on the user with no timeout of its own. Holding the
	// service lock across it left the vault open and unlockable - Lock Now and
	// app-lock both wait on this mutex - for as long as the dialog sat there.
	var path string
	s.promptWithoutServiceLock(func() { path, err = save(keyfile) })
	if err != nil {
		return "", err
	}
	// Re-read after the lock was open: the vault can have been locked, or a
	// keyfile enrolled, while the dialog was up.
	if v, err = s.reopenedVaultLocked(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if summariseFactors(v.ListSlots()).HasKeyfile {
		_ = os.Remove(path)
		return "", ErrKeyfileAlreadyEnrolled
	}

	kinds := []string{vault.FactorKeyfile}
	creds := vault.Credentials{Keyfile: keyfile.Secret}
	if keyfilePassword != "" {
		kinds = []string{vault.FactorPassword, vault.FactorKeyfile}
		creds.Password = keyfilePassword
	}
	if _, err := v.AddSlot(vault.SlotSpec{
		Label:     "Keyfile",
		Kinds:     kinds,
		Creds:     creds,
		KeyfileID: keyfile.ID,
	}, nil); err != nil {
		// The file on disk is now useless; leaving it would look enrolled.
		_ = os.Remove(path)
		return "", err
	}
	serviceLogger().Info("Steam Guard keyfile enrolled", "requiresPassword", keyfilePassword != "")
	return path, nil
}

// EnrollVaultPassword adds a password that opens the vault on its own.
//
// This is how someone gets a password back. A vault can legitimately end up with
// none - the user removed it in favour of a security key, or an older build
// folded the password into a keyfile slot and left no standalone one - and
// without this there would be no way to add one short of rebuilding the vault.
func (s *Service) EnrollVaultPassword(password, newPassword string) error {
	if err := passwordpolicy.ValidateNew(newPassword); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.unlockedVaultLocked(password)
	if err != nil {
		return err
	}
	if summariseFactors(v.ListSlots()).PasswordOpens {
		return ErrPasswordAlreadyEnrolled
	}
	if _, err := v.AddSlot(vault.SlotSpec{
		Label: "Password",
		Kinds: []string{vault.FactorPassword},
		Creds: vault.PasswordOnly(newPassword),
	}, nil); err != nil {
		return err
	}
	serviceLogger().Info("Steam Guard password enrolled as a way in")
	return nil
}

// pairsPasswordWithSecurityKey reports whether some way in needs both, which is
// the case a password change cannot complete without the device.
func pairsPasswordWithSecurityKey(slots []vault.SlotInfo) bool {
	for _, slot := range slots {
		if slices.Contains(slot.Factors, vault.FactorPassword) &&
			slices.Contains(slot.Factors, vault.FactorSecurityKey) {
			return true
		}
	}
	return false
}

func passwordOnlySlotIDs(slots []vault.SlotInfo) []string {
	var ids []string
	for _, slot := range slots {
		if len(slot.Factors) == 1 && slot.Factors[0] == vault.FactorPassword {
			ids = append(ids, slot.ID)
		}
	}
	return ids
}

// RemoveVaultFactor drops one enrolled way in.
//
// The vault itself refuses only to remove the very last slot. The rules that
// stop a user stranding themselves in practice - do not leave a paper backup key
// as the only way in, do not take the backup key away while a losable factor is
// enrolled - are decided here, and they are decided here rather than only in the
// settings screen because the outcome of getting them wrong is a vault nobody
// can read. Enrolment already enforces its half server-side; removal is the
// direction that loses data.
func (s *Service) RemoveVaultFactor(password, slotID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.unlockedVaultLocked(password)
	if err != nil {
		return err
	}
	slotID = strings.TrimSpace(slotID)
	for _, factor := range summariseFactors(v.ListSlots()).Factors {
		if factor.ID != slotID || factor.Removable {
			continue
		}
		// The last way in has a sentinel of its own that callers already match;
		// only the practical-lockout rules need the new one.
		if factor.Blocks == blockLastWayIn {
			return vault.ErrLastSlot
		}
		return fmt.Errorf("%w: %s", ErrFactorNotRemovable, factor.Blocks)
	}
	if err := v.RemoveSlot(slotID); err != nil {
		return err
	}
	serviceLogger().Info("Steam Guard factor removed")
	return nil
}

// unlockedVaultLocked opens and unlocks the vault with the password alone.
// Enrolment always authenticates this way, so a user who cannot present the
// password cannot change which factors exist.
func (s *Service) unlockedVaultLocked(password string) (*vault.Vault, error) {
	return s.unlockedVaultWithLocked(vault.Credentials{Password: password})
}

// managementWindow is how long a verified management authentication stays good
// for the actions that follow it, refreshed each time it is used.
//
// It has to outlive the dialogs the user walks through between authenticating
// and the change landing, and those are paced by a person: adding a keyfile can
// mint a backup key first and stop to have it written down. Two minutes cut that
// flow in half. What it still bounds is the case it exists for - an unlocked
// vault, remembered for the whole session, answering factor changes to a caller
// that never presented anything.
const managementWindow = 10 * time.Minute

// credentialsSupplied reports whether the caller offered anything at all to
// check. Nothing to check is not the same as a wrong answer: a vault with no
// password is managed with an empty one.
func credentialsSupplied(creds vault.Credentials) bool {
	return creds.Password != "" || len(creds.Keyfile) != 0 ||
		len(creds.RecoveryCode) != 0 || len(creds.SecurityKey) != 0
}

// reopenedVaultLocked returns the vault to a caller that already authorised for
// it and then ran a prompt with s.mu released. Authorisation happened before the
// prompt and is not asked for again: the user has been standing in a dialog, and
// the management window can have lapsed while they were. What does have to be
// rechecked is that the vault is still open, because Lock Now or the app lock
// can have fired while the lock was free.
func (s *Service) reopenedVaultLocked() (*vault.Vault, error) {
	v, exists, err := s.openVaultLocked()
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, vault.ErrNotFound
	}
	if v.IsLocked() {
		return nil, vault.ErrLocked
	}
	return v, nil
}

// withServiceLock runs fn under s.mu, for a caller that reached a vault-mutating
// step from a path that does not already hold it. fn must not take s.mu itself.
func (s *Service) withServiceLock(fn func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fn()
}

// promptWithoutServiceLock runs a call that waits on the user - a native dialog,
// an authenticator ceremony - with s.mu released, retaking it before returning.
// Callers hold s.mu and must re-read any vault state they cared about: it can
// have changed while the lock was open.
func (s *Service) promptWithoutServiceLock(fn func()) {
	s.mu.Unlock()
	defer s.mu.Lock()
	fn()
}

func (s *Service) openManagementGateLocked() {
	s.managementVerified = time.Now().Add(managementWindow)
}

func (s *Service) closeManagementGateLocked() {
	s.managementVerified = time.Time{}
}

func (s *Service) managementGateOpenLocked() bool {
	return !s.managementVerified.IsZero() && time.Now().Before(s.managementVerified)
}

// unlockedVaultWithLocked returns an unlocked vault to a caller that is about to
// change which factors exist, having established that the caller can actually
// present a way in.
//
// Management has to accept every factor, not just the password. The unlock lease
// is five minutes, so by the time someone reaches the settings screen the vault
// is usually locked again, and on a vault whose slot needs a password and a
// keyfile a password alone cannot reopen it - which made every factor action
// impossible once the first combined factor was enrolled.
//
// An already-open vault is not authorisation. With "remember for session" the
// lease lasts the whole session, so returning the vault to anyone who asked let
// a caller with no credentials at all mint a backup key, or drop a factor. The
// gate is what proves someone was asked; the credentials are checked directly
// when the caller has any, and the gate covers the case where the answer was
// given to UnlockVaultForManagement a moment ago and cannot be repeated here
// without prompting the user for a second key touch.
func (s *Service) unlockedVaultWithLocked(creds vault.Credentials) (*vault.Vault, error) {
	v, exists, err := s.openVaultLocked()
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, vault.ErrNotFound
	}
	if v.IsLocked() {
		if err := s.unlockWithSecurityKeyFallbackLocked(v, &creds); err != nil {
			return nil, err
		}
		defer wipe(creds.SecurityKey)
		s.openManagementGateLocked()
		return v, nil
	}
	// Checked before the credentials so the common path - the settings screen
	// authenticating and then acting - never raises a second device prompt.
	// Using it refreshes it: a flow that keeps working is a user still present,
	// and several actions can follow one authentication.
	if s.managementGateOpenLocked() {
		s.openManagementGateLocked()
		return v, nil
	}
	if !credentialsSupplied(creds) {
		return nil, ErrManagementAuthRequired
	}
	if err := s.verifyCredentialsLocked(v, creds); err != nil {
		return nil, err
	}
	s.openManagementGateLocked()
	return v, nil
}

// verifyCredentialsLocked checks the supplied factors against the enrolled
// slots, asking the security key if nothing else was supplied.
func (s *Service) verifyCredentialsLocked(v *vault.Vault, creds vault.Credentials) error {
	err := v.VerifyCredentials(creds)
	if err == nil || len(creds.SecurityKey) != 0 || !needsSecurityKey(err) {
		return err
	}
	credID, secret, keyErr := s.evaluateSecurityKeyLocked(v)
	if keyErr != nil || len(secret) == 0 {
		return joinSecurityKeyFailure(err, keyErr, creds)
	}
	defer wipe(secret)
	creds.SecurityKey = secret
	creds.SecurityKeyID = credID
	return v.VerifyCredentials(creds)
}

// unlockWithSecurityKeyFallbackLocked unlocks v with creds, asking an enrolled
// security key for its secret if the slots need one and the caller had no way to
// supply it. The evaluated secret is written back into creds so a caller that
// has to rebuild slots afterwards does not prompt the user for a second tap;
// creds.SecurityKey belongs to the caller to wipe once it is done with it.
func (s *Service) unlockWithSecurityKeyFallbackLocked(v *vault.Vault, creds *vault.Credentials) error {
	err := s.unlockVaultWithLocked(v, *creds, false)
	if err == nil || len(creds.SecurityKey) != 0 || !needsSecurityKey(err) {
		return err
	}
	credID, secret, keyErr := s.evaluateSecurityKeyLocked(v)
	if keyErr != nil || len(secret) == 0 {
		return joinSecurityKeyFailure(err, keyErr, *creds)
	}
	creds.SecurityKey = secret
	creds.SecurityKeyID = credID
	return s.unlockVaultWithLocked(v, *creds, false)
}

// fillSecurityKeyLocked adds an enrolled security key's secret to creds when
// what the caller supplied does not open v on its own.
//
// Backup and restore rekey a copy of the header, which re-derives every factor
// of every slot the credentials can open. The security-key descriptors travel
// with a backup exactly so an enrolled key still opens a copy on another
// machine - but neither path ever asked the device, so a vault whose only
// interactive way in is a key could be backed up only by transcribing the paper
// backup key with the key sitting in the port. Failure is silent on purpose:
// the caller reports what the credentials could not do, not that a key it may
// not have was not offered.
func (s *Service) fillSecurityKeyLocked(v *vault.Vault, creds *vault.Credentials) {
	if len(creds.SecurityKey) != 0 {
		return
	}
	if err := v.VerifyCredentials(*creds); err == nil || !needsSecurityKey(err) {
		return
	}
	credID, secret, keyErr := s.evaluateSecurityKeyLocked(v)
	if keyErr != nil || len(secret) == 0 {
		return
	}
	creds.SecurityKey = secret
	creds.SecurityKeyID = credID
}

// joinSecurityKeyFailure reports why the key was no help when the key was the
// only thing that could have helped. The unlock error underneath describes a
// password that was never typed, so a cancelled prompt or an unenrolled key
// reached the user as "password not accepted"; where the caller supplied
// something else to be wrong about, that error is still the honest one.
func joinSecurityKeyFailure(unlockErr, keyErr error, creds vault.Credentials) error {
	if keyErr == nil || credentialsSupplied(creds) {
		return unlockErr
	}
	return errors.Join(unlockErr, keyErr)
}

// RenameVaultFactor changes what a way in is called in the list. Nothing about
// the vault's security changes; it is how two security keys stop being two rows
// reading "Security key".
func (s *Service) RenameVaultFactor(password, factorID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.unlockedVaultLocked(password)
	if err != nil {
		return err
	}
	factorID = strings.TrimSpace(factorID)
	label := uniqueFactorName(name, "Security key", excludeSlot(v.ListSlots(), factorID))
	return v.RenameSlot(factorID, label)
}

// excludeSlot drops one slot from a list, so a rename that keeps the same name
// is not treated as a collision with itself.
func excludeSlot(slots []vault.SlotInfo, id string) []vault.SlotInfo {
	kept := make([]vault.SlotInfo, 0, len(slots))
	for _, slot := range slots {
		if slot.ID != id {
			kept = append(kept, slot)
		}
	}
	return kept
}

// UnlockVaultForManagement opens the vault with every factor the user can
// present, so the factor actions that follow work from an open vault instead of
// each having to re-authenticate. Enrolling and removing factors need the vault
// key, and the password is only one of the ways to reach it.
//
// This is the gate the settings screen puts in front of every factor change. The
// vault may well already be open from an earlier action, in which case unlocking
// proves nothing and the credentials are checked directly instead - an answer
// that is never checked is worse than not asking.
func (s *Service) UnlockVaultForManagement(password, keyfilePath, backupKey string) error {
	creds, err := buildVaultCredentials(password, keyfilePath, backupKey)
	if err != nil {
		return err
	}
	defer wipe(creds.Keyfile)
	defer wipe(creds.RecoveryCode)
	s.mu.Lock()
	defer s.mu.Unlock()
	v, exists, err := s.openVaultLocked()
	if err != nil {
		return err
	}
	if !exists {
		return vault.ErrNotFound
	}
	if v.IsLocked() {
		// Unlocking with these credentials is itself the proof, so the answer is
		// not demanded a second time. It used to be, and on a key-only vault
		// that meant two Windows Hello prompts for one management action,
		// because the evaluated secret was wiped before it could be reused.
		if err := s.unlockWithSecurityKeyFallbackLocked(v, &creds); err != nil {
			return err
		}
		defer wipe(creds.SecurityKey)
	} else if err := s.verifyCredentialsLocked(v, creds); err != nil {
		return err
	}
	s.openManagementGateLocked()
	return nil
}

// saveKeyfileDialog writes the keyfile itself rather than handing the secret to
// the frontend, so the material never enters the webview.
func saveKeyfileDialog(keyfile vault.Keyfile) (string, error) {
	app := application.Get()
	if app == nil {
		return "", ErrFactorCancelled
	}
	dialog := app.Dialog.SaveFile()
	dialog.SetMessage("Save Steam Guard keyfile")
	dialog.SetFilename("steam-guard-keyfile.txt")
	dialog.AddFilter("Text file", "*.txt")
	if owner := dialogOwnerWindow(); owner != nil {
		dialog.AttachToWindow(owner)
	}
	path, err := dialog.PromptForSingleSelection()
	logDialogOutcome("save-keyfile", path != "", err)
	if err != nil {
		if dialogCancelled(err) {
			return "", ErrFactorCancelled
		}
		return "", err
	}
	if strings.TrimSpace(path) == "" {
		return "", ErrFactorCancelled
	}
	if !filepath.IsAbs(path) {
		return "", ErrInvalidBackupDestination
	}
	encoded := keyfile.Encode()
	defer wipe(encoded)
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return "", err
	}
	return path, nil
}
