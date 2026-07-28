package confirmationapi

import "testing"

// Shaped after a real trade-offer detail page: the partner block, then the two
// item lists Steam marks primary and secondary.
const tradeDetailHTML = `<div class="mobileconf_trade_area">
	<div class="mobileconf_trade_partner_info">
		<div style="position:relative">
			<img src="https://avatars.fastly.steamstatic.com/b9acd_full.jpg" class="partner_avatar" data-miniprofile="136440806">
			<div class="mobileconf_offer_friend"></div>
		</div>
		<div>
			<div><strong>You offered</strong> a trade to</div>
			<div data-miniprofile="136440806">
				<span><strong>UvuOsas</strong></span>
				<span class="mobileconf_offer_partnerlevel">
					<div class="friendPlayerLevel lvl_20"><span class="friendPlayerLevelNum">21</span></div>
				</span>
				<span class="mobileconf_offer_partneryears">
					<img src="https://community.fastly.steamstatic.com/public/images/badges/02_years/steamyears13_54.png" />
				</span>
			</div>
		</div>
	</div>
	<div class="tradeoffer" id="tradeofferid_9259768837">
		<div class="tradeoffer_items_ctn active">
			<div class="tradeoffer_items primary">
				<div class="tradeoffer_items_header">You will give up 2 items</div>
				<div class="tradeoffer_item_list">
					<div class="trade_item " data-economy-item="classinfo/730/1989288098/302028390"><img src="https://community.fastly.steamstatic.com/economy/image/aaa/96fx96f"></div>
					<div class="trade_item " data-economy-item="classinfo/730/1989278753/302028390"><img src="https://community.fastly.steamstatic.com/economy/image/bbb/96fx96f"></div>
				</div>
			</div>
			<div class="tradeoffer_items secondary">
				<div class="tradeoffer_items_header">You will receive 1 item</div>
				<div class="tradeoffer_item_list">
					<div class="trade_item " data-economy-item="classinfo/730/1989320848/302028390"><img src="https://community.fastly.steamstatic.com/economy/image/ccc/96fx96f"></div>
				</div>
			</div>
		</div>
	</div>
</div>`

func TestDecodeTradeReadsBothSidesAndThePartner(t *testing.T) {
	t.Parallel()

	trade := decodeTrade(tradeDetailHTML)
	if trade == nil {
		t.Fatal("trade detail page did not decode")
	}
	if trade.Partner == nil {
		t.Fatal("partner missing")
	}
	if trade.Partner.Name != "UvuOsas" || trade.Partner.Level != 21 {
		t.Fatalf("partner = %#v", trade.Partner)
	}
	// The page carries Steam's 32-bit account id; a profile link needs the 64-bit form.
	if trade.Partner.ProfileURL != "https://steamcommunity.com/profiles/76561198096706534" {
		t.Fatalf("profile = %q", trade.Partner.ProfileURL)
	}
	if trade.Partner.AvatarURL == "" || trade.Partner.YearsBadgeURL == "" {
		t.Fatalf("partner images = %#v", trade.Partner)
	}
	if trade.Give.Header != "You will give up 2 items" || len(trade.Give.Items) != 2 {
		t.Fatalf("give = %#v", trade.Give)
	}
	if trade.Receive.Header != "You will receive 1 item" || len(trade.Receive.Items) != 1 {
		t.Fatalf("receive = %#v", trade.Receive)
	}
	first := trade.Give.Items[0]
	if first.AppID != "730" || first.ClassID != "1989288098" || first.InstanceID != "302028390" || first.ImageURL == "" {
		t.Fatalf("item = %#v", first)
	}
}

// A market listing has no trade structure, and must not be reported as one.
func TestDecodeTradeIgnoresAPageWithoutATrade(t *testing.T) {
	t.Parallel()

	if trade := decodeTrade(`<div class="mobileconf_listing_prices">You want to sell...</div>`); trade != nil {
		t.Fatalf("trade = %#v", trade)
	}
}

func TestDecodeTradeRejectsAnUnusableItemReference(t *testing.T) {
	t.Parallel()

	trade := decodeTrade(`<div class="tradeoffer_items primary">
		<div class="tradeoffer_items_header">You will give up 1 item</div>
		<div class="trade_item " data-economy-item="classinfo/730/not-a-number/302028390"><img src="https://community.fastly.steamstatic.com/economy/image/aaa/96fx96f"></div>
	</div>`)
	if trade != nil && len(trade.Give.Items) != 0 {
		t.Fatalf("items = %#v", trade.Give.Items)
	}
}
