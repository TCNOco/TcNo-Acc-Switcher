package basic

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/tidwall/gjson"
)

type builtInUniqueIDResolver func(d platform.Descriptor, ctx platform.PathTokenContext) (string, error)

var builtInUniqueIDResolvers = map[string]builtInUniqueIDResolver{
	"rockstar": builtInUniqueIDRockstarEmail,
}

func ReadUniqueID(platformKey string, d platform.Descriptor, platformFolder string) (string, error) {
	method := strings.TrimSpace(d.UniqueIdMethod)
	ctx := platform.PathTokenContext{PlatformFolder: platformFolder}
	slog.Debug("read unique id begin", "platform", platformKey, "method", method, "file", d.UniqueIdFile)

	switch strings.ToUpper(method) {
	case "REGKEY":
		id, err := uniqueFromRegKey(d)
		logUniqueIDResult(platformKey, method, id, err)
		return id, err
	case "CREATE_ID_FILE":
		id, err := uniqueFromCreateIDFile(d, ctx)
		logUniqueIDResult(platformKey, method, id, err)
		return id, err
	case "BUILTIN":
		id, err := uniqueFromBuiltIn(platformKey, d, ctx)
		logUniqueIDResult(platformKey, method, id, err)
		return id, err
	case "LEVELDB":
		id, err := uniqueFromLevelDB(d, platformFolder, ctx)
		logUniqueIDResult(platformKey, method, id, err)
		return id, err
	case "STEAM":
		return "", fmt.Errorf("STEAM unique id: use Steam service")
	default:
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(method)), "JSON_SELECT") ||
			strings.HasPrefix(strings.TrimSpace(d.UniqueIdFile), "JSON_SELECT") {
			id, err := uniqueFromJSONSelect(d, ctx)
			logUniqueIDResult(platformKey, method, id, err)
			return id, err
		}
		id, err := uniqueFromFileRegex(d, ctx)
		logUniqueIDResult(platformKey, method, id, err)
		return id, err
	}
}

func uniqueFromLevelDB(d platform.Descriptor, platformFolder string, ctx platform.PathTokenContext) (string, error) {
	raw := strings.TrimSpace(d.UniqueIdFile)
	if raw == "" {
		return "", fmt.Errorf("empty UniqueIdFile")
	}
	if !isLevelDBReference(raw) {
		return "", fmt.Errorf("LEVELDB unique id requires leveldb: reference")
	}
	vars := resolveDescriptorVariables(d, platformFolder, ctx, "", false)
	ref := expandDescriptorVariables(raw, vars)
	if parsed, err := parseLevelDBReference(ref); err == nil {
		expandedPath := platform.ExpandPathTokens(platform.ExpandWindowsPath(parsed.Path), ctx)
		slog.Debug("unique id leveldb resolve", "ref", ref, "expandedPath", expandedPath, "key", parsed.Key, "jsonPath", parsed.JSONPath)
	} else {
		slog.Debug("unique id leveldb resolve", "ref", ref, "parseErr", err)
	}
	return resolveLevelDBReference(ref, ctx)
}

func logUniqueIDResult(platformKey, method, id string, err error) {
	if err != nil {
		slog.Debug("read unique id failed", "platform", platformKey, "method", method, "err", err)
		return
	}
	slog.Debug("read unique id success", "platform", platformKey, "method", method, "valuePreview", previewLevelDBValue(strings.TrimSpace(id)))
}

func uniqueFromBuiltIn(platformKey string, d platform.Descriptor, ctx platform.PathTokenContext) (string, error) {
	resolver, ok := builtInUniqueIDResolvers[strings.ToLower(strings.TrimSpace(platformKey))]
	if !ok {
		return "", fmt.Errorf("builtin unique id unsupported for platform %q", platformKey)
	}
	id, err := resolver(d, ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(id), nil
}

func uniqueFromRegKey(d platform.Descriptor) (string, error) {
	enc := stripREG(strings.TrimSpace(d.UniqueIdFile))
	if enc == "" {
		return "", fmt.Errorf("empty UniqueIdFile")
	}
	if kp, vglob, ok := splitRegistryKeyPathAndValueGlob(enc); ok {
		_, v, typ, err := winutil.RegistryReadFirstValueMatchingNameGlob(kp, vglob)
		if err != nil {
			return "", fmt.Errorf("unique id registry REG:%s: %w", enc, err)
		}
		return registryCellToUniqueString(v, typ)
	}
	v, typ, err := winutil.RegistryRead(enc)
	if err != nil {
		return "", fmt.Errorf("unique id registry REG:%s: %w", enc, err)
	}
	return registryCellToUniqueString(v, typ)
}

func registryCellToUniqueString(v any, typ uint32) (string, error) {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x), nil
	case []byte:
		h := sha1.Sum(x)
		return hex.EncodeToString(h[:]), nil
	default:
		return fmt.Sprintf("%v_%d", x, typ), nil
	}
}

