package steam

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/profileimage"

	"golang.org/x/net/html"
)

const miniprofileCacheTTL = 24 * time.Hour

func miniprofileCachePath(steamID64 string) (string, error) {
	r, err := paths.LoginCacheDir("Steam")
	if err != nil {
		return "", err
	}
	return filepath.Join(r, "MiniProfileCache", steamID64+".html"), nil
}

// ReadCachedMiniprofileHTML returns on-disk sanitized miniprofile HTML if the file exists (ignores TTL).
func ReadCachedMiniprofileHTML(steamID64 string) string {
	p, err := miniprofileCachePath(steamID64)
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(p)
	if err != nil || len(data) == 0 {
		return ""
	}
	return string(data)
}

func deleteMiniprofileCache(steamID64 string) {
	p, err := miniprofileCachePath(steamID64)
	if err != nil {
		return
	}
	_ = os.Remove(p)
}

func ClearAllMiniprofileHTMLCache() error {
	r, err := paths.LoginCacheDir("Steam")
	if err != nil {
		return err
	}
	dir := filepath.Join(r, "MiniProfileCache")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
	return nil
}

func FetchMiniprofile(ctx context.Context, client *http.Client, steamID64 string, maxAgeDays int) (frameImgURL string, sanitizedHTML string, err error) {
	steamID64 = strings.TrimSpace(steamID64)
	if steamID64 == "" {
		return "", "", fmt.Errorf("empty steam id")
	}
	formats, ferr := FormatsFromID64(steamID64)
	if ferr != nil {
		return "", "", fmt.Errorf("steam id: %w", ferr)
	}
	id32 := strings.TrimSpace(formats.ID32)
	if id32 == "" {
		return "", "", fmt.Errorf("could not derive steam id32")
	}

	cachePath, err := miniprofileCachePath(steamID64)
	if err != nil {
		return "", "", err
	}

	var page string
	fromDisk := false
	if st, statErr := os.Stat(cachePath); statErr == nil && !st.IsDir() && time.Since(st.ModTime()) < miniprofileCacheTTL {
		data, rerr := os.ReadFile(cachePath)
		if rerr == nil && len(data) > 0 {
			page = string(data)
			fromDisk = true
		}
	}
	url := fmt.Sprintf("https://steamcommunity.com/miniprofile/%s", id32)
	if !fromDisk {
		req, rerr := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if rerr != nil {
			return "", "", rerr
		}
		req.Header.Set("User-Agent", "TcNo Account Switcher")
		resp, derr := client.Do(req)
		if derr != nil {
			return "", "", derr
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", "", fmt.Errorf("miniprofile HTTP %d", resp.StatusCode)
		}
		body, rerr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		if rerr != nil {
			return "", "", rerr
		}
		page = string(body)
		container := extractMiniprofileContainerHTML(page)
		sanitizedHTML = sanitizeMiniprofileHTML(container)
		if strings.TrimSpace(container) != "" && strings.TrimSpace(sanitizedHTML) == "" {
			steamLog.Warn("miniprofile sanitize produced empty output",
				slog.String("steamId", tailSteamID(steamID64)))
		}
		if strings.TrimSpace(sanitizedHTML) != "" {
			rewritten, embErr := embedMiniprofileHTMLAssets(ctx, client, steamID64, sanitizedHTML, maxAgeDays)
			if embErr != nil {
				steamLog.Warn("miniprofile asset embed failed",
					slog.String("steamId", tailSteamID(steamID64)),
					slog.Any("err", embErr))
			} else if strings.TrimSpace(rewritten) != "" {
				sanitizedHTML = rewritten
			}
			_ = os.MkdirAll(filepath.Dir(cachePath), 0o755)
			_ = fsutil.WriteFileAtomic(cachePath, []byte(sanitizedHTML), 0o644)
		}
		page = sanitizedHTML // frame extract from same snapshot we cached
	} else {
		sanitizedHTML = page
	}

	frameImgURL = extractAvatarFrameImgURL(page)
	return frameImgURL, strings.TrimSpace(sanitizedHTML), nil
}

func extractAvatarFrameImgURL(page string) string {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return ""
	}
	var walk func(*html.Node) string
	walk = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "div" {
			if classAttr(n, "playersection_avatar_frame") {
				if img := firstDescendantElement(n, "img"); img != nil {
					if u := attrVal(img, "src"); u != "" && isSafeSteamAssetURL(u) {
						return u
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if u := walk(c); u != "" {
				return u
			}
		}
		return ""
	}
	return walk(doc)
}

func extractMiniprofileContainerHTML(page string) string {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return ""
	}
	var walk func(*html.Node) *html.Node
	walk = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode && n.Data == "div" && classAttr(n, "miniprofile_container") {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if hit := walk(c); hit != nil {
				return hit
			}
		}
		return nil
	}
	node := walk(doc)
	if node == nil {
		return ""
	}
	var buf bytes.Buffer
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&buf, c); err != nil {
			return ""
		}
	}
	// Re-wrap as single root for fragment sanitize
	return `<div class="miniprofile_container">` + buf.String() + `</div>`
}

