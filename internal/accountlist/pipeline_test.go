package accountlist

import (
	"reflect"
	"testing"

	"TcNo-Acc-Switcher/internal/platform"
)

func TestOrderedIDsKeepsSavedOrderAndSortsMissing(t *testing.T) {
	got := OrderedIDs(
		map[string]string{"z": "Zed", "a": "Ada", "m": "Mia"},
		[]string{"missing", "m", "z", "m"},
	)
	want := []string{"m", "z", "m", "a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("OrderedIDs() = %#v, want %#v", got, want)
	}
}

func TestMergeUsesZeroEnrichmentForMissingRows(t *testing.T) {
	type listRow struct {
		id   string
		name string
	}
	type enrichRow struct {
		id   string
		note string
	}
	type outRow struct {
		id   string
		name string
		note string
	}

	got := Merge(
		[]listRow{{id: "a", name: "Ada"}, {id: "b", name: "Ben"}},
		[]enrichRow{{id: "b", note: "has note"}},
		func(row listRow) string { return row.id },
		func(row enrichRow) string { return row.id },
		func(list listRow, enrich enrichRow) outRow {
			return outRow{id: list.id, name: list.name, note: enrich.note}
		},
	)
	want := []outRow{{id: "a", name: "Ada"}, {id: "b", name: "Ben", note: "has note"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Merge() = %#v, want %#v", got, want)
	}
}

func TestShortcutCountsIgnoresBlankNames(t *testing.T) {
	total, pinned := ShortcutCounts([]platform.GameShortcutEntry{
		{FileName: "one.lnk", Pinned: true},
		{FileName: "  "},
		{FileName: "two.url"},
	})
	if total != 2 || pinned != 1 {
		t.Fatalf("ShortcutCounts() = (%d, %d), want (2, 1)", total, pinned)
	}
}
