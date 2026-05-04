package basic

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/cli"
	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/stats"
	"TcNo-Acc-Switcher/internal/tray"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type FlowDeps struct {
	PS *platform.PlatformService
}

func readDescriptor(platformKey string) (platform.Descriptor, []byte, error) {
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

// CurrentLiveUniqueID returns the UniqueID from the live install folder (same basis as AccountDTO.currentSession), or "" if unknown.
func CurrentLiveUniqueID(deps FlowDeps, platformKey string) (string, error) {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return "", nil
	}
	d, _, err := readDescriptor(platformKey)
	if err != nil {
		return "", err
	}
	folder, err := resolveExeFolder(deps, platformKey)
	if err != nil {
		return "", err
	}
	u, err := ReadUniqueID(d, folder)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(u), nil
}

func SaveCurrent(deps FlowDeps, platformKey, accountName string) error {
	defer platform.EmitActionBarStatus("")

	d, _, err := readDescriptor(platformKey)
	if err != nil {
		return err
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		return err
	}

	if d.ExitBeforeInteract {
		if err := winutil.ErrIfCannotKill(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod)); err != nil {
			return err
		}
		platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatform", platformKey)
		_ = winutil.KillByName(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod))
	}
	platform.EmitActionBarStatusI18n("Status_ActionBar_SavingSession")

	return saveCurrentAfterKill(deps, platformKey, accountName, d)
}

