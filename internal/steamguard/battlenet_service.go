package steamguard

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/battlenetauth"
	"TcNo-Acc-Switcher/internal/passwordpolicy"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/steamguard/qrimage"
	"TcNo-Acc-Switcher/internal/steamguard/qrrender"
	"TcNo-Acc-Switcher/internal/steamguard/securefile"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
	"github.com/makiuchi-d/gozxing"
	multiqr "github.com/makiuchi-d/gozxing/multi/qrcode"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const battleNetView = "battlenet-authenticators"

type BattleNetAccountSummary struct {
	AccountLink          string `json:"accountLink"`
	Serial               string `json:"serial"`
	Label                string `json:"label"`
	Pending              bool   `json:"pending"`
	HasRestoreCode       bool   `json:"hasRestoreCode"`
	RequiresAccountSetup bool   `json:"requiresAccountSetup"`
}

type BattleNetCodeView struct {
	Code      string `json:"code"`
	ExpiresAt int64  `json:"expiresAt"`
}

type BattleNetRecoveryView struct {
	Serial      string `json:"serial"`
	RestoreCode string `json:"restoreCode"`
	URI         string `json:"uri"`
	QRImage     string `json:"qrImage"`
}

func (BattleNetRecoveryView) String() string   { return "Battle.net recovery details (redacted)" }
func (BattleNetRecoveryView) GoString() string { return "Battle.net recovery details (redacted)" }

func (s *Service) RevealBattleNetRecovery(serial, token string) (BattleNetRecoveryView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.battleNetVaultLocked(token)
	if err != nil {
		return BattleNetRecoveryView{}, err
	}
	a, err := battleNetRecord(v, serial)
	if err != nil {
		return BattleNetRecoveryView{}, err
	}
	uri, err := a.URI()
	if err != nil {
		return BattleNetRecoveryView{}, err
	}
	image, err := qrrender.SVGDataURI(uri)
	if err != nil {
		return BattleNetRecoveryView{}, err
	}
	return BattleNetRecoveryView{a.Serial, a.RestoreCode, uri, image}, nil
}

func (BattleNetCodeView) String() string   { return "Battle.net code (redacted)" }
func (BattleNetCodeView) GoString() string { return "Battle.net code (redacted)" }

func (s *Service) RequestBattleNetView(requestID string) error {
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	return s.issueSensitiveView(battleNetView, requestID)
}

func (s *Service) battleNetVaultLocked(token string) (*vault.Vault, error) {
	if err := security.RequireUnlocked(); err != nil {
		return nil, err
	}
	v, exists, err := s.openVaultLocked()
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrVaultNotReady
	}
	if err := s.authorizeModalLocked(v, battleNetView, token); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *Service) UnlockBattleNet(password, keyfilePath, backupKey string, remember bool, token string) error {
	creds, err := buildVaultCredentials(password, keyfilePath, backupKey)
	if err != nil {
		return err
	}
	defer wipe(creds.Keyfile)
	defer wipe(creds.RecoveryCode)
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.battleNetVaultLocked(token)
	if err != nil {
		return err
	}
	if !v.IsLocked() {
		return nil
	}
	err = s.unlockVaultWithLocked(v, creds, remember)
	if needsSecurityKey(err) {
		id, secret, keyErr := s.evaluateSecurityKeyLocked(v)
		if keyErr != nil {
			return keyErr
		}
		defer wipe(secret)
		creds.SecurityKeyID = id
		creds.SecurityKey = secret
		err = s.unlockVaultWithLocked(v, creds, remember)
	}
	return err
}

func battleNetSummary(a battlenetauth.Account) BattleNetAccountSummary {
	return BattleNetAccountSummary{Serial: a.Serial, Label: a.DisplayName(), Pending: a.Pending, HasRestoreCode: a.RestoreCode != "", AccountLink: a.AccountLink, RequiresAccountSetup: a.RequiresAccountSetup}
}

func (s *Service) ListBattleNetAccounts(token string) ([]BattleNetAccountSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.battleNetVaultLocked(token)
	if err != nil {
		return nil, err
	}
	records, err := v.ListProviderRecords(vault.ProviderBattleNet)
	if err != nil {
		return nil, err
	}
	out := make([]BattleNetAccountSummary, 0, len(records))
	for _, record := range records {
		raw, err := v.GetRecord(record.ID)
		if err != nil {
			return nil, err
		}
		a, err := battlenetauth.ParseAccount(raw)
		wipe(raw)
		if err != nil {
			return nil, err
		}
		if record.SteamID64 != "battlenet:"+a.Serial {
			return nil, battlenetauth.ErrInvalid
		}
		out = append(out, battleNetSummary(a))
	}
	return out, nil
}

