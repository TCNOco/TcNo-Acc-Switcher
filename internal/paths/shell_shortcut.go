package paths

import "strings"

// ShellShortcutBaseName normalizes text for WScript.Shell .lnk Save: after [WindowsFileName],
// only ASCII letters, digits, and a small punctuation set remain (other runes become '_').
// This avoids COM save failures from exotic Unicode in the Desktop path.
func ShellShortcutBaseName(name string, maxRunes int) string {
	s := WindowsFileName(name, 0)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '_', r == '-', r == '.', r == '(', r == ')':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	for strings.Contains(out, "__") {
		out = strings.ReplaceAll(out, "__", "_")
	}
	for strings.Contains(out, " _") || strings.Contains(out, "_ ") {
		out = strings.ReplaceAll(out, " _", "_")
		out = strings.ReplaceAll(out, "_ ", "_")
	}
	out = strings.TrimRight(out, " .")
	out = strings.Trim(out, "._")
	if out == "" || out == "." || out == ".." {
		return ""
	}
	if windowsReservedFileStem(out) {
		out = out + "_"
	}
	if maxRunes > 0 {
		rr := []rune(out)
		if len(rr) > maxRunes {
			out = string(rr[:maxRunes])
			out = strings.TrimRight(out, " .")
			out = strings.Trim(out, "._")
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
