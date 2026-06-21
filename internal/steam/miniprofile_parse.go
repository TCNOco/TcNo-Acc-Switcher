package steam

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

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

func classAttrPrefix(n *html.Node, prefix string) bool {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, "class") {
			for _, tok := range strings.Fields(a.Val) {
				if strings.HasPrefix(tok, prefix) {
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
	if strings.HasPrefix(lu, "/img/profiles/steam/") {
		return true
	}
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

func findFirstDivWithClass(root *html.Node, want string) *html.Node {
	var walk func(*html.Node) *html.Node
	walk = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "div") && classAttr(n, want) {
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

func elementTextContent(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var collect func(*html.Node)
	collect = func(cur *html.Node) {
		if cur.Type == html.TextNode {
			b.WriteString(cur.Data)
		}
		for c := cur.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	collect(n)
	return b.String()
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

var miniAllowedTags = map[string]bool{
	"div": true, "span": true, "img": true, "video": true, "source": true,
	"b": true, "i": true, "strong": true, "em": true, "p": true, "br": true,
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

func sanitizeNode(n *html.Node, out *bytes.Buffer) {
	switch n.Type {
	case html.TextNode:
		out.WriteString(html.EscapeString(n.Data))
		return
	case html.ElementNode:
		if classAttr(n, "miniprofile_gamesection") {
			return
		}
		if classAttrPrefix(n, "friend_status_") {
			return
		}
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

func extractAvatarFrameImgURL(page string) string {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return ""
	}
	return extractAvatarFrameImgURLFromDoc(doc)
}

func extractAvatarFrameImgURLFromDoc(doc *html.Node) string {
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
	return extractMiniprofileContainerHTMLFromDoc(doc)
}

func extractMiniprofileContainerHTMLFromDoc(doc *html.Node) string {
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
	return `<div class="miniprofile_container">` + buf.String() + `</div>`
}

func findMiniprofileAvatarMountPoint(root *html.Node) *html.Node {
	if n := findFirstDivWithClass(root, "playerAvatarAutoSizeInner"); n != nil {
		return n
	}
	if n := findFirstDivWithClass(root, "playersection_avatar"); n != nil {
		if classAttr(n, "playersection_avatar_frame") {
			return nil
		}
		return n
	}
	return nil
}
