package basic

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// GameStatVarContext holds built-in substitutions for GameStats.json Vars templates.
type GameStatVarContext struct {
	AccountID       string
	AccountUsername string
	Username        string
}

// ResolveGameStatsVarTemplates resolves all Vars entries for a game definition using stored
// user values (when the definition is a plain label), computed templates, and dependency order.
func ResolveGameStatsVarTemplates(defVars map[string]string, stored map[string]string, ctx GameStatVarContext) map[string]string {
	if defVars == nil {
		defVars = map[string]string{}
	}
	if stored == nil {
		stored = map[string]string{}
	}
	out := map[string]string{}

	keys := make([]string, 0, len(defVars))
	for k := range defVars {
		keys = append(keys, k)
	}
	sortStringsFold(keys)

	order, _ := topoSortGameStatVars(keys, defVars)

	resolved := map[string]string{
		"ACCOUNTID":       strings.TrimSpace(ctx.AccountID),
		"ACCOUNTUSERNAME": strings.TrimSpace(ctx.AccountUsername),
		// Compatibility alias for legacy/common token names.
		// Uses raw username/display source (no AccountID fallback) when available.
		"USERNAME": strings.TrimSpace(ctx.Username),
	}
	for _, k := range keys {
		rawDef := strings.TrimSpace(defVars[k])
		if rawDef != "" && !gameStatVarNeedsComputation(rawDef) {
			resolved[k] = strings.TrimSpace(stored[k])
		}
	}

	iters := len(keys) + 3
	for iter := 0; iter < iters; iter++ {
		for _, k := range order {
			rawDef := strings.TrimSpace(defVars[k])
			if rawDef == "" || !gameStatVarNeedsComputation(rawDef) {
				continue
			}
			s := expandGameStatPercentTokens(rawDef, resolved)
			s = applyVariableTransformPipeline(s)
			resolved[k] = strings.TrimSpace(s)
		}
	}

	for _, k := range keys {
		rawDef := strings.TrimSpace(defVars[k])
		switch {
		case rawDef == "":
			out[k] = strings.TrimSpace(stored[k])
		case !gameStatVarNeedsComputation(rawDef):
			out[k] = strings.TrimSpace(stored[k])
		default:
			out[k] = strings.TrimSpace(resolved[k])
		}
	}
	return out
}

func gameStatVarNeedsComputation(def string) bool {
	def = strings.TrimSpace(def)
	if def == "" {
		return false
	}
	if strings.Contains(def, "%") {
		return true
	}
	return strings.Contains(def, "|") && splitTransformPipeline(def).hasTransforms
}

func extractPercentTokens(s string) []string {
	var out []string
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		j := i + 1
		for j < len(s) && (unicode.IsLetter(rune(s[j])) || unicode.IsDigit(rune(s[j])) || s[j] == '_') {
			j++
		}
		if j > i+1 && j < len(s) && s[j] == '%' {
			out = append(out, s[i+1:j])
			i = j
		}
	}
	return out
}

func topoSortGameStatVars(keys []string, defVars map[string]string) ([]string, bool) {
	// Kahn topological sort on edges: if A's template references %B%, then B before A.
	inDeg := map[string]int{}
	adj := map[string][]string{}
	for _, k := range keys {
		inDeg[k] = 0
	}
	for _, k := range keys {
		for _, tok := range extractPercentTokens(defVars[k]) {
			if strings.EqualFold(tok, "ACCOUNTID") || strings.EqualFold(tok, "ACCOUNTUSERNAME") {
				continue
			}
			dep := resolveTokenToVarKey(tok, keys)
			if dep == "" || dep == k {
				continue
			}
			adj[dep] = append(adj[dep], k)
			inDeg[k]++
		}
	}

	var q []string
	for _, k := range keys {
		if inDeg[k] == 0 {
			q = append(q, k)
		}
	}
	sortStringsFold(q)

	var order []string
	for len(q) > 0 {
		n := q[0]
		q = q[1:]
		order = append(order, n)
		nxt := append([]string(nil), adj[n]...)
		sortStringsFold(nxt)
		for _, m := range nxt {
			inDeg[m]--
			if inDeg[m] == 0 {
				q = append(q, m)
			}
		}
		sortStringsFold(q)
	}
	if len(order) != len(keys) {
		// Cycle or missing deps — fall back to stable key order
		return keys, false
	}
	return order, true
}

func resolveTokenToVarKey(tok string, keys []string) string {
	for _, k := range keys {
		if strings.EqualFold(k, tok) {
			return k
		}
	}
	return ""
}

func lookupFold(vars map[string]string, tok string) string {
	if v, ok := vars[tok]; ok {
		return v
	}
	tl := strings.ToLower(tok)
	for k, v := range vars {
		if strings.ToLower(k) == tl {
			return v
		}
	}
	return ""
}

func expandGameStatPercentTokens(s string, vars map[string]string) string {
	out := s
	for _, tok := range extractPercentTokens(s) {
		val := lookupFold(vars, tok)
		out = strings.ReplaceAll(out, "%"+tok+"%", val)
	}
	return out
}

type pipelineSplit struct {
	base           string
	transformExprs []string
	hasTransforms  bool
}

