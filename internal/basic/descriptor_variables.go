package basic

import (
	"log/slog"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
)

var descriptorVarsLog = slog.Default().With("component", "descriptor-variables")

func resolveDescriptorVariables(d platform.Descriptor, folder string, ctx platform.PathTokenContext, accountCacheRoot string, saved bool) map[string]string {
	out := map[string]string{}
	for k, raw := range d.Extras.Variables {
		name := strings.ToLower(strings.TrimSpace(k))
		if name == "" {
			continue
		}
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if isLevelDBReference(v) {
			ref := v
			if saved {
				ref = mapSavedLevelDBReference(d, folder, ctx, ref, accountCacheRoot)
			}
			descriptorVarsLog.Debug("resolve variable via leveldb", "name", name, "saved", saved, "ref", ref)
			if resolved, err := resolveLevelDBReference(ref, ctx); err == nil {
				out[name] = strings.TrimSpace(resolved)
				descriptorVarsLog.Debug("resolved variable via leveldb", "name", name, "valuePreview", previewLevelDBValue(out[name]))
				continue
			} else {
				descriptorVarsLog.Debug("resolve variable via leveldb failed", "name", name, "saved", saved, "ref", ref, "err", err)
				// Do not treat failed leveldb references as plain paths/strings.
				// Keep variable empty so callers never receive literal ".\\leveldb:..." text.
				out[name] = ""
				continue
			}
		}
		out[name] = strings.TrimSpace(expandDescriptorVariables(expandPlatformPath(v, folder, ctx), out))
		descriptorVarsLog.Debug("resolved variable via template", "name", name, "valuePreview", previewLevelDBValue(out[name]))
	}
	return out
}

func expandDescriptorVariables(s string, vars map[string]string) string {
	out := s
	for k, v := range vars {
		if strings.TrimSpace(k) == "" {
			continue
		}
		out = strings.ReplaceAll(out, "%"+k+"%", v)
	}
	return out
}

func expandWithDescriptorVariables(s string, vars map[string]string, folder string, ctx platform.PathTokenContext) string {
	return expandDescriptorVariables(expandPlatformPath(s, folder, ctx), vars)
}

func resolveDescriptorValue(d platform.Descriptor, raw, folder string, ctx platform.PathTokenContext, vars map[string]string, accountCacheRoot string, saved bool) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	v = strings.TrimSpace(expandDescriptorVariables(v, vars))
	if resolved, handled, err := resolveLatestModifiedFileValue(v, folder, ctx); handled {
		if err != nil {
			descriptorVarsLog.Debug("resolve descriptor value via latest modified file failed", "saved", saved, "value", v, "err", err)
			return ""
		}
		descriptorVarsLog.Debug("resolved descriptor value via latest modified file", "value", resolved)
		return strings.TrimSpace(resolved)
	}
	if isLevelDBReference(v) {
		ref := v
		if saved {
			ref = mapSavedLevelDBReference(d, folder, ctx, ref, accountCacheRoot)
		}
		descriptorVarsLog.Debug("resolve descriptor value via leveldb", "saved", saved, "ref", ref)
		if resolved, err := resolveLevelDBReference(ref, ctx); err == nil {
			out := strings.TrimSpace(resolved)
			descriptorVarsLog.Debug("resolved descriptor value via leveldb", "valuePreview", previewLevelDBValue(out))
			return out
		} else {
			descriptorVarsLog.Debug("resolve descriptor value via leveldb failed", "saved", saved, "ref", ref, "err", err)
			// Do not degrade to plain path expansion for command values.
			return ""
		}
	}
	if resolved, handled, err := resolveSQLiteValue(v, folder, ctx); handled {
		if err != nil {
			descriptorVarsLog.Debug("resolve descriptor value via sqlite failed", "saved", saved, "value", v, "err", err)
			return ""
		}
		descriptorVarsLog.Debug("resolved descriptor value via sqlite", "valuePreview", previewLevelDBValue(resolved))
		return strings.TrimSpace(resolved)
	}
	return strings.TrimSpace(expandPlatformPath(v, folder, ctx))
}

func mapSavedLevelDBReference(d platform.Descriptor, folder string, ctx platform.PathTokenContext, ref, accountCacheRoot string) string {
	parsed, err := parseLevelDBReference(ref)
	if err != nil {
		return ref
	}
	livePath := expandPlatformPath(parsed.Path, folder, ctx)
	savedPath := mapLivePathToSavedPath(d, folder, ctx, livePath, accountCacheRoot)
	if strings.TrimSpace(savedPath) == "" {
		return ref
	}
	mapped := "leveldb:" + savedPath + ":" + parsed.Key
	if strings.TrimSpace(parsed.JSONPath) != "" {
		mapped += ":" + parsed.JSONPath
	}
	return mapped
}
