package basic

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/tidwall/gjson"
	htmlnet "golang.org/x/net/html"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/gamestatsimage"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/profileimage"
)

const gameStatsImageDownloadMaxBytes = 2 << 20 // 2 MiB

func gameDefinitionUsesJSONOnly(def gameDefinition) bool {
	if len(def.Collect) == 0 {
		return false
	}
	for _, ci := range def.Collect {
		key := strings.TrimSpace(ci.Source)
		if key == "" {
			return false
		}
		if strings.ToLower(key) != "json" {
			return false
		}
	}
	return true
}

func substituteGameStatsURL(tmpl string, vars map[string]string) string {
	out := tmpl
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
}

// GameStatsHTTPError is returned when the stats URL responds with a non-success HTTP status.
type GameStatsHTTPError struct {
	StatusCode int
}

func (e *GameStatsHTTPError) Error() string {
	return fmt.Sprintf("HTTP %d", e.StatusCode)
}

// isGameStatsResourceNotFound reports whether err indicates the remote resource does not exist
// (so we should not keep game stats enabled for that account/URL).
func isGameStatsResourceNotFound(err error) bool {
	var he *GameStatsHTTPError
	return errors.As(err, &he) && (he.StatusCode == http.StatusNotFound || he.StatusCode == http.StatusGone)
}

