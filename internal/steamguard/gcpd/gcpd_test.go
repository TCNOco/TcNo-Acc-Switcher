package gcpd

import (
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)

// page wraps table markup in the minimum surroundings a real GCPD response has:
// the English marker the parser demands, and the generic_kv_table class.
func page(tables string) []byte {
	return []byte(`<html><head><title>Steam Community</title></head><body>
<h1>Personal Game Data</h1>` + tables + `</body></html>`)
}

func cooldownTable(rows string) string {
	return `<table class="generic_kv_table"><tr><th>Cooldown Expiration</th><th>Level</th></tr>` + rows + `</table>`
}

func TestParseReadsAFutureCooldown(t *testing.T) {
	t.Parallel()
	got := Parse(page(cooldownTable(`<tr><td>2026-08-14 09:30:00</td><td>0</td></tr>`)), testNow)
	if got.Outcome != OutcomeParsed || !got.HasCooldown || got.Permanent {
		t.Fatalf("Parse = %#v", got)
	}
	want := time.Date(2026, 8, 14, 9, 30, 0, 0, time.UTC)
	if !got.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v", got.ExpiresAt, want)
	}
}

func TestParseTakesTheEarliestFutureCooldown(t *testing.T) {
	t.Parallel()
	// Past rows are expired history. Only a future row constrains the account,
	// and if several do, the nearest one is when they can play again.
	rows := `<tr><td>2026-08-20 00:00:00</td><td>0</td></tr>` +
		`<tr><td>2026-08-09 00:00:00</td><td>0</td></tr>` +
		`<tr><td>2026-01-01 00:00:00</td><td>0</td></tr>`
	got := Parse(page(cooldownTable(rows)), testNow)
	want := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)
	if !got.HasCooldown || !got.ExpiresAt.Equal(want) {
		t.Fatalf("Parse = %#v, want expiry %v", got, want)
	}
}

func TestParseReportsNoCooldownWhenEveryRowIsInThePast(t *testing.T) {
	t.Parallel()
	// This is a positive verdict, not a failure: the caller must be able to
	// clear a stored cooldown on the strength of it.
	rows := `<tr><td>2026-01-01 00:00:00</td><td>0</td></tr><tr><td>2025-06-05 12:00:00</td><td>0</td></tr>`
	got := Parse(page(cooldownTable(rows)), testNow)
	if got.Outcome != OutcomeParsed || got.HasCooldown {
		t.Fatalf("Parse = %#v", got)
	}
}

func TestParseReportsNoCooldownWhenNoCooldownTableIsPresent(t *testing.T) {
	t.Parallel()
	other := `<table class="generic_kv_table"><tr><th>Matchmaking Mode</th><th>Wins</th></tr>` +
		`<tr><td>Premier</td><td>42</td></tr></table>`
	got := Parse(page(other), testNow)
	if got.Outcome != OutcomeParsed || got.HasCooldown {
		t.Fatalf("Parse = %#v", got)
	}
}

func TestParseDetectsAPermanentCooldown(t *testing.T) {
	t.Parallel()
	// Steam renders a non-expiring cooldown as a non-date with a Level beside it.
	got := Parse(page(cooldownTable(`<tr><td>Never</td><td>1</td></tr>`)), testNow)
	if got.Outcome != OutcomeParsed || !got.HasCooldown || !got.Permanent {
		t.Fatalf("Parse = %#v", got)
	}
	if !got.ExpiresAt.IsZero() {
		t.Fatalf("ExpiresAt = %v, want zero for a permanent cooldown", got.ExpiresAt)
	}
}

func TestParseOutranksADatedRowWithAPermanentOne(t *testing.T) {
	t.Parallel()
	rows := `<tr><td>Never</td><td>1</td></tr><tr><td>2026-08-09 00:00:00</td><td>0</td></tr>`
	got := Parse(page(cooldownTable(rows)), testNow)
	if !got.Permanent || !got.ExpiresAt.IsZero() {
		t.Fatalf("Parse = %#v", got)
	}
}

