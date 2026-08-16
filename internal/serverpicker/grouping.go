package serverpicker

import (
	"sort"
	"strings"
)

// sameLocationKm is how close two POPs must be to count as one place. Stockholm
// Kista and Bromma sit ~7 km apart and Steam will happily route you through
// either, so blocking one alone achieves nothing; Mumbai and Chennai are ~1000
// km apart and genuinely differ in ping, so they stay separate rows.
const sameLocationKm = 50

// Group is one row of the table: a place, and every POP Steam runs there.
type Group struct {
	ID      string // first member POP id; stable across restarts
	Label   string
	Country string // ISO 3166-1 alpha-2, lowercase; "" when unknown
	Name    string // country display name, searchable
	Region  string
	Members []POP
}

// RelayIPs is every relay address the group covers, which is exactly what one
// firewall rule has to block for the group's checkbox to mean anything.
func (g Group) RelayIPs() []string {
	var ips []string
	for _, m := range g.Members {
		ips = append(ips, m.Relay...)
	}
	return ips
}

// MemberIDs lists the POP ids behind the group, in display order.
func (g Group) MemberIDs() []string {
	ids := make([]string, 0, len(g.Members))
	for _, m := range g.Members {
		ids = append(ids, m.ID)
	}
	return ids
}

// buildGroups turns a game's POPs into table rows. POPs merge when they are the
// same country within [sameLocationKm] of each other, so the ISP splits Valve
// publishes in China (Mobile/Telecom/Unicom at identical coordinates) and the
// two Stockholm datacentres collapse to one checkbox.
func buildGroups(cfg SDRConfig, game Game) []Group {
	var groups []Group
	for _, pop := range cfg.POPs {
		if !game.accepts(pop.Desc) {
			continue
		}
		c := resolveCountry(pop)
		idx := -1
		for i := range groups {
			if groups[i].Country != c.Code || groups[i].Region != c.Region {
				continue
			}
			head := groups[i].Members[0]
			if haversineKm(head.Lon, head.Lat, pop.Lon, pop.Lat) <= sameLocationKm {
				idx = i
				break
			}
		}
		if idx == -1 {
			groups = append(groups, Group{
				Country: c.Code,
				Name:    c.Name,
				Region:  c.Region,
				Members: []POP{pop},
			})
			continue
		}
		groups[idx].Members = append(groups[idx].Members, pop)
	}

	for i := range groups {
		sort.Slice(groups[i].Members, func(a, b int) bool {
			return groups[i].Members[a].ID < groups[i].Members[b].ID
		})
		groups[i].ID = groups[i].Members[0].ID
		groups[i].Label = groupLabel(groups[i].Members)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Label != groups[j].Label {
			return groups[i].Label < groups[j].Label
		}
		return groups[i].ID < groups[j].ID
	})
	return groups
}

// groupLabel names a group from what its members have in common. Steam
// descriptions of co-located POPs share a prefix and differ only in the suffix
// naming the carrier or building ("Stockholm - Kista (Sweden)" /
// "Stockholm - Bromma (Sweden)"), so the shared prefix plus the country reads
// as the place itself: "Stockholm (Sweden)".
func groupLabel(members []POP) string {
	if len(members) == 0 {
		return ""
	}
	if len(members) == 1 {
		return members[0].Desc
	}

	prefix := strings.Fields(members[0].Desc)
	for _, m := range members[1:] {
		words := strings.Fields(m.Desc)
		n := len(prefix)
		if len(words) < n {
			n = len(words)
		}
		i := 0
		for i < n && strings.EqualFold(prefix[i], words[i]) {
			i++
		}
		prefix = prefix[:i]
	}
	// Trailing separators are an artefact of the split, not part of the name.
	for len(prefix) > 0 {
		last := strings.Trim(prefix[len(prefix)-1], "-–,:")
		if last != "" {
			prefix[len(prefix)-1] = last
			break
		}
		prefix = prefix[:len(prefix)-1]
	}

	label := strings.Join(prefix, " ")
	if label == "" {
		return members[0].Desc
	}
	if suffix := parenSuffix(members[0].Desc); suffix != "" && !strings.HasSuffix(label, suffix) {
		label += " " + suffix
	}
	return label
}

// parenSuffix returns the trailing "(Country)" of a description, if any.
func parenSuffix(desc string) string {
	desc = strings.TrimSpace(desc)
	if !strings.HasSuffix(desc, ")") {
		return ""
	}
	open := strings.LastIndex(desc, "(")
	if open == -1 {
		return ""
	}
	return desc[open:]
}