func splitTransformPipeline(s string) pipelineSplit {
	s = strings.TrimSpace(s)
	if s == "" {
		return pipelineSplit{}
	}
	baseStart := 0
	var transforms []string
	i := 0
	for i < len(s) {
		if s[i] != '|' {
			i++
			continue
		}
		candidate := strings.TrimSpace(s[i+1:])
		name, _, _, ok := parseLeadingFuncCall(candidate)
		if !ok {
			i++
			continue
		}
		if name == "" {
			i++
			continue
		}
		base := strings.TrimSpace(s[baseStart:i])
		rest := s[i+1:]
		transforms = collectTransforms(rest)
		return pipelineSplit{base: base, transformExprs: transforms, hasTransforms: len(transforms) > 0}
	}
	return pipelineSplit{base: strings.TrimSpace(s), hasTransforms: false}
}

func collectTransforms(rest string) []string {
	rest = strings.TrimSpace(rest)
	var out []string
	for rest != "" {
		name, args, after, ok := parseLeadingFuncCall(rest)
		if !ok || name == "" {
			break
		}
		out = append(out, formatFuncCall(name, args))
		after = strings.TrimSpace(after)
		if strings.HasPrefix(after, "|") {
			after = strings.TrimSpace(after[1:])
		}
		rest = after
	}
	return out
}

func formatFuncCall(name string, args []string) string {
	var b strings.Builder
	b.WriteString(name)
	b.WriteByte('(')
	for i, a := range args {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(a)
	}
	b.WriteByte(')')
	return b.String()
}

// parseLeadingFuncCall parses Name(args) at start; args are comma-separated with quoted strings using ' or ".
func parseLeadingFuncCall(s string) (name string, args []string, remainder string, ok bool) {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && (unicode.IsLetter(rune(s[i])) || unicode.IsDigit(rune(s[i])) || s[i] == '_') {
		i++
	}
	if i == 0 || i >= len(s) || s[i] != '(' {
		return "", nil, s, false
	}
	name = s[:i]
	depth := 0
	j := i
	for ; j < len(s); j++ {
		if s[j] == '(' {
			depth++
		} else if s[j] == ')' {
			depth--
			if depth == 0 {
				break
			}
		}
	}
	if j >= len(s) || s[j] != ')' {
		return "", nil, s, false
	}
	inner := s[i+1 : j]
	remainder = strings.TrimSpace(s[j+1:])
	args = splitFuncArgs(inner)
	return name, args, remainder, true
}

func splitFuncArgs(inner string) []string {
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return nil
	}
	var out []string
	var cur strings.Builder
	inQuote := byte(0)
	for i := 0; i < len(inner); i++ {
		ch := inner[i]
		switch {
		case inQuote != 0:
			if ch == inQuote {
				inQuote = 0
				continue
			}
			cur.WriteByte(ch)
		case ch == '\'' || ch == '"':
			inQuote = ch
		case ch == ',':
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(ch)
		}
	}
	out = append(out, strings.TrimSpace(cur.String()))
	return out
}

func applyVariableTransformPipeline(s string) string {
	ps := splitTransformPipeline(s)
	cur := strings.TrimSpace(ps.base)
	for _, expr := range ps.transformExprs {
		cur = applyOneTransform(cur, expr)
	}
	return cur
}

func applyOneTransform(input string, expr string) string {
	name, args, _, ok := parseLeadingFuncCall(expr)
	if !ok {
		return input
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "replacefirst":
		if len(args) < 2 {
			return input
		}
		oldS, newS := unquoteArg(args[0]), unquoteArg(args[1])
		return replaceFirst(input, oldS, newS)
	case "replacelast":
		if len(args) < 2 {
			return input
		}
		oldS, newS := unquoteArg(args[0]), unquoteArg(args[1])
		return replaceLast(input, oldS, newS)
	case "replaceall":
		if len(args) < 2 {
			return input
		}
		oldS, newS := unquoteArg(args[0]), unquoteArg(args[1])
		return strings.ReplaceAll(input, oldS, newS)
	case "substring":
		if len(args) != 1 {
			return input
		}
		return substringArg(input, strings.TrimSpace(args[0]))
	default:
		return input
	}
}

func unquoteArg(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

func replaceFirst(s, old, new string) string {
	if old == "" {
		return s
	}
	i := strings.Index(s, old)
	if i < 0 {
		return s
	}
	return s[:i] + new + s[i+len(old):]
}

func replaceLast(s, old, new string) string {
	if old == "" {
		return s
	}
	i := strings.LastIndex(s, old)
	if i < 0 {
		return s
	}
	return s[:i] + new + s[i+len(old):]
}

func substringArg(s, spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return s
	}
	if strings.Contains(spec, ":") {
		k := strings.IndexByte(spec, ':')
		left := strings.TrimSpace(spec[:k])
		right := strings.TrimSpace(spec[k+1:])
		start, end := 0, len(s)
		var err error
		if left != "" {
			start, err = strconv.Atoi(left)
			if err != nil || start < 0 {
				start = 0
			}
			if start > len(s) {
				return ""
			}
		}
		if right != "" {
			end, err = strconv.Atoi(right)
			if err != nil || end < 0 {
				end = len(s)
			}
			if end > len(s) {
				end = len(s)
			}
		} else {
			end = len(s)
		}
		if start > end {
			return ""
		}
		return s[start:end]
	}
	n, err := strconv.Atoi(spec)
	if err != nil {
		return s
	}
	if n < 0 || n > len(s) {
		return ""
	}
	return s[n:]
}

func sortStringsFold(ss []string) {
	sort.Slice(ss, func(i, j int) bool {
		return strings.ToLower(ss[i]) < strings.ToLower(ss[j])
	})
}
