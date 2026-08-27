package confirmationapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

// Steam describes an item on its economy hover endpoint: the same description the
// community site shows when an item is hovered in a trade. It is the fullest
// account of what is actually being traded that Steam exposes here.
const (
	itemClassBase       = "https://steamcommunity.com/economy/itemclasshover/"
	maxItemClassBytes   = 512 << 10
	maxItemDescriptions = 32
	maxItemTags         = 32
	maxItemTextBytes    = 512
)

// ItemDescription is one line of an item's description block. Name identifies the
// kind of line ("exterior_wear", "stattrak_score"), and Color is Steam's own
// emphasis for it.
type ItemDescription struct {
	Value string `json:"value"`
	Color string `json:"color,omitempty"`
	Name  string `json:"name,omitempty"`
}

// ItemTag is one categorised property: rarity, exterior, weapon, collection.
type ItemTag struct {
	Category     string `json:"category"`
	CategoryName string `json:"categoryName"`
	Name         string `json:"name"`
	Color        string `json:"color,omitempty"`
}

// ItemClass is everything worth showing about a traded item.
type ItemClass struct {
	Name           string            `json:"name"`
	MarketHashName string            `json:"marketHashName,omitempty"`
	Type           string            `json:"type,omitempty"`
	NameColor      string            `json:"nameColor,omitempty"`
	IconURL        string            `json:"iconUrl,omitempty"`
	Tradable       bool              `json:"tradable"`
	Marketable     bool              `json:"marketable"`
	Descriptions   []ItemDescription `json:"descriptions,omitempty"`
	Tags           []ItemTag         `json:"tags,omitempty"`
}

// itemClassWire mirrors the JSON Steam passes to BuildHover.
type itemClassWire struct {
	Name           string `json:"name"`
	MarketHashName string `json:"market_hash_name"`
	Type           string `json:"type"`
	NameColor      string `json:"name_color"`
	IconURL        string `json:"icon_url"`
	Tradable       int    `json:"tradable"`
	Marketable     int    `json:"marketable"`
	Descriptions   []struct {
		Value string `json:"value"`
		Color string `json:"color"`
		Name  string `json:"name"`
	} `json:"descriptions"`
	Tags []struct {
		Category     string `json:"category"`
		CategoryName string `json:"category_name"`
		Name         string `json:"name"`
		Color        string `json:"color"`
	} `json:"tags"`
}

// FetchItemClass loads the full description of one item. appID, classID and
// instanceID come from the trade's own markup, so this only ever asks about an
// item the confirmation already refers to.
func (c *Client) FetchItemClass(ctx context.Context, credentials Credentials, appID, classID, instanceID string) (ItemClass, error) {
	if c == nil || c.protocol == nil || ctx == nil || c.offline == nil {
		return ItemClass{}, &Error{Kind: FailureInvalid}
	}
	if c.offline() {
		return ItemClass{}, &Error{Kind: FailureOffline}
	}
	if err := validateCredentials(credentials); err != nil {
		return ItemClass{}, err
	}
	if !validEconomyID(appID) || !validEconomyID(classID) || !validEconomyID(instanceID) {
		return ItemClass{}, &Error{Kind: FailureInvalid}
	}
	endpoint := itemClassBase + url.PathEscape(appID) + "/" + url.PathEscape(classID) + "/" + url.PathEscape(instanceID)
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return ItemClass{}, &Error{Kind: FailureInvalid}
	}
	parsed.RawQuery = url.Values{"content_only": {"1"}, "l": {"english"}}.Encode()

	headers := make(http.Header)
	headers.Set("User-Agent", MobileUserAgent)
	headers.Set("Cookie", confirmationCookie(credentials))
	response, err := c.protocol.Do(ctx, protocol.Request{
		Method: http.MethodGet, Endpoint: parsed.String(), Route: protocol.RouteRequest,
		Header: headers, Timeout: RequestTimeout, MaxResponseBytes: maxItemClassBytes,
	})
	if err != nil {
		classified := classifyProtocolError(err)
		logTransportFailure("itemclass", classified)
		return ItemClass{}, classified
	}
	item, ok := decodeItemClass(response.Body)
	if !ok {
		return ItemClass{}, &Error{Kind: FailureFailed}
	}
	return item, nil
}

// decodeItemClass lifts the item JSON out of the BuildHover call the page ends
// with. The surrounding document is markup and script, so the payload is found by
// scanning rather than parsed as a whole.
func decodeItemClass(body []byte) (ItemClass, bool) {
	payload, ok := buildHoverPayload(string(body))
	if !ok {
		return ItemClass{}, false
	}
	var wire itemClassWire
	if err := json.Unmarshal([]byte(payload), &wire); err != nil {
		return ItemClass{}, false
	}
	return itemClassFromWire(wire), true
}

