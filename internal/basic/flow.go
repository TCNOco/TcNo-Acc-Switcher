package basic

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/stability"
	"TcNo-Acc-Switcher/internal/stats"
	"TcNo-Acc-Switcher/internal/tray"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type FlowDeps struct {
	PS *platform.PlatformService
}

type FlowContext struct {
	PlatformKey string
	Descriptor  platform.Descriptor
	Settings    platform.PlatformSettings
	Folder      string
	PathCtx     platform.PathTokenContext
}

const (
	electronKillForegroundWait   = 20 * time.Second
	electronKillForegroundSettle = 450 * time.Millisecond
)

func PrepareFlow(deps FlowDeps, platformKey string) (FlowContext, error) {
	d, _, err := readDescriptor(platformKey)
	if err != nil {
		return FlowContext{}, err
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		return FlowContext{}, err
	}
	folder, err := resolveExeFolder(deps, platformKey)
	if err != nil {
		return FlowContext{}, err
	}
	return FlowContext{
		PlatformKey: platformKey,
		Descriptor:  d,
		Settings:    ps,
		Folder:      folder,
		PathCtx:     platform.PathTokenContext{PlatformFolder: folder},
	}, nil
}

func readDescriptor(platformKey string) (platform.Descriptor, []byte, error) {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return platform.Descriptor{}, nil, err
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
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
	u, err := ReadUniqueID(platformKey, d, folder)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(u), nil
}

func SaveCurrent(deps FlowDeps, platformKey, accountName string) (err error) {
	defer finishActionBarStatus(&err)
	platform.EmitActionBarStatusI18n("Status_Init")
	closeSharedLevelDBHandles("SaveCurrent.begin")
	defer closeSharedLevelDBHandles("SaveCurrent.end")

	fc, err := PrepareFlow(deps, platformKey)
	if err != nil {
		return err
	}

	if fc.Descriptor.ExitBeforeInteract || fc.Descriptor.ExitBeforeSave {
		if err := killPlatformExes(deps, fc); err != nil {
			return err
		}
	}
	platform.EmitActionBarStatusI18n("Status_ActionBar_SavingSession")

	return saveCurrentAfterKill(deps, accountName, fc)
}

