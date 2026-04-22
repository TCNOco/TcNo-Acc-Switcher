package basic

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// FlowDeps gathers dependencies for account operations.
type FlowDeps struct {
	PS *platform.PlatformService
}

func readDescriptor(deps FlowDeps, platformKey string) (platform.Descriptor, []byte, error) {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return platform.Descriptor{}, nil, err
	}
	pj, err := platform.ResolvePlatformsJSONPath(exeDir)
	if err != nil {
		return platform.Descriptor{}, nil, err
	}
	raw, err := os.ReadFile(pj)
	if err != nil {
		return platform.Descriptor{}, nil, err
	}
	d, err := platform.ParseDescriptor(raw, platformKey)
	return d, raw, err
}

func resolveExeFolder(deps FlowDeps, platformKey string) (string, error) {
	if deps.PS == nil {
		return "", fmt.Errorf("platform service not set")
	}
	return deps.PS.GetPlatformInstallFolder(platformKey)
}

// SaveCurrent copies live login state into LoginCache/<platform>/<accountName>/.
func SaveCurrent(deps FlowDeps, platformKey, accountName string) error {
	defer platform.EmitActionBarStatus("")

	d, _, err := readDescriptor(deps, platformKey)
	if err != nil {
		return err
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		return err
	}

	if d.ExitBeforeInteract {
		platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatform", platformKey)
		_ = winutil.KillByName(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod))
	}
	platform.EmitActionBarStatusI18n("Status_ActionBar_SavingSession")

	return saveCurrentAfterKill(deps, platformKey, accountName, d)
}

// saveCurrentAfterKill persists login files (callers handle process kill + status).
func saveCurrentAfterKill(deps FlowDeps, platformKey, accountName string, d platform.Descriptor) error {
	folder, err := resolveExeFolder(deps, platformKey)
	if err != nil {
		return err
	}
	ctx := platform.PathTokenContext{PlatformFolder: folder}

	uid, err := ensureUniqueIDOnSave(d, ctx)
	if err != nil {
		return err
	}

	destRoot, err := accountCacheDir(platformKey, accountName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return err
	}

	regDump := map[string]string{}

	for liveKey, cacheRel := range d.LoginFiles {
		liveKey = strings.TrimSpace(liveKey)
		if isREG(liveKey) {
			enc := stripREG(liveKey)
			v, _, err := winutil.RegistryRead(enc)
			if err != nil {
				if d.AllFilesRequired {
					return err
				}
				continue
			}
			var s string
			switch x := v.(type) {
			case string:
				s = x
			case []byte:
				s = winutil.HexEncodeBinary(x)
			case uint32:
				s = fmt.Sprintf("%d", x)
			default:
				s = fmt.Sprint(x)
			}
			regDump[liveKey] = s
			continue
		}
		if isJSONSelectFirst(liveKey) || isJSONSelectLast(liveKey) {
			pfx := "JSON_SELECT_FIRST"
			if isJSONSelectLast(liveKey) {
				pfx = "JSON_SELECT_LAST"
			}
			fp, jp, ok := parseJSONSelect(pfx, liveKey)
			if !ok {
				return fmt.Errorf("bad JSON_SELECT key")
			}
			fp = expandPlatformPath(fp, folder, ctx)
			data, err := os.ReadFile(fp)
			if err != nil {
				if d.AllFilesRequired {
					return err
				}
				continue
			}
			chunk := gjson.GetBytes(data, jp).Raw
			if chunk == "" {
				chunk = gjson.GetBytes(data, jp).String()
			}
			dst := filepath.Join(destRoot, filepath.FromSlash(cacheRel))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := fsutil.WriteFileAtomic(dst, []byte(chunk), 0o644); err != nil {
				return err
			}
			continue
		}
		src := expandPlatformPath(liveKey, folder, ctx)
		dst := filepath.Join(destRoot, filepath.FromSlash(cacheRel))
		if strings.Contains(filepath.Base(src), "*") {
			matches, _ := filepath.Glob(src)
			for _, m := range matches {
				if err := copyFileToDir(m, filepath.Dir(dst)); err != nil && d.AllFilesRequired {
					return err
				}
			}
			continue
		}
		st, err := os.Stat(src)
		if err != nil {
			if d.AllFilesRequired {
				return err
			}
			continue
		}
		if st.IsDir() {
			if err := copyDir(src, dst); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := copyFile(src, dst); err != nil {
				return err
			}
		}
	}

	if len(regDump) > 0 {
		b, err := json.MarshalIndent(regDump, "", "  ")
		if err != nil {
			return err
		}
		if err := fsutil.WriteFileAtomic(filepath.Join(destRoot, "reg.json"), b, 0o644); err != nil {
			return err
		}
	}

	ids, err := readIDs(platformKey)
	if err != nil {
		return err
	}
	ids[uid] = accountName
	if err := writeIDs(platformKey, ids); err != nil {
		return err
	}

	return saveProfileImage(deps, d, platformKey, folder, uid, ctx)
}