func TestParseIgnoresANonDateRowWithoutALevel(t *testing.T) {
	t.Parallel()
	// Level 0 beside a non-date is a header artefact or an empty row, not a
	// permanent ban. Treating it as one would strand the account forever.
	for _, rows := range []string{
		`<tr><td>Never</td><td>0</td></tr>`,
		`<tr><td>Never</td></tr>`,
		`<tr><td></td><td>1</td></tr>`,
		`<tr><td>-</td><td>not a number</td></tr>`,
	} {
		got := Parse(page(cooldownTable(rows)), testNow)
		if got.Outcome != OutcomeParsed || got.HasCooldown {
			t.Fatalf("rows %q: Parse = %#v", rows, got)
		}
	}
}

func rankTable(rows string) string {
	return `<table class="generic_kv_table"><tr><th>Matchmaking Mode</th><th>Wins</th><th>Ties</th><th>Losses</th><th>Skill</th></tr>` + rows + `</table>`
}

func TestParseReadsPremierAndWingmanFromTheSameBody(t *testing.T) {
	t.Parallel()
	// The whole point: these ranks ride along in the response the cooldown sweep
	// already downloads, so they cost no extra request.
	rows := `<tr><td>Premier</td><td>42</td><td>1</td><td>17</td><td>15,234</td></tr>` +
		`<tr><td>Wingman</td><td>8</td><td>0</td><td>3</td><td>11</td></tr>` +
		`<tr><td>Competitive</td><td>90</td><td>2</td><td>60</td><td>7</td></tr>`
	got := Parse(page(rankTable(rows)), testNow)
	if got.Outcome != OutcomeParsed {
		t.Fatalf("Parse = %#v", got)
	}
	if got.Ranks.Premier != (Rank{Found: true, Value: 15234, Wins: 42}) {
		t.Fatalf("Premier = %#v", got.Ranks.Premier)
	}
	if got.Ranks.Wingman != (Rank{Found: true, Value: 11, Wins: 8}) {
		t.Fatalf("Wingman = %#v", got.Ranks.Wingman)
	}
	// Wins is real here, unlike the per-map case: this is Steam's own total.
	if got.Ranks.Competitive != (Rank{Found: true, Value: 7, Wins: 90}) {
		t.Fatalf("Competitive = %#v", got.Ranks.Competitive)
	}
}

func TestParseCountsAPremierLossAsHavingPlayed(t *testing.T) {
	t.Parallel()
	// A real account: 0 wins, 0 ties, 1 loss, no rating yet. Premier cannot be
	// queued without Prime, so this row is proof of Prime - and gating on wins
	// alone reported the account as Non-Prime.
	rows := `<tr><td>Premier</td><td>0</td><td>0</td><td>1</td><td></td></tr>`
	got := Parse(page(rankTable(rows)), testNow)
	if !got.Ranks.PremierPlayed {
		t.Fatalf("PremierPlayed = false for a row with a recorded loss: %#v", got.Ranks)
	}
	if got.Ranks.Premier.Found {
		t.Fatalf("Premier = %#v, want absent for a blank rating", got.Ranks.Premier)
	}
}

func TestParseCountsAPremierTieAsHavingPlayed(t *testing.T) {
	t.Parallel()
	rows := `<tr><td>Premier</td><td>0</td><td>2</td><td>0</td><td></td></tr>`
	if got := Parse(page(rankTable(rows)), testNow); !got.Ranks.PremierPlayed {
		t.Fatalf("PremierPlayed = false for a row with recorded ties: %#v", got.Ranks)
	}
}

func TestParseDoesNotCountAnEmptyPremierRowAsHavingPlayed(t *testing.T) {
	t.Parallel()
	// All zero and no last match: nothing here says the account ever queued, so
	// Prime stays for the store page to answer.
	rows := `<tr><td>Premier</td><td>0</td><td>0</td><td>0</td><td></td></tr>`
	if got := Parse(page(rankTable(rows)), testNow); got.Ranks.PremierPlayed {
		t.Fatalf("PremierPlayed = true for an empty row: %#v", got.Ranks)
	}
}

