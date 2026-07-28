package app

import "testing"

func TestSecurityLifecycleRegistrationIsIdempotent(t *testing.T) {
	locks := make(map[securityLifecycleTrigger]int)
	lifecycle := newSecurityLifecycle(func() error { return nil })
	registerCalls := 0
	register := func(trigger securityLifecycleTrigger, handler func()) {
		registerCalls++
		handler()
		locks[trigger]++
	}

	lifecycle.register(register)
	lifecycle.register(register)

	if registerCalls != len(portableSecurityTriggers) {
		t.Fatalf("registered %d hooks, want %d", registerCalls, len(portableSecurityTriggers))
	}
	for _, trigger := range portableSecurityTriggers {
		if locks[trigger] != 1 {
			t.Fatalf("trigger %q invoked %d times, want 1", trigger, locks[trigger])
		}
	}
}

func TestSecurityLifecycleTriggerLocks(t *testing.T) {
	lockCalls := 0
	lifecycle := newSecurityLifecycle(func() error {
		lockCalls++
		return nil
	})

	for _, trigger := range portableSecurityTriggers {
		lifecycle.handle(trigger)
	}
	if lockCalls != len(portableSecurityTriggers) {
		t.Fatalf("lock called %d times, want %d", lockCalls, len(portableSecurityTriggers))
	}
}
