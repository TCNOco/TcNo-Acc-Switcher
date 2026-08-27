package confirmationapi

import "testing"

// Steam embeds small thumbnails in its own pages and the window draws them
// larger, so the size segment is rewritten. Only a real size segment may be
// touched: rewriting anything else would turn a working URL into a 404.
func TestResizeEconomyImageOnlyRewritesASizeSegment(t *testing.T) {
	const hash = "IzMF03bi9WpSBq-S"
	cases := map[string]string{
		"https://community.fastly.steamstatic.com/economy/image/" + hash + "/73fx73f":   "https://community.fastly.steamstatic.com/economy/image/" + hash + "/" + economyImageSize,
		"https://community.fastly.steamstatic.com/economy/image/" + hash + "/96fx96f":   "https://community.fastly.steamstatic.com/economy/image/" + hash + "/" + economyImageSize,
		"https://community.fastly.steamstatic.com/economy/image/" + hash + "/330x192":   "https://community.fastly.steamstatic.com/economy/image/" + hash + "/" + economyImageSize,
		"https://community.fastly.steamstatic.com/economy/image/" + hash:                "https://community.fastly.steamstatic.com/economy/image/" + hash,
		"https://avatars.fastly.steamstatic.com/b9acd83b_full.jpg":                      "https://avatars.fastly.steamstatic.com/b9acd83b_full.jpg",
		"https://community.fastly.steamstatic.com/public/images/badges/02_years/13.png": "https://community.fastly.steamstatic.com/public/images/badges/02_years/13.png",
	}
	for raw, want := range cases {
		if got := ResizeEconomyImage(raw); got != want {
			t.Fatalf("ResizeEconomyImage(%q) = %q, want %q", raw, got, want)
		}
	}
}

// The shape Steam's itemclasshover endpoint returns: markup, then a script that
// hands the item to BuildHover. Taken from a real CS2 response.
const itemHoverResponse = `	<div class="clienthoverpage_content">
		<div class="inventory_iteminfo" id="economy_item_6a66085625e52"></div>
	</div>
	<script type="text/javascript">
		var g_rgAppContextData = {"730":{"appid":730,"name":"Counter-Strike 2"}};
		fnInitItemDisplay = function() {
			BuildHover( 'economy_item_6a66085625e52',  {"appid":"730","classid":"7993039571","instanceid":"902684101","icon_url":"i0CoZ81Ui0m","name":"StatTrak\u2122 MAG-7 | SWAG-7","market_hash_name":"StatTrak\u2122 MAG-7 | SWAG-7 (Minimal Wear)","name_color":"8847ff","type":"StatTrak\u2122 Restricted Shotgun","tradable":1,"marketable":1,"descriptions":[{"type":"html","value":"Exterior: Minimal Wear","name":"exterior_wear"},{"type":"html","value":" ","name":"blank"},{"type":"html","value":"StatTrak\u2122 Confirmed Kills: 4","color":"CF6A32","name":"stattrak_score"},{"type":"html","value":"Silver designs.\n\n<i>You either have it or you don't<\/i>","name":"description"}],"tags":[{"internal_name":"CSGO_Type_Shotgun","name":"Shotgun","category":"Type","category_name":"Type"},{"internal_name":"Rarity_Mythical_Weapon","name":"Restricted","category":"Rarity","color":"8847ff","category_name":"Quality"},{"internal_name":"WearCategory1","name":"Minimal Wear","category":"Exterior","category_name":"Exterior"}]} );
			$('economy_item_6a66085625e52').show();
		}
	</script>`