func ensureUniqueIDOnSave(d platform.Descriptor, ctx platform.PathTokenContext) (string, error) {
	if strings.EqualFold(strings.TrimSpace(d.UniqueIdMethod), "CREATE_ID_FILE") {
		p := platform.ExpandPathTokens(platform.ExpandWindowsPath(d.UniqueIdFile), ctx)
		if data, err := os.ReadFile(p); err == nil && len(strings.TrimSpace(string(data))) > 0 {
			return strings.TrimSpace(string(data)), nil
		}
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		id := hex.EncodeToString(b)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return "", err
		}
		if err := fsutil.WriteFileAtomic(p, []byte(id), 0o644); err != nil {
			return "", err
		}
		return id, nil
	}
	return ReadUniqueID(d, ctx.PlatformFolder)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyFileToDir(src, dir string) error {
	return copyFile(src, filepath.Join(dir, filepath.Base(src)))
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, de os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		t := filepath.Join(dst, rel)
		if de.IsDir() {
			return os.MkdirAll(t, 0o755)
		}
		return copyFile(path, t)
	})
}

// Login restores account from cache to live paths.
func Login(deps FlowDeps, platformKey, accountName string) error {
	d, _, err := readDescriptor(deps, platformKey)
	if err != nil {
		return err
	}
	folder, err := resolveExeFolder(deps, platformKey)
	if err != nil {
		return err
	}
	ctx := platform.PathTokenContext{PlatformFolder: folder}
	srcRoot, err := accountCacheDir(platformKey, accountName)
	if err != nil {
		return err
	}
	regData, _ := os.ReadFile(filepath.Join(srcRoot, "reg.json"))
	var regDump map[string]string
	if len(regData) > 0 {
		_ = json.Unmarshal(regData, &regDump)
	}

	for liveKey, cacheRel := range d.LoginFiles {
		liveKey = strings.TrimSpace(liveKey)
		if isREG(liveKey) {
			v, ok := regDump[liveKey]
			if !ok {
				continue
			}
			if strings.HasPrefix(strings.ToLower(v), "(hex)") {
				raw, err := parseHexReg(v)
				if err != nil {
					return err
				}
				if err := winutil.RegistryWrite(stripREG(liveKey), raw); err != nil {
					return err
				}
			} else {
				if err := winutil.RegistryWrite(stripREG(liveKey), v); err != nil {
					return err
				}
			}
			continue
		}
		if isJSONSelectFirst(liveKey) || isJSONSelectLast(liveKey) {
			pfx := "JSON_SELECT_FIRST"
			if isJSONSelectLast(liveKey) {
				pfx = "JSON_SELECT_LAST"
			}
			fp, jp, ok := parseJSONSelect(pfx, liveKey)
			if !ok {
				return fmt.Errorf("bad JSON_SELECT")
			}
			fp = expandPlatformPath(fp, folder, ctx)
			cacheFile := filepath.Join(srcRoot, filepath.FromSlash(cacheRel))
			chunk, err := os.ReadFile(cacheFile)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(fp)
			if os.IsNotExist(err) {
				data = []byte("{}")
				err = nil
			} else if err != nil {
				return err
			}
			ns, err := sjson.SetRawBytes(data, jp, chunk)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
				return err
			}
			if err := fsutil.WriteFileAtomic(fp, ns, 0o644); err != nil {
				return err
			}
			continue
		}
		src := filepath.Join(srcRoot, filepath.FromSlash(cacheRel))
		dst := expandPlatformPath(liveKey, folder, ctx)
		st, err := os.Stat(src)
		if err != nil {
			if d.AllFilesRequired {
				return err
			}
			continue
		}
		if st.IsDir() {
			if err := copyDir(src, dst); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := copyFile(src, dst); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseHexReg(s string) ([]byte, error) {
	return winutil.ParseHexString(s)
}

// ClearCurrentLogin removes/clears live session per PathListToClear + LoginFiles semantics.
func ClearCurrentLogin(deps FlowDeps, platformKey string) error {
	d, _, err := readDescriptor(deps, platformKey)
	if err != nil {
		return err
	}
	folder, err := resolveExeFolder(deps, platformKey)
	if err != nil {
		return err
	}
	ctx := platform.PathTokenContext{PlatformFolder: folder}
	for _, p := range d.PathListToClear {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if isREG(p) {
			enc := stripREG(p)
			if d.RegDeleteOnClear {
				_ = winutil.RegistryDelete(enc)
			} else {
				_ = winutil.RegistryWrite(enc, "")
			}
			continue
		}
		lp := expandPlatformPath(p, folder, ctx)
		if strings.Contains(filepath.Base(lp), "*") {
			matches, _ := filepath.Glob(lp)
			for _, m := range matches {
				_ = os.RemoveAll(m)
			}
			continue
		}
		_ = os.RemoveAll(lp)
	}
	return nil
}

// SwapTo switches to the account identified by uniqueID (must exist in ids.json).
func SwapTo(deps FlowDeps, platformKey, uniqueID string) error {
	defer platform.EmitActionBarStatus("")

	d, _, err := readDescriptor(deps, platformKey)
	if err != nil {
		return err
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		return err
	}
	folder, err := resolveExeFolder(deps, platformKey)
	if err != nil {
		return err
	}

	platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatform", platformKey)
	_ = winutil.KillByName(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod))

	ids, err := readIDs(platformKey)
	if err != nil {
		return err
	}
	cur, err := ReadUniqueID(d, folder)
	if err == nil && cur != "" {
		if prevName, ok := ids[cur]; ok {
			targetName, ok2 := ids[uniqueID]
			if ok2 && prevName != targetName {
				platform.EmitActionBarStatusI18n("Status_ActionBar_SavingSession")
				if err := saveCurrentAfterKill(deps, platformKey, prevName, d); err != nil {
					return err
				}
			}
		}
	}

	platform.EmitActionBarStatusI18n("Status_ActionBar_ClearingSession")
	if err := ClearCurrentLogin(deps, platformKey); err != nil {
		return err
	}
	accName, ok := ids[uniqueID]
	if !ok || strings.TrimSpace(accName) == "" {
		return fmt.Errorf("unknown account id")
	}
	platform.EmitActionBarStatusI18n("Status_ActionBar_RestoringAccount")
	if err := Login(deps, platformKey, accName); err != nil {
		return err
	}
	if !ps.AutoStart {
		return nil
	}
	platform.EmitActionBarStatusI18nPlatform("Status_StartingPlatform", platformKey)
	return launchBasicNoStatus(deps, platformKey)
}

// LaunchBasic starts the platform executable with ExeExtraArgs.
func LaunchBasic(deps FlowDeps, platformKey string) error {
	defer platform.EmitActionBarStatus("")
	platform.EmitActionBarStatusI18nPlatform("Status_StartingPlatform", platformKey)
	return launchBasicNoStatus(deps, platformKey)
}

// AddNew clears session and launches without saving (parity with C# Basic).
func AddNew(deps FlowDeps, platformKey string) error {
	defer platform.EmitActionBarStatus("")

	d, _, err := readDescriptor(deps, platformKey)
	if err != nil {
		return err
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		return err
	}
	platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatform", platformKey)
	_ = winutil.KillByName(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod))
	platform.EmitActionBarStatusI18n("Status_ActionBar_ClearingSession")
	if err := ClearCurrentLogin(deps, platformKey); err != nil {
		return err
	}
	platform.EmitActionBarStatusI18nPlatform("Status_StartingPlatform", platformKey)
	return launchBasicNoStatus(deps, platformKey)
}

