package serverpicker

import "strings"

// keywordFilter decides which POPs of an app's SDR config belong to a game
// entry. CS2 and CS2 (Perfect World) share appid 730 but expose disjoint POP
// sets, and "China" in the POP description is the only thing separating them.
type keywordFilter string

const (
	filterNone    keywordFilter = ""
	filterInclude keywordFilter = "include"
	filterExclude keywordFilter = "exclude"
)

// Game is one entry of the game dropdown.
type Game struct {
	ID       string
	Name     string
	AppID    int
	Filter   keywordFilter
	Keywords []string
}

// GameDTO is the UI-facing form of [Game].
type GameDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	AppID int    `json:"appId"`
}

var games = []Game{
	{ID: "cs2", Name: "Counter-Strike 2", AppID: 730, Filter: filterExclude, Keywords: []string{"China"}},
	{ID: "cs2_perfect_world", Name: "Counter-Strike 2 (Perfect World)", AppID: 730, Filter: filterInclude, Keywords: []string{"China"}},
	{ID: "deadlock", Name: "Deadlock", AppID: 1422450},
	{ID: "marathon", Name: "Marathon", AppID: 3065800},
	{ID: "the_finals", Name: "THE FINALS", AppID: 2073850},
}

func gameByID(id string) (Game, bool) {
	id = strings.TrimSpace(strings.ToLower(id))
	for _, g := range games {
		if g.ID == id {
			return g, true
		}
	}
	return Game{}, false
}

// accepts reports whether a POP description passes the game's keyword filter.
func (g Game) accepts(description string) bool {
	switch g.Filter {
	case filterInclude:
		return containsAny(description, g.Keywords)
	case filterExclude:
		return !containsAny(description, g.Keywords)
	default:
		return true
	}
}

func containsAny(s string, keywords []string) bool {
	for _, k := range keywords {
		if k != "" && strings.Contains(s, k) {
			return true
		}
	}
	return false
}