func saveCurrentAfterKill(deps FlowDeps, platformKey, accountName string, d platform.Descriptor) error {
	accountName = paths.WindowsFileName(strings.TrimSpace(accountName), 200)
	if accountName == "" {
		return fmt.Errorf("account name is empty or invalid after normalization")
	}
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
			first := true
			if isJSONSelectLast(liveKey) {
				pfx = "JSON_SELECT_LAST"
				first = false
			}
			fp, jp, delimiter, ok := parseJSONSelectWithDelimiter(pfx, liveKey)
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
			res := gjson.GetBytes(data, jp)
			selected := strings.TrimSpace(res.String())
			if res.IsArray() && len(res.Array()) > 0 {
				if first {
					selected = strings.TrimSpace(res.Array()[0].String())
				} else {
					a := res.Array()
					selected = strings.TrimSpace(a[len(a)-1].String())
				}
			} else if delimiter != "" && selected != "" {
				parts := strings.Split(selected, delimiter)
				if len(parts) > 0 {
					if first {
						selected = strings.TrimSpace(parts[0])
					} else {
						selected = strings.TrimSpace(parts[len(parts)-1])
					}
				}
			}
			chunk, err := json.Marshal(selected)
			if err != nil {
				return err
			}
			dst := filepath.Join(destRoot, filepath.FromSlash(cacheRel))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := fsutil.WriteFileAtomic(dst, chunk, 0o644); err != nil {
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
	if err := touchLastUsed(platformKey, uid); err != nil {
		return err
	}

	return saveProfileImage(d, platformKey, folder, uid, ctx)
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

func Login(deps FlowDeps, platformKey, accountName string) error {
	d, _, err := readDescriptor(platformKey)
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
	// Set UniqueIdFile from ids.json before restoring LoginFiles (update logged-in account)
	if strings.EqualFold(strings.TrimSpace(d.UniqueIdMethod), "REGKEY") {
		uidKey := stripREG(strings.TrimSpace(d.UniqueIdFile))
		if uidKey != "" {
			if ids, err := readIDs(platformKey); err == nil {
				var targetID string
				wantName := strings.TrimSpace(accountName)
				for uid, name := range ids {
					if strings.TrimSpace(name) == wantName {
						targetID = strings.TrimSpace(uid)
						break
					}
				}
				if targetID != "" {
					if err := winutil.RegistryWrite(uidKey, targetID); err != nil {
						return err
					}
				}
			}
		}
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

func ClearCurrentLogin(deps FlowDeps, platformKey string) error {
	d, _, err := readDescriptor(platformKey)
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
		pattern, err := platform.ResolveSafeDeletePattern(p, ctx)
		if err != nil {
			return err
		}
		for _, target := range platform.ExpandDeletePatternMatches(pattern) {
			if err := platform.ValidateDeleteTargetPath(target); err != nil {
				return err
			}
			_ = os.RemoveAll(target)
		}
	}
	return nil
}

func recordBasicTrayRecent(platformKey, uniqueID string) {
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	if platformKey == "" || uniqueID == "" {
		return
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil || ps.TrayAccNumber <= 0 {
		return
	}
	ids, err := readIDs(platformKey)
	if err != nil {
		return
	}
	name := strings.TrimSpace(ids[uniqueID])
	if name == "" {
		name = uniqueID
	}
	idx, err := cli.LoadPlatformIndex()
	if err != nil {
		return
	}
	short := cli.ShortTokenForPlatform(idx, platformKey)
	if short == "" {
		return
	}
	arg := "+" + short + ":" + uniqueID
	_ = tray.AddUser(platformKey, arg, name, ps.TrayAccNumber)
	tray.RefreshMenuIfSet()
}

func SwapTo(deps FlowDeps, platformKey, uniqueID string, extraLaunchArgs []string) error {
	defer platform.EmitActionBarStatus("")

	d, _, err := readDescriptor(platformKey)
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

	cur, curErr := ReadUniqueID(d, folder)
	if curErr == nil &&
		strings.TrimSpace(cur) != "" &&
		strings.EqualFold(strings.TrimSpace(cur), strings.TrimSpace(uniqueID)) &&
		len(extraLaunchArgs) == 0 {
		return nil
	}

	platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatform", platformKey)
	if err := winutil.ErrIfCannotKill(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod)); err != nil {
		return err
	}
	_ = winutil.KillByName(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod))

	ids, err := readIDs(platformKey)
	if err != nil {
		return err
	}
	if curErr == nil && cur != "" {
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
	_ = touchLastUsed(platformKey, uniqueID)
	recordBasicTrayRecent(platformKey, uniqueID)
	if err := stats.IncrementSwitches(platformKey); err != nil {
		return err
	}
	if !ps.AutoStart {
		tray.MaybeHideMainWindow()
		return nil
	}
	platform.EmitActionBarStatusI18nPlatform("Status_StartingPlatform", platformKey)
	if err := launchBasicNoStatus(deps, platformKey, extraLaunchArgs); err != nil {
		return err
	}
	tray.MaybeHideMainWindow()
	return nil
}

func LaunchBasic(deps FlowDeps, platformKey string, extraLaunchArgs []string) error {
	defer platform.EmitActionBarStatus("")
	platform.EmitActionBarStatusI18nPlatform("Status_StartingPlatform", platformKey)
	return launchBasicNoStatus(deps, platformKey, extraLaunchArgs)
}

func AddNew(deps FlowDeps, platformKey string) error {
	defer platform.EmitActionBarStatus("")

	d, _, err := readDescriptor(platformKey)
	if err != nil {
		return err
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		return err
	}
	platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatform", platformKey)
	if err := winutil.ErrIfCannotKill(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod)); err != nil {
		return err
	}
	_ = winutil.KillByName(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod))
	platform.EmitActionBarStatusI18n("Status_ActionBar_ClearingSession")
	if err := ClearCurrentLogin(deps, platformKey); err != nil {
		return err
	}
	platform.EmitActionBarStatusI18nPlatform("Status_StartingPlatform", platformKey)
	return launchBasicNoStatus(deps, platformKey, nil)
}

func launchBasicNoStatus(deps FlowDeps, platformKey string, extraLaunchArgs []string) error {
	return launchBasicNoStatusAs(deps, platformKey, false, extraLaunchArgs)
}

func launchBasicNoStatusAs(deps FlowDeps, platformKey string, forceAdmin bool, extraLaunchArgs []string) error {
	log.Printf("basic: launch begin platform=%s forceAdmin=%t extraArgs=%d", platformKey, forceAdmin, len(extraLaunchArgs))
	d, _, err := readDescriptor(platformKey)
	if err != nil {
		log.Printf("basic: launch read descriptor failed platform=%s err=%v", platformKey, err)
		return err
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		log.Printf("basic: launch load settings failed platform=%s err=%v", platformKey, err)
		return err
	}
	if deps.PS == nil {
		return fmt.Errorf("platform service not set")
	}
	exe, err := deps.PS.ResolvePlatformExeFullPath(platformKey)
	if err != nil || exe == "" {
		log.Printf("basic: launch resolve exe failed platform=%s exe=%s err=%v", platformKey, exe, err)
		return fmt.Errorf("executable not found")
	}
	var args []string
	if strings.TrimSpace(d.ExeExtraArgs) != "" {
		args = append(args, strings.Fields(d.ExeExtraArgs)...)
	}
	args = append(args, platform.LaunchArgTokens(ps.LaunchArguments)...)
	if len(extraLaunchArgs) > 0 {
		args = append(args, extraLaunchArgs...)
	}
	admin := ps.RunAsAdmin
	if forceAdmin {
		admin = true
	}
	opts := winutil.StartOpts{
		Admin:         admin,
		Method:        winutil.StartingMethod(strings.TrimSpace(ps.StartingMethod)),
		HideWindow:    false,
		WorkingDir:    filepath.Dir(exe),
		AsDesktopUser: winutil.IsProcessElevated() && !admin,
	}
	log.Printf("basic: start request platform=%s exe=%s args=%d method=%s admin=%t", platformKey, exe, len(args), opts.Method, opts.Admin)
	if err := winutil.Start(exe, args, opts); err != nil {
		log.Printf("basic: start failed platform=%s exe=%s err=%v", platformKey, exe, err)
		return err
	}
	log.Printf("basic: start launched platform=%s exe=%s", platformKey, exe)
	return nil
}

func LaunchBasicAs(deps FlowDeps, platformKey string, forceAdmin bool, extraLaunchArgs []string) error {
	defer platform.EmitActionBarStatus("")
	platform.EmitActionBarStatusI18nPlatform("Status_StartingPlatform", platformKey)
	return launchBasicNoStatusAs(deps, platformKey, forceAdmin, extraLaunchArgs)
}