// The aggregate table Steam actually ships carries a Last Match column, which
// is enough on its own even if every count reads zero.
func TestParseCountsALastMatchTimestampAsHavingPlayed(t *testing.T) {
	t.Parallel()
	body := `<table class="generic_kv_table"><tr><th>Matchmaking Mode</th><th>Wins</th><th>Ties</th>` +
		`<th>Losses</th><th>Skill Group</th><th>Last Match</th></tr>` +
		`<tr><td>Premier</td><td>0</td><td>0</td><td>0</td><td></td><td>2026-08-05 20:16:57 GMT</td></tr></table>`
	if got := Parse(page(body), testNow); !got.Ranks.PremierPlayed {
		t.Fatalf("PremierPlayed = false despite a last-match timestamp: %#v", got.Ranks)
	}
}

func TestParseReadsCompetitiveFromTheAggregateTableWhenPerMapIsBlank(t *testing.T) {
	t.Parallel()
	// A real account shape: the overall standing is on the aggregate row and
	// every per-map skill group is empty. Reading only the per-map table
	// reported no competitive rank at all for these accounts.
	body := page(
		rankTable(`<tr><td>Premier</td><td>84</td><td>2</td><td>60</td><td>22,887</td></tr>`+
			`<tr><td>Competitive</td><td>854</td><td>136</td><td>858</td><td>12</td></tr>`) +
			perMapTable(`<tr><td>Ranked Competitive</td><td>Mirage</td><td>5</td><td></td></tr>`+
				`<tr><td>Ranked Competitive</td><td>Nuke</td><td>1</td><td></td></tr>`),
	)
	got := Parse(body, testNow)
	if got.Ranks.Competitive != (Rank{Found: true, Value: 12, Wins: 854}) {
		t.Fatalf("Competitive = %#v", got.Ranks.Competitive)
	}
	if got.Ranks.Premier != (Rank{Found: true, Value: 22887, Wins: 84}) {
		t.Fatalf("Premier = %#v", got.Ranks.Premier)
	}
}

func TestParseKeepsTheAggregateCompetitiveOverThePerMapMaximum(t *testing.T) {
	t.Parallel()
	// Steam's own overall figure wins. The per-map maximum is a number we
	// compute, and it is higher here, so a precedence slip would be visible.
	body := page(
		rankTable(`<tr><td>Competitive</td><td>10</td><td>0</td><td>4</td><td>6</td></tr>`) +
			perMapTable(`<tr><td>Ranked Competitive</td><td>Mirage</td><td>5</td><td>18</td></tr>`),
	)
	if got := Parse(body, testNow); got.Ranks.Competitive.Value != 6 {
		t.Fatalf("Competitive = %#v, want the aggregate value 6", got.Ranks.Competitive)
	}
}

func TestParseStillFallsBackToThePerMapMaximum(t *testing.T) {
	t.Parallel()
	// The other real shape: no Competitive row at all, groups only on the maps.
	body := page(
		rankTable(`<tr><td>Premier</td><td>9</td><td>0</td><td>0</td><td></td></tr>`) +
			perMapTable(`<tr><td>Ranked Competitive</td><td>Vertigo</td><td>2</td><td>5</td></tr>`+
				`<tr><td>Ranked Competitive</td><td>Mirage</td><td>2</td><td>2</td></tr>`),
	)
	got := Parse(body, testNow)
	if got.Ranks.Competitive != (Rank{Found: true, Value: 5, Wins: -1}) {
		t.Fatalf("Competitive = %#v", got.Ranks.Competitive)
	}
	// A blank skill cell is not rank 0: the account is still in placements.
	if got.Ranks.Premier.Found {
		t.Fatalf("Premier = %#v, want absent for a blank skill group", got.Ranks.Premier)
	}
}

