//go:build windows

package qrregion

import "testing"

func TestSelectionRectUsesPhysicalVirtualScreenCoordinates(t *testing.T) {
	state := &overlayState{
		virtualScreen: Rect{Left: -1920, Top: -200, Right: 0, Bottom: 880},
		start:         nativePoint{X: 700, Y: 500},
		current:       nativePoint{X: 100, Y: 50},
	}
	want := Rect{Left: -1820, Top: -150, Right: -1220, Bottom: 300}
	if got := selectionRect(state); got != want {
		t.Fatalf("selection = %#v, want %#v", got, want)
	}
}

func TestClientPointClampsToOverlayBounds(t *testing.T) {
	bounds := Rect{Left: 100, Top: 200, Right: 2020, Bottom: 1280}
	negative := uintptr(uint16(0xffff)) | uintptr(uint32(uint16(0xffff)))<<16
	if got := clientPoint(negative, bounds); got != (nativePoint{}) {
		t.Fatalf("negative point = %#v", got)
	}
	outOfBounds := uintptr(uint16(2500)) | uintptr(uint32(uint16(1500)))<<16
	want := nativePoint{X: 1920, Y: 1080}
	if got := clientPoint(outOfBounds, bounds); got != want {
		t.Fatalf("out-of-bounds point = %#v, want %#v", got, want)
	}
}

func TestSelectionRectSpansMonitorsFromNegativeOrigin(t *testing.T) {
	// Virtual desktop: 1920x1080 secondary left of a 2560x1440 primary.
	virtualScreen := Rect{Left: -1920, Top: 0, Right: 2560, Bottom: 1440}
	state := &overlayState{
		virtualScreen: virtualScreen,
		start:         nativePoint{X: 200, Y: 300},   // secondary monitor
		current:       nativePoint{X: 2400, Y: 1000}, // primary monitor
	}
	want := Rect{Left: -1720, Top: 300, Right: 480, Bottom: 1000}
	if got := selectionRect(state); got != want {
		t.Fatalf("selection = %#v, want %#v", got, want)
	}
}

func TestInstructionBandCentersOnCursorMonitor(t *testing.T) {
	virtualScreen := Rect{Left: -1920, Top: -120, Right: 2560, Bottom: 1440}
	monitor := Rect{Left: -1920, Top: -120, Right: 0, Bottom: 960}
	band := instructionBandRect(virtualScreen, monitor)
	// Monitor is 1920 wide, so the 900px band is centred at client x=960.
	want := nativeRect{Left: 510, Top: instructionBandTop, Right: 1410, Bottom: instructionBandTop + instructionBandHeight}
	if band != want {
		t.Fatalf("band = %#v, want %#v", band, want)
	}
}

func TestDragDamageCoversBothRectanglesAndTheirBorders(t *testing.T) {
	// Dragging up and left: the damage has to include where the rectangle was, or
	// the old edges stay painted on screen.
	previous := nativeRect{Left: 400, Top: 300, Right: 900, Bottom: 700}
	current := nativeRect{Left: 250, Top: 300, Right: 400, Bottom: 520}
	edge := int32(selectionBorderWidth + 1)
	want := nativeRect{Left: 250 - edge, Top: 300 - edge, Right: 900 + edge, Bottom: 700 + edge}
	if got := dragDamageRect(previous, current); got != want {
		t.Fatalf("damage = %#v, want %#v", got, want)
	}
}

func TestDragDamageStaysLocalToTheSelection(t *testing.T) {
	// The point of the narrowed repaint: a small rectangle on a large desktop
	// must not damage the whole virtual screen.
	previous := nativeRect{Left: 1000, Top: 600, Right: 1100, Bottom: 700}
	current := nativeRect{Left: 1000, Top: 600, Right: 1104, Bottom: 700}
	damage := dragDamageRect(previous, current)
	if width := damage.Right - damage.Left; width > 120 {
		t.Fatalf("damage width = %d, want it bounded by the selection", width)
	}
	if height := damage.Bottom - damage.Top; height > 120 {
		t.Fatalf("damage height = %d, want it bounded by the selection", height)
	}
}

func TestInstructionBandClampsToNarrowMonitor(t *testing.T) {
	virtualScreen := Rect{Left: 0, Top: 0, Right: 800, Bottom: 600}
	band := instructionBandRect(virtualScreen, virtualScreen)
	if band.Right-band.Left != 800 {
		t.Fatalf("band width = %d, want 800", band.Right-band.Left)
	}
	if band.Left != 0 {
		t.Fatalf("band left = %d, want 0", band.Left)
	}
}
