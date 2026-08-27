// Package primestatus reads CS2 Prime ownership out of the store page.
//
// Valve exposes this nowhere else a web request can reach: the GCPD
// "primeaccount" tab is empty even for a Prime account, and the account licenses
// page serves a shell with no table to this client. The store page, fetched with
// the account's own session, marks each purchase section the account already
// owns - and Prime is sold as a package like any other.
package primestatus

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"

	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Outcome separates "the account has no Prime" from "this page could not be
// read". They are not interchangeable: a page we failed to understand must not
// become a Non-Prime badge on someone's Prime account.
type Outcome int

const (
	OutcomeUnrecognised Outcome = iota
	OutcomeNotSignedIn
	OutcomeParsed
)

func (o Outcome) String() string {
	switch o {
	case OutcomeNotSignedIn:
		return "not_signed_in"
	case OutcomeParsed:
		return "parsed"
	default:
		return "unrecognised"
	}
}

// Result reports package ownership, not Prime status. They are not the same:
// accounts that reached rank 21 before 2019 were granted Prime without ever
// holding this package, and the store page shows them "Add to Cart". Deciding
// Prime is the caller's job, from this plus the Premier activity in GCPD.
type Result struct {
	Outcome          Outcome
	OwnsPrimePackage bool
}

const (
	maxBodyBytes = 4 << 20
	// Bounds the DOM walk on a page we do not control the size of.
	maxNodes = 400_000

	// primeSubID is the Prime Status Upgrade package. The purchase section's id
	// carries it, which is what lets the ownership flag be read from the right
	// block instead of anywhere on a page that has many.
	primeSubID = "54029"

	purchaseSectionClass = "game_area_purchase_game"
)

// ownedFlagClasses are the ways a purchase section says the account already
// holds that package. Steam renders more than one variant, so all of them are
// matched, and only ever inside the Prime section.
var ownedFlagClasses = []string{
	"in_own_library",
	"package_in_library_flag",
	"ds_owned_flag",
	"game_area_already_owned",
}

// primeTitleMarkers back up the sub id. Valve could renumber the package; the
// heading is the other thing that names it. Matched case-insensitively against
// the section's own title only, never the whole page.
var primeTitleMarkers = []string{"prime status"}

// Parse reads a CS2 store page response.
func Parse(body []byte) Result {
	if len(body) == 0 || len(body) > maxBodyBytes {
		return Result{}
	}
	// Signed out, the store still renders every purchase section - with no
	// ownership flag on any of them. Reading that as "owns nothing" would report
	// every account as Non-Prime the moment a session lapsed.
	//
	// g_AccountID, not g_steamID: the latter is a steamcommunity.com variable and
	// appears nowhere on a store page.
	switch readSession(body) {
	case signedOut:
		return Result{Outcome: OutcomeNotSignedIn}
	case unknownSession:
		return Result{}
	}
	if !bytes.Contains(body, []byte(purchaseSectionClass)) {
		return Result{}
	}

	doc, err := nethtml.Parse(bytes.NewReader(body))
	if err != nil {
		return Result{}
	}
	section := findPrimeSection(doc)
	if section == nil {
		// The page rendered but carries no Prime section - an age gate, a region
		// where the package is not sold, or a layout change. Not a verdict.
		return Result{}
	}
	return Result{Outcome: OutcomeParsed, OwnsPrimePackage: containsOwnedFlag(section)}
}

type sessionState int

const (
	unknownSession sessionState = iota
	signedOut
	signedIn
)

var accountIDPattern = regexp.MustCompile(`g_AccountID\s*=\s*(\d+)`)

// readSession reads the store's account id. Zero, or a page that carries none at
// all, is not a session we can draw an ownership conclusion from.
func readSession(body []byte) sessionState {
	match := accountIDPattern.FindSubmatch(body)
	if match == nil {
		return unknownSession
	}
	if id, err := strconv.ParseUint(string(match[1]), 10, 64); err != nil || id == 0 {
		return signedOut
	}
	return signedIn
}

func findPrimeSection(root *nethtml.Node) *nethtml.Node {
	var found *nethtml.Node
	budget := maxNodes
	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		if node == nil || found != nil || budget <= 0 {
			return
		}
		budget--
		if node.Type == nethtml.ElementNode && hasClass(node, purchaseSectionClass) && isPrimeSection(node) {
			found = node
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

// isPrimeSection identifies the Prime block by sub id, falling back to its
// heading. Both are scoped to this one section.
func isPrimeSection(node *nethtml.Node) bool {
	if strings.Contains(attr(node, "id"), primeSubID) {
		return true
	}
	return sectionTitleMentionsPrime(node)
}

func sectionTitleMentionsPrime(section *nethtml.Node) bool {
	var found bool
	budget := maxNodes
	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		if node == nil || found || budget <= 0 {
			return
		}
		budget--
		if node.Type == nethtml.ElementNode && node.DataAtom == atom.H2 && hasClass(node, "title") {
			title := strings.ToLower(textOf(node))
			for _, marker := range primeTitleMarkers {
				if strings.Contains(title, marker) {
					found = true
					return
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(section)
	return found
}

func containsOwnedFlag(section *nethtml.Node) bool {
	var found bool
	budget := maxNodes
	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		if node == nil || found || budget <= 0 {
			return
		}
		budget--
		if node.Type == nethtml.ElementNode {
			for _, class := range ownedFlagClasses {
				if hasClass(node, class) {
					found = true
					return
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(section)
	return found
}

// hasClass matches a whole class token, so game_area_purchase_game does not also
// match game_area_purchase_game_wrapper.
func hasClass(node *nethtml.Node, want string) bool {
	for _, field := range strings.Fields(attr(node, "class")) {
		if field == want {
			return true
		}
	}
	return false
}

func attr(node *nethtml.Node, name string) string {
	for _, a := range node.Attr {
		if strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

func textOf(node *nethtml.Node) string {
	var b strings.Builder
	budget := 4096
	var walk func(*nethtml.Node)
	walk = func(n *nethtml.Node) {
		if n == nil || budget <= 0 {
			return
		}
		if n.Type == nethtml.TextNode {
			budget -= len(n.Data)
			b.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return b.String()
}
