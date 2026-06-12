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
	"strconv"
	"strings"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/profileimage"

	"golang.org/x/net/html"
)

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
	if st, statErr := os.Stat(cachePath); statErr == nil && !st.IsDir() && !profileimage.FileOlderThanDays(cachePath, maxAgeDays) {
		data, rerr := os.ReadFile(cachePath)
		if rerr == nil && len(data) > 0 {
			page = sanitizeMiniprofileHTML(string(data))
			if strings.TrimSpace(page) != "" {
				fromDisk = true
			}
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
		page = sanitizedHTML
	} else {
		sanitizedHTML = page
	}

	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return "", strings.TrimSpace(sanitizedHTML), nil
	}
	frameImgURL = extractAvatarFrameImgURLFromDoc(doc)
	return frameImgURL, strings.TrimSpace(sanitizedHTML), nil
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
			res, derr := profileimage.DownloadIfNeeded(ctx, client, PlatformKey, steamID64+"_featuredbadge", u, maxAgeDays)
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

func ExtractMiniprofileDisplayName(fragment string) string {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return ""
	}
	body, err := parseFragmentInBody(fragment)
	if err != nil || body == nil {
		return ""
	}
	var walk func(*html.Node) string
	walk = func(n *html.Node) string {
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "span") && classAttrPrefix(n, "persona") {
			if t := strings.TrimSpace(elementTextContent(n)); t != "" {
				return t
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if hit := walk(c); hit != "" {
				return hit
			}
		}
		return ""
	}
	return walk(body)
}

func ExtractMiniprofileAvatarMediaURL(fragment string) string {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return ""
	}
	body, err := parseFragmentInBody(fragment)
	if err != nil || body == nil {
		return ""
	}
	mount := findMiniprofileAvatarMountPoint(body)
	if mount == nil {
		return ""
	}
	if v := firstDescendantElement(mount, "video"); v != nil {
		if u, _ := pickNameplateSource(v); u != "" {
			return u
		}
		if u := attrVal(v, "src"); isSafeSteamAssetURL(u) {
			return u
		}
	}
	if img := firstDescendantElement(mount, "img"); img != nil {
		if u := attrVal(img, "src"); isSafeSteamAssetURL(u) {
			return u
		}
	}
	return ""
}

func ReplaceMiniprofileMainAvatar(fragment, publicAvatarURL string) string {
	publicAvatarURL = strings.TrimSpace(publicAvatarURL)
	fragment = strings.TrimSpace(fragment)
	if fragment == "" || publicAvatarURL == "" {
		return fragment
	}
	if !isSafeSteamAssetURL(publicAvatarURL) {
		return fragment
	}

	body, err := parseFragmentInBody(fragment)
	if err != nil || body == nil {
		return fragment
	}

	mount := findMiniprofileAvatarMountPoint(body)
	if mount == nil {
		return fragment
	}

	htmlRemoveAllChildren(mount)
	htmlAppendChild(mount, &html.Node{
		Type: html.ElementNode,
		Data: "img",
		Attr: []html.Attribute{
			{Key: "class", Val: "avatar"},
			{Key: "src", Val: publicAvatarURL},
			{Key: "alt", Val: ""},
		},
	})

	var buf bytes.Buffer
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&buf, c); err != nil {
			return fragment
		}
	}
	return sanitizeMiniprofileHTML(strings.TrimSpace(buf.String()))
}

func miniprofileAvatarURLWithModTimeBust(publicURL, steamID64 string) string {
	publicURL = strings.TrimSpace(publicURL)
	steamID64 = strings.TrimSpace(steamID64)
	if publicURL == "" || steamID64 == "" {
		return publicURL
	}
	p, ok := profileimage.CachedFilePath(PlatformKey, steamID64)
	if !ok {
		return publicURL
	}
	st, err := os.Stat(p)
	if err != nil {
		return publicURL
	}
	ms := st.ModTime().UnixMilli()
	if ms <= 0 {
		return publicURL
	}
	sep := "?"
	if strings.Contains(publicURL, "?") {
		sep = "&"
	}
	return publicURL + sep + "_tcv=" + strconv.FormatInt(ms, 10)
}

func ApplySteamManualAvatarMiniprofile(fragment, steamID64 string) string {
	steamID64 = strings.TrimSpace(steamID64)
	if steamID64 == "" || strings.TrimSpace(fragment) == "" {
		return fragment
	}
	if !profileimage.HasManualProfileMarker(PlatformKey, steamID64) {
		return fragment
	}
	u, ok := profileimage.FindCached(PlatformKey, steamID64)
	if !ok {
		return fragment
	}
	u = miniprofileAvatarURLWithModTimeBust(u, steamID64)
	return ReplaceMiniprofileMainAvatar(fragment, u)
}