func TestParseReadsRanksAndCooldownFromOnePage(t *testing.T) {
	t.Parallel()
	body := page(
		cooldownTable(`<tr><td>2026-08-14 09:30:00</td><td>0</td></tr>`) +
			rankTable(`<tr><td>Premier</td><td>42</td><td>1</td><td>17</td><td>15,234</td></tr>`),
	)
	got := Parse(body, testNow)
	if !got.HasCooldown {
		t.Fatalf("cooldown lost when a rank table is present: %#v", got)
	}
	if !got.Ranks.Premier.Found || got.Ranks.Premier.Value != 15234 {
		t.Fatalf("Premier = %#v", got.Ranks.Premier)
	}
}

func perMapTable(rows string) string {
	return `<table class="generic_kv_table"><tr><th>Matchmaking Mode</th><th>Map</th><th>Wins</th><th>Skill</th></tr>` + rows + `</table>`
}

func TestParseNeverReadsThePerMapTableAsTheAggregate(t *testing.T) {
	t.Parallel()
	// Both tables lead with "Matchmaking Mode"; only the per-map one has a Map
	// column. Reading it as the aggregate would report one map's skill group as
	// the account's Premier rating - a confidently wrong number.
	got := Parse(page(perMapTable(`<tr><td>Premier</td><td>Mirage</td><td>10</td><td>18</td></tr>`)), testNow)
	if got.Outcome != OutcomeParsed {
		t.Fatalf("Parse = %#v", got)
	}
	if got.Ranks.Premier.Found || got.Ranks.Wingman.Found {
		t.Fatalf("per-map table produced aggregate ranks: %#v", got.Ranks)
	}
}

func TestParseReducesPerMapSkillGroupsToTheHighest(t *testing.T) {
	t.Parallel()
	// This is what the third-party CompRank metric already shows (a max over
	// per-map competitive ranks), so an authenticated read drops straight in.
	rows := `<tr><td>Competitive</td><td>Mirage</td><td>10</td><td>12</td></tr>` +
		`<tr><td>Competitive</td><td>Inferno</td><td>4</td><td>15</td></tr>` +
		`<tr><td>Competitive</td><td>Nuke</td><td>2</td><td>9</td></tr>`
	got := Parse(page(perMapTable(rows)), testNow)
	if got.Ranks.Competitive != (Rank{Found: true, Value: 15, Wins: -1}) {
		t.Fatalf("Competitive = %#v", got.Ranks.Competitive)
	}
}

func TestParseSkipsAPerMapTableWiderThanTheWalkReads(t *testing.T) {
	t.Parallel()
	// A row with more cells than maxCellsPerRow means the column the header named
	// may not be at that index any more. Reading it anyway is exactly how this
	// produces a wrong rank instead of a missing one.
	var header strings.Builder
	var cells strings.Builder
	header.WriteString(`<tr><th>Matchmaking Mode</th><th>Map</th><th>Skill</th>`)
	cells.WriteString(`<tr><td>Competitive</td><td>Mirage</td><td>15</td>`)
	for i := 0; i < maxCellsPerRow; i++ {
		header.WriteString(`<th>Filler</th>`)
		cells.WriteString(`<td>1</td>`)
	}
	header.WriteString(`</tr>`)
	cells.WriteString(`</tr>`)
	body := page(`<table class="generic_kv_table">` + header.String() + cells.String() + `</table>`)

	got := Parse(body, testNow)
	if got.Outcome != OutcomeParsed {
		t.Fatalf("Parse = %#v", got)
	}
	if got.Ranks.Competitive.Found {
		t.Fatalf("a truncated per-map table produced a rank: %#v", got.Ranks.Competitive)
	}
}

