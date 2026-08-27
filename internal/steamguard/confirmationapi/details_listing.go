package confirmationapi

import (
	"strconv"
	"strings"

	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// A market-listing confirmation is mostly boilerplate: how to post the listing,
// and a paragraph of advice for anyone who did not create it. The generic text
// parser flattens the whole page into one block, which buries what actually
// decides the answer: what the sale pays, what the buyer pays, and how the
// market for the item looks. Those live in their own markup, and the item
// itself is JSON, so both are read as structure rather than as prose.

const (
	listingPricesClass = "mobileconf_listing_prices"
	maxListingEntries  = 12
)

// ListingMarket is how the wider market for the item looks, read as numbers so
// the window can word it in the user's own language rather than repeating
// Steam's sentence. A zero count means Steam did not state one.
type ListingMarket struct {
	ForSale      int    `json:"forSale,omitempty"`
	ForSalePrice string `json:"forSalePrice,omitempty"`
	SoldRecently int    `json:"soldRecently,omitempty"`
	// Text carries any line the numbers could not be read from, exactly as Steam
	// wrote it. That is every line on an account whose Steam language is not
	// English: showing Steam's own words beats showing nothing.
	Text []string `json:"text,omitempty"`
}

// ListingDetails is what a market listing says once the boilerplate is dropped.
type ListingDetails struct {
	// Receive is what the sale pays the seller and BuyerPays what it costs the
	// buyer. Steam labels these in the account's own language, so they are taken
	// by their position in its block and labelled by the window instead. The
	// amounts stay verbatim: they are already in the account's currency and
	// formatting, and reformatting a price is a good way to misstate one.
	Receive   string `json:"receive,omitempty"`
	BuyerPays string `json:"buyerPays,omitempty"`
	// Prices is Steam's own labelled pairs, and is filled instead of the two
	// above whenever the block is not the two-entry shape it has always been —
	// so a reworked page still shows what it says, just untranslated.
	Prices []TextField   `json:"prices,omitempty"`
	Market ListingMarket `json:"market"`
	// Item is Steam's own description of what is being listed.
	Item *ItemClass `json:"item,omitempty"`
}

// decodeListing returns nil when the page is not a market listing, so callers
// fall back to the text fields.
func decodeListing(html string) *ListingDetails {
	contextNode := &nethtml.Node{Type: nethtml.ElementNode, DataAtom: atom.Body, Data: "body"}
	nodes, err := nethtml.ParseFragment(strings.NewReader(html), contextNode)
	if err != nil {
		return nil
	}
	listing := &ListingDetails{}
	for _, node := range nodes {
		if block := findListingPrices(node); block != nil {
			readListingEntries(block, listing)
			break
		}
	}
	if len(listing.Prices) == 0 && listing.Market.empty() {
		return nil
	}
	// Steam's block has held exactly these two prices, in this order, for as long
	// as this page has existed.
	if len(listing.Prices) == 2 {
		listing.Receive = listing.Prices[0].Value
		listing.BuyerPays = listing.Prices[1].Value
		listing.Prices = nil
	}
	// The same BuildHover payload a hovered trade item carries, so the listed item
	// is described without a second request. Absent on a page that omits it.
	if item, ok := decodeItemClass([]byte(html)); ok && item.Name != "" {
		listing.Item = &item
	}
	return listing
}

func findListingPrices(node *nethtml.Node) *nethtml.Node {
	if node == nil || excludedNode(node) {
		return nil
	}
	if node.Type == nethtml.ElementNode && classAttr(node, listingPricesClass) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findListingPrices(child); found != nil {
			return found
		}
	}
	return nil
}

// readListingEntries walks the block's rows. A row is a child that has element
// children of its own; the block's heading is plain text, which is how the two
// are told apart without depending on Steam's wording.
func readListingEntries(block *nethtml.Node, listing *ListingDetails) {
	for row := block.FirstChild; row != nil; row = row.NextSibling {
		if row.Type != nethtml.ElementNode || excludedNode(row) || !hasElementChild(row) {
			continue
		}
		for entry := row.FirstChild; entry != nil; entry = entry.NextSibling {
			if entry.Type != nethtml.ElementNode || excludedNode(entry) {
				continue
			}
			if len(listing.Prices)+len(listing.Market.Text) >= maxListingEntries {
				return
			}
			label, value := splitListingEntry(entry)
			switch {
			case label != "" && value != "":
				listing.Prices = append(listing.Prices, TextField{Label: clampText(label), Value: clampText(value)})
			case label != "":
				listing.Market.read(clampText(label))
			}
		}
	}
}

// splitListingEntry reads one entry as "label<br>value". The line break is the
// only thing separating a price from its heading, so the split is on that rather
// than on punctuation, which a currency or a locale could supply anywhere.
// An entry with no break is a single line and comes back as the label alone.
func splitListingEntry(entry *nethtml.Node) (string, string) {
	var before, after strings.Builder
	target := &before
	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if excludedNode(child) {
				continue
			}
			if child.Type == nethtml.ElementNode && child.DataAtom == atom.Br {
				if target == &before {
					target = &after
				}
				continue
			}
			if child.Type == nethtml.TextNode {
				target.WriteString(child.Data)
				target.WriteByte(' ')
			}
			walk(child)
		}
	}
	walk(entry)
	return collapseText(before.String()), collapseText(after.String())
}

// The wording Steam wraps the market numbers in; the sentences themselves are
// never shown.
//
// This only recognises English, because Steam answers the detail page in the
// account's language and there is no reliable way to read every one of them. A
// line that does not match is kept verbatim rather than half-understood, so a
// German account sees Steam's German rather than a mistranslation.
const (
	forSaleMarker = " for sale starting at "
	volumePrefix  = "Volume: "
	soldMarker    = " sold in the last 24 hours"
)

func (m ListingMarket) empty() bool {
	return m.ForSale == 0 && m.SoldRecently == 0 && len(m.Text) == 0
}

func (m *ListingMarket) read(line string) {
	if count, price, ok := countBefore(line, forSaleMarker); ok && price != "" {
		m.ForSale = count
		m.ForSalePrice = price
		return
	}
	if rest, found := strings.CutPrefix(line, volumePrefix); found {
		if count, trailing, ok := countBefore(rest, soldMarker); ok && trailing == "" {
			m.SoldRecently = count
			return
		}
	}
	m.Text = append(m.Text, line)
}

// countBefore reads "<count><marker><rest>". The count may carry English
// thousands separators, which is safe to strip only because nothing but English
// reaches here — elsewhere a full stop or a comma means the opposite.
func countBefore(line, marker string) (int, string, bool) {
	index := strings.Index(line, marker)
	if index <= 0 {
		return 0, "", false
	}
	digits := strings.ReplaceAll(strings.TrimSpace(line[:index]), ",", "")
	count, err := strconv.Atoi(digits)
	if err != nil || count < 0 {
		return 0, "", false
	}
	return count, strings.TrimSpace(line[index+len(marker):]), true
}

func hasElementChild(node *nethtml.Node) bool {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == nethtml.ElementNode && !excludedNode(child) {
			return true
		}
	}
	return false
}
