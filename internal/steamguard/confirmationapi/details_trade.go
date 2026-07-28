package confirmationapi

import (
	"strconv"
	"strings"

	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Steam's confirmation detail page describes a trade far more fully than the list
// does: it names the other party and lays out both sides of the exchange as items.
// The generic text parser reduces all of that to sentences, so this reads the
// structure instead.

const (
	maxTradeItems     = 128
	steamID64Base     = uint64(76561197960265728)
	steamProfilesBase = "https://steamcommunity.com/profiles/"
)

// TradeParty is the account on the other side of a trade offer.
type TradeParty struct {
	Name string `json:"name"`
	// AvatarURL and ProfileURL are Steam URLs. The caller decides whether to
	// fetch the avatar; nothing here loads it.
	AvatarURL  string `json:"avatarUrl,omitempty"`
	ProfileURL string `json:"profileUrl,omitempty"`
	// Level is Steam's profile level, 0 when the page does not show one.
	Level int `json:"level,omitempty"`
	// YearsBadgeURL is the "years of service" badge, a rough account age signal.
	YearsBadgeURL string `json:"yearsBadgeUrl,omitempty"`
}

// TradeItem identifies one item well enough to show it and to ask Steam for its
// full description later. The triple is what an item-class lookup needs.
type TradeItem struct {
	AppID      string `json:"appId"`
	ClassID    string `json:"classId"`
	InstanceID string `json:"instanceId"`
	ImageURL   string `json:"imageUrl,omitempty"`
}

// TradeSide is one half of the exchange, with the wording Steam used for it.
type TradeSide struct {
	Header string      `json:"header"`
	Items  []TradeItem `json:"items"`
}

// TradeDetails is the structured form of a trade-offer confirmation.
type TradeDetails struct {
	Partner *TradeParty `json:"partner,omitempty"`
	Give    TradeSide   `json:"give"`
	Receive TradeSide   `json:"receive"`
}

// decodeTrade reads the trade structure out of a detail page. It returns nil when
// the page is not a trade offer — a market listing, for one — so callers fall back
// to the text fields.
func decodeTrade(html string) *TradeDetails {
	contextNode := &nethtml.Node{Type: nethtml.ElementNode, DataAtom: atom.Body, Data: "body"}
	nodes, err := nethtml.ParseFragment(strings.NewReader(html), contextNode)
	if err != nil {
		return nil
	}
	details := &TradeDetails{}
	for _, node := range nodes {
		walkTrade(node, details)
	}
	if len(details.Give.Items) == 0 && len(details.Receive.Items) == 0 && details.Partner == nil {
		return nil
	}
	return details
}

func walkTrade(node *nethtml.Node, details *TradeDetails) {
	if node == nil || excludedNode(node) {
		return
	}
	if node.Type == nethtml.ElementNode {
		switch {
		case classAttr(node, "mobileconf_trade_partner_info"):
			details.Partner = parseTradeParty(node)
			return
		case classAttr(node, "tradeoffer_items"):
			side := parseTradeSide(node)
			// Steam marks the offering side primary and the receiving side
			// secondary; the headers say which is which in words.
			if classAttr(node, "secondary") {
				details.Receive = side
			} else {
				details.Give = side
			}
			return
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkTrade(child, details)
	}
}

func parseTradeParty(node *nethtml.Node) *TradeParty {
	party := &TradeParty{}
	var walk func(*nethtml.Node)
	walk = func(current *nethtml.Node) {
		if current == nil {
			return
		}
		if current.Type == nethtml.ElementNode {
			switch {
			case current.DataAtom == atom.Img && classAttr(current, "partner_avatar"):
				party.AvatarURL = safeSteamURL(attrValue(current, "src"))
			case current.DataAtom == atom.Img && party.YearsBadgeURL == "" && strings.Contains(attrValue(current, "src"), "/badges/"):
				party.YearsBadgeURL = safeSteamURL(attrValue(current, "src"))
			case classAttr(current, "friendPlayerLevelNum"):
				if level, err := strconv.Atoi(strings.TrimSpace(nodeText(current))); err == nil && level >= 0 && level < 10000 {
					party.Level = level
				}
			case current.DataAtom == atom.Strong && party.Name == "":
				if name := strings.TrimSpace(nodeText(current)); name != "" && !strings.EqualFold(name, "You offered") {
					party.Name = name
				}
			}
			// The account id is Steam's 32-bit form; the 64-bit id is what a
			// profile URL needs.
			if party.ProfileURL == "" {
				if raw := attrValue(current, "data-miniprofile"); raw != "" {
					if accountID, err := strconv.ParseUint(raw, 10, 32); err == nil && accountID != 0 {
						party.ProfileURL = steamProfilesBase + strconv.FormatUint(steamID64Base+accountID, 10)
					}
				}
			}
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	if party.Name == "" && party.AvatarURL == "" && party.ProfileURL == "" {
		return nil
	}
	return party
}

func parseTradeSide(node *nethtml.Node) TradeSide {
	side := TradeSide{Items: make([]TradeItem, 0, 8)}
	var walk func(*nethtml.Node)
	walk = func(current *nethtml.Node) {
		if current == nil || len(side.Items) > maxTradeItems {
			return
		}
		if current.Type == nethtml.ElementNode {
			if classAttr(current, "tradeoffer_items_header") && side.Header == "" {
				side.Header = strings.TrimSpace(nodeText(current))
			}
			if classAttr(current, "trade_item") {
				if item, ok := parseTradeItem(current); ok {
					side.Items = append(side.Items, item)
				}
				return
			}
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return side
}

// parseTradeItem reads the appid/classid/instanceid triple Steam puts on each
// tile, which is what identifies the item everywhere else.
func parseTradeItem(node *nethtml.Node) (TradeItem, bool) {
	reference := attrValue(node, "data-economy-item")
	parts := strings.Split(strings.TrimPrefix(reference, "classinfo/"), "/")
	if len(parts) != 3 {
		return TradeItem{}, false
	}
	item := TradeItem{AppID: parts[0], ClassID: parts[1], InstanceID: parts[2]}
	if !validEconomyID(item.AppID) || !validEconomyID(item.ClassID) || !validEconomyID(item.InstanceID) {
		return TradeItem{}, false
	}
	var findImage func(*nethtml.Node)
	findImage = func(current *nethtml.Node) {
		if current == nil || item.ImageURL != "" {
			return
		}
		if current.Type == nethtml.ElementNode && current.DataAtom == atom.Img {
			// Steam's trade page embeds a 73px or 96px thumbnail; the tiles are
			// bigger than that, so ask for a render that does not need upscaling.
			item.ImageURL = ResizeEconomyImage(safeSteamURL(attrValue(current, "src")))
			return
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			findImage(child)
		}
	}
	findImage(node)
	return item, true
}

func validEconomyID(value string) bool {
	if value == "" || len(value) > 24 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// safeSteamURL keeps only absolute https URLs, the same bar the icon field is
// held to. Anything else becomes empty rather than reaching a caller.
func safeSteamURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "https://") || len(raw) > 512 {
		return ""
	}
	return raw
}

func attrValue(node *nethtml.Node, name string) string {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, name) {
			return attribute.Val
		}
	}
	return ""
}

func classAttr(node *nethtml.Node, class string) bool {
	for _, field := range strings.Fields(attrValue(node, "class")) {
		if field == class {
			return true
		}
	}
	return false
}

func nodeText(node *nethtml.Node) string {
	var builder strings.Builder
	var walk func(*nethtml.Node)
	walk = func(current *nethtml.Node) {
		if current == nil || builder.Len() > maxDetailsText {
			return
		}
		if current.Type == nethtml.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.Join(strings.Fields(builder.String()), " ")
}