func TestParseSkipsAnAggregateTableWiderThanTheWalkReads(t *testing.T) {
	t.Parallel()
	var header strings.Builder
	var cells strings.Builder
	header.WriteString(`<tr><th>Matchmaking Mode</th><th>Wins</th><th>Skill</th>`)
	cells.WriteString(`<tr><td>Premier</td><td>42</td><td>15234</td>`)
	for i := 0; i < maxCellsPerRow; i++ {
		header.WriteString(`<th>Filler</th>`)
		cells.WriteString(`<td>1</td>`)
	}
	header.WriteString(`</tr>`)
	cells.WriteString(`</tr>`)

	got := Parse(page(`<table class="generic_kv_table">`+header.String()+cells.String()+`</table>`), testNow)
	if got.Ranks.Premier.Found {
		t.Fatalf("a truncated aggregate table produced a rank: %#v", got.Ranks.Premier)
	}
}

func TestParseSkipsARankTableWhoseColumnsCannotBeNamed(t *testing.T) {
	t.Parallel()
	// No positional fallback on purpose. The C++ reference defaults to fixed
	// column indices here, which is how a layout change becomes a wrong rating
	// rather than a missing one.
	renamed := `<table class="generic_kv_table">` +
		`<tr><th>Matchmaking Mode</th><th>Victories</th><th>Rating</th></tr>` +
		`<tr><td>Premier</td><td>42</td><td>15234</td></tr></table>`
	got := Parse(page(renamed), testNow)
	if got.Ranks.Any() {
		t.Fatalf("ranks read from an unnameable table: %#v", got.Ranks)
	}
}

func TestParseDistinguishesAnUnrankedWingmanFromAnAbsentRow(t *testing.T) {
	t.Parallel()
	// An unranked Wingman account really reports 0, so a bare int could not say
	// whether the row was missing.
	got := Parse(page(rankTable(`<tr><td>Wingman</td><td>0</td><td>0</td><td>0</td><td>0</td></tr>`)), testNow)
	if got.Ranks.Wingman != (Rank{Found: true, Value: 0, Wins: 0}) {
		t.Fatalf("Wingman = %#v, want Found with a zero value", got.Ranks.Wingman)
	}
	if got.Ranks.Premier.Found {
		t.Fatalf("Premier = %#v, want not Found", got.Ranks.Premier)
	}
}

func TestParseReportsNoRanksOnAPageWithoutThem(t *testing.T) {
	t.Parallel()
	got := Parse(page(cooldownTable(`<tr><td>2026-08-14 09:30:00</td><td>0</td></tr>`)), testNow)
	if got.Ranks.Any() {
		t.Fatalf("ranks = %#v, want none", got.Ranks)
	}
}

func TestParseIgnoresANonNumericSkillCell(t *testing.T) {
	t.Parallel()
	got := Parse(page(rankTable(`<tr><td>Premier</td><td>42</td><td>1</td><td>17</td><td>Unranked</td></tr>`)), testNow)
	if got.Ranks.Premier.Found {
		t.Fatalf("Premier = %#v, want not Found", got.Ranks.Premier)
	}
}

