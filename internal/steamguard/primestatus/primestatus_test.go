package primestatus

import "testing"

// page wraps sections in the least markup Parse insists on: a signed-in marker
// and at least one purchase section.
func page(sections string) []byte {
	return []byte(`<html><head><title>Counter-Strike 2 on Steam</title>` +
		`<script>g_AccountID = 104322402;</script></head><body>` +
		sections + `</body></html>`)
}

// primeSection is the real markup, trimmed to the parts Parse looks at.
func primeSection(ownedFlag string) string {
	return `<div class="game_area_purchase_game_wrapper">` +
		`<div class="game_area_purchase_game" id="game_area_purchase_section_add_to_cart_54029">` +
		ownedFlag +
		`<h2 class="title">Buy Prime Status Upgrade</h2>` +
		`<div class="game_purchase_action"><div class="btn_addtocart">Add to Cart</div></div>` +
		`</div></div>`
}

const ownedFlag = `<div class="package_in_library_flag in_own_library"><span class="icon">&#9776;</span> <span>In library</span></div>`

// baseGameSection is the free CS2 package. Every account that has launched the
// game owns it, so it carries the same flag - which is exactly why the flag has
// to be read inside the Prime block and not anywhere on the page.
const baseGameSection = `<div class="game_area_purchase_game" id="game_area_purchase_section_add_to_cart_298963">` +
	ownedFlag + `<h2 class="title">Play Counter-Strike 2</h2></div>`

func TestParseReportsThePackageAsOwned(t *testing.T) {
	t.Parallel()
	got := Parse(page(primeSection(ownedFlag)))
	if got.Outcome != OutcomeParsed || !got.OwnsPrimePackage {
		t.Fatalf("Parse = %#v, want the package owned", got)
	}
}

func TestParseReportsThePackageAsNotOwned(t *testing.T) {
	t.Parallel()
	got := Parse(page(primeSection("")))
	if got.Outcome != OutcomeParsed || got.OwnsPrimePackage {
		t.Fatalf("Parse = %#v, want the package not owned", got)
	}
}

// The shape actually captured from a live account: a price and an Add to Cart
// button, no ownership flag anywhere in the section.
func TestParseReadsTheCapturedUnownedSection(t *testing.T) {
	t.Parallel()
	captured := `<div class="game_area_purchase_game_wrapper">` +
		`<div class="game_area_purchase_game" id="game_area_purchase_section_add_to_cart_54029" role="region">` +
		`<form name="add_to_cart_54029"><input type="hidden" name="subid" value="54029"></form>` +
		`<h2 id="game_area_purchase_section_add_to_cart_title_54029" class="title">Buy Prime Status Upgrade</h2>` +
		`<div class="game_purchase_action"><div class="game_purchase_action_bg">` +
		`<div class="game_purchase_price price" data-price-final="24699">R 246.99</div>` +
		`<div class="btn_addtocart"><a id="btn_add_to_cart_54029"><span>Add to Cart</span></a></div>` +
		`</div></div></div></div>`
	got := Parse(page(captured))
	if got.Outcome != OutcomeParsed || got.OwnsPrimePackage {
		t.Fatalf("Parse = %#v, want parsed and not owned", got)
	}
}

// Steam renders more than one ownership marker; a section carrying the other
// variant must not read as unowned.
func TestParseAcceptsTheDsOwnedFlagVariant(t *testing.T) {
	t.Parallel()
	section := `<div class="game_area_purchase_game" id="game_area_purchase_section_add_to_cart_54029">` +
		`<div class="ds_owned_flag ds_flag">IN LIBRARY</div>` +
		`<h2 class="title">Buy Prime Status Upgrade</h2></div>`
	if got := Parse(page(section)); !got.OwnsPrimePackage {
		t.Fatalf("Parse = %#v, want the ds_owned_flag variant recognised", got)
	}
}

func TestParseIgnoresOwnershipOfAnotherPackage(t *testing.T) {
	t.Parallel()
	// The base game is free and owned by everyone who has played, so a page-wide
	// search for the flag would report every CS2 player as Prime.
	got := Parse(page(baseGameSection + primeSection("")))
	if got.Outcome != OutcomeParsed || got.OwnsPrimePackage {
		t.Fatalf("Parse = %#v, want the base game's ownership ignored", got)
	}
}

func TestParseFindsThePrimeSectionByTitleWhenTheSubIDChanges(t *testing.T) {
	t.Parallel()
	renumbered := `<div class="game_area_purchase_game" id="game_area_purchase_section_add_to_cart_99999">` +
		ownedFlag + `<h2 class="title">Buy Prime Status Upgrade</h2></div>`
	got := Parse(page(renumbered))
	if got.Outcome != OutcomeParsed || !got.OwnsPrimePackage {
		t.Fatalf("Parse = %#v, want the section found by its heading", got)
	}
}

func TestParseReportsNotSignedIn(t *testing.T) {
	t.Parallel()
	// Signed out the store renders every section with no ownership flag anywhere,
	// which would otherwise read as a confident "owns no Prime".
	body := []byte(`<html><head><script>g_AccountID = 0;</script></head><body>` +
		primeSection("") + `</body></html>`)
	if got := Parse(body); got.Outcome != OutcomeNotSignedIn {
		t.Fatalf("Parse = %#v, want not-signed-in", got)
	}
}

func TestParseDeclinesAPageWithNoPrimeSection(t *testing.T) {
	t.Parallel()
	// An age gate, or a region where the package is not sold. Not a verdict.
	if got := Parse(page(baseGameSection)); got.Outcome != OutcomeUnrecognised {
		t.Fatalf("Parse = %#v, want unrecognised", got)
	}
}

func TestParseDeclinesAPageWithNoSignedInMarker(t *testing.T) {
	t.Parallel()
	body := []byte(`<html><body>` + primeSection(ownedFlag) + `</body></html>`)
	if got := Parse(body); got.Outcome != OutcomeUnrecognised {
		t.Fatalf("Parse = %#v, want unrecognised", got)
	}
}

func TestParseDeclinesAnEmptyBody(t *testing.T) {
	t.Parallel()
	if got := Parse(nil); got.Outcome != OutcomeUnrecognised {
		t.Fatalf("Parse = %#v", got)
	}
}

// game_area_purchase_game_wrapper must not be mistaken for the section itself.
func TestParseMatchesWholeClassTokens(t *testing.T) {
	t.Parallel()
	onlyWrapper := `<div class="game_area_purchase_game_wrapper" id="add_to_cart_54029">` + ownedFlag + `</div>`
	if got := Parse(page(onlyWrapper + primeSection(""))); got.OwnsPrimePackage {
		t.Fatalf("Parse = %#v, want the wrapper not treated as the section", got)
	}
}
