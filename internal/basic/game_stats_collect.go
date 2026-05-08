package basic

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	htmlnet "golang.org/x/net/html"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/profileimage"
)

const gameStatsImageDownloadMaxBytes = 2 << 20 // 2 MiB

func substituteGameStatsURL(tmpl string, vars map[string]string) string {
	out := tmpl
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
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
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
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

func collectStatsFromHTML(platformKey, accountID string, def gameDefinition, doc *htmlnet.Node) (map[string]string, error) {
	out := map[string]string{}
	missing := 0
	for itemName, ci := range def.Collect {
		itemName = strings.TrimSpace(itemName)
		xpathExpr := strings.TrimSpace(ci.XPath)
		if xpathExpr == "" {
			continue
		}
		nodes, err := htmlquery.QueryAll(doc, xpathExpr)
		if err != nil || len(nodes) == 0 {
			missing++
			continue
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
			if attr != "" {
				for _, a := range node.Attr {
					if strings.EqualFold(a.Key, attr) {
						text = strings.TrimSpace(a.Val)
						break
					}
				}
			}
		default:
			continue
		}
		if sf := strings.TrimSpace(ci.SelectFunc); sf != "" {
			text = applySelectFunc(text, sf)
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

		out[itemName] = strings.ReplaceAll(displayAs, "%x%", xVal)
	}
	gameStatsLog.Debug("collect stats from html", "platform", platformKey, "accountID", accountID, "requestedMetrics", len(def.Collect), "collectedMetrics", len(out), "missingMetrics", missing)
	return out, nil
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
