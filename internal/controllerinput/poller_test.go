package controllerinput

import (
	"testing"
	"time"
)

func TestAdvancePollStateMapsButtonsAndStick(t *testing.T) {
	state := newPollState()
	now := time.Unix(100, 0)
	pads := []snapshot{{
		Connected: true,
		Buttons:   buttonA | buttonB | buttonY | buttonStart | buttonLB | buttonRB | buttonDpadUp,
		ThumbLX:   -stickThreshold,
	}}

	next, actions := advancePollState(state, pads, now)

	assertActions(t, actions,
		ActionUp,
		ActionLeft,
		ActionActivate,
		ActionBack,
		ActionPalette,
		ActionContext,
		ActionHistoryBack,
		ActionHistoryForward,
	)
	if !next.held[ActionUp] || !next.held[ActionLeft] || !next.held[ActionActivate] {
		t.Fatalf("expected held state to track active inputs: %#v", next.held)
	}
	if next.nextRepeatAt[ActionUp] != now.Add(initialRepeatDelay) {
		t.Fatalf("expected repeat to arm for up: got %v want %v", next.nextRepeatAt[ActionUp], now.Add(initialRepeatDelay))
	}
}

func TestAdvancePollStateMapsXInputStickYDirection(t *testing.T) {
	now := time.Unix(150, 0)

	_, upActions := advancePollState(newPollState(), []snapshot{{
		Connected: true,
		ThumbLY:   stickThreshold,
	}}, now)
	assertActions(t, upActions, ActionUp)

	_, downActions := advancePollState(newPollState(), []snapshot{{
		Connected: true,
		ThumbLY:   -stickThreshold,
	}}, now)
	assertActions(t, downActions, ActionDown)
}

func TestAdvancePollStateRepeatsDirectionsOnly(t *testing.T) {
	now := time.Unix(200, 0)
	state, first := advancePollState(newPollState(), []snapshot{{
		Connected: true,
		Buttons:   buttonDpadRight | buttonA,
	}}, now)
	assertActions(t, first, ActionRight, ActionActivate)

	state, second := advancePollState(state, []snapshot{{
		Connected: true,
		Buttons:   buttonDpadRight | buttonA,
	}}, now.Add(initialRepeatDelay-time.Millisecond))
	if len(second) != 0 {
		t.Fatalf("expected no repeat before deadline, got %v", second)
	}

	state, third := advancePollState(state, []snapshot{{
		Connected: true,
		Buttons:   buttonDpadRight | buttonA,
	}}, now.Add(initialRepeatDelay))
	assertActions(t, third, ActionRight)

	_, fourth := advancePollState(state, []snapshot{{
		Connected: true,
		Buttons:   buttonDpadRight | buttonA,
	}}, now.Add(initialRepeatDelay+repeatDelay))
	assertActions(t, fourth, ActionRight)
}

func TestAdvancePollStateClearsRepeatAfterRelease(t *testing.T) {
	now := time.Unix(300, 0)
	state, _ := advancePollState(newPollState(), []snapshot{{
		Connected: true,
		Buttons:   buttonDpadDown,
	}}, now)

	state, actions := advancePollState(state, nil, now.Add(50*time.Millisecond))
	if len(actions) != 0 {
		t.Fatalf("expected no actions on release, got %v", actions)
	}
	if !state.nextRepeatAt[ActionDown].IsZero() {
		t.Fatalf("expected repeat deadline to clear on release, got %v", state.nextRepeatAt[ActionDown])
	}

	_, actions = advancePollState(state, []snapshot{{
		Connected: true,
		Buttons:   buttonDpadDown,
	}}, now.Add(60*time.Millisecond))
	assertActions(t, actions, ActionDown)
}

func TestAdvancePollStateCombinesMultipleControllers(t *testing.T) {
	state := newPollState()
	now := time.Unix(400, 0)
	pads := []snapshot{
		{Connected: true, Buttons: buttonLB},
		{Connected: true, Buttons: buttonRB | buttonStart},
	}

	_, actions := advancePollState(state, pads, now)
	assertActions(t, actions, ActionPalette, ActionHistoryBack, ActionHistoryForward)
}

func assertActions(t *testing.T, got []Action, want ...Action) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected action count: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected action at %d: got %q want %q", i, got[i], want[i])
		}
	}
}