func uniqueFromCreateIDFile(d platform.Descriptor, ctx platform.PathTokenContext) (string, error) {
	p := platform.ExpandPathTokens(platform.ExpandWindowsPath(d.UniqueIdFile), ctx)
	data, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("read unique id file %s: %w", p, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func uniqueFromFileRegex(d platform.Descriptor, ctx platform.PathTokenContext) (string, error) {
	p := platform.ExpandPathTokens(platform.ExpandWindowsPath(d.UniqueIdFile), ctx)
	data, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("read unique id file %s: %w", p, err)
	}
	re, err := platform.ExpandRegex(d.UniqueIdRegex)
	if err != nil {
		return "", fmt.Errorf("unique id regex compile: %w", err)
	}
	if re == nil {
		return strings.TrimSpace(string(data)), nil
	}
	m := re.FindStringSubmatch(string(data))
	if len(m) > 1 {
		return strings.TrimSpace(m[1]), nil
	}
	if len(m) == 1 {
		return strings.TrimSpace(m[0]), nil
	}
	return "", fmt.Errorf("unique id regex: no match")
}

func uniqueFromJSONSelect(d platform.Descriptor, ctx platform.PathTokenContext) (string, error) {
	key := strings.TrimSpace(d.UniqueIdFile)
	if !strings.HasPrefix(key, "JSON_SELECT") {
		m := strings.TrimSpace(d.UniqueIdMethod)
		if m != "" {
			key = m + "::" + key
		}
	}
	var filePath, jsonPath, delimiter string
	var ok bool
	first := strings.HasPrefix(key, "JSON_SELECT_FIRST")
	if first {
		filePath, jsonPath, delimiter, ok = parseJSONSelectWithDelimiter("JSON_SELECT_FIRST", key)
	} else {
		filePath, jsonPath, delimiter, ok = parseJSONSelectWithDelimiter("JSON_SELECT_LAST", key)
	}
	if !ok {
		return "", fmt.Errorf("bad JSON_SELECT UniqueIdFile")
	}
	filePath = expandPlatformPath(filePath, ctx.PlatformFolder, ctx)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read unique id JSON file %s: %w", filePath, err)
	}
	res := gjson.GetBytes(data, jsonPath)
	s := strings.TrimSpace(res.String())
	if res.IsArray() && len(res.Array()) > 0 {
		if first {
			s = strings.TrimSpace(res.Array()[0].String())
		} else {
			a := res.Array()
			s = strings.TrimSpace(a[len(a)-1].String())
		}
	} else if delimiter != "" && s != "" {
		parts := strings.Split(s, delimiter)
		if len(parts) > 0 {
			if first {
				s = strings.TrimSpace(parts[0])
			} else {
				s = strings.TrimSpace(parts[len(parts)-1])
			}
		}
	}
	return strings.TrimSpace(s), nil
}

func builtInUniqueIDRockstarEmail(d platform.Descriptor, ctx platform.PathTokenContext) (string, error) {
	pat := strings.TrimSpace(d.UniqueIdFile)
	if pat == "" {
		return "", fmt.Errorf("empty built-in unique id file pattern")
	}
	globPat := platform.ExpandPathTokens(platform.ExpandWindowsPath(pat), ctx)
	files, err := filepath.Glob(globPat)
	if err != nil || len(files) == 0 {
		return "", fmt.Errorf("rockstar builtin unique id: no files matched %s", globPat)
	}
	sort.Slice(files, func(i, j int) bool {
		ist, ierr := os.Stat(files[i])
		jst, jerr := os.Stat(files[j])
		if ierr != nil || jerr != nil {
			return files[i] > files[j]
		}
		return ist.ModTime().After(jst.ModTime())
	})
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil || len(data) == 0 {
			continue
		}
		email := parseRockstarEmail(data)
		if strings.TrimSpace(email) != "" {
			slog.Debug("builtin unique id rockstar email extracted", "file", f, "email", email)
			return strings.TrimSpace(email), nil
		}
	}
	return "", fmt.Errorf("rockstar builtin unique id: no <Email> found in %d files", len(files))
}

func parseRockstarEmail(data []byte) string {
	startTag := []byte("<Email>")
	endTag := []byte("</Email>")
	search := data
	for {
		i := bytes.Index(search, startTag)
		if i < 0 {
			return ""
		}
		search = search[i+len(startTag):]
		j := bytes.Index(search, endTag)
		if j < 0 {
			return ""
		}
		v := strings.TrimSpace(string(search[:j]))
		if v != "" {
			return v
		}
		search = search[j+len(endTag):]
	}
}
