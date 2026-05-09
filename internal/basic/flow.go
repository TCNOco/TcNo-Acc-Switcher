package basic

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/cli"
	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/stats"
	"TcNo-Acc-Switcher/internal/tray"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type FlowDeps struct {
	PS *platform.PlatformService
}

func logFlow() *slog.Logger {
	return slog.Default().With("component", "basic-flow")
}

const (
	electronKillForegroundWait   = 20 * time.Second
	electronKillForegroundSettle = 450 * time.Millisecond
)

func primaryExeImageForKill(exes []string) string {
	const svc = "SERVICE:"
	for _, raw := range exes {
		e := strings.TrimSpace(raw)
		if e == "" || strings.HasPrefix(strings.ToUpper(e), strings.ToUpper(svc)) {
			continue
		}
		base := filepath.Base(e)
		if !strings.HasSuffix(strings.ToLower(base), ".exe") {
			base = strings.TrimSpace(e) + ".exe"
		}
		return base
	}
	return ""
}

// electronBeforeKillSynth runs the same launch as the platform button, then waits for that exe to own the foreground.
func electronBeforeKillSynth(deps FlowDeps, platformKey string, exes []string) func() error {
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil || winutil.ClosingMethod(ps.ClosingMethod) != winutil.ClosingElectron {
		return nil
	}
	want := primaryExeImageForKill(exes)
	if want == "" {
		return nil
	}
	return func() error {
		if err := launchBasicNoStatus(deps, platformKey, nil); err != nil {
			return err
		}
		if !winutil.WaitForegroundForExe(want, electronKillForegroundWait) {
			logFlow().Warn("electron kill: foreground wait timeout", "image", want)
		}
		time.Sleep(electronKillForegroundSettle)
		return nil
	}
}

// regDumpEntry is one value in reg.json. Legacy files used a plain JSON string per key.
// For LoginFiles keys ending with :* (all values in a key), Values holds each value name → entry.
type regDumpEntry struct {
	V      string                  `json:"v,omitempty"`
	T      uint32                  `json:"t,omitempty"`
	Values map[string]regDumpEntry `json:"values,omitempty"`
}

func registryValueStringForDump(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return winutil.HexEncodeBinary(x)
	case uint32:
		return fmt.Sprintf("%d", x)
	case uint64:
		return fmt.Sprintf("%d", x)
	default:
		return fmt.Sprint(x)
	}
}

func regDumpFromJSON(data []byte) (map[string]regDumpEntry, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]regDumpEntry, len(raw))
	for k, rm := range raw {
		var s string
		if err := json.Unmarshal(rm, &s); err == nil {
			out[k] = regDumpEntry{V: s, T: 0}
			continue
		}
		var e regDumpEntry
		if err := json.Unmarshal(rm, &e); err != nil {
			return nil, fmt.Errorf("reg.json key %q: %w", k, err)
		}
		out[k] = e
	}
	return out, nil
}