// buildHoverPayload returns the JSON object passed to BuildHover. It matches
// braces while honouring string escapes, because item descriptions contain both.
func buildHoverPayload(document string) (string, bool) {
	index := strings.Index(document, "BuildHover(")
	if index < 0 {
		return "", false
	}
	start := strings.Index(document[index:], "{")
	if start < 0 {
		return "", false
	}
	start += index
	depth := 0
	inString := false
	escaped := false
	for offset, r := range document[start:] {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && inString:
			escaped = true
		case r == '"':
			inString = !inString
		case inString:
			// Braces inside a string are text, not structure.
		case r == '{':
			depth++
		case r == '}':
			depth--
			if depth == 0 {
				return document[start : start+offset+1], true
			}
		}
	}
	return "", false
}

func itemClassFromWire(wire itemClassWire) ItemClass {
	item := ItemClass{
		Name:           clampText(wire.Name),
		MarketHashName: clampText(wire.MarketHashName),
		Type:           clampText(wire.Type),
		NameColor:      safeColor(wire.NameColor),
		IconURL:        itemImageURL(wire.IconURL),
		Tradable:       wire.Tradable != 0,
		Marketable:     wire.Marketable != 0,
	}
	for _, description := range wire.Descriptions {
		if len(item.Descriptions) >= maxItemDescriptions {
			break
		}
		// Steam pads its blocks with blank lines; they carry nothing to show.
		value := clampText(stripMarkup(description.Value))
		if value == "" {
			continue
		}
		item.Descriptions = append(item.Descriptions, ItemDescription{
			Value: value, Color: safeColor(description.Color), Name: clampText(description.Name),
		})
	}
	for _, tag := range wire.Tags {
		if len(item.Tags) >= maxItemTags {
			break
		}
		item.Tags = append(item.Tags, ItemTag{
			Category:     clampText(tag.Category),
			CategoryName: clampText(tag.CategoryName),
			Name:         clampText(tag.Name),
			Color:        safeColor(tag.Color),
		})
	}
	return item
}

// economyImageSize is the render Steam is asked for. Its trade pages embed 73px
// and 96px thumbnails and the window draws them into tiles larger than that, so
// they arrived upscaled and soft. Asking for the bigger render costs one query
// segment, and stays well inside the icon policy's 512px cap.
const economyImageSize = "256fx256f"

// itemImageURL turns Steam's bare icon hash into a URL on its economy CDN.
func itemImageURL(iconURL string) string {
	iconURL = strings.TrimSpace(iconURL)
	if iconURL == "" || len(iconURL) > 512 || strings.ContainsAny(iconURL, "\"'<>\\ ") {
		return ""
	}
	if strings.HasPrefix(iconURL, "https://") {
		return ResizeEconomyImage(iconURL)
	}
	return "https://community.fastly.steamstatic.com/economy/image/" + iconURL + "/" + economyImageSize
}

// ResizeEconomyImage asks Steam for a larger render of an economy image by
// rewriting the size segment its own markup embedded. Anything that is not one —
// an avatar, a years badge, a static asset — comes back untouched.
func ResizeEconomyImage(raw string) string {
	const marker = "/economy/image/"
	index := strings.Index(raw, marker)
	if index < 0 {
		return raw
	}
	cut := index + len(marker)
	slash := strings.LastIndex(raw[cut:], "/")
	if slash <= 0 || !economySizeSegment(raw[cut+slash+1:]) {
		return raw
	}
	return raw[:cut+slash+1] + economyImageSize
}

// economySizeSegment reports whether a path segment is one of Steam's render
// sizes — "96fx96f", "330x192" — so nothing else in a URL is mistaken for one
// and rewritten into a broken link.
func economySizeSegment(segment string) bool {
	if segment == "" || len(segment) > 16 {
		return false
	}
	digits := false
	for _, r := range segment {
		switch {
		case r >= '0' && r <= '9':
			digits = true
		case r == 'f' || r == 'x':
		default:
			return false
		}
	}
	return digits
}

// safeColor keeps Steam's hex colours and drops anything else, so a colour can be
// applied to text without carrying arbitrary content into a style.
func safeColor(value string) string {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 && len(value) != 3 {
		return ""
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return ""
		}
	}
	return "#" + value
}

// stripMarkup removes the light HTML Steam puts in descriptions, keeping the text.
func stripMarkup(value string) string {
	var builder strings.Builder
	depth := 0
	for _, r := range value {
		switch {
		case r == '<':
			depth++
		case r == '>':
			if depth > 0 {
				depth--
			}
		case depth == 0:
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func clampText(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > maxItemTextBytes {
		return value[:maxItemTextBytes]
	}
	return value
}