func saveCurrentAfterKill(deps FlowDeps, accountName string, fc FlowContext) error {
	accountName = paths.WindowsFileName(strings.TrimSpace(accountName), 200)
	if accountName == "" {
		return fmt.Errorf("account name is empty or invalid after normalization")
	}
	platform.EmitActionBarStatusI18n("Status_GetUniqueId")
	uid, err := ensureUniqueIDOnSave(fc.PlatformKey, fc.Descriptor, fc.PathCtx)
	if err != nil {
		return err
	}

	idsFileData, err := readIdsFile(fc.PlatformKey)
	if err != nil {
		return err
	}
	normalizeTagMaps(&idsFileData)
	for existingUID, existingName := range idsFileData.IDs {
		if strings.EqualFold(strings.TrimSpace(existingUID), strings.TrimSpace(uid)) {
			continue
		}
		normalizedExisting := paths.WindowsFileName(strings.TrimSpace(existingName), 200)
		if !strings.EqualFold(normalizedExisting, accountName) {
			continue
		}
		delete(idsFileData.IDs, existingUID)
		if idsFileData.LastUsed != nil {
			delete(idsFileData.LastUsed, existingUID)
		}
		delete(idsFileData.AccountTags, existingUID)
		if oldDestRoot, derr := accountCacheDir(fc.PlatformKey, existingName); derr == nil {
			logFlow().Debug("remove superseded account cache", "path", oldDestRoot)
			if rerr := fsutil.RemoveAllWithRetry(oldDestRoot, 2*time.Second, os.RemoveAll); rerr != nil {
				logFlow().Warn("remove superseded account cache", "path", oldDestRoot, "err", rerr)
			}
		}
		_ = profileimage.DeleteCached(fc.PlatformKey, existingUID)
	}
	pruneUnusedTagDefinitions(&idsFileData)
	if err := writeIdsFile(fc.PlatformKey, idsFileData); err != nil {
		return err
	}

	destRoot, err := accountCacheDir(fc.PlatformKey, accountName)
	if err != nil {
		return err
	}
	logFlow().Debug("clear account cache before save", "path", destRoot)
	if err := fsutil.RemoveAllWithRetry(destRoot, 2*time.Second, os.RemoveAll); err != nil {
		return fmt.Errorf("clear account cache %s: %w", destRoot, err)
	}
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", destRoot, err)
	}

	regDump := map[string]regDumpEntry{}

	platform.EmitActionBarStatusI18n("Status_CopyingFiles")
	for liveKey, cacheRel := range fc.Descriptor.LoginFiles {
		liveKey = strings.TrimSpace(liveKey)
		if isREG(liveKey) {
			platform.EmitActionBarStatusI18n("Status_UpdatingRegistry")
			enc := stripREG(liveKey)
			if kp, ok := winutil.RegistryKeyPathForAllValuesSpecifier(enc); ok {
				all, err := winutil.RegistryReadAllValuesInKey(kp)
				if err != nil {
					if fc.Descriptor.AllFilesRequired {
						return fmt.Errorf("registry read all values %s: %w", liveKey, err)
					}
					logFlow().Debug("skipping optional registry enumerate failure", "key", liveKey, "err", err)
					continue
				}
				inner := make(map[string]regDumpEntry, len(all))
				for vn, cell := range all {
					inner[vn] = regDumpEntry{V: registryValueStringForDump(cell.Val), T: cell.Typ}
				}
				regDump[liveKey] = regDumpEntry{Values: inner}
				continue
			}
			if kp, vglob, ok := splitRegistryKeyPathAndValueGlob(enc); ok {
				matched, err := winutil.RegistryReadValuesMatchingNameGlob(kp, vglob)
				if err != nil {
					if fc.Descriptor.AllFilesRequired {
						return fmt.Errorf("registry read glob values %s: %w", liveKey, err)
					}
					logFlow().Debug("skipping optional registry glob read failure", "key", liveKey, "err", err)
					continue
				}
				if len(matched) == 0 && fc.Descriptor.AllFilesRequired {
					return fmt.Errorf("registry glob %s matched no values", liveKey)
				}
				inner := make(map[string]regDumpEntry, len(matched))
				for vn, cell := range matched {
					inner[vn] = regDumpEntry{V: registryValueStringForDump(cell.Val), T: cell.Typ}
				}
				regDump[liveKey] = regDumpEntry{Values: inner}
				continue
			}
			v, typ, err := winutil.RegistryRead(enc)
			if err != nil {
				if fc.Descriptor.AllFilesRequired {
					return fmt.Errorf("registry read %s: %w", liveKey, err)
				}
				logFlow().Debug("skipping optional registry read failure", "key", liveKey, "err", err)
				continue
			}
			regDump[liveKey] = regDumpEntry{V: registryValueStringForDump(v), T: typ}
			continue
		}
		if isJSONSelect(liveKey) {
			first := false
			last := false
			plain := false
			var fp, jp, delimiter string
			var ok bool
			switch {
			case isJSONSelectFirst(liveKey):
				first = true
				fp, jp, delimiter, ok = parseJSONSelectWithDelimiter("JSON_SELECT_FIRST", liveKey)
			case isJSONSelectLast(liveKey):
				last = true
				fp, jp, delimiter, ok = parseJSONSelectWithDelimiter("JSON_SELECT_LAST", liveKey)
			default:
				plain = true
				fp, jp, ok = parseJSONSelectPlain("JSON_SELECT", liveKey)
			}
			if !ok {
				return fmt.Errorf("bad JSON_SELECT key")
			}
			fp = expandPlatformPath(fp, fc.Folder, fc.PathCtx)
			emitUpdatingFileStatus(fp)
			data, err := os.ReadFile(fp)
			if err != nil {
				if fc.Descriptor.AllFilesRequired {
					return fmt.Errorf("read %s: %w", fp, err)
				}
				logFlow().Debug("skipping missing optional login file", "path", fp, "err", err)
				continue
			}
			res := gjson.GetBytes(data, jp)
			selected := strings.TrimSpace(res.String())
			if plain {
			} else if res.IsArray() && len(res.Array()) > 0 {
				if first {
					selected = strings.TrimSpace(res.Array()[0].String())
				} else if last {
					a := res.Array()
					selected = strings.TrimSpace(a[len(a)-1].String())
				}
			} else if selected != "" {
				if delimiter == "" {
					delimiter = ","
				}
				parts := strings.Split(selected, delimiter)
				if len(parts) > 0 {
					if first {
						selected = strings.TrimSpace(parts[0])
					} else if last {
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
				return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
			}
			if err := fsutil.WriteFileAtomic(dst, chunk, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", dst, err)
			}
			logFlow().Debug("wrote login fragment cache", "dst", dst)
			continue
		}
		src := expandPlatformPath(liveKey, fc.Folder, fc.PathCtx)
		dst := filepath.Join(destRoot, filepath.FromSlash(cacheRel))
		emitUpdatingFileStatus(src)
		if hasGlobPattern(src) {
			matches, _ := filepath.Glob(src)
			globDestRoot := globDestinationRoot(destRoot, cacheRel)
			if err := os.MkdirAll(globDestRoot, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", globDestRoot, err)
			}
			for _, m := range matches {
				st, err := os.Stat(m)
				if err != nil {
					if fc.Descriptor.AllFilesRequired {
						return fmt.Errorf("stat %s: %w", m, err)
					}
					logFlow().Debug("glob match missing", "path", m, "err", err)
					continue
				}
				if st.IsDir() {
					if err := copyDir(m, filepath.Join(globDestRoot, filepath.Base(m))); err != nil {
						if fc.Descriptor.AllFilesRequired {
							return err
						}
					}
					continue
				}
				if err := copyFileToDir(m, globDestRoot); err != nil && fc.Descriptor.AllFilesRequired {
					return err
				}
			}
			continue
		}
		st, err := os.Stat(src)
		if err != nil {
			if fc.Descriptor.AllFilesRequired {
				return fmt.Errorf("login file not found: %s: %w", src, err)
			}
			logFlow().Debug("skipping missing optional login path", "path", src, "err", err)
			continue
		}
		if st.IsDir() {
			if err := copyDir(src, dst); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
			}
			if err := copyFile(src, dst); err != nil {
				return err
			}
		}
	}

	if strings.EqualFold(strings.TrimSpace(fc.Descriptor.UniqueIdMethod), "REGKEY") {
		platform.EmitActionBarStatusI18n("Status_UpdatingRegistry")
		uidKey := strings.TrimSpace(fc.Descriptor.UniqueIdFile)
		enc := stripREG(uidKey)
		if enc != "" {
			mapKey := uidKey
			if !isREG(mapKey) {
				mapKey = "REG:" + enc
			}
			if _, exists := regDump[mapKey]; !exists {
				if kp, vglob, ok := splitRegistryKeyPathAndValueGlob(enc); ok {
					matched, err := winutil.RegistryReadValuesMatchingNameGlob(kp, vglob)
					if err != nil {
						if fc.Descriptor.AllFilesRequired {
							return fmt.Errorf("registry read unique id %s: %w", uidKey, err)
						}
						logFlow().Debug("skipping optional unique id registry read failure", "key", uidKey, "err", err)
					} else if len(matched) == 0 {
						if fc.Descriptor.AllFilesRequired {
							return fmt.Errorf("registry glob unique id %s matched no values", uidKey)
						}
						logFlow().Debug("skipping optional unique id registry glob: no matches", "key", uidKey)
					} else {
						inner := make(map[string]regDumpEntry, len(matched))
						for vn, cell := range matched {
							inner[vn] = regDumpEntry{V: registryValueStringForDump(cell.Val), T: cell.Typ}
						}
						regDump[mapKey] = regDumpEntry{Values: inner}
					}
				} else {
					v, typ, err := winutil.RegistryRead(enc)
					if err != nil {
						if fc.Descriptor.AllFilesRequired {
							return fmt.Errorf("registry read unique id %s: %w", uidKey, err)
						}
						logFlow().Debug("skipping optional unique id registry read failure", "key", uidKey, "err", err)
					} else {
						regDump[mapKey] = regDumpEntry{V: registryValueStringForDump(v), T: typ}
					}
				}
			}
		}
	}

	if len(regDump) > 0 {
		platform.EmitActionBarStatusI18n("Status_UpdatingRegistry")
		b, err := json.MarshalIndent(regDump, "", "  ")
		if err != nil {
			return err
		}
		regPath := filepath.Join(destRoot, "reg.json")
		if err := fsutil.WriteFileAtomic(regPath, b, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", regPath, err)
		}
		logFlow().Debug("wrote registry dump cache", "path", regPath)
	}

	ids, err := readIDs(fc.PlatformKey)
	if err != nil {
		return err
	}
	ids[uid] = accountName
	if err := writeIDs(fc.PlatformKey, ids); err != nil {
		return err
	}
	if err := touchLastUsed(fc.PlatformKey, uid); err != nil {
		return err
	}
	syncBasicTrayKnownAccounts(fc.PlatformKey, ids)

	platform.EmitActionBarStatusI18n("Status_HandlingImage")
	return queueAutomatedProfileImage(fc.PlatformKey, uid, accountName, fc.Descriptor, fc.Folder)
}