func battleNetRecord(v *vault.Vault, serial string) (battlenetauth.Account, error) {
	identity, err := vault.ProviderIdentity(vault.ProviderBattleNet, serial)
	if err != nil {
		return battlenetauth.Account{}, err
	}
	records, err := v.ListProviderRecords(vault.ProviderBattleNet)
	if err != nil {
		return battlenetauth.Account{}, err
	}
	for _, record := range records {
		if record.SteamID64 == identity {
			raw, err := v.GetRecord(record.ID)
			if err != nil {
				return battlenetauth.Account{}, err
			}
			defer wipe(raw)
			a, err := battlenetauth.ParseAccount(raw)
			if err != nil {
				return a, err
			}
			if a.Serial != serial {
				return battlenetauth.Account{}, battlenetauth.ErrInvalid
			}
			return a, nil
		}
	}
	return battlenetauth.Account{}, ErrAccountNotFound
}

func (s *Service) GetBattleNetCode(serial, token string) (BattleNetCodeView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.battleNetVaultLocked(token)
	if err != nil {
		return BattleNetCodeView{}, err
	}
	a, err := battleNetRecord(v, serial)
	if err != nil {
		return BattleNetCodeView{}, err
	}
	code, expires, err := a.Code(time.Now())
	return BattleNetCodeView{code, expires * 1000}, err
}

func (s *Service) CopyBattleNetCode(serial, token string) error {
	view, err := s.GetBattleNetCode(serial, token)
	if err != nil {
		return err
	}
	return s.copyBattleNetCodeView(view)
}

func (s *Service) CopyBattleNetCodeForAccount(serial, accountID, token string) error {
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
	if strings.TrimSpace(accountID) == "" || !strings.EqualFold(strings.TrimSpace(a.AccountLink), strings.TrimSpace(accountID)) {
		s.mu.Unlock()
		return errors.New("this authenticator is no longer linked to the selected account")
	}
	code, expires, err := a.Code(time.Now())
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return s.copyBattleNetCodeView(BattleNetCodeView{Code: code, ExpiresAt: expires * 1000})
}

func (s *Service) copyBattleNetCodeView(view BattleNetCodeView) error {
	defer func() { view.Code = "" }()
	s.mu.Lock()
	clipboard, ok := s.clipboard.(interface {
		CopyNumeric(string, time.Duration) error
	})
	s.mu.Unlock()
	if !ok {
		return ErrSensitiveView
	}
	return clipboard.CopyNumeric(view.Code, time.Until(time.UnixMilli(view.ExpiresAt)))
}

func readBattleNetImport(source string) ([]byte, error) {
	if strings.HasPrefix(source, "otpauth:") {
		if len(source) > 8192 {
			return nil, battlenetauth.ErrInvalid
		}
		return []byte(source), nil
	}
	if !filepath.IsAbs(source) {
		return nil, ErrPathNotAbsolute
	}
	if err := validateCredentialImportPath(source); err != nil {
		return nil, err
	}
	switch strings.ToLower(filepath.Ext(source)) {
	case ".png", ".jpg", ".jpeg":
		return readBattleNetQRImage(source)
	}
	f, err := os.Open(source)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > battlenetauth.MaxImportSize {
		return nil, battlenetauth.ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(f, battlenetauth.MaxImportSize+1))
	if err != nil || len(raw) > battlenetauth.MaxImportSize {
		wipe(raw)
		return nil, battlenetauth.ErrInvalid
	}
	return raw, nil
}

func readBattleNetQRImage(source string) (payload []byte, resultErr error) {
	defer func() {
		if recover() != nil {
			wipe(payload)
			payload = nil
			resultErr = battlenetauth.ErrInvalid
		}
	}()
	frame, err := qrimage.Load(source)
	if err != nil {
		return nil, battlenetauth.ErrInvalid
	}
	defer frame.Wipe()
	bitmap, err := gozxing.NewBinaryBitmapFromImage(frame.Image())
	if err != nil {
		return nil, battlenetauth.ErrInvalid
	}
	results, err := multiqr.NewQRCodeMultiReader().DecodeMultiple(bitmap, nil)
	if err != nil || len(results) != 1 {
		return nil, battlenetauth.ErrInvalid
	}
	text := results[0].GetText()
	if _, err := battlenetauth.ParseURI(text); err != nil {
		return nil, err
	}
	return []byte(text), nil
}

func validateCredentialImportPath(path string) error {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		linked, err := securefile.IsReparsePoint(current)
		if err != nil {
			return err
		}
		if linked {
			return battlenetauth.ErrInvalid
		}
		if filepath.Dir(current) == current {
			return nil
		}
	}
}

func (s *Service) PreviewBattleNetImport(source, password, token string) ([]BattleNetAccountSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.battleNetVaultLocked(token)
	if err != nil {
		return nil, err
	}
	if v.IsLocked() {
		return nil, vault.ErrLocked
	}
	raw, err := readBattleNetImport(source)
	if err != nil {
		return nil, err
	}
	defer wipe(raw)
	accounts, err := battlenetauth.Import(raw, password)
	if err != nil {
		return nil, err
	}
	out := make([]BattleNetAccountSummary, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, battleNetSummary(a))
	}
	return out, nil
}

