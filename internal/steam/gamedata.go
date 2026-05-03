package steam

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
)

// Error messages match i18n keys; the Windows frontend maps them in PlatformSteam.
var (
	errSteamDataInvalidID   = errors.New("Toast_NoValidSteamId")
	errSteamDataSameAccount = errors.New("Toast_SameAccount")
)

func errNoSteamUserdataPath(p string) error {
	p = filepath.Clean(p)
	// First line is the i18n key; second line is shown literally in the toast.
	return errors.New("Toast_NoFindSteamUserdata\npath: " + p)
}

func errNoGameBackupPath(p string) error {
	p = filepath.Clean(p)
	return errors.New("Toast_NoFindGameBackup\npath: " + p)
}

func normalizeSteamAppID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errSteamDataInvalidID
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return "", errSteamDataInvalidID
		}
	}
	return s, nil
}

func steamUserdataGamePath(steamRoot, id32, appID string) string {
	return filepath.Join(steamRoot, "userdata", id32, appID)
}

func steamBackupGamePath(steamID32, appID string) (string, error) {
	r, err := paths.DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(r, "Backups", "Steam", steamID32, appID), nil
}

func (s *SteamService) currentActiveSteamID64() (string, error) {
	root, err := s.steamInstallRoot()
	if err != nil {
		return "", err
	}
	users, err := ParseLoginUsers(LoginUsersPath(root))
	if err != nil {
		return "", err
	}
	return ActiveSessionSteamID64(users), nil
}

// CurrentLiveSteamID64 returns the SteamID64 with MostRecent=="1" in loginusers.vdf, or "" if none/ambiguous.
func CurrentLiveSteamID64() (string, error) {
	var s SteamService
	return s.currentActiveSteamID64()
}

// CopySteamGameSettingsFrom copies <steam>/userdata/{source32}/{appID} into the current session
// account folder, auto-backing up the destination tree into Backups/Steam first (legacy C# behavior).
func (s *SteamService) CopySteamGameSettingsFrom(sourceSteamID64, appID string) error {
	appID, err := normalizeSteamAppID(appID)
	if err != nil {
		return err
	}
	if _, e := FormatsFromID64(strings.TrimSpace(sourceSteamID64)); e != nil {
		return errSteamDataInvalidID
	}
	root, err := s.steamInstallRoot()
	if err != nil {
		return err
	}
	if strings.TrimSpace(root) == "" {
		return errSteamDataInvalidID
	}
	destID64, err := s.currentActiveSteamID64()
	if err != nil {
		return err
	}
	if destID64 == "" {
		return errSteamDataInvalidID
	}
	if strings.TrimSpace(sourceSteamID64) == strings.TrimSpace(destID64) {
		return errSteamDataSameAccount
	}
	srcFmt, err := FormatsFromID64(strings.TrimSpace(sourceSteamID64))
	if err != nil {
		return errSteamDataInvalidID
	}
	dstFmt, err := FormatsFromID64(strings.TrimSpace(destID64))
	if err != nil {
		return errSteamDataInvalidID
	}
	srcDir := steamUserdataGamePath(root, srcFmt.ID32, appID)
	dstDir := steamUserdataGamePath(root, dstFmt.ID32, appID)

	st, err := os.Stat(srcDir)
	if err != nil || !st.IsDir() {
		return errNoSteamUserdataPath(srcDir)
	}
	if st2, e := os.Stat(dstDir); e == nil && st2.IsDir() {
		toBackup, berr := steamBackupGamePath(dstFmt.ID32, appID)
		if berr != nil {
			return berr
		}
		if err := os.RemoveAll(toBackup); err != nil {
			return err
		}
		if err := fsutil.CopyDir(dstDir, toBackup); err != nil {
			return err
		}
		if err := os.RemoveAll(dstDir); err != nil {
			return err
		}
	}
	if err := fsutil.CopyDir(srcDir, dstDir); err != nil {
		return err
	}
	return nil
}

// RestoreSteamGameSettingsTo restores from Backups/Steam/{id32}/{appID} into userdata.
func (s *SteamService) RestoreSteamGameSettingsTo(steamID64, appID string) error {
	appID, err := normalizeSteamAppID(appID)
	if err != nil {
		return err
	}
	f, err := FormatsFromID64(strings.TrimSpace(steamID64))
	if err != nil {
		return errSteamDataInvalidID
	}
	root, err := s.steamInstallRoot()
	if err != nil {
		return err
	}
	backup, err := steamBackupGamePath(f.ID32, appID)
	if err != nil {
		return err
	}
	st, err := os.Stat(backup)
	if err != nil || !st.IsDir() {
		return errNoGameBackupPath(backup)
	}
	live := steamUserdataGamePath(root, f.ID32, appID)
	if err := os.RemoveAll(live); err != nil {
		return err
	}
	return fsutil.CopyDir(backup, live)
}

// BackupSteamGameData copies live userdata for this game to Backups/Steam. Returns the backup path on success.
func (s *SteamService) BackupSteamGameData(steamID64, appID string) (string, error) {
	appID, err := normalizeSteamAppID(appID)
	if err != nil {
		return "", err
	}
	f, err := FormatsFromID64(strings.TrimSpace(steamID64))
	if err != nil {
		return "", errSteamDataInvalidID
	}
	root, err := s.steamInstallRoot()
	if err != nil {
		return "", err
	}
	src := steamUserdataGamePath(root, f.ID32, appID)
	st, err := os.Stat(src)
	if err != nil || !st.IsDir() {
		return "", errNoSteamUserdataPath(src)
	}
	dest, err := steamBackupGamePath(f.ID32, appID)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	if err := fsutil.CopyDir(src, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// SteamGameDataAppIDSets is app folder names under Steam userdata and under our Backups/Steam for this account.
type SteamGameDataAppIDSets struct {
	UserdataAppIDs []string `json:"userdataAppIds"`
	BackupAppIDs   []string `json:"backupAppIds"`
}

// GetSteamGameDataAppIDSets lists numeric subfolder names in userdata/{id32}/ and Backups/Steam/{id32}/
// so the client can show Copy/Backup only when local game data exists, and Restore when a backup exists.
func (s *SteamService) GetSteamGameDataAppIDSets(steamID64 string) (SteamGameDataAppIDSets, error) {
	steamID64 = strings.TrimSpace(steamID64)
	f, err := FormatsFromID64(steamID64)
	if err != nil {
		return SteamGameDataAppIDSets{}, err
	}
	root, err := s.steamInstallRoot()
	if err != nil {
		return SteamGameDataAppIDSets{}, err
	}
	if strings.TrimSpace(root) == "" {
		return SteamGameDataAppIDSets{}, errSteamDataInvalidID
	}
	uPath := filepath.Join(root, "userdata", f.ID32)
	userdata := listNumericSubdirNames(uPath)

	dr, err := paths.DataRoot()
	if err != nil {
		return SteamGameDataAppIDSets{}, err
	}
	backup := listNumericSubdirNames(filepath.Join(dr, "Backups", "Steam", f.ID32))
	return SteamGameDataAppIDSets{UserdataAppIDs: userdata, BackupAppIDs: backup}, nil
}

func listNumericSubdirNames(dir string) []string {
	ent, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ent {
		if !e.IsDir() {
			continue
		}
		n := e.Name()
		if isAllDigitRunes(n) {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

func isAllDigitRunes(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