func ensureUniqueIDOnSave(platformKey string, d platform.Descriptor, ctx platform.PathTokenContext) (string, error) {
	if strings.EqualFold(strings.TrimSpace(d.UniqueIdMethod), "CREATE_ID_FILE") {
		p := platform.ExpandPathTokens(platform.ExpandWindowsPath(d.UniqueIdFile), ctx)
		if data, err := os.ReadFile(p); err == nil && len(strings.TrimSpace(string(data))) > 0 {
			return strings.TrimSpace(string(data)), nil
		}
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return "", fmt.Errorf("generate unique id: %w", err)
		}
		id := hex.EncodeToString(b)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return "", wrapNeedsAdminIfPermission(fmt.Errorf("mkdir %s: %w", filepath.Dir(p), err))
		}
		if err := fsutil.WriteFileAtomic(p, []byte(id), 0o644); err != nil {
			return "", wrapNeedsAdminIfPermission(fmt.Errorf("write %s: %w", p, err))
		}
		return id, nil
	}
	return ReadUniqueID(platformKey, d, ctx.PlatformFolder)
}

func Login(deps FlowDeps, fc FlowContext, accountName string) error {
	closeSharedLevelDBHandles("Login.begin")
	defer closeSharedLevelDBHandles("Login.end")

	ctx := fc.PathCtx
	folder := fc.Folder
	srcRoot, err := accountCacheDir(fc.PlatformKey, accountName)
	if err != nil {
		return err
	}
	regData, _ := os.ReadFile(filepath.Join(srcRoot, "reg.json"))
	var regDump map[string]regDumpEntry
	if len(regData) > 0 {
		var err error
		regDump, err = regDumpFromJSON(regData)
		if err != nil {
			return fmt.Errorf("reg.json: %w", err)
		}
	}
	if strings.EqualFold(strings.TrimSpace(fc.Descriptor.UniqueIdMethod), "REGKEY") {
		platform.EmitActionBarStatusI18n("Status_UpdatingRegistry")
		uidKey := strings.TrimSpace(fc.Descriptor.UniqueIdFile)
		if stripREG(uidKey) != "" {
			if ids, err := readIDs(fc.PlatformKey); err == nil {
				var targetID string
				wantName := strings.TrimSpace(accountName)
				for uid, name := range ids {
					if strings.TrimSpace(name) == wantName {
						targetID = strings.TrimSpace(uid)
						break
					}
				}
				if targetID != "" {
					enc := stripREG(uidKey)
					mapKey := uidKey
					if !isREG(mapKey) {
						mapKey = "REG:" + enc
					}
					var wrote bool
					if kp, vglob, ok := splitRegistryKeyPathAndValueGlob(enc); ok {
						if bundle, ok := regDump[mapKey]; ok && len(bundle.Values) > 0 {
							if vn := firstValueNameMatchingGlob(bundle.Values, vglob); vn != "" {
								cell := bundle.Values[vn]
								wk := "REG:" + kp + ":" + vn
								if err := writeRegistryFromRegDump(wk, regDumpEntry{V: targetID, T: cell.T}); err != nil {
									return err
								}
								wrote = true
							}
						}
						if !wrote {
							vn, _, typ, err := winutil.RegistryReadFirstValueMatchingNameGlob(kp, vglob)
							if err == nil && vn != "" {
								full := "REG:" + kp + ":" + vn
								if err := writeRegistryFromRegDump(full, regDumpEntry{V: targetID, T: typ}); err != nil {
									return err
								}
								wrote = true
							}
						}
					} else if e, ok := regDumpLookup(regDump, uidKey); ok {
						if err := writeRegistryFromRegDump(uidKey, regDumpEntry{V: targetID, T: e.T}); err != nil {
							return err
						}
						wrote = true
					}
					if !wrote {
						if err := winutil.RegistryWrite(stripREG(uidKey), targetID); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	for liveKey, cacheRel := range fc.Descriptor.LoginFiles {
		liveKey = strings.TrimSpace(liveKey)
		if isREG(liveKey) {
			platform.EmitActionBarStatusI18n("Status_UpdatingRegistry")
			e, ok := regDumpLookup(regDump, liveKey)
			if !ok {
				continue
			}
			if err := writeRegistryFromRegDump(liveKey, e); err != nil {
				return err
			}
			continue
		}
		if isJSONSelect(liveKey) {
			var fp, jp string
			var ok bool
			switch {
			case isJSONSelectFirst(liveKey):
				fp, jp, ok = parseJSONSelect("JSON_SELECT_FIRST", liveKey)
			case isJSONSelectLast(liveKey):
				fp, jp, ok = parseJSONSelect("JSON_SELECT_LAST", liveKey)
			default:
				fp, jp, ok = parseJSONSelectPlain("JSON_SELECT", liveKey)
			}
			if !ok {
				return fmt.Errorf("bad JSON_SELECT")
			}
			fp = expandPlatformPath(fp, folder, ctx)
			emitUpdatingFileStatus(fp)
			cacheFile := filepath.Join(srcRoot, filepath.FromSlash(cacheRel))
			chunk, err := os.ReadFile(cacheFile)
			if err != nil {
				return fmt.Errorf("read %s: %w", cacheFile, err)
			}
			data, err := os.ReadFile(fp)
			if os.IsNotExist(err) {
				data = []byte("{}")
				err = nil
			} else if err != nil {
				return fmt.Errorf("read %s: %w", fp, err)
			}
			ns, err := sjson.SetRawBytes(data, jp, chunk)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", filepath.Dir(fp), err)
			}
			if err := fsutil.WriteFileAtomic(fp, ns, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", fp, err)
			}
			logFlow().Debug("merged JSON login file", "path", fp)
			continue
		}
		src := filepath.Join(srcRoot, filepath.FromSlash(cacheRel))
		dst := expandPlatformPath(liveKey, folder, ctx)
		emitUpdatingFileStatus(dst)
		if hasGlobPattern(dst) {
			globDstRoot := globPatternBaseDir(dst)
			if err := os.MkdirAll(globDstRoot, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", globDstRoot, err)
			}
			st, err := os.Stat(src)
			if err != nil {
				if fc.Descriptor.AllFilesRequired {
					return fmt.Errorf("stat %s: %w", src, err)
				}
				logFlow().Debug("skipping missing optional restore source", "path", src, "err", err)
				continue
			}
			if st.IsDir() {
				entries, err := os.ReadDir(src)
				if err != nil {
					if fc.Descriptor.AllFilesRequired {
						return fmt.Errorf("readdir %s: %w", src, err)
					}
					logFlow().Debug("skipping restore source readdir failure", "path", src, "err", err)
					continue
				}
				for _, entry := range entries {
					from := filepath.Join(src, entry.Name())
					to := filepath.Join(globDstRoot, entry.Name())
					if entry.IsDir() {
						if err := copyDir(from, to); err != nil {
							if fc.Descriptor.AllFilesRequired {
								return err
							}
						}
						continue
					}
					if err := copyFile(from, to); err != nil && fc.Descriptor.AllFilesRequired {
						return err
					}
				}
			} else {
				if err := copyFileToDir(src, globDstRoot); err != nil && fc.Descriptor.AllFilesRequired {
					return err
				}
			}
			continue
		}
		st, err := os.Stat(src)
		if err != nil {
			if fc.Descriptor.AllFilesRequired {
				return fmt.Errorf("restore source not found: %s: %w", src, err)
			}
			logFlow().Debug("skipping missing optional restore source", "path", src, "err", err)
			continue
		}
		if st.IsDir() {
			if err := copyDir(src, dst); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
			}
			if err := copyFile(src, dst); err != nil {
				return err
			}
		}
	}
	return nil
}

func ClearCurrentLogin(deps FlowDeps, fc FlowContext) error {
	ctx := fc.PathCtx
	folder := fc.Folder
	for _, p := range fc.Descriptor.PathListToClear {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if isREG(p) {
			platform.EmitActionBarStatusI18n("Status_UpdatingRegistry")
			enc := stripREG(p)
			if _, ok := winutil.RegistryKeyPathForAllValuesSpecifier(enc); ok {
				_ = winutil.RegistryClearLoginKey(enc, fc.Descriptor.RegDeleteOnClear)
				continue
			}
			if kp, vglob, ok := splitRegistryKeyPathAndValueGlob(enc); ok {
				_ = winutil.RegistryClearValuesMatchingNameGlob(kp, vglob, fc.Descriptor.RegDeleteOnClear)
				continue
			}
			if fc.Descriptor.RegDeleteOnClear {
				_ = winutil.RegistryDelete(enc)
			} else {
				_ = winutil.RegistryWrite(enc, "")
			}
			continue
		}
		if isJSONEmptyValue(p) {
			fp, jp, ok := parseJSONPathAction("JSON_EMPTY_VALUE", p)
			if !ok {
				return fmt.Errorf("bad JSON_EMPTY_VALUE")
			}
			fp = expandPlatformPath(fp, folder, ctx)
			emitUpdatingFileStatus(fp)
			data, err := os.ReadFile(fp)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return fmt.Errorf("read %s: %w", fp, err)
			}
			ns, err := sjson.SetBytes(data, jp, "")
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", filepath.Dir(fp), err)
			}
			if err := fsutil.WriteFileAtomic(fp, ns, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", fp, err)
			}
			logFlow().Debug("cleared JSON value on session clear", "path", fp, "jsonPath", jp)
			continue
		}
		pattern, err := platform.ResolveSafeDeletePattern(p, ctx)
		if err != nil {
			return err
		}
		for _, target := range platform.ExpandDeletePatternMatches(pattern) {
			emitUpdatingFileStatus(target)
			if err := platform.ValidateDeleteTargetPath(target); err != nil {
				return err
			}
			logFlow().Debug("delete on session clear", "path", target)
			_ = os.RemoveAll(target)
		}
	}
	return nil
}

func SwapTo(deps FlowDeps, platformKey, uniqueID string, extraLaunchArgs []string) (err error) {
	defer finishActionBarStatus(&err)
	platform.EmitActionBarStatusI18n("Status_Init")
	closeSharedLevelDBHandles("SwapTo.begin")
	defer closeSharedLevelDBHandles("SwapTo.end")

	fc, err := PrepareFlow(deps, platformKey)
	if err != nil {
		return err
	}
	d := fc.Descriptor
	ps := fc.Settings
	folder := fc.Folder

	platform.EmitActionBarStatusI18n("Status_GetUniqueId")
	cur, curErr := ReadUniqueID(platformKey, d, folder)
	if curErr == nil &&
		strings.TrimSpace(cur) != "" &&
		strings.EqualFold(strings.TrimSpace(cur), strings.TrimSpace(uniqueID)) &&
		len(extraLaunchArgs) == 0 {
		return nil
	}

	if err := killPlatformExes(deps, fc); err != nil {
		return err
	}

	ids, err := readIDs(platformKey)
	if err != nil {
		return err
	}
	if curErr == nil && strings.TrimSpace(cur) != "" {
		if prevName, ok := ids[cur]; ok {
			if !strings.EqualFold(strings.TrimSpace(cur), strings.TrimSpace(uniqueID)) {
				platform.EmitActionBarStatusI18n("Status_ActionBar_SavingSession")
				if err := saveCurrentAfterKill(deps, prevName, fc); err != nil {
					return wrapNeedsAdminIfPermission(err)
				}
			}
		}
	}

	platform.EmitActionBarStatusI18n("Status_ActionBar_ClearingSession")
	if err := ClearCurrentLogin(deps, fc); err != nil {
		return wrapNeedsAdminIfPermission(err)
	}
	accName, ok := ids[uniqueID]
	if !ok || strings.TrimSpace(accName) == "" {
		return fmt.Errorf("unknown account id")
	}
	platform.EmitActionBarStatusI18n("Status_ActionBar_RestoringAccount")
	if err := Login(deps, fc, accName); err != nil {
		return wrapNeedsAdminIfPermission(err)
	}
	_ = touchLastUsed(fc.PlatformKey, uniqueID)
	recordBasicTrayRecent(platformKey, uniqueID)
	stability.OnSuccessfulSwitch(platformKey)
	if err := stats.IncrementSwitches(platformKey); err != nil {
		return err
	}
	platform.TriggerDiscordPresenceRefresh()
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

func AddNew(deps FlowDeps, platformKey string) (err error) {
	defer finishActionBarStatus(&err)
	platform.EmitActionBarStatusI18n("Status_Init")
	closeSharedLevelDBHandles("AddNew.begin")
	defer closeSharedLevelDBHandles("AddNew.end")

	fc, err := PrepareFlow(deps, platformKey)
	if err != nil {
		return err
	}
	if err := killPlatformExes(deps, fc); err != nil {
		return err
	}
	platform.EmitActionBarStatusI18n("Status_ActionBar_ClearingSession")
	if err := ClearCurrentLogin(deps, fc); err != nil {
		return err
	}
	if !fc.Settings.AutoStart {
		tray.MaybeHideMainWindow()
		return nil
	}
	platform.EmitActionBarStatusI18nPlatform("Status_StartingPlatform", platformKey)
	if err := launchBasicNoStatus(deps, platformKey, nil); err != nil {
		return err
	}
	tray.MaybeHideMainWindow()
	return nil
}

func LaunchBasicAs(deps FlowDeps, platformKey string, forceAdmin bool, extraLaunchArgs []string) error {
	defer platform.EmitActionBarStatus("")
	platform.EmitActionBarStatusI18nPlatform("Status_StartingPlatform", platformKey)
	return launchBasicNoStatusAs(deps, platformKey, forceAdmin, extraLaunchArgs)
}