// launchBasicNoStatus is LaunchBasic without footer status (caller owns messages).
func launchBasicNoStatus(deps FlowDeps, platformKey string) error {
	return launchBasicNoStatusAs(deps, platformKey, false)
}

// launchBasicNoStatusAs starts the platform exe; if forceAdmin is true, always requests elevation.
func launchBasicNoStatusAs(deps FlowDeps, platformKey string, forceAdmin bool) error {
	d, _, err := readDescriptor(deps, platformKey)
	if err != nil {
		return err
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		return err
	}
	if deps.PS == nil {
		return fmt.Errorf("platform service not set")
	}
	exe, err := deps.PS.ResolvePlatformExeFullPath(platformKey)
	if err != nil || exe == "" {
		return fmt.Errorf("executable not found")
	}
	var args []string
	if strings.TrimSpace(d.ExeExtraArgs) != "" {
		args = append(args, strings.Fields(d.ExeExtraArgs)...)
	}
	admin := ps.RunAsAdmin
	if forceAdmin {
		admin = true
	}
	opts := winutil.StartOpts{
		Admin:         admin,
		Method:        winutil.StartingMethod(strings.TrimSpace(ps.StartingMethod)),
		HideWindow:    false,
		AsDesktopUser: winutil.IsProcessElevated() && !admin,
	}
	return winutil.Start(exe, args, opts)
}

// LaunchBasicAs starts the platform executable; if forceAdmin is true, always requests elevation.
func LaunchBasicAs(deps FlowDeps, platformKey string, forceAdmin bool) error {
	defer platform.EmitActionBarStatus("")
	platform.EmitActionBarStatusI18nPlatform("Status_StartingPlatform", platformKey)
	return launchBasicNoStatusAs(deps, platformKey, forceAdmin)
}
