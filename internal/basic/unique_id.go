package basic

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/tidwall/gjson"
)

func ReadUniqueID(d platform.Descriptor, platformFolder string) (string, error) {
	method := strings.TrimSpace(d.UniqueIdMethod)
	ctx := platform.PathTokenContext{PlatformFolder: platformFolder}

	switch strings.ToUpper(method) {
	case "REGKEY":
		return uniqueFromRegKey(d)
	case "CREATE_ID_FILE":
		return uniqueFromCreateIDFile(d, ctx)
	case "STEAM":
		return "", fmt.Errorf("STEAM unique id: use Steam service")
	default:
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(method)), "JSON_SELECT") ||
			strings.HasPrefix(strings.TrimSpace(d.UniqueIdFile), "JSON_SELECT") {
			return uniqueFromJSONSelect(d, ctx)
		}
		return uniqueFromFileRegex(d, ctx)
	}
}

func uniqueFromRegKey(d platform.Descriptor) (string, error) {
	enc := stripREG(strings.TrimSpace(d.UniqueIdFile))
	if enc == "" {
		return "", fmt.Errorf("empty UniqueIdFile")
	}
	v, typ, err := winutil.RegistryRead(enc)
	if err != nil {
		return "", err
	}
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
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func uniqueFromFileRegex(d platform.Descriptor, ctx platform.PathTokenContext) (string, error) {
	p := platform.ExpandPathTokens(platform.ExpandWindowsPath(d.UniqueIdFile), ctx)
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
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
		return "", err
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
