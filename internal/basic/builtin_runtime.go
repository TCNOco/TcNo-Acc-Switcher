package basic

import (
	"os"
	"regexp"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
)

var battleNetAccountDBPathRegex = regexp.MustCompile(`(?im)Opened database at:\s+.*[\\/]Battle\.net[\\/]Account[\\/]([0-9]+)[\\/]account\.db`)

func resolveBuiltInRuntimeVariables(platformKey string, d platform.Descriptor, folder string, ctx platform.PathTokenContext, vars map[string]string, accountCacheRoot string, saved bool) map[string]string {
	out := map[string]string{}
	for k, v := range vars {
		out[k] = v
	}
	rawBuiltInUserID := strings.TrimSpace(d.Extras.BuiltInUserId)
	if rawBuiltInUserID == "" {
		return out
	}
	resolvedUserID := resolveBuiltInUserID(platformKey, d, rawBuiltInUserID, folder, ctx, out, accountCacheRoot, saved)
	if strings.TrimSpace(resolvedUserID) == "" {
		return out
	}
	out["builtinuserid"] = resolvedUserID
	out["BuiltInUserId"] = resolvedUserID
	return out
}

func resolveBuiltInUserID(platformKey string, d platform.Descriptor, rawBuiltInUserID, folder string, ctx platform.PathTokenContext, vars map[string]string, accountCacheRoot string, saved bool) string {
	resolved := strings.TrimSpace(resolveDescriptorValue(d, rawBuiltInUserID, folder, ctx, vars, accountCacheRoot, saved))
	if resolved == "" {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(platformKey), "BattleNet") {
		if data, err := os.ReadFile(resolved); err == nil && len(data) > 0 {
			if accountID := parseBattleNetAccountIDFromLogData(data); accountID != "" {
				return accountID
			}
		}
	}
	return resolved
}

func parseBattleNetAccountIDFromLogData(data []byte) string {
	all := battleNetAccountDBPathRegex.FindAllSubmatch(data, -1)
	if len(all) == 0 {
		return ""
	}
	for i := len(all) - 1; i >= 0; i-- {
		if len(all[i]) < 2 {
			continue
		}
		id := strings.TrimSpace(string(all[i][1]))
		if id != "" {
			return id
		}
	}
	return ""
}
