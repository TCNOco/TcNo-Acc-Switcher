package controllerinput

import "time"

// maxControllers is XInput's fixed slot count; slots are addressed by index.
const maxControllers = 4

// emptySlotProbeInterval is how long a slot that came back empty is left alone
// before it is probed again: long enough that four dead slots cost nothing,
// short enough that plugging a controller in is still noticed promptly.
const emptySlotProbeInterval = time.Second

const emptySlotSkipTicks = int(emptySlotProbeInterval / pollInterval)

// slotBackoff spaces out probes of XInput slots that came back empty.
//
// XInputGetState on a slot with no controller in it is far more expensive than
// on a connected one — it goes looking for a device instead of reading one — so
// probing all four slots at the poll rate burns measurable CPU on a machine with
// no controller attached at all, which is the common case. Connected slots keep
// the full poll rate.
type slotBackoff struct {
	skip [maxControllers]int
}

// due reports whether the slot should be probed on this tick, consuming a tick
// of its remaining wait when it should not.
func (b *slotBackoff) due(index int) bool {
	if index < 0 || index >= len(b.skip) {
		return true
	}
	if b.skip[index] > 0 {
		b.skip[index]--
		return false
	}
	return true
}

// record sets the slot's next probe from what the last one found.
func (b *slotBackoff) record(index int, connected bool) {
	if index < 0 || index >= len(b.skip) {
		return
	}
	if connected {
		b.skip[index] = 0
		return
	}
	b.skip[index] = emptySlotSkipTicks
}