func TestDecodeItemClassReadsTheHoverPayload(t *testing.T) {
	t.Parallel()

	item, ok := decodeItemClass([]byte(itemHoverResponse))
	if !ok {
		t.Fatal("hover payload did not decode")
	}
	if item.Name != "StatTrak™ MAG-7 | SWAG-7" || item.Type != "StatTrak™ Restricted Shotgun" {
		t.Fatalf("item = %#v", item)
	}
	if item.NameColor != "#8847ff" || !item.Tradable || !item.Marketable {
		t.Fatalf("item flags = %#v", item)
	}
	// A bare icon hash becomes a URL on Steam's economy CDN, at the render size
	// the window actually draws rather than the thumbnail Steam's own page uses.
	if item.IconURL != "https://community.fastly.steamstatic.com/economy/image/i0CoZ81Ui0m/"+economyImageSize {
		t.Fatalf("icon = %q", item.IconURL)
	}
	// Blank padding lines carry nothing and are dropped; markup is reduced to text.
	if len(item.Descriptions) != 3 {
		t.Fatalf("descriptions = %#v", item.Descriptions)
	}
	if item.Descriptions[0].Value != "Exterior: Minimal Wear" {
		t.Fatalf("first description = %#v", item.Descriptions[0])
	}
	if item.Descriptions[1].Color != "#CF6A32" || item.Descriptions[1].Name != "stattrak_score" {
		t.Fatalf("stattrak description = %#v", item.Descriptions[1])
	}
	if item.Descriptions[2].Value != "Silver designs.\n\nYou either have it or you don't" {
		t.Fatalf("markup was not stripped: %q", item.Descriptions[2].Value)
	}
	if len(item.Tags) != 3 || item.Tags[1].Name != "Restricted" || item.Tags[1].CategoryName != "Quality" {
		t.Fatalf("tags = %#v", item.Tags)
	}
}

// Braces appear inside item names and descriptions, so the payload cannot be found
// by matching to the first closing brace.
func TestBuildHoverPayloadHonoursBracesInsideStrings(t *testing.T) {
	t.Parallel()

	document := `BuildHover( 'id', {"name":"a } brace","descriptions":[{"value":"{nested}"}]} );`
	payload, ok := buildHoverPayload(document)
	if !ok || payload != `{"name":"a } brace","descriptions":[{"value":"{nested}"}]}` {
		t.Fatalf("payload = %q, ok = %v", payload, ok)
	}
}

func TestDecodeItemClassRejectsAPageWithoutAnItem(t *testing.T) {
	t.Parallel()

	if _, ok := decodeItemClass([]byte(`<div>no hover here</div>`)); ok {
		t.Fatal("a page without an item decoded")
	}
}

func TestSafeColorKeepsOnlyHex(t *testing.T) {
	t.Parallel()

	if safeColor("8847ff") != "#8847ff" || safeColor("#CF6A32") != "#CF6A32" {
		t.Fatal("hex colours were not kept")
	}
	for _, value := range []string{"", "red", "javascript:x", "88 47 ff", "8847fff"} {
		if got := safeColor(value); got != "" {
			t.Fatalf("safeColor(%q) = %q", value, got)
		}
	}
}

// Steam quotes numbers inconsistently in this payload — the two captured
// responses in this package already disagree over appid — and a quoted flag
// used to fail the unmarshal and drop the whole item description.
func TestDecodeItemClassAcceptsQuotedFlags(t *testing.T) {
	t.Parallel()

	document := `BuildHover( 'id', {"name":"Quoted","tradable":"1","marketable":"0"} );`
	item, ok := decodeItemClass([]byte(document))
	if !ok {
		t.Fatal("quoted flags did not decode")
	}
	if !item.Tradable {
		t.Error(`tradable "1" did not read as true`)
	}
	// "0" is truthy as a string; the flag has to follow the number it encodes.
	if item.Marketable {
		t.Error(`marketable "0" did not read as false`)
	}
}

func TestDecodeItemClassTreatsAbsentFlagsAsUnset(t *testing.T) {
	t.Parallel()

	item, ok := decodeItemClass([]byte(`BuildHover( 'id', {"name":"Bare"} );`))
	if !ok {
		t.Fatal("payload without flags did not decode")
	}
	if item.Tradable || item.Marketable {
		t.Fatalf("absent flags = %#v", item)
	}
}