func classAttr(n *html.Node, want string) bool {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, "class") {
			for _, tok := range strings.Fields(a.Val) {
				if tok == want {
					return true
				}
			}
		}
	}
	return false
}

func firstChildElement(n *html.Node, tag string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && strings.EqualFold(c.Data, tag) {
			return c
		}
	}
	return nil
}

func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}

func isSafeSteamAssetURL(u string) bool {
	u = strings.TrimSpace(u)
	if u == "" {
		return false
	}
	lu := strings.ToLower(u)
	if !strings.HasPrefix(lu, "https://") {
		return false
	}
	return strings.Contains(lu, "steamstatic.com") ||
		strings.Contains(lu, "steamcommunity.com") ||
		strings.Contains(lu, "steamusercontent.com") ||
		strings.Contains(lu, "akamaihd.net")
}

func parseFragmentInBody(fragment string) (*html.Node, error) {
	wrapped := "<!DOCTYPE html><html><head></head><body>" + fragment + "</body></html>"
	doc, err := html.Parse(strings.NewReader(wrapped))
	if err != nil {
		return nil, err
	}
	body := findFirstElement(doc, "body")
	if body == nil {
		return nil, fmt.Errorf("parse fragment: no body")
	}
	return body, nil
}

func findFirstElement(root *html.Node, tag string) *html.Node {
	var walk func(*html.Node) *html.Node
	walk = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, tag) {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if hit := walk(c); hit != nil {
				return hit
			}
		}
		return nil
	}
	return walk(root)
}

func sanitizeMiniprofileHTML(fragment string) string {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return ""
	}
	body, err := parseFragmentInBody(fragment)
	if err != nil || body == nil {
		return ""
	}
	var out bytes.Buffer
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		sanitizeNode(c, &out)
	}
	return strings.TrimSpace(out.String())
}

var miniAllowedTags = map[string]bool{
	"div": true, "span": true, "img": true, "video": true, "source": true,
	"b": true, "i": true, "strong": true, "em": true, "p": true, "br": true,
}

func sanitizeNode(n *html.Node, out *bytes.Buffer) {
	switch n.Type {
	case html.TextNode:
		out.WriteString(html.EscapeString(n.Data))
		return
	case html.ElementNode:
		if !miniAllowedTags[strings.ToLower(n.Data)] {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				sanitizeNode(c, out)
			}
			return
		}
		tag := strings.ToLower(n.Data)
		out.WriteString("<")
		out.WriteString(tag)
		for _, a := range filterMiniAttrs(tag, n.Attr) {
			out.WriteString(" ")
			out.WriteString(a.Key)
			out.WriteString(`="`)
			out.WriteString(html.EscapeString(a.Val))
			out.WriteString(`"`)
		}
		if tag == "br" || tag == "img" || tag == "source" {
			out.WriteString("/>")
			return
		}
		out.WriteString(">")
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			sanitizeNode(c, out)
		}
		out.WriteString("</")
		out.WriteString(tag)
		out.WriteString(">")
	default:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			sanitizeNode(c, out)
		}
	}
}

func filterMiniAttrs(tag string, attrs []html.Attribute) []html.Attribute {
	var out []html.Attribute
	for _, a := range attrs {
		k := strings.ToLower(a.Key)
		if strings.HasPrefix(k, "on") {
			continue
		}
		switch tag {
		case "div", "span":
			if k == "class" {
				out = append(out, html.Attribute{Key: k, Val: a.Val})
			}
		case "img":
			switch k {
			case "class", "alt":
				out = append(out, html.Attribute{Key: k, Val: a.Val})
			case "src":
				if isSafeSteamAssetURL(a.Val) {
					out = append(out, html.Attribute{Key: "src", Val: a.Val})
				}
			case "srcset":
				if s := sanitizeSrcset(a.Val); s != "" {
					out = append(out, html.Attribute{Key: "srcset", Val: s})
				}
			}
		case "video":
			switch k {
			case "class":
				out = append(out, html.Attribute{Key: k, Val: a.Val})
			case "playsinline", "autoplay", "muted", "loop":
				out = append(out, html.Attribute{Key: k, Val: a.Val})
			}
		case "source":
			switch k {
			case "src":
				if isSafeSteamAssetURL(a.Val) {
					out = append(out, html.Attribute{Key: "src", Val: a.Val})
				}
			case "type":
				out = append(out, html.Attribute{Key: "type", Val: a.Val})
			}
		}
	}
	return out
}

func htmlRemoveAllChildren(n *html.Node) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		c.Parent = nil
		c.PrevSibling = nil
		c.NextSibling = nil
		c = next
	}
	n.FirstChild = nil
	n.LastChild = nil
}

func htmlAppendChild(parent, child *html.Node) {
	child.Parent = parent
	child.PrevSibling = nil
	child.NextSibling = nil
	if parent.FirstChild == nil {
		parent.FirstChild = child
		parent.LastChild = child
		return
	}
	parent.LastChild.NextSibling = child
	child.PrevSibling = parent.LastChild
	parent.LastChild = child
}