// livePage mirrors a real ?tab=matchmaking response, captured 2026-08-07.
// Column names, ordering, the " GMT" suffix on every instant and the empty
// Skill Group cells are all exactly as Steam ships them; only the values are
// changed.
const livePage = `<html><head><title>Steam Community</title></head><body>
<h1>Personal Game Data</h1>
<table class="generic_kv_table">
<tr><th>Competitive Cooldown Expiration</th><th>Competitive Cooldown Level</th><th>Acknowledged</th></tr>
<tr><td>2026-08-11 13:38:16 GMT</td><td>&nbsp;</td><td>No</td></tr>
</table>
<table class="generic_kv_table">
<tr><th>Matchmaking Mode</th><th>Wins</th><th>Ties</th><th>Losses</th><th>Skill Group</th><th>Last Match</th><th>Region</th></tr>
<tr><td>Premier</td><td>9</td><td>0</td><td>0</td><td>15,234</td><td>2026-08-04 12:15:36 GMT</td><td>7</td></tr>
<tr><td>Wingman</td><td>1</td><td>0</td><td>0</td><td>&nbsp;</td><td>2025-12-09 05:27:20 GMT</td><td>4</td></tr>
</table>
<table class="generic_kv_table">
<tr><th>Matchmaking Mode</th><th>Map</th><th>Wins</th><th>Ties</th><th>Losses</th><th>Skill Group</th><th>Last Match</th><th>Region</th></tr>
<tr><td>Ranked Competitive</td><td>Dust II</td><td>0</td><td>0</td><td>1</td><td>&nbsp;</td><td>2026-01-24 08:59:47 GMT</td><td>4</td></tr>
<tr><td>Ranked Competitive</td><td>Vertigo</td><td>2</td><td>0</td><td>0</td><td>5</td><td>2026-08-01 14:40:45 GMT</td><td>3</td></tr>
<tr><td>Ranked Competitive</td><td>Mirage</td><td>2</td><td>0</td><td>0</td><td>2</td><td>2026-07-30 14:11:18 GMT</td><td>7</td></tr>
</table>
<table class="generic_kv_table">
<tr><th>Matchmaking Mode</th><th>Last Match</th></tr>
<tr><td>Deathmatch</td><td>2026-07-05 11:35:21 GMT</td></tr>
</table>
</body></html>`

func TestParseReadsALiveCooldownWithItsTimezoneSuffix(t *testing.T) {
	t.Parallel()
	// The bug this locks down: Steam appends " GMT" to every instant, a strict
	// Go parse rejected the whole cell, and a rejected cooldown row is
	// indistinguishable from no cooldown at all - so a real, active cooldown
	// reported as clean.
	got := Parse([]byte(livePage), time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC))
	if got.Outcome != OutcomeParsed {
		t.Fatalf("Parse = %#v", got)
	}
	if !got.HasCooldown || got.Permanent {
		t.Fatalf("live cooldown missed: %#v", got)
	}
	want := time.Date(2026, 8, 11, 13, 38, 16, 0, time.UTC)
	if !got.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v", got.ExpiresAt, want)
	}
}

func TestParseReadsLiveRanksAlongsideTheCooldown(t *testing.T) {
	t.Parallel()
	got := Parse([]byte(livePage), time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC))
	if got.Ranks.Premier != (Rank{Found: true, Value: 15234, Wins: 9}) {
		t.Fatalf("Premier = %#v", got.Ranks.Premier)
	}
	// The per-map table is the source for CompRank: highest skill group wins,
	// and a blank one is skipped rather than read as zero.
	if got.Ranks.Competitive != (Rank{Found: true, Value: 5, Wins: -1}) {
		t.Fatalf("Competitive = %#v", got.Ranks.Competitive)
	}
	// This account has played Wingman but has no skill group yet. That is an
	// absent rank, not rank 0.
	if got.Ranks.Wingman.Found {
		t.Fatalf("Wingman = %#v, want not Found", got.Ranks.Wingman)
	}
}

func TestParseHandlesTheLiveColumnCount(t *testing.T) {
	t.Parallel()
	// The per-map table ships eight columns. maxCellsPerRow must leave room, or
	// the truncation guard silently costs the competitive rank.
	if maxCellsPerRow <= 8 {
		t.Fatalf("maxCellsPerRow = %d leaves no headroom over the live table's 8 columns", maxCellsPerRow)
	}
}

func TestParseDetectsTheLoginPage(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`<html><body><script>g_steamID = false;</script>Personal Game Data</body></html>`,
		`<html><head><title>Sign In</title></head><body>Personal Game Data</body></html>`,
	} {
		if got := Parse([]byte(body), testNow); got.Outcome != OutcomeNotSignedIn {
			t.Fatalf("Parse(%q) = %#v, want OutcomeNotSignedIn", body, got)
		}
	}
}