func (s *Service) ImportBattleNet(source, password string, selected []string, replace bool, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.battleNetVaultLocked(token)
	if err != nil {
		return err
	}
	raw, err := readBattleNetImport(source)
	if err != nil {
		return err
	}
	defer wipe(raw)
	accounts, err := battlenetauth.Import(raw, password)
	if err != nil {
		return err
	}
	existing, err := v.ListProviderRecords(vault.ProviderBattleNet)
	if err != nil {
		return err
	}
	wanted := map[string]bool{}
	for _, serial := range selected {
		if wanted[serial] {
			return battlenetauth.ErrInvalid
		}
		wanted[serial] = true
	}
	updates := []vault.RecordUpdate{}
	defer func() {
		for _, update := range updates {
			wipe(update.Plaintext)
		}
	}()
	for _, a := range accounts {
		if !wanted[a.Serial] {
			continue
		}
		delete(wanted, a.Serial)
		identity, _ := vault.ProviderIdentity(vault.ProviderBattleNet, a.Serial)
		for _, record := range existing {
			if record.SteamID64 == identity && !replace {
				return errors.New("an authenticator with this serial already exists; select replacement explicitly")
			}
		}
		data, err := json.Marshal(a)
		if err != nil {
			return err
		}
		updates = append(updates, vault.RecordUpdate{SteamID64: identity, Plaintext: data})
	}
	if len(wanted) != 0 || len(updates) == 0 {
		return battlenetauth.ErrInvalid
	}
	return v.PutRecords(updates)
}

func (s *Service) PickBattleNetImport() (string, error) {
	app := application.Get()
	if app == nil {
		return "", ErrUnsupportedInput
	}
	dialog := app.Dialog.OpenFile().SetTitle("Import Battle.net authenticators").AddFilter("Authenticator backups and QR images", "*.json;*.txt;*.png;*.jpg;*.jpeg")
	if owner := dialogOwnerWindow(); owner != nil {
		dialog = dialog.AttachToWindow(owner)
	}
	path, err := dialog.PromptForSingleSelection()
	if dialogCancelled(err) {
		return "", nil
	}
	return path, err
}

func (s *Service) SetBattleNetLabel(serial, label, token string) error {
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
	a.Label = strings.TrimSpace(label)
	if err := a.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	defer wipe(raw)
	_, err = v.PutProviderRecord(vault.ProviderBattleNet, a.Serial, raw)
	return err
}

func (s *Service) LinkBattleNetAccount(serial, accountID, label, token string) error {
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
	a.AccountLink = strings.TrimSpace(accountID)
	if strings.TrimSpace(label) != "" {
		a.Label = strings.TrimSpace(label)
	}
	if err := a.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	defer wipe(raw)
	_, err = v.PutProviderRecord(vault.ProviderBattleNet, a.Serial, raw)
	return err
}

func (s *Service) ExportBattleNet(serials []string, password, keyfilePath, backupKey, exportPassword, token string) (string, error) {
	if err := passwordpolicy.ValidateNew(exportPassword); err != nil {
		return "", err
	}
	s.mu.Lock()
	_, authErr := s.battleNetVaultLocked(token)
	s.mu.Unlock()
	if authErr != nil {
		return "", authErr
	}
	app := application.Get()
	if app == nil {
		return "", ErrUnsupportedInput
	}
	destination, err := app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{Title: "Export Battle.net authenticators", Filename: "battlenet-vault.json", Window: dialogOwnerWindow()}).PromptForSingleSelection()
	if dialogCancelled(err) || destination == "" {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(destination) {
		return "", ErrPathNotAbsolute
	}
	creds, err := buildVaultCredentials(password, keyfilePath, backupKey)
	if err != nil {
		return "", err
	}
	defer wipe(creds.Keyfile)
	defer wipe(creds.RecoveryCode)
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.battleNetVaultLocked(token)
	if err != nil {
		return "", err
	}
	verifyErr := v.VerifyCredentials(creds)
	if needsSecurityKey(verifyErr) {
		id, secret, keyErr := s.evaluateSecurityKeyLocked(v)
		if keyErr != nil {
			return "", keyErr
		}
		defer wipe(secret)
		creds.SecurityKeyID = id
		creds.SecurityKey = secret
		verifyErr = v.VerifyCredentials(creds)
	}
	if verifyErr != nil {
		return "", verifyErr
	}
	accounts := make([]battlenetauth.Account, 0, len(serials))
	for _, serial := range serials {
		a, err := battleNetRecord(v, serial)
		if err != nil {
			return "", err
		}
		accounts = append(accounts, a)
	}
	body, err := battlenetauth.Export(accounts, exportPassword)
	if err != nil {
		return "", err
	}
	defer wipe(body)
	file, err := securefile.CreateNew(destination)
	if err != nil {
		return "", err
	}
	_, writeErr := file.Write(body)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return "", err
	}
	return destination, nil
}
