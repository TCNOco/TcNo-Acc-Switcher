// Package logredact removes explicitly labelled credentials from diagnostic
// output. It deliberately does not guess whether arbitrary unlabelled text is
// secret.
package logredact

import (
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"runtime"
	"strings"
)

const Replacement = "[REDACTED]"

var (
	labelledSecretPattern = regexp.MustCompile(`(?i)(^|[^[:alnum:]_])((?:"|')?(?:password|passphrase|app[_-]?password|steam[_-]?guard[_-]?password|shared[_-]?secret|identity[_-]?secret|secret[_-]?1|revocation[_-]?code|access[_-]?token|refresh[_-]?token|oauth[_-]?token|bearer[_-]?token|token|session[_-]?id|sessionid|session|steamloginsecure|steamlogin|webcookie|set-cookie|cookie|qr[_-]?(?:auth[_-]?)?challenge|challenge)(?:"|')?[[:space:]]*(?::|=)[[:space:]]*)("(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^[:space:],;}\]]+)`)
	steamQRPattern        = regexp.MustCompile(`(?i)https://s\.team/q/[0-9]+/[0-9]+`)
	authorizationPattern  = regexp.MustCompile(`(?i)(authorization[[:space:]]*(?::|=)[[:space:]]*)(?:bearer[[:space:]]+)?[^[:space:],;}\]]+`)
)

// RedactText replaces credentials whose field or parameter name identifies
// them as secret, along with canonical Steam QR login challenge URLs.
func RedactText(text string) string {
	if text == "" {
		return ""
	}
	text = replaceLabelled(text, labelledSecretPattern)
	text = authorizationPattern.ReplaceAllString(text, `${1}`+Replacement)
	return steamQRPattern.ReplaceAllString(text, Replacement)
}

func replaceLabelled(text string, pattern *regexp.Regexp) string {
	matches := pattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text
	}
	var out strings.Builder
	out.Grow(len(text))
	last := 0
	for _, match := range matches {
		valueStart, valueEnd := match[6], match[7]
		out.WriteString(text[last:valueStart])
		value := text[valueStart:valueEnd]
		switch {
		case strings.HasPrefix(value, `"`):
			out.WriteString(`"` + Replacement + `"`)
		case strings.HasPrefix(value, `'`):
			out.WriteString(`'` + Replacement + `'`)
		default:
			out.WriteString(Replacement)
		}
		last = valueEnd
	}
	out.WriteString(text[last:])
	return out.String()
}

// FormatValue formats a diagnostic value after recursively removing labelled
// fields. Errors are reduced to their redacted public text.
func FormatValue(value any) string {
	return RedactText(fmt.Sprint(sanitizeAny(value, "", 0, make(map[visit]struct{}))))
}

func sanitizeAttr(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()
	if isSecretKey(attr.Key) {
		attr.Value = slog.StringValue(Replacement)
		return attr
	}
	if attr.Value.Kind() == slog.KindGroup {
		group := attr.Value.Group()
		for i := range group {
			group[i] = sanitizeAttr(group[i])
		}
		attr.Value = slog.GroupValue(group...)
		return attr
	}
	if attr.Value.Kind() == slog.KindString {
		attr.Value = slog.StringValue(RedactText(attr.Value.String()))
		return attr
	}
	if attr.Value.Kind() == slog.KindAny {
		attr.Value = slog.AnyValue(sanitizeAny(attr.Value.Any(), attr.Key, 0, make(map[visit]struct{})))
	}
	return attr
}

type visit struct {
	typ reflect.Type
	ptr uintptr
}

func sanitizeAny(value any, key string, depth int, seen map[visit]struct{}) any {
	if isSecretKey(key) {
		return Replacement
	}
	if value == nil {
		return nil
	}
	if err, ok := value.(error); ok {
		return RedactText(err.Error())
	}
	if stringer, ok := value.(fmt.Stringer); ok {
		return RedactText(safeString(stringer))
	}
	return sanitizeReflect(reflect.ValueOf(value), key, depth, seen)
}

func sanitizeReflect(value reflect.Value, key string, depth int, seen map[visit]struct{}) any {
	if !value.IsValid() {
		return nil
	}
	if isSecretKey(key) {
		return Replacement
	}
	if depth >= 8 {
		return "[VALUE OMITTED]"
	}

	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		if value.Kind() == reflect.Pointer {
			current := visit{typ: value.Type(), ptr: value.Pointer()}
			if _, ok := seen[current]; ok {
				return "[CYCLE]"
			}
			seen[current] = struct{}{}
			defer delete(seen, current)
		}
		value = value.Elem()
	}

	if value.CanInterface() {
		if err, ok := value.Interface().(error); ok {
			return RedactText(err.Error())
		}
		if stringer, ok := value.Interface().(fmt.Stringer); ok {
			return RedactText(safeString(stringer))
		}
	}

	switch value.Kind() {
	case reflect.String:
		return RedactText(value.String())
	case reflect.Bool:
		return value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Interface()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Interface()
	case reflect.Float32, reflect.Float64:
		return value.Interface()
	case reflect.Struct:
		if isSteamQRChallenge(value.Type()) {
			return map[string]any{"Version": value.FieldByName("Version").Interface(), "ClientID": Replacement}
		}
		result := make(map[string]any)
		typ := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name := diagnosticFieldName(field)
			if name == "-" || name == "" {
				continue
			}
			result[name] = sanitizeReflect(value.Field(i), name, depth+1, seen)
		}
		return result
	case reflect.Map:
		result := make(map[string]any)
		iter := value.MapRange()
		for iter.Next() {
			mapKey := fmt.Sprint(iter.Key().Interface())
			result[mapKey] = sanitizeReflect(iter.Value(), mapKey, depth+1, seen)
		}
		return result
	case reflect.Slice, reflect.Array:
		result := make([]any, value.Len())
		for i := 0; i < value.Len(); i++ {
			result[i] = sanitizeReflect(value.Index(i), key, depth+1, seen)
		}
		return result
	default:
		if value.CanInterface() {
			return value.Interface()
		}
		return "[VALUE OMITTED]"
	}
}

func diagnosticFieldName(field reflect.StructField) string {
	for _, tagName := range []string{"json", "yaml"} {
		if tag := strings.Split(field.Tag.Get(tagName), ",")[0]; tag != "" {
			return tag
		}
	}
	return field.Name
}

func isSteamQRChallenge(typ reflect.Type) bool {
	return typ.Name() == "Challenge" && strings.HasSuffix(typ.PkgPath(), "/internal/steamguard/qr")
}

func isSecretKey(key string) bool {
	normalized := strings.ToLower(key)
	normalized = strings.NewReplacer("_", "", "-", "", ".", "", " ", "").Replace(normalized)
	switch normalized {
	case "password", "passphrase", "apppassword", "steamguardpassword",
		"sharedsecret", "identitysecret", "secret1", "revocationcode",
		"token", "accesstoken", "refreshtoken", "oauthtoken", "bearertoken",
		"authorization", "session", "sessionid", "steamlogin", "steamloginsecure",
		"webcookie", "cookie", "setcookie", "qrchallenge", "qrauthchallenge", "challenge":
		return true
	}
	return strings.HasSuffix(normalized, "password") || strings.HasSuffix(normalized, "secret") || strings.HasSuffix(normalized, "token")
}

func safeString(value fmt.Stringer) (result string) {
	defer func() {
		if recover() != nil {
			result = "[STRING VALUE PANICKED]"
		}
	}()
	result = value.String()
	runtime.KeepAlive(value)
	return result
}
