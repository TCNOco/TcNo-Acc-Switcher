package controllerinput

const EventName = "controller:action"

type Action string

const (
	ActionUp             Action = "up"
	ActionDown           Action = "down"
	ActionLeft           Action = "left"
	ActionRight          Action = "right"
	ActionActivate       Action = "activate"
	ActionBack           Action = "back"
	ActionPalette        Action = "palette"
	ActionContext        Action = "context"
	ActionHistoryBack    Action = "historyBack"
	ActionHistoryForward Action = "historyForward"
)
