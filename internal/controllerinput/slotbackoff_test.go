package controllerinput

import "testing"

func TestSlotBackoffProbesAConnectedSlotEveryTick(t *testing.T) {
	var b slotBackoff
	for tick := 0; tick < emptySlotSkipTicks+2; tick++ {
		if !b.due(0) {
			t.Fatalf("tick %d: a connected slot must be probed every tick", tick)
		}
		b.record(0, true)
	}
}

func TestSlotBackoffSkipsAnEmptySlotForTheWholeInterval(t *testing.T) {
	var b slotBackoff
	if !b.due(1) {
		t.Fatal("a slot that has never been probed must be probed")
	}
	b.record(1, false)

	for tick := 0; tick < emptySlotSkipTicks; tick++ {
		if b.due(1) {
			t.Fatalf("tick %d: an empty slot must stay skipped for %d ticks", tick, emptySlotSkipTicks)
		}
	}
	if !b.due(1) {
		t.Fatalf("an empty slot must be re-probed after %d ticks", emptySlotSkipTicks)
	}
}

func TestSlotBackoffReturnsToFullRateOnHotPlug(t *testing.T) {
	var b slotBackoff
	b.record(2, false)
	for tick := 0; tick < emptySlotSkipTicks; tick++ {
		b.due(2)
	}
	if !b.due(2) {
		t.Fatal("the re-probe that finds the new controller must happen")
	}

	b.record(2, true)
	if !b.due(2) {
		t.Fatal("a slot that just filled must go back to every tick")
	}
}

func TestSlotBackoffTracksSlotsIndependently(t *testing.T) {
	var b slotBackoff
	b.record(0, true)
	b.record(3, false)

	if !b.due(0) {
		t.Fatal("a connected slot must not inherit an empty slot's backoff")
	}
	if b.due(3) {
		t.Fatal("an empty slot must not be re-probed because another slot is connected")
	}
}