func setAttr(n *html.Node, key, val string) {
	var out []html.Attribute
	found := false
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			out = append(out, html.Attribute{Key: a.Key, Val: val})
			found = true
		} else {
			out = append(out, a)
		}
	}
	if !found {
		out = append(out, html.Attribute{Key: key, Val: val})
	}
	n.Attr = out
}

func removeAttr(n *html.Node, key string) {
	var out []html.Attribute
	for _, a := range n.Attr {
		if !strings.EqualFold(a.Key, key) {
			out = append(out, a)
		}
	}
	n.Attr = out
}

func firstDescendantElement(root *html.Node, tag string) *html.Node {
	var walk func(*html.Node) *html.Node
	walk = func(n *html.Node) *html.Node {
		if n != root && n.Type == html.ElementNode && strings.EqualFold(n.Data, tag) {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if hit := walk(c); hit != nil {
				return hit
			}
		}
		return nil
	}
	return walk(root)
}

// pickNameplateSource prefers a WebM source; otherwise the first <source> with a usable URL.
func pickNameplateSource(video *html.Node) (url, mime string) {
	var sources []*html.Node
	for c := video.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && strings.EqualFold(c.Data, "source") {
			sources = append(sources, c)
		}
	}
	for _, s := range sources {
		u := attrVal(s, "src")
		t := strings.ToLower(attrVal(s, "type"))
		lu := strings.ToLower(u)
		if u == "" || !isSafeSteamAssetURL(u) {
			continue
		}
		if strings.HasSuffix(lu, ".webm") || strings.Contains(t, "webm") {
			return u, attrVal(s, "type")
		}
	}
	if len(sources) > 0 {
		u := attrVal(sources[0], "src")
		if isSafeSteamAssetURL(u) {
			return u, attrVal(sources[0], "type")
		}
	}
	return "", ""
}

func findFirstFeaturedImg(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "div" && classAttr(n, "miniprofile_featuredcontainer") {
		if im := firstDescendantElement(n, "img"); im != nil {
			if u := attrVal(im, "src"); u != "" && isSafeSteamAssetURL(u) {
				return im
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if hit := findFirstFeaturedImg(c); hit != nil {
			return hit
		}
	}
	return nil
}

func inferVideoMIME(publicPath, hint string) string {
	h := strings.TrimSpace(strings.ToLower(hint))
	if strings.Contains(h, "webm") {
		return "video/webm"
	}
	if strings.Contains(h, "mp4") {
		return "video/mp4"
	}
	lp := strings.ToLower(publicPath)
	if strings.HasSuffix(lp, ".webm") {
		return "video/webm"
	}
	if strings.HasSuffix(lp, ".mp4") {
		return "video/mp4"
	}
	return "video/webm"
}

func embedMiniprofileHTMLAssets(ctx context.Context, client *http.Client, steamID64, fragment string, maxAgeDays int) (string, error) {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return "", nil
	}
	body, err := parseFragmentInBody(fragment)
	if err != nil {
		return "", err
	}

	var nameplate *html.Node
	var findNameplate func(*html.Node)
	findNameplate = func(n *html.Node) {
		if nameplate != nil {
			return
		}
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "video") && classAttr(n, "miniprofile_nameplate") {
			nameplate = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findNameplate(c)
		}
	}
	findNameplate(body)

	if nameplate != nil {
		srcURL, mimeHint := pickNameplateSource(nameplate)
		if srcURL != "" {
			res, derr := profileimage.DownloadIfNeeded(ctx, client, PlatformKey, steamID64+"_nameplate", srcURL, maxAgeDays)
			if derr == nil && res != nil {
				mt := inferVideoMIME(res.PublicURL, mimeHint)
				htmlRemoveAllChildren(nameplate)
				htmlAppendChild(nameplate, &html.Node{
					Type: html.ElementNode,
					Data: "source",
					Attr: []html.Attribute{
						{Key: "src", Val: res.PublicURL},
						{Key: "type", Val: mt},
					},
				})
			}
		}
	}

	if feat := findFirstFeaturedImg(body); feat != nil {
		u := attrVal(feat, "src")
		if u != "" {
			res, derr := profileimage.DownloadIfNeeded(ctx, client, PlatformKey, steamID64+"_featuredcontainer", u, maxAgeDays)
			if derr == nil && res != nil {
				setAttr(feat, "src", res.PublicURL)
				removeAttr(feat, "srcset")
			}
		}
	}

	var buf bytes.Buffer
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&buf, c); err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(buf.String()), nil
}

func sanitizeSrcset(val string) string {
	parts := strings.Split(val, ",")
	var kept []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		fields := strings.Fields(p)
		if len(fields) == 0 {
			continue
		}
		u := fields[0]
		if !isSafeSteamAssetURL(u) {
			continue
		}
		if len(fields) > 1 {
			kept = append(kept, u+" "+strings.Join(fields[1:], " "))
		} else {
			kept = append(kept, u)
		}
	}
	return strings.Join(kept, ", ")
}