func TestParseRejectsUnrecognisedBodies(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"empty":          "",
		"plain text":     "hello",
		"unrelated html": `<html><body><table class="other"><tr><td>x</td></tr></table></body></html>`,
	}
	for name, body := range cases {
		got := Parse([]byte(body), testNow)
		if got.Outcome == OutcomeParsed {
			t.Fatalf("%s: Parse = %#v, want not OutcomeParsed", name, got)
		}
	}
}

func TestParseRefusesToClearOnATruncatedResponse(t *testing.T) {
	t.Parallel()
	// net/html recovers from truncation by closing every open element, so a
	// response cut short mid-table yields a cooldown table with no data rows -
	// which looks exactly like a clean account. Clearing on that would delete a
	// real cooldown, so an unclosed document is not trusted at all.
	body := `<html><body><h1>Personal Game Data</h1>` +
		`<table class="generic_kv_table"><tr><th>Cooldown Expiration</th><th>Level</th></tr>` +
		`<tr><td>2026-08-14 09:30:00`
	if got := Parse([]byte(body), testNow); got.Outcome != OutcomeUnrecognised {
		t.Fatalf("Parse = %#v, want OutcomeUnrecognised", got)
	}
}

func TestParseRefusesToClearOnANonEnglishRender(t *testing.T) {
	t.Parallel()
	// The whole point of the English-marker guard: cooldown detection matches an
	// English header, so a localised page finds no cooldown table. Reporting
	// that as "parsed, no cooldown" would silently delete a real cooldown.
	body := []byte(`<html><body><table class="generic_kv_table">` +
		`<tr><th>Abklingzeit-Ablauf</th><th>Stufe</th></tr>` +
		`<tr><td>2026-08-14 09:30:00</td><td>0</td></tr></table></body></html>`)
	if got := Parse(body, testNow); got.Outcome != OutcomeUnrecognised {
		t.Fatalf("Parse = %#v, want OutcomeUnrecognised", got)
	}
}

func TestParseRejectsAnOversizedBody(t *testing.T) {
	t.Parallel()
	body := make([]byte, maxBodyBytes+1)
	copy(body, page(cooldownTable(`<tr><td>2026-08-14 09:30:00</td><td>0</td></tr>`)))
	if got := Parse(body, testNow); got.Outcome != OutcomeUnrecognised {
		t.Fatalf("Parse = %#v, want OutcomeUnrecognised", got)
	}
}

func TestParseRejectsAbsurdlyDistantExpiries(t *testing.T) {
	t.Parallel()
	got := Parse(page(cooldownTable(`<tr><td>9999-01-01 00:00:00</td><td>0</td></tr>`)), testNow)
	if got.Outcome != OutcomeParsed || got.HasCooldown {
		t.Fatalf("Parse = %#v", got)
	}
}

func TestParseAcceptsALeapDay(t *testing.T) {
	t.Parallel()
	now := time.Date(2028, 2, 1, 0, 0, 0, 0, time.UTC)
	got := Parse(page(cooldownTable(`<tr><td>2028-02-29 00:00:00</td><td>0</td></tr>`)), now)
	if !got.HasCooldown || got.ExpiresAt.Day() != 29 {
		t.Fatalf("Parse = %#v", got)
	}
}

func TestParseHandlesMarkupNoiseInsideCells(t *testing.T) {
	t.Parallel()
	rows := `<tr><td><span> 2026-08-14 09:30:00 </span><!-- c --></td><td><b>0</b></td></tr>`
	got := Parse(page(cooldownTable(rows)), testNow)
	if !got.HasCooldown {
		t.Fatalf("Parse = %#v", got)
	}
}

func TestParseStaysBoundedOnAHugeTable(t *testing.T) {
	t.Parallel()
	var builder strings.Builder
	for i := 0; i < 10_000; i++ {
		builder.WriteString(`<tr><td>2026-01-01 00:00:00</td><td>0</td></tr>`)
	}
	got := Parse(page(cooldownTable(builder.String())), testNow)
	if got.Outcome != OutcomeParsed {
		t.Fatalf("Parse = %#v", got)
	}
}