func fetchGameStatsHTML(ctx context.Context, urlStr, cookiesHeader string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TcNo-Acc-Switcher")
	if strings.TrimSpace(cookiesHeader) != "" {
		req.Header.Set("Cookie", cookiesHeader)
	}
	resp, err := appclient.Shared.Do(req)
	if err != nil {
		gameStatsLog.Debug("game stats fetch transport error", "url", urlStr, "err", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		gameStatsLog.Debug("game stats fetch non-2xx", "url", urlStr, "status", resp.StatusCode)
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if msg := strings.TrimSpace(gjson.GetBytes(body, "Error").String()); msg != "" {
			return nil, fmt.Errorf("%s", msg)
		}
		return nil, &GameStatsHTTPError{StatusCode: resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	gameStatsLog.Debug("game stats fetch success", "url", urlStr, "bytes", len(data))
	return data, nil
}

func nodeInnerHTML(n *htmlnet.Node) string {
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		_ = htmlnet.Render(&b, c)
	}
	return b.String()
}

// applyDisplayPlaceholders replaces tokens like %fill% in DisplayAs using numeric ranges over the
// collected value (same string later substituted for %x%). Rules are applied in order; each rule's
// ranges are checked in order and the first inclusive match wins.
func applyDisplayPlaceholders(displayAs, metricValue string, rules []displayPlaceholderRule) string {
	if len(rules) == 0 {
		return displayAs
	}
	s := displayAs
	for _, rule := range rules {
		key := strings.TrimSpace(rule.Key)
		if key == "" {
			continue
		}
		from := strings.TrimSpace(rule.From)
		if from == "" {
			from = "x"
		}
		token := "%" + key + "%"
		val := rule.Default
		if strings.EqualFold(from, "x") {
			n, err := strconv.ParseFloat(strings.TrimSpace(metricValue), 64)
			if err == nil {
				for _, r := range rule.Ranges {
					if r.Min != nil && n < *r.Min {
						continue
					}
					if r.Max != nil && n > *r.Max {
						continue
					}
					val = r.Value
					break
				}
			}
		}
		s = strings.ReplaceAll(s, token, val)
	}
	return s
}

// displayFormatCommaNumberRE matches a plain decimal number (optional fraction) for comma formatting.
var displayFormatCommaNumberRE = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

// applyDisplayFormat returns a human-oriented rendering of raw for templates. Empty format is a no-op.
// Supported: commaNumber (aliases: comma_number, thousands, us_thousands) — US-style thousands separators.
func applyDisplayFormat(raw, format string) string {
	f := strings.ToLower(strings.TrimSpace(format))
	switch f {
	case "", "none":
		return raw
	case "commanumber", "comma_number", "thousands", "us_thousands":
		s := strings.TrimSpace(raw)
		if !displayFormatCommaNumberRE.MatchString(s) {
			return raw
		}
		neg := strings.HasPrefix(s, "-")
		if neg {
			s = s[1:]
		}
		parts := strings.SplitN(s, ".", 2)
		intDigits := parts[0]
		if intDigits == "" {
			return raw
		}
		out := addThousandSeparators(intDigits, ',')
		if neg {
			out = "-" + out
		}
		if len(parts) == 2 && parts[1] != "" {
			out += "." + parts[1]
		}
		return out
	default:
		return raw
	}
}

func addThousandSeparators(intDigits string, sep byte) string {
	n := len(intDigits)
	if n <= 3 {
		return intDigits
	}
	lead := n % 3
	if lead == 0 {
		lead = 3
	}
	var b strings.Builder
	b.WriteString(intDigits[:lead])
	for i := lead; i < n; i += 3 {
		b.WriteByte(sep)
		b.WriteString(intDigits[i : i+3])
	}
	return b.String()
}

func collectStatsFromHTML(platformKey, accountID string, def gameDefinition, doc *htmlnet.Node, rawBody []byte) (map[string]string, error) {
	out := map[string]string{}
	missing := 0
	for itemName, ci := range def.Collect {
		itemName = strings.TrimSpace(itemName)
		text := ""
		var ok bool
		source := strings.ToLower(strings.TrimSpace(ci.Source))
		if source == "json" {
			text, ok = extractMetricFromJSON(rawBody, ci)
		} else {
			text, ok = extractMetricFromHTML(doc, ci)
		}
		if !ok {
			missing++
			continue
		}
		if sf := strings.TrimSpace(ci.SelectFunc); sf != "" {
			text = applySelectFunc(text, sf)
		}
		for _, expr := range ci.Pipeline {
			expr = strings.TrimSpace(expr)
			if expr == "" {
				continue
			}
			text = applySelectFunc(text, expr)
		}
		if strings.TrimSpace(ci.NoDisplayIf) != "" && text == strings.TrimSpace(ci.NoDisplayIf) {
			continue
		}

		if strings.EqualFold(itemName, "%profileimage%") {
			if strings.TrimSpace(platformKey) != "" && text != "" {
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				defer cancel()
				_, _ = profileimage.DownloadIfNeeded(ctx, appclient.Shared, platformKey, accountID, text, 7)
			}
			continue
		}

		displayAs := strings.TrimSpace(ci.DisplayAs)
		if displayAs == "" {
			displayAs = "%x%"
		}

		xVal := text
		if strings.EqualFold(strings.TrimSpace(ci.SpecialType), "imagedownload") && text != "" {
			dataURI, err := downloadImageAsDataURI(text)
			if err != nil {
				continue
			}
			xVal = dataURI
		}

		imgToken := ""
		if imgPath := strings.TrimSpace(ci.ImageFromPath); imgPath != "" && len(rawBody) > 0 {
			imgURL := strings.TrimSpace(gjson.GetBytes(rawBody, imgPath).String())
			if imgURL != "" {
				cacheDir := strings.TrimSpace(ci.ImageCacheDir)
				if cacheDir == "" {
					cacheDir = "gs"
				}
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				localURL, imgErr := gamestatsimage.DownloadIfNeeded(ctx, appclient.Shared, cacheDir, imgURL, gamestatsimage.DefaultMaxAgeDays)
				cancel()
				if imgErr == nil {
					imgToken = localURL
				}
			}
		}

		displayAs = applyDisplayPlaceholders(displayAs, text, ci.DisplayPlaceholders)
		displayAs = strings.ReplaceAll(displayAs, "%x_fmt%", applyDisplayFormat(text, ci.DisplayFormat))
		displayAs = strings.ReplaceAll(displayAs, "%img%", imgToken)
		out[itemName] = strings.ReplaceAll(displayAs, "%x%", xVal)
	}
	gameStatsLog.Debug("collect stats from html", "platform", platformKey, "accountID", accountID, "requestedMetrics", len(def.Collect), "collectedMetrics", len(out), "missingMetrics", missing)
	return out, nil
}

func extractMetricFromHTML(doc *htmlnet.Node, ci collectInstruction) (string, bool) {
	xpathExpr := strings.TrimSpace(ci.XPath)
	if xp := strings.TrimSpace(ci.Path); xp != "" {
		xpathExpr = xp
	}
	if xpathExpr == "" {
		return "", false
	}
	nodes, err := htmlquery.QueryAll(doc, xpathExpr)
	if err != nil || len(nodes) == 0 {
		return "", false
	}
	node := nodes[0]
	text := ""
	switch strings.ToLower(strings.TrimSpace(ci.Select)) {
	case "innertext":
		text = strings.TrimSpace(htmlquery.InnerText(node))
	case "innerhtml":
		text = strings.TrimSpace(nodeInnerHTML(node))
	case "attribute":
		attr := strings.TrimSpace(ci.SelectAttribute)
		if attr == "" {
			return "", false
		}
		for _, a := range node.Attr {
			if strings.EqualFold(a.Key, attr) {
				text = strings.TrimSpace(a.Val)
				break
			}
		}
	default:
		return "", false
	}
	return text, true
}

func extractMetricFromJSON(raw []byte, ci collectInstruction) (string, bool) {
	paths := make([]string, 0, 1+len(ci.FallbackPaths))
	if p := strings.TrimSpace(ci.Path); p != "" {
		paths = append(paths, p)
	}
	for _, p := range ci.FallbackPaths {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}
	reducer := strings.ToLower(strings.TrimSpace(ci.Reducer))
	if reducer == "" {
		reducer = "firstnotnull"
	}

	var results []gjson.Result
	for _, p := range paths {
		r := gjson.GetBytes(raw, p)
		if r.Exists() {
			results = append(results, r)
		}
	}

	switch reducer {
	case "firstnotnull":
		for _, r := range results {
			if r.Type != gjson.Null {
				return jsonResultToString(r), true
			}
		}
		return "", false
	case "firstnotnullorzero":
		for _, r := range results {
			if r.Type == gjson.Null {
				continue
			}
			if resultIsZeroish(r) {
				continue
			}
			return jsonResultToString(r), true
		}
		return "", false
	case "maxnumber":
		best := math.Inf(-1)
		found := false
		for _, r := range results {
			for _, n := range flattenResultNumbers(r) {
				if n > best {
					best = n
					found = true
				}
			}
		}
		if !found {
			return "", false
		}
		if math.Mod(best, 1) == 0 {
			return strconv.FormatInt(int64(best), 10), true
		}
		return strconv.FormatFloat(best, 'f', -1, 64), true
	case "firstmatchinarray":
		// If primary/fallback paths were provided, try them first.
		for _, r := range results {
			if r.Type == gjson.Null {
				continue
			}
			if strings.TrimSpace(jsonResultToString(r)) == "" {
				continue
			}
			return jsonResultToString(r), true
		}
		arrayPath := reducerOptionString(ci.ReducerOptions, "arrayPath")
		valuePath := reducerOptionString(ci.ReducerOptions, "valuePath")
		requiredPath := reducerOptionString(ci.ReducerOptions, "requiredPath")
		matchPath := reducerOptionString(ci.ReducerOptions, "matchPath")
		matchEquals := reducerOptionString(ci.ReducerOptions, "matchEquals")
		valueGtRaw := reducerOptionString(ci.ReducerOptions, "valueGt")
		allMatch := reducerOptionAllMatch(ci.ReducerOptions)
		valueGt := math.Inf(-1)
		hasValueGt := false
		if valueGtRaw != "" {
			if n, err := strconv.ParseFloat(valueGtRaw, 64); err == nil {
				valueGt = n
				hasValueGt = true
			}
		}
		if arrayPath == "" || valuePath == "" {
			return "", false
		}
		arr := gjson.GetBytes(raw, arrayPath)
		if !arr.Exists() || !arr.IsArray() {
			return "", false
		}
		for _, item := range arr.Array() {
			if len(allMatch) > 0 && !matcherPass(item, allMatch) {
				continue
			}
			if matchPath != "" {
				mv := item.Get(matchPath)
				if !mv.Exists() || mv.Type == gjson.Null || strings.TrimSpace(mv.String()) != matchEquals {
					continue
				}
			}
			if requiredPath != "" {
				req := item.Get(requiredPath)
				if !req.Exists() || req.Type == gjson.Null || strings.TrimSpace(req.String()) == "" {
					continue
				}
			}
			val := item.Get(valuePath)
			if !val.Exists() || val.Type == gjson.Null {
				continue
			}
			if hasValueGt {
				vn, err := strconv.ParseFloat(strings.TrimSpace(jsonResultToString(val)), 64)
				if err != nil || !(vn > valueGt) {
					continue
				}
			}
			return jsonResultToString(val), true
		}
		return "", false
	default:
		return "", false
	}
}

func flattenResultNumbers(r gjson.Result) []float64 {
	if r.Type == gjson.Number {
		return []float64{r.Float()}
	}
	if r.IsArray() {
		var out []float64
		for _, it := range r.Array() {
			out = append(out, flattenResultNumbers(it)...)
		}
		return out
	}
	s := strings.TrimSpace(r.String())
	if s == "" {
		return nil
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return []float64{n}
	}
	// stringified JSON array support
	var arr []float64
	if err := json.Unmarshal([]byte(s), &arr); err == nil {
		return arr
	}
	return nil
}

func resultIsZeroish(r gjson.Result) bool {
	switch r.Type {
	case gjson.Number:
		return r.Float() == 0
	case gjson.String:
		s := strings.TrimSpace(r.String())
		return s == "" || s == "0" || s == "0.0"
	case gjson.False, gjson.Null:
		return true
	}
	return false
}

func jsonResultToString(r gjson.Result) string {
	if r.Type == gjson.JSON {
		if r.IsArray() || r.IsObject() {
			return strings.TrimSpace(r.Raw)
		}
	}
	return strings.TrimSpace(r.String())
}

// applySelectFunc supports chain expressions like:
// .split('?level=')[1].split('"')[0]
func applySelectFunc(input, expr string) string {
	cur := input
	steps, ok := parseSelectFuncSteps(expr)
	if !ok {
		return cur
	}
	for _, st := range steps {
		switch st.Name {
		case "split":
			if len(st.Args) != 1 {
				return cur
			}
			delim := unquoteArg(st.Args[0])
			parts := strings.Split(cur, delim)
			idx := 0
			if st.Index != nil {
				idx = *st.Index
			}
			if idx < 0 || idx >= len(parts) {
				cur = ""
			} else {
				cur = parts[idx]
			}
		case "replace":
			if len(st.Args) < 2 {
				return cur
			}
			cur = strings.ReplaceAll(cur, unquoteArg(st.Args[0]), unquoteArg(st.Args[1]))
		case "trim":
			cur = strings.TrimSpace(cur)
		case "tolower":
			cur = strings.ToLower(cur)
		case "toupper":
			cur = strings.ToUpper(cur)
		case "substring":
			if len(st.Args) != 1 {
				return cur
			}
			cur = substringArg(cur, strings.TrimSpace(st.Args[0]))
		default:
			return cur
		}
	}
	return cur
}

type selectFuncStep struct {
	Name  string
	Args  []string
	Index *int
}

type reducerMatchCond struct {
	Path  string
	Op    string
	Value string
}

func reducerOptionString(opts map[string]any, key string) string {
	if opts == nil {
		return ""
	}
	v, ok := opts[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return strings.TrimSpace(strconv.FormatFloat(x, 'f', -1, 64))
	case int:
		return strconv.Itoa(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

func reducerOptionAllMatch(opts map[string]any) []reducerMatchCond {
	if opts == nil {
		return nil
	}
	raw, ok := opts["allMatch"]
	if !ok || raw == nil {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	var out []reducerMatchCond
	for _, it := range items {
		obj, ok := it.(map[string]any)
		if !ok {
			continue
		}
		p := reducerOptionString(obj, "path")
		if p == "" {
			continue
		}
		op := strings.ToLower(reducerOptionString(obj, "op"))
		if op == "" {
			op = "eq"
		}
		out = append(out, reducerMatchCond{
			Path:  p,
			Op:    op,
			Value: reducerOptionString(obj, "value"),
		})
	}
	return out
}

func matcherPass(item gjson.Result, conds []reducerMatchCond) bool {
	for _, c := range conds {
		r := item.Get(c.Path)
		switch c.Op {
		case "eq":
			if !r.Exists() || strings.TrimSpace(r.String()) != c.Value {
				return false
			}
		case "neq":
			if r.Exists() && strings.TrimSpace(r.String()) == c.Value {
				return false
			}
		case "gt", "gte", "lt", "lte":
			n, err := strconv.ParseFloat(strings.TrimSpace(jsonResultToString(r)), 64)
			if err != nil {
				return false
			}
			cn, err := strconv.ParseFloat(strings.TrimSpace(c.Value), 64)
			if err != nil {
				return false
			}
			switch c.Op {
			case "gt":
				if !(n > cn) {
					return false
				}
			case "gte":
				if !(n >= cn) {
					return false
				}
			case "lt":
				if !(n < cn) {
					return false
				}
			case "lte":
				if !(n <= cn) {
					return false
				}
			}
		case "exists":
			if !r.Exists() || r.Type == gjson.Null {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func parseSelectFuncSteps(expr string) ([]selectFuncStep, bool) {
	var steps []selectFuncStep
	rest := strings.TrimSpace(expr)
	for rest != "" {
		if !strings.HasPrefix(rest, ".") {
			return nil, false
		}
		rest = strings.TrimSpace(rest[1:])
		name, args, after, ok := parseLeadingFuncCall(rest)
		if !ok || name == "" {
			return nil, false
		}
		step := selectFuncStep{
			Name: strings.ToLower(strings.TrimSpace(name)),
			Args: args,
		}
		rest = strings.TrimSpace(after)
		if strings.HasPrefix(rest, "[") {
			end := strings.Index(rest, "]")
			if end <= 1 {
				return nil, false
			}
			n, err := strconv.Atoi(strings.TrimSpace(rest[1:end]))
			if err != nil {
				return nil, false
			}
			step.Index = &n
			rest = strings.TrimSpace(rest[end+1:])
		}
		steps = append(steps, step)
	}
	return steps, true
}

func downloadImageAsDataURI(imageURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "TcNo-Acc-Switcher/1.0")
	resp, err := appclient.Shared.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("image HTTP %d", resp.StatusCode)
	}
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, io.LimitReader(resp.Body, gameStatsImageDownloadMaxBytes)); err != nil {
		return "", err
	}
	raw := buf.Bytes()
	if len(raw) == 0 {
		return "", fmt.Errorf("empty image body")
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(ct), "image/") {
		ct = "image/jpeg"
	}
	b64 := base64.StdEncoding.EncodeToString(raw)
	return fmt.Sprintf("data:%s;base64,%s", ct, b64), nil
}

func writeGameStatsDebugHTML(accountID, game string, htmlBytes []byte) {
	root, err := paths.DataRoot()
	if err != nil {
		return
	}
	safeAcc := paths.SanitizePathSegment(accountID)
	safeGame := paths.SanitizePathSegment(game)
	if safeAcc == "" {
		safeAcc = "account"
	}
	if safeGame == "" {
		safeGame = "game"
	}
	p := filepath.Join(root, "temp", fmt.Sprintf("download-%s-%s.html", safeAcc, safeGame))
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = fsutil.WriteFileAtomic(p, htmlBytes, 0o644)
	slog.Debug("game stats debug html written", "component", "game-stats", "game", game, "accountID", accountID, "path", p, "bytes", len(htmlBytes))
}
