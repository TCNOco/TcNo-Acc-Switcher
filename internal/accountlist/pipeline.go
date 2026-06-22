package accountlist

import (
	"sort"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
)

func Merge[ListRow any, EnrichRow any, Out any](
	list []ListRow,
	enrich []EnrichRow,
	listID func(ListRow) string,
	enrichID func(EnrichRow) string,
	merge func(ListRow, EnrichRow) Out,
) []Out {
	enrichByID := make(map[string]EnrichRow, len(enrich))
	for _, row := range enrich {
		enrichByID[enrichID(row)] = row
	}
	out := make([]Out, 0, len(list))
	for _, row := range list {
		out = append(out, merge(row, enrichByID[listID(row)]))
	}
	return out
}

func OrderedIDs(ids map[string]string, savedOrder []string) []string {
	seen := map[string]struct{}{}
	keys := make([]string, 0, len(ids))
	for _, id := range savedOrder {
		if _, ok := ids[id]; ok {
			keys = append(keys, id)
			seen[id] = struct{}{}
		}
	}

	missing := make([]string, 0, len(ids)-len(seen))
	for id := range ids {
		if _, ok := seen[id]; !ok {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	return append(keys, missing...)
}

func ShortcutCounts(entries []platform.GameShortcutEntry) (total int, pinned int) {
	for _, entry := range entries {
		if strings.TrimSpace(entry.FileName) == "" {
			continue
		}
		total++
		if entry.Pinned {
			pinned++
		}
	}
	return total, pinned
}
