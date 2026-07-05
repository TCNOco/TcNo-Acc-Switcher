package controllerinput

import "time"

const (
	maxThumbValue = 32767

	initialRepeatDelay = 280 * time.Millisecond
	repeatDelay        = 120 * time.Millisecond
)

const (
	buttonDpadUp    = 0x0001
	buttonDpadDown  = 0x0002
	buttonDpadLeft  = 0x0004
	buttonDpadRight = 0x0008
	buttonBack      = 0x0020
	buttonStart     = 0x0010
	buttonLB        = 0x0100
	buttonRB        = 0x0200
	buttonA         = 0x1000
	buttonB         = 0x2000
	buttonY         = 0x8000
)

var (
	allActions = [...]Action{
		ActionUp,
		ActionDown,
		ActionLeft,
		ActionRight,
		ActionActivate,
		ActionBack,
		ActionPalette,
		ActionContext,
		ActionHistoryBack,
		ActionHistoryForward,
	}
	repeatableActions = [...]Action{
		ActionUp,
		ActionDown,
		ActionLeft,
		ActionRight,
	}
	stickThreshold = int16(maxThumbValue * 45 / 100)
)

type snapshot struct {
	Connected bool
	Buttons   uint16
	ThumbLX   int16
	ThumbLY   int16
}

type pollState struct {
	held         map[Action]bool
	nextRepeatAt map[Action]time.Time
}

func newPollState() pollState {
	held := make(map[Action]bool, len(allActions))
	nextRepeatAt := make(map[Action]time.Time, len(repeatableActions))
	for _, action := range allActions {
		held[action] = false
	}
	for _, action := range repeatableActions {
		nextRepeatAt[action] = time.Time{}
	}
	return pollState{
		held:         held,
		nextRepeatAt: nextRepeatAt,
	}
}

func advancePollState(state pollState, pads []snapshot, now time.Time) (pollState, []Action) {
	signals := readSignals(pads)
	next := pollState{
		held:         make(map[Action]bool, len(state.held)),
		nextRepeatAt: make(map[Action]time.Time, len(state.nextRepeatAt)),
	}
	for action, held := range state.held {
		next.held[action] = held
	}
	for action, at := range state.nextRepeatAt {
		next.nextRepeatAt[action] = at
	}

	actions := make([]Action, 0, 4)
	for _, action := range allActions {
		active := signals[action]
		wasHeld := state.held[action]
		next.held[action] = active

		if !active {
			if isRepeatable(action) {
				next.nextRepeatAt[action] = time.Time{}
			}
			continue
		}

		if !wasHeld {
			actions = append(actions, action)
			if isRepeatable(action) {
				next.nextRepeatAt[action] = now.Add(initialRepeatDelay)
			}
			continue
		}

		if isRepeatable(action) && !state.nextRepeatAt[action].IsZero() && !now.Before(state.nextRepeatAt[action]) {
			actions = append(actions, action)
			next.nextRepeatAt[action] = now.Add(repeatDelay)
		}
	}

	return next, actions
}

func readSignals(pads []snapshot) map[Action]bool {
	signals := make(map[Action]bool, len(allActions))
	for _, action := range allActions {
		signals[action] = false
	}
	for _, pad := range pads {
		if !pad.Connected {
			continue
		}
		signals[ActionActivate] = signals[ActionActivate] || hasButton(pad.Buttons, buttonA)
		signals[ActionBack] = signals[ActionBack] || hasButton(pad.Buttons, buttonB)
		signals[ActionContext] = signals[ActionContext] || hasButton(pad.Buttons, buttonY) || hasButton(pad.Buttons, buttonBack)
		signals[ActionPalette] = signals[ActionPalette] || hasButton(pad.Buttons, buttonStart)
		signals[ActionHistoryBack] = signals[ActionHistoryBack] || hasButton(pad.Buttons, buttonLB)
		signals[ActionHistoryForward] = signals[ActionHistoryForward] || hasButton(pad.Buttons, buttonRB)
		signals[ActionUp] = signals[ActionUp] || hasButton(pad.Buttons, buttonDpadUp) || pad.ThumbLY >= stickThreshold
		signals[ActionDown] = signals[ActionDown] || hasButton(pad.Buttons, buttonDpadDown) || pad.ThumbLY <= -stickThreshold
		signals[ActionLeft] = signals[ActionLeft] || hasButton(pad.Buttons, buttonDpadLeft) || pad.ThumbLX <= -stickThreshold
		signals[ActionRight] = signals[ActionRight] || hasButton(pad.Buttons, buttonDpadRight) || pad.ThumbLX >= stickThreshold
	}
	return signals
}

func hasButton(buttons uint16, mask uint16) bool {
	return buttons&mask != 0
}

func isRepeatable(action Action) bool {
	switch action {
	case ActionUp, ActionDown, ActionLeft, ActionRight:
		return true
	default:
		return false
	}
}