func splitRegistryPathValue(enc string) (keyPath, valueName string, ok bool) {
	enc = strings.TrimSpace(enc)
	idx := strings.LastIndex(enc, ":")
	if idx <= 0 || idx >= len(enc)-1 {
		return "", "", false
	}
	keyPath = enc[:idx]
	valueName = enc[idx+1:]
	if !strings.Contains(keyPath, `\`) {
		return "", "", false
	}
	return keyPath, valueName, true
}

// splitRegistryKeyPathAndValueGlob is true when the value part uses * ? [ (but not the reserved whole-key name "*").
func splitRegistryKeyPathAndValueGlob(enc string) (keyPath, glob string, ok bool) {
	kp, v, ok := splitRegistryPathValue(enc)
	if !ok || v == "*" || !hasGlobPattern(v) {
		return "", "", false
	}
	return kp, v, true
}

func regDumpLookup(m map[string]regDumpEntry, descriptorKey string) (regDumpEntry, bool) {
	k := strings.TrimSpace(descriptorKey)
	if e, ok := m[k]; ok {
		return e, true
	}
	base := stripREG(k)
	if !isREG(k) {
		if e, ok := m["REG:"+base]; ok {
			return e, true
		}
	}
	if isREG(k) {
		if e, ok := m[base]; ok {
			return e, true
		}
	}
	// Concrete REG:path:value may be stored under REG:path:* or REG:path:ValueName_* bundle entries.
	keyPath, valName, ok := splitRegistryPathValue(base)
	if ok && valName != "" && valName != "*" {
		for wildKey, bundle := range m {
			if len(bundle.Values) == 0 {
				continue
			}
			wb := strings.TrimSpace(stripREG(wildKey))
			wkPath, wValPart, wok := splitRegistryPathValue(wb)
			if !wok || !strings.EqualFold(wkPath, keyPath) {
				continue
			}
			switch {
			case wValPart == "*":
				if ve, ok := bundle.Values[valName]; ok {
					return ve, true
				}
			case hasGlobPattern(wValPart):
				matched, err := filepath.Match(wValPart, valName)
				if err != nil || !matched {
					continue
				}
				if ve, ok := bundle.Values[valName]; ok {
					return ve, true
				}
			}
		}
	}
	return regDumpEntry{}, false
}

// firstValueNameMatchingGlob picks one value name in a reg.json bundle that matches valueNameGlob.
// If several names match, the lexicographically smallest is used so behavior is stable.
func firstValueNameMatchingGlob(values map[string]regDumpEntry, valueNameGlob string) string {
	var names []string
	for vn := range values {
		ok, err := filepath.Match(valueNameGlob, vn)
		if err != nil || !ok {
			continue
		}
		names = append(names, vn)
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return names[0]
}

func writeRegistryFromRegDump(liveKey string, e regDumpEntry) error {
	if len(e.Values) > 0 {
		enc := stripREG(liveKey)
		kp, valPart, ok := splitRegistryPathValue(enc)
		if !ok {
			return fmt.Errorf("registry dump has values map but key %q is not a valid registry path", liveKey)
		}
		if valPart != "*" && !hasGlobPattern(valPart) {
			return fmt.Errorf("registry dump has values map but key %q must use value :* or a glob (* ? [)", liveKey)
		}
		for valName, ent := range e.Values {
			full := "REG:" + kp + ":" + valName
			if err := writeRegistryFromRegDump(full, ent); err != nil {
				return err
			}
		}
		return nil
	}
	enc := stripREG(liveKey)
	v := strings.TrimSpace(e.V)
	if v == "" {
		if e.T != 0 {
			return winutil.RegistryWriteHint(enc, "", e.T)
		}
		return nil
	}
	if strings.HasPrefix(strings.ToLower(v), "(hex)") {
		switch e.T {
		case winutil.RegValueTypeDWORD, winutil.RegValueTypeQWORD:
			return winutil.RegistryWriteHint(enc, v, e.T)
		default:
			raw, err := parseHexReg(v)
			if err != nil {
				return err
			}
			return winutil.RegistryWriteHint(enc, raw, e.T)
		}
	}
	return winutil.RegistryWriteHint(enc, v, e.T)
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

	d, _, err := readDescriptor(platformKey)
	if err != nil {
		return err
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		return err
	}

	if d.ExitBeforeInteract || d.ExitBeforeSave {
		if err := winutil.ErrIfCannotKill(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod)); err != nil {
			platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatformFailed", platformKey)
			return err
		}
		platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatform", platformKey)
		_ = winutil.KillByName(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod), electronBeforeKillSynth(deps, platformKey, d.ExesToEnd))
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

	platform.EmitActionBarStatusI18n("Status_GetUniqueId")
	uid, err := ensureUniqueIDOnSave(platformKey, d, ctx)
	if err != nil {
		return err
	}

	idsFileData, err := readIdsFile(platformKey)
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
		if oldDestRoot, derr := accountCacheDir(platformKey, existingName); derr == nil {
			logFlow().Debug("remove superseded account cache", "path", oldDestRoot)
			_ = os.RemoveAll(oldDestRoot)
		}
		_ = profileimage.DeleteCached(platformKey, existingUID)
	}
	pruneUnusedTagDefinitions(&idsFileData)
	if err := writeIdsFile(platformKey, idsFileData); err != nil {
		return err
	}

	destRoot, err := accountCacheDir(platformKey, accountName)
	if err != nil {
		return err
	}
	logFlow().Debug("clear account cache before save", "path", destRoot)
	_ = os.RemoveAll(destRoot)
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", destRoot, err)
	}

	regDump := map[string]regDumpEntry{}

	platform.EmitActionBarStatusI18n("Status_CopyingFiles")
	for liveKey, cacheRel := range d.LoginFiles {
		liveKey = strings.TrimSpace(liveKey)
		if isREG(liveKey) {
			platform.EmitActionBarStatusI18n("Status_UpdatingRegistry")
			enc := stripREG(liveKey)
			if kp, ok := winutil.RegistryKeyPathForAllValuesSpecifier(enc); ok {
				all, err := winutil.RegistryReadAllValuesInKey(kp)
				if err != nil {
					if d.AllFilesRequired {
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
					if d.AllFilesRequired {
						return fmt.Errorf("registry read glob values %s: %w", liveKey, err)
					}
					logFlow().Debug("skipping optional registry glob read failure", "key", liveKey, "err", err)
					continue
				}
				if len(matched) == 0 && d.AllFilesRequired {
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
				if d.AllFilesRequired {
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
			fp = expandPlatformPath(fp, folder, ctx)
			emitUpdatingFileStatus(fp)
			data, err := os.ReadFile(fp)
			if err != nil {
				if d.AllFilesRequired {
					return fmt.Errorf("read %s: %w", fp, err)
				}
				logFlow().Debug("skipping missing optional login file", "path", fp, "err", err)
				continue
			}
			res := gjson.GetBytes(data, jp)
			selected := strings.TrimSpace(res.String())
			if plain {
				// Plain JSON_SELECT returns selected value directly (no split/select behavior).
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
		src := expandPlatformPath(liveKey, folder, ctx)
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
					if d.AllFilesRequired {
						return fmt.Errorf("stat %s: %w", m, err)
					}
					logFlow().Debug("glob match missing", "path", m, "err", err)
					continue
				}
				if st.IsDir() {
					if err := copyDir(m, filepath.Join(globDestRoot, filepath.Base(m))); err != nil {
						if d.AllFilesRequired {
							return err
						}
					}
					continue
				}
				if err := copyFileToDir(m, globDestRoot); err != nil && d.AllFilesRequired {
					return err
				}
			}
			continue
		}
		st, err := os.Stat(src)
		if err != nil {
			if d.AllFilesRequired {
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

	if strings.EqualFold(strings.TrimSpace(d.UniqueIdMethod), "REGKEY") {
		platform.EmitActionBarStatusI18n("Status_UpdatingRegistry")
		uidKey := strings.TrimSpace(d.UniqueIdFile)
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
						if d.AllFilesRequired {
							return fmt.Errorf("registry read unique id %s: %w", uidKey, err)
						}
						logFlow().Debug("skipping optional unique id registry read failure", "key", uidKey, "err", err)
					} else if len(matched) == 0 {
						if d.AllFilesRequired {
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
						if d.AllFilesRequired {
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
	syncBasicTrayKnownAccounts(platformKey, ids)

	platform.EmitActionBarStatusI18n("Status_HandlingImage")
	return queueAutomatedProfileImage(platformKey, uid, accountName, d, folder)
}

func ensureUniqueIDOnSave(platformKey string, d platform.Descriptor, ctx platform.PathTokenContext) (string, error) {
	if strings.EqualFold(strings.TrimSpace(d.UniqueIdMethod), "CREATE_ID_FILE") {
		p := platform.ExpandPathTokens(platform.ExpandWindowsPath(d.UniqueIdFile), ctx)
		if data, err := os.ReadFile(p); err == nil && len(strings.TrimSpace(string(data))) > 0 {
			return strings.TrimSpace(string(data)), nil
		}
		b := make([]byte, 8)
		_, _ = rand.Read(b)
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

func finishActionBarStatus(err *error) {
	if err != nil && *err != nil {
		platform.EmitActionBarStatusI18n("Status_FailedLog")
		return
	}
	platform.EmitActionBarStatus("")
}

func emitUpdatingFileStatus(path string) {
	file := strings.TrimSpace(filepath.Base(path))
	if file == "." || file == string(os.PathSeparator) {
		file = ""
	}
	if file == "" {
		file = strings.TrimSpace(path)
	}
	if file == "" {
		platform.EmitActionBarStatusI18n("Status_CopyingFiles")
		return
	}
	platform.EmitActionBarStatusI18nVars("Status_UpdatingFile", map[string]string{"file": file})
}

func wrapNeedsAdminIfPermission(err error) error {
	if err == nil || winutil.IsNeedsAdmin(err) {
		return err
	}
	if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "access is denied") {
		return winutil.NewNeedsAdminError(err.Error())
	}
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()
	if _, err = io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}
	logFlow().Debug("copied file", "src", src, "dst", dst)
	return nil
}

func copyFileToDir(src, dir string) error {
	return copyFile(src, filepath.Join(dir, filepath.Base(src)))
}

func copyDir(src, dst string) error {
	logFlow().Debug("copy directory tree", "src", src, "dst", dst)
	return filepath.WalkDir(src, func(path string, de os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk %s: %w", path, err)
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("rel %s: %w", path, err)
		}
		t := filepath.Join(dst, rel)
		if de.IsDir() {
			if err := os.MkdirAll(t, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", t, err)
			}
			return nil
		}
		return copyFile(path, t)
	})
}

func hasGlobPattern(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

func globDestinationRoot(destRoot, cacheRel string) string {
	cacheRel = strings.TrimSpace(cacheRel)
	dst := filepath.Join(destRoot, filepath.FromSlash(cacheRel))
	if strings.HasSuffix(cacheRel, "\\") || strings.HasSuffix(cacheRel, "/") {
		return dst
	}
	return filepath.Dir(dst)
}

func globPatternBaseDir(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "."
	}
	idx := strings.IndexAny(path, "*?[")
	if idx < 0 {
		return filepath.Dir(path)
	}
	prefix := path[:idx]
	if strings.HasSuffix(prefix, "\\") || strings.HasSuffix(prefix, "/") {
		return filepath.Clean(prefix)
	}
	return filepath.Dir(prefix)
}

func Login(deps FlowDeps, platformKey, accountName string) error {
	closeSharedLevelDBHandles("Login.begin")
	defer closeSharedLevelDBHandles("Login.end")
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
	var regDump map[string]regDumpEntry
	if len(regData) > 0 {
		var err error
		regDump, err = regDumpFromJSON(regData)
		if err != nil {
			return fmt.Errorf("reg.json: %w", err)
		}
	}
	// Set UniqueIdFile from ids.json before restoring LoginFiles (update logged-in account)
	if strings.EqualFold(strings.TrimSpace(d.UniqueIdMethod), "REGKEY") {
		platform.EmitActionBarStatusI18n("Status_UpdatingRegistry")
		uidKey := strings.TrimSpace(d.UniqueIdFile)
		if stripREG(uidKey) != "" {
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

	for liveKey, cacheRel := range d.LoginFiles {
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
				if d.AllFilesRequired {
					return fmt.Errorf("stat %s: %w", src, err)
				}
				logFlow().Debug("skipping missing optional restore source", "path", src, "err", err)
				continue
			}
			if st.IsDir() {
				entries, err := os.ReadDir(src)
				if err != nil {
					if d.AllFilesRequired {
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
							if d.AllFilesRequired {
								return err
							}
						}
						continue
					}
					if err := copyFile(from, to); err != nil && d.AllFilesRequired {
						return err
					}
				}
			} else {
				if err := copyFileToDir(src, globDstRoot); err != nil && d.AllFilesRequired {
					return err
				}
			}
			continue
		}
		st, err := os.Stat(src)
		if err != nil {
			if d.AllFilesRequired {
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
			platform.EmitActionBarStatusI18n("Status_UpdatingRegistry")
			enc := stripREG(p)
			if _, ok := winutil.RegistryKeyPathForAllValuesSpecifier(enc); ok {
				_ = winutil.RegistryClearLoginKey(enc, d.RegDeleteOnClear)
				continue
			}
			if kp, vglob, ok := splitRegistryKeyPathAndValueGlob(enc); ok {
				_ = winutil.RegistryClearValuesMatchingNameGlob(kp, vglob, d.RegDeleteOnClear)
				continue
			}
			if d.RegDeleteOnClear {
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
	syncBasicTrayKnownAccounts(platformKey, ids)
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

func syncBasicTrayKnownAccounts(platformKey string, ids map[string]string) {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil || ps.TrayAccNumber <= 0 {
		return
	}
	idx, err := cli.LoadPlatformIndex()
	if err != nil {
		return
	}
	short := cli.ShortTokenForPlatform(idx, platformKey)
	if short == "" {
		return
	}

	argNames := make(map[string]string, len(ids))
	for uniqueID, name := range ids {
		uniqueID = strings.TrimSpace(uniqueID)
		if uniqueID == "" {
			continue
		}
		argNames["+"+short+":"+uniqueID] = strings.TrimSpace(name)
	}
	_ = tray.SyncPlatformUsers(platformKey, argNames, ps.TrayAccNumber)
}

func SyncAllTrayKnownAccounts() {
	idx, err := cli.LoadPlatformIndex()
	if err != nil {
		return
	}
	for _, platformKey := range idx.OrderedNames {
		if strings.EqualFold(strings.TrimSpace(platformKey), "Steam") {
			continue
		}
		ids, err := readIDs(platformKey)
		if err != nil {
			continue
		}
		syncBasicTrayKnownAccounts(platformKey, ids)
	}
}

func SwapTo(deps FlowDeps, platformKey, uniqueID string, extraLaunchArgs []string) (err error) {
	defer finishActionBarStatus(&err)
	platform.EmitActionBarStatusI18n("Status_Init")
	closeSharedLevelDBHandles("SwapTo.begin")
	defer closeSharedLevelDBHandles("SwapTo.end")

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

	platform.EmitActionBarStatusI18n("Status_GetUniqueId")
	cur, curErr := ReadUniqueID(platformKey, d, folder)
	if curErr == nil &&
		strings.TrimSpace(cur) != "" &&
		strings.EqualFold(strings.TrimSpace(cur), strings.TrimSpace(uniqueID)) &&
		len(extraLaunchArgs) == 0 {
		return nil
	}

	platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatform", platformKey)
	if err := winutil.ErrIfCannotKill(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod)); err != nil {
		platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatformFailed", platformKey)
		return err
	}
	_ = winutil.KillByName(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod), electronBeforeKillSynth(deps, platformKey, d.ExesToEnd))

	ids, err := readIDs(platformKey)
	if err != nil {
		return err
	}
	// Persist live LoginFiles → account cache before clear/restore (must run per unique ID,
	// not per display name — two accounts can legally share the same visible name).
	if curErr == nil && strings.TrimSpace(cur) != "" {
		if prevName, ok := ids[cur]; ok {
			if !strings.EqualFold(strings.TrimSpace(cur), strings.TrimSpace(uniqueID)) {
				platform.EmitActionBarStatusI18n("Status_ActionBar_SavingSession")
				if err := saveCurrentAfterKill(deps, platformKey, prevName, d); err != nil {
					return wrapNeedsAdminIfPermission(err)
				}
			}
		}
	}

	platform.EmitActionBarStatusI18n("Status_ActionBar_ClearingSession")
	if err := ClearCurrentLogin(deps, platformKey); err != nil {
		return wrapNeedsAdminIfPermission(err)
	}
	accName, ok := ids[uniqueID]
	if !ok || strings.TrimSpace(accName) == "" {
		return fmt.Errorf("unknown account id")
	}
	platform.EmitActionBarStatusI18n("Status_ActionBar_RestoringAccount")
	if err := Login(deps, platformKey, accName); err != nil {
		return wrapNeedsAdminIfPermission(err)
	}
	_ = touchLastUsed(platformKey, uniqueID)
	recordBasicTrayRecent(platformKey, uniqueID)
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
		platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatformFailed", platformKey)
		return err
	}
	_ = winutil.KillByName(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod), electronBeforeKillSynth(deps, platformKey, d.ExesToEnd))
	platform.EmitActionBarStatusI18n("Status_ActionBar_ClearingSession")
	if err := ClearCurrentLogin(deps, platformKey); err != nil {
		return err
	}
	if !ps.AutoStart {
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

func launchBasicNoStatus(deps FlowDeps, platformKey string, extraLaunchArgs []string) error {
	return launchBasicNoStatusAs(deps, platformKey, false, extraLaunchArgs)
}

func launchBasicNoStatusAs(deps FlowDeps, platformKey string, forceAdmin bool, extraLaunchArgs []string) error {
	logFlow().Debug("launch begin", "platform", platformKey, "forceAdmin", forceAdmin, "extraArgs", len(extraLaunchArgs))
	d, _, err := readDescriptor(platformKey)
	if err != nil {
		logFlow().Warn("launch read descriptor failed", "platform", platformKey, "err", err)
		return err
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		logFlow().Warn("launch load settings failed", "platform", platformKey, "err", err)
		return err
	}
	if deps.PS == nil {
		return fmt.Errorf("platform service not set")
	}
	exe, err := deps.PS.ResolvePlatformExeFullPath(platformKey)
	if err != nil || exe == "" {
		logFlow().Warn("launch resolve exe failed", "platform", platformKey, "exe", exe, "err", err)
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
	logFlow().Debug("start request", "platform", platformKey, "exe", exe, "args", len(args), "method", opts.Method, "admin", opts.Admin)
	if err := winutil.Start(exe, args, opts); err != nil {
		logFlow().Warn("start failed", "platform", platformKey, "exe", exe, "err", err)
		return err
	}
	logFlow().Debug("start launched", "platform", platformKey, "exe", exe)
	return nil
}

func LaunchBasicAs(deps FlowDeps, platformKey string, forceAdmin bool, extraLaunchArgs []string) error {
	defer platform.EmitActionBarStatus("")
	platform.EmitActionBarStatusI18nPlatform("Status_StartingPlatform", platformKey)
	return launchBasicNoStatusAs(deps, platformKey, forceAdmin, extraLaunchArgs)
}
