package qrcapture

import "sort"

func intersect(a, b Rect) (Rect, bool) {
	r := Rect{
		Left:   max32(a.Left, b.Left),
		Top:    max32(a.Top, b.Top),
		Right:  min32(a.Right, b.Right),
		Bottom: min32(a.Bottom, b.Bottom),
	}
	return r, r.Valid()
}

func visibleRegions(bounds Rect, monitors []Rect) []Rect {
	if !bounds.Valid() {
		return nil
	}
	seen := make(map[Rect]struct{}, len(monitors))
	regions := make([]Rect, 0, len(monitors))
	for _, monitor := range monitors {
		region, ok := intersect(bounds, monitor)
		if !ok {
			continue
		}
		if _, exists := seen[region]; exists {
			continue
		}
		seen[region] = struct{}{}
		regions = append(regions, region)
	}
	sort.Slice(regions, func(i, j int) bool {
		leftArea := int64(regions[i].Width()) * int64(regions[i].Height())
		rightArea := int64(regions[j].Width()) * int64(regions[j].Height())
		if leftArea != rightArea {
			return leftArea > rightArea
		}
		if regions[i].Top != regions[j].Top {
			return regions[i].Top < regions[j].Top
		}
		return regions[i].Left < regions[j].Left
	})
	return regions
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
