// Package gcpd parses Steam's "Game Personal Data" page for CS2 (app 730).
//
// The page is the only place Valve exposes an account's competitive matchmaking
// cooldown, and it is only readable while authenticated as that account. It is
// undocumented HTML, so the parser's job is less "extract the value" than
// "refuse to guess": a page it does not recognise must be indistinguishable, to
// the caller, from a page it never fetched.
package gcpd

import (
	"bytes"
	"strconv"
	"strings"
	"time"
	"unicode"

	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Outcome says whether the response may be trusted to update stored state.
//
// The zero value is OutcomeUnrecognised, so a parser path that forgets to set it
// fails closed rather than reporting a confident "no cooldown".
type Outcome int

const (
	// OutcomeUnrecognised is a 200 that is not a GCPD page this build
	// understands. Change nothing; do not stamp a freshness timestamp.
	OutcomeUnrecognised Outcome = iota
	// OutcomeNotSignedIn is Steam's login page, served because the session
	// cookie was rejected. Change nothing.
	OutcomeNotSignedIn
	// OutcomeParsed is a GCPD page that was understood end to end. This is the
	// only outcome that may write to the store - including writing "no cooldown".
	OutcomeParsed
)

func (o Outcome) String() string {
	switch o {
	case OutcomeNotSignedIn:
		return "not-signed-in"
	case OutcomeParsed:
		return "parsed"
	default:
		return "unrecognised"
	}
}

// Rank is one matchmaking mode's standing.
//
// Found distinguishes "the row was not on the page" from a genuine zero - an
// unranked Wingman account really does report 0, so a bare int cannot say which
// happened.
type Rank struct {
	Found bool
	Value int
	Wins  int
}

// Ranks are the matchmaking standings carried by the same page as the cooldown.
//
// They cost no extra request: Steam puts them in the response the cooldown sweep
// already downloads.
type Ranks struct {
	Premier Rank
	Wingman Rank
	// Competitive is Steam's overall competitive skill group where the page
	// carries one, otherwise the highest per-map group - which is what the
	// existing third-party CompRank metric reduces to. Wins is -1 in the per-map
	// case only: those are per map, and summing them would invent a number Steam
	// never shows.
	Competitive Rank

	// PremierPlayed reports a Premier row with any match on it - won, tied, lost,
	// or merely carrying a last-match timestamp - whether or not a rating had
	// been earned yet. Premier is Prime-gated, so this is conclusive evidence of
	// Prime, including for an account still in placements whose rating cell is
	// blank and whose Premier rank is therefore not Found.
	PremierPlayed bool
}

// Any reports whether the page carried any rank at all.
func (r Ranks) Any() bool { return r.Premier.Found || r.Wingman.Found || r.Competitive.Found }

// Result is what a single GCPD fetch yielded.
type Result struct {
	Outcome     Outcome
	HasCooldown bool
	// ExpiresAt is the earliest future cooldown expiry, in UTC. Zero when
	// HasCooldown is false or when Permanent is true.
	ExpiresAt time.Time
	Permanent bool
	Ranks     Ranks
	// HasGameData reports that the page carried at least one table, i.e. that
	// Steam holds CS2 records for this account. It separates "has never played"
	// from "has played and has no Premier history", which is the difference
	// between saying nothing about Prime and guessing at it.
	HasGameData bool
}

const (
	maxBodyBytes = 512 << 10

	// Bounds on the DOM walk. A page that exceeds any of them is malformed or
	// hostile; either way it is not worth the CPU to keep going.
	maxTables       = 64
	maxRowsPerTable = 256
	// The per-map standings table is eight columns wide as Steam ships it today,
	// so this needs headroom: at exactly eight, one added column would trip the
	// truncation guard and silently cost the competitive rank.
	maxCellsPerRow = 24
	maxCellBytes   = 256

	// maxCooldownAhead rejects absurd timestamps. The longest real competitive
	// cooldown is measured in days; a year-9999 row is a parse artefact.
	maxCooldownAhead = 400 * 24 * time.Hour
)

// timestampLayouts are how Steam renders an instant on this page.
//
// It ships the zone as a trailing abbreviation ("2026-08-11 13:38:16 GMT"). The
// C++ reference reads these with sscanf, which ignores anything after the
// seconds, so it never had to notice; a strict Go parse rejects the whole cell,
// and a rejected cooldown row reads as "no cooldown" rather than as an error.
// The bare layout stays as a fallback in case Valve drops the suffix.
var timestampLayouts = []string{
	"2006-01-02 15:04:05 MST",
	"2006-01-02 15:04:05",
}

// parseGCPDTimestamp reads one of Steam's rendered instants as UTC.
func parseGCPDTimestamp(cell string) (time.Time, bool) {
	cell = strings.TrimSpace(cell)
	if cell == "" {
		return time.Time{}, false
	}
	for _, layout := range timestampLayouts {
		if parsed, err := time.Parse(layout, cell); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

// englishMarkers are strings that only appear on an English-rendered GCPD page.
//
// Cooldown detection matches the English word "Cooldown" in a table header, so
// on a non-English render no cooldown table is found - which, without this
// check, would look identical to "this account has no cooldown" and would
// silently clear a real one. Requiring a positive marker before returning
// OutcomeParsed turns that silent data loss into a no-op.
var englishMarkers = []string{
	"Personal Game Data",
	"Matchmaking Mode",
	"Cooldown Expiration",
	"Competitive Wins",
}

// loginMarkers identify the sign-in page Steam serves when the session cookie is
// missing or stale. Redirects are disabled on the request, but Steam also
// renders the login page with a 200 in some flows, so the body is authoritative.
var loginMarkers = []string{
	"g_steamID = false",
	"<title>Sign In",
}

// Parse reads a GCPD matchmaking-tab response.
//
// now is passed in rather than read from the clock so the caller controls the
// definition of "future".
func Parse(body []byte, now time.Time) Result {
	if len(body) == 0 || len(body) > maxBodyBytes {
		return Result{}
	}
	// Byte-scan before parsing: a login page is not worth building a DOM for,
	// and misreading one as an empty GCPD page would clear real cooldowns.
	for _, marker := range loginMarkers {
		if bytes.Contains(body, []byte(marker)) {
			return Result{Outcome: OutcomeNotSignedIn}
		}
	}
	if !bytes.Contains(body, []byte("generic_kv_table")) && !bytes.Contains(body, []byte("Personal Game Data")) {
		return Result{}
	}
	if !hasEnglishMarker(body) {
		return Result{}
	}
	// Require a closed document. net/html recovers from truncation by silently
	// closing every open element, so a response cut short mid-table parses into
	// a cooldown table with no data rows - indistinguishable from a clean
	// account, and clearing on it would delete a real cooldown.
	if !bytes.Contains(bytes.ToLower(body), []byte("</html>")) {
		return Result{}
	}

	doc, err := nethtml.Parse(bytes.NewReader(body))
	if err != nil || doc == nil {
		return Result{}
	}
	// One walk, then route by header text. A page with no cooldown table is a
	// recognisably English GCPD render of a clean account - a positive verdict,
	// not a failure.
	result := Result{Outcome: OutcomeParsed}
	tables := collectTables(doc)
	result.HasGameData = len(tables) > 0
	for _, table := range tables {
		switch {
		case headerMentionsCooldown(table[0]):
			readCooldownTable(table, now, &result)
		case isAggregateRankTable(table[0]):
			readRankTable(table, &result.Ranks)
		case isPerMapRankTable(table[0]):
			readPerMapRankTable(table, &result.Ranks)
		}
	}
	return result
}

func hasEnglishMarker(body []byte) bool {
	for _, marker := range englishMarkers {
		if bytes.Contains(body, []byte(marker)) {
			return true
		}
	}
	return false
}

// collectTables returns every generic_kv_table that has a header and at least
// one data row. Steam ships several on this tab, so header text - not position -
// decides what each one is.
func collectTables(root *nethtml.Node) [][]row {
	var out [][]row
	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		if node == nil || len(out) >= maxTables {
			return
		}
		if node.Type == nethtml.ElementNode && node.DataAtom == atom.Table && hasClass(node, "generic_kv_table") {
			if rows := readRows(node); len(rows) >= 2 && len(rows[0]) > 0 {
				out = append(out, rows)
			}
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return out
}

func headerMentionsCooldown(header row) bool {
	return len(header) > 0 && strings.Contains(strings.ToLower(header[0]), "cooldown")
}

// isAggregateRankTable matches the per-mode standings table and rejects the
// per-map one that shares its first header cell.
//
// Both start with "Matchmaking Mode"; only the per-map table has a Map column.
// Skipping on any "map"-ish header is deliberately over-eager: mistaking the
// per-map table for the aggregate would report one map's skill group as the
// account's rank, and a missing rank is far better than a wrong one.
func isAggregateRankTable(header row) bool {
	if len(header) == 0 || !strings.Contains(strings.ToLower(header[0]), "matchmaking mode") {
		return false
	}
	for _, cell := range header {
		if strings.Contains(strings.ToLower(cell), "map") {
			return false
		}
	}
	return true
}

// isPerMapRankTable matches the sibling of the aggregate table: same leading
// header cell, plus a Map column.
func isPerMapRankTable(header row) bool {
	if len(header) == 0 || !strings.Contains(strings.ToLower(header[0]), "matchmaking mode") {
		return false
	}
	for _, cell := range header {
		if strings.Contains(strings.ToLower(cell), "map") {
			return true
		}
	}
	return false
}

// readPerMapRankTable reduces the per-map skill groups to the highest one.
//
// That matches what the third-party CompRank metric already shows (a max over
// per-map competitive ranks), so an authenticated read is a drop-in for it
// rather than a differently-shaped number.
func readPerMapRankTable(rows []row, ranks *Ranks) {
	header := rows[0]
	if header.truncated() {
		return
	}
	skillCol := -1
	for i, cell := range header {
		if strings.Contains(strings.ToLower(cell), "skill") {
			skillCol = i
			break
		}
	}
	if skillCol < 0 {
		return
	}
	best, found := 0, false
	for _, cells := range rows[1:] {
		// A truncated row means the named column may not be where the header
		// said. Reading it anyway is how this produces a wrong rank.
		if cells.truncated() || len(cells) <= skillCol {
			continue
		}
		value, err := parseCount(cells[skillCol])
		if err != nil || value <= 0 {
			continue
		}
		if !found || value > best {
			best, found = value, true
		}
	}
	// Never overwrites a value the aggregate table already supplied: that is
	// Steam's own overall standing, where this is a maximum computed over maps.
	if found && !ranks.Competitive.Found {
		ranks.Competitive = Rank{Found: true, Value: best, Wins: -1}
	}
}

// readRankTable folds the per-mode standings into ranks.
//
// Columns are located by header name only. The C++ reference falls back to fixed
// column indices when the names do not match; that is how a layout change turns
// into a confidently wrong rating, so here a table whose columns cannot be named
// is skipped entirely.
func readRankTable(rows []row, ranks *Ranks) {
	header := rows[0]
	if header.truncated() {
		return
	}
	skillCol, winsCol := -1, -1
	// Ties, losses and the last-match timestamp are read only to answer "has this
	// account played this mode at all". Wins alone says no for a player whose
	// entire Premier history is defeats.
	tiesCol, lossesCol, lastMatchCol := -1, -1, -1
	for i, cell := range header {
		lower := strings.ToLower(cell)
		switch {
		case skillCol < 0 && strings.Contains(lower, "skill"):
			skillCol = i
		case winsCol < 0 && strings.Contains(lower, "wins"):
			winsCol = i
		case tiesCol < 0 && strings.Contains(lower, "ties"):
			tiesCol = i
		case lossesCol < 0 && strings.Contains(lower, "losses"):
			lossesCol = i
		case lastMatchCol < 0 && strings.Contains(lower, "last match"):
			lastMatchCol = i
		}
	}
	if skillCol < 0 {
		return
	}

	countAt := func(cells row, col int) int {
		if col < 0 || len(cells) <= col {
			return -1
		}
		parsed, err := parseCount(cells[col])
		if err != nil {
			return -1
		}
		return parsed
	}

	for _, cells := range rows[1:] {
		if cells.truncated() || len(cells) <= skillCol {
			continue
		}
		wins := countAt(cells, winsCol)
		mode := strings.ToLower(cells[0])

		// Read before the skill group, not after: an account still in placements
		// has matches here and a blank rating, and skipping the row for the blank
		// threw away the only proof that it plays a Prime-gated mode.
		//
		// Any recorded match counts, not just a win. A row reading 0 wins, 0 ties,
		// 1 loss is still an account that queued Premier, which cannot be done
		// without Prime.
		if strings.Contains(mode, "premier") && !ranks.PremierPlayed {
			played := wins > 0 || countAt(cells, tiesCol) > 0 || countAt(cells, lossesCol) > 0
			if !played && lastMatchCol >= 0 && len(cells) > lastMatchCol {
				played = strings.TrimSpace(cells[lastMatchCol]) != ""
			}
			ranks.PremierPlayed = played
		}

		value, err := parseCount(cells[skillCol])
		if err != nil {
			continue
		}
		switch {
		case strings.Contains(mode, "premier"):
			ranks.Premier = Rank{Found: true, Value: value, Wins: wins}
		case strings.Contains(mode, "wingman"):
			ranks.Wingman = Rank{Found: true, Value: value, Wins: wins}
		case strings.Contains(mode, "competitive"):
			// Steam records the competitive skill group in either place and, in
			// practice, only one of them per account: an account with a group
			// here has blanks in the per-map table and vice versa. Overwrites
			// the per-map value if both somehow appear - this is Steam's own
			// overall figure, where the other is a maximum we compute.
			ranks.Competitive = Rank{Found: true, Value: value, Wins: wins}
		}
	}
}

// parseCount reads an integer cell, tolerating the thousands separators Steam
// renders into a Premier rating.
func parseCount(cell string) (int, error) {
	cleaned := strings.Map(func(r rune) rune {
		if r == ',' || r == ' ' {
			return -1
		}
		return r
	}, strings.TrimSpace(cell))
	if cleaned == "" {
		return 0, strconv.ErrSyntax
	}
	return strconv.Atoi(cleaned)
}

// readCooldownTable folds one table's data rows into result.
//
// Two shapes appear. A parseable timestamp is an expiring cooldown, and the
// earliest future one wins - past rows are expired history and are ignored. A
// non-date first cell alongside a numeric second cell is how Steam renders a
// non-expiring cooldown ("Never", with a Level column beside it).
func readCooldownTable(rows []row, now time.Time, result *Result) {
	for _, cells := range rows[1:] {
		if len(cells) == 0 {
			continue
		}
		expiry, ok := parseGCPDTimestamp(cells[0])
		if !ok {
			if cells[0] == "" || len(cells) < 2 {
				continue
			}
			if level, err := strconv.Atoi(cells[1]); err == nil && level >= 1 {
				result.HasCooldown = true
				result.Permanent = true
				result.ExpiresAt = time.Time{}
			}
			continue
		}
		if result.Permanent {
			// A non-expiring cooldown outranks any dated row on the page.
			continue
		}
		if !expiry.After(now) || expiry.Sub(now) > maxCooldownAhead {
			continue
		}
		if !result.HasCooldown || expiry.Before(result.ExpiresAt) {
			result.HasCooldown = true
			result.ExpiresAt = expiry
		}
	}
}

type row []string

func readRows(table *nethtml.Node) []row {
	var rows []row
	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		if node == nil || len(rows) >= maxRowsPerTable {
			return
		}
		if node.Type == nethtml.ElementNode && node.DataAtom == atom.Tr {
			if cells := readCells(node); len(cells) > 0 {
				rows = append(rows, cells)
			}
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(table)
	return rows
}

// truncatedCell marks a row that had more cells than the walk will read.
//
// Silently dropping the overflow is what turns a wider-than-expected table into
// a confidently wrong value: the column a header named is simply not there any
// more, and whatever sits at that index gets read instead. Callers that resolve
// columns by index must treat a row carrying this as unusable.
const truncatedCell = "\x00truncated"

func readCells(tr *nethtml.Node) row {
	var cells row
	for child := tr.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != nethtml.ElementNode || (child.DataAtom != atom.Td && child.DataAtom != atom.Th) {
			continue
		}
		if len(cells) >= maxCellsPerRow {
			cells = append(cells, truncatedCell)
			break
		}
		text := visibleText(child)
		if len(text) > maxCellBytes {
			text = text[:maxCellBytes]
		}
		cells = append(cells, text)
	}
	return cells
}

func (r row) truncated() bool {
	return len(r) > 0 && r[len(r)-1] == truncatedCell
}

func hasClass(node *nethtml.Node, want string) bool {
	for _, attr := range node.Attr {
		if attr.Key != "class" {
			continue
		}
		for _, field := range strings.Fields(attr.Val) {
			if field == want {
				return true
			}
		}
	}
	return false
}

func visibleText(node *nethtml.Node) string {
	var builder strings.Builder
	appendVisibleText(&builder, node)
	return strings.Join(strings.FieldsFunc(builder.String(), unicode.IsSpace), " ")
}

func appendVisibleText(builder *strings.Builder, node *nethtml.Node) {
	if node == nil || builder.Len() > maxCellBytes*4 {
		return
	}
	if node.Type == nethtml.CommentNode || (node.Type == nethtml.ElementNode &&
		(node.DataAtom == atom.Script || node.DataAtom == atom.Style || node.DataAtom == atom.Noscript || node.DataAtom == atom.Svg)) {
		return
	}
	if node.Type == nethtml.TextNode {
		builder.WriteString(node.Data)
		builder.WriteByte(' ')
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		appendVisibleText(builder, child)
	}
}
