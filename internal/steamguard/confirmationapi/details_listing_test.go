package confirmationapi

import (
	"encoding/json"
	"testing"
)

// The markup Steam serves for a market listing, reduced to the parts this reads:
// a boilerplate lead-in, the price block, the BuildHover item, and a boilerplate
// security notice. Whitespace and the stray empty div are Steam's own.
const listingDetailsHTML = `<div>
	<div style="padding: 10px; color: #7A7A7A; text-align: center">To post this listing, confirm the item and price and select "Create Listing" below.</div>
	<div class="mobileconf_listing_prices">
		<div style="font-size: 20px;">
			You want to sell...		</div>
		<div style="color: #66C0F4; margin-top: 6px;">
			<div style="display: inline-block; min-width: 37.5%;">
				You receive<br>
				R 0.89							</div>
						<div style="display: inline-block">
				Buyer pays<br>
				R 1.23							</div>
		</div>
		<div style="margin-top: 6px; color: #CACACA">
			<div>
				17 for sale starting at R 1.23			</div>
			<div>
				Volume: 3 sold in the last 24 hours			</div>
		</div>
	</div>
</div>
<div style="text-align: center; margin: 0 10px">
	<script type="text/javascript">
		BuildHover( 'confiteminfo', {"appid":753,"classid":"2014197349","instanceid":"0","icon_url":"abc123","descriptions":[{"value":"Kitty Cat Loves Nice Treats"}],"tradable":1,"name":"Wanderer","type":"Kitty Cat: Jigsaw Puzzles Trading Card","market_hash_name":"500580-Wanderer","marketable":1}, UserYou );
	</script>
</div>
<div style="clear: both; padding: 10px; color: #7A7A7A; text-align: center">
	<span style="color: white; font-weight: bold;">Didn't create this listing?</span>
	Then your account or computer may have been compromised. If you don't recognize this activity, please cancel the listing immediately and change your Steam password.</div>`

func TestDecodeListingReadsPricesAndMarketWithoutTheBoilerplate(t *testing.T) {
	listing := decodeListing(listingDetailsHTML)
	if listing == nil {
		t.Fatal("listing = nil, want the parsed price block")
	}
	// Taken by position, so the window labels them from its own translations
	// rather than repeating Steam's English. The amounts stay verbatim.
	if listing.Receive != "R 0.89" || listing.BuyerPays != "R 1.23" {
		t.Fatalf("prices = %q / %q", listing.Receive, listing.BuyerPays)
	}
	if len(listing.Prices) != 0 {
		t.Fatalf("prices = %#v, want Steam's labels dropped once both were read", listing.Prices)
	}
	// Numbers, not sentences: "17 for sale starting at R 1.23" and "Volume: 3
	// sold in the last 24 hours" are Steam's wording, and none of it is shown.
	want := ListingMarket{ForSale: 17, ForSalePrice: "R 1.23", SoldRecently: 3}
	if listing.Market.ForSale != want.ForSale || listing.Market.ForSalePrice != want.ForSalePrice ||
		listing.Market.SoldRecently != want.SoldRecently {
		t.Fatalf("market = %#v, want %#v", listing.Market, want)
	}
	// "You want to sell..." is the block's heading and carries nothing the type
	// label does not already say; the two notices around the block are fixed text
	// on every listing. None of them may reach the window.
	if len(listing.Market.Text) != 0 {
		t.Fatalf("market text = %#v, want every line read as numbers", listing.Market.Text)
	}
	if listing.Item == nil {
		t.Fatal("item = nil, want the BuildHover description")
	}
	if listing.Item.Name != "Wanderer" || listing.Item.Type != "Kitty Cat: Jigsaw Puzzles Trading Card" {
		t.Fatalf("item = %#v", listing.Item)
	}
	if len(listing.Item.Descriptions) != 1 || listing.Item.Descriptions[0].Value != "Kitty Cat Loves Nice Treats" {
		t.Fatalf("item descriptions = %#v", listing.Item.Descriptions)
	}
}

// Only English is recognised, because Steam answers the detail page in the
// account's own language. A line that does not parse is kept verbatim so the
// user still reads Steam's own words, rather than being half-understood into a
// number the window would then word wrongly.
func TestListingMarketKeepsWordingItCannotReadAsNumbers(t *testing.T) {
	var market ListingMarket
	market.read("Volumen: 3 verkauft")
	market.read("3 vendus au cours des 24 dernières heures")
	if market.ForSale != 0 || market.SoldRecently != 0 {
		t.Fatalf("market = %#v, want no numbers guessed from another language", market)
	}
	want := []string{"Volumen: 3 verkauft", "3 vendus au cours des 24 dernières heures"}
	if len(market.Text) != len(want) {
		t.Fatalf("text = %#v, want %#v", market.Text, want)
	}
	for index, line := range want {
		if market.Text[index] != line {
			t.Fatalf("text %d = %q, want %q", index, market.Text[index], line)
		}
	}
}

// A popular item runs into four figures, and Steam groups those.
func TestListingMarketReadsAGroupedCount(t *testing.T) {
	var market ListingMarket
	market.read("1,204 for sale starting at $0.03")
	if market.ForSale != 1204 || market.ForSalePrice != "$0.03" {
		t.Fatalf("market = %#v", market)
	}
}

// A listing replaces the text fields rather than joining them: everything the
// generic parser returns for one is either in the listing or is the boilerplate.
func TestDecodeDetailsPrefersTheListingOverTheProseBlock(t *testing.T) {
	body, err := json.Marshal(map[string]any{"success": true, "html": listingDetailsHTML})
	if err != nil {
		t.Fatal(err)
	}
	fields, trade, listing, err := decodeDetails(body)
	if err != nil {
		t.Fatal(err)
	}
	if listing == nil {
		t.Fatal("listing = nil")
	}
	if len(fields) != 0 {
		t.Fatalf("fields = %#v, want none once the listing parsed", fields)
	}
	if trade != nil {
		t.Fatalf("trade = %#v, want nil for a listing", trade)
	}
}

// A page with no price block is not a listing, and must not lose its text.
func TestDecodeDetailsKeepsTextWhenThereIsNoListing(t *testing.T) {
	body, err := json.Marshal(map[string]any{
		"success": true,
		"html":    `<div><p>Confirm your phone number change.</p></div>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	fields, _, listing, err := decodeDetails(body)
	if err != nil {
		t.Fatal(err)
	}
	if listing != nil {
		t.Fatalf("listing = %#v, want nil", listing)
	}
	if len(fields) == 0 {
		t.Fatal("fields = none, want the page's text")
	}
}
