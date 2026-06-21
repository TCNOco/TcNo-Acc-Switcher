package paths

import (
	"strings"
	"unicode"
)

// WindowsFileName normalizes arbitrary user text for a single Windows path segment
// (folder name, .lnk base name). Letters and numbers from any script are kept;
// Windows-forbidden characters, control/format characters, and most symbols (e.g. emoji)
// become underscores. Trailing spaces and dots are stripped. maxRunes > 0 truncates by rune count.
func WindowsFileName(name string, maxRunes int) string {
	s := strings.TrimSpace(name)
	s = strings.ToValidUTF8(s, "")
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 32:
			b.WriteRune('_')
		case r == '<' || r == '>' || r == ':' || r == '"' || r == '/' || r == '\\' || r == '|' || r == '?' || r == '*':
			b.WriteRune('_')
		case unicode.Is(unicode.Cf, r):
			continue
		case unicode.IsLetter(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) || unicode.Is(unicode.Me, r):
			b.WriteRune(r)
		case unicode.IsNumber(r):
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '-' || r == '.' || r == '(' || r == ')' || r == ',' || r == '\'' || r == '&' || r == '+' || r == '[' || r == ']':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	trimCollapsed := func(s string) string {
		out := strings.TrimRight(s, " .")
		out = strings.TrimSpace(out)
		var b strings.Builder
		b.Grow(len(out))
		var prev byte
		for i := 0; i < len(out); i++ {
			c := out[i]
			switch c {
			case '_':
				if prev == '_' {
					continue
				}
				prev = '_'
				b.WriteByte('_')
			case ' ':
				if prev == '_' {
					continue
				}
				if i+1 < len(out) && out[i+1] == '_' {
					continue
				}
				prev = ' '
				b.WriteByte(' ')
			default:
				prev = c
				b.WriteByte(c)
			}
		}
		return strings.Trim(strings.Trim(b.String(), "."), "_")
	}
	out := trimCollapsed(b.String())
	if out == "" || out == "." || out == ".." {
		return ""
	}
	if windowsReservedFileStem(out) {
		out = out + "_"
	}
	if maxRunes > 0 {
		rr := []rune(out)
		if len(rr) > maxRunes {
			out = trimCollapsed(string(rr[:maxRunes]))
		}
	}
	if out == "" || out == "." || out == ".." {
		return ""
	}
	if windowsReservedFileStem(out) {
		out = out + "_"
	}
	return out
}
