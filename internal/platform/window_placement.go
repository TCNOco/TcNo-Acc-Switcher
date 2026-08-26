package platform

import "errors"

// ErrNoSettingsService is returned when a placement write is asked for without a
// service to serialise it against the rest of the settings file.
var ErrNoSettingsService = errors.New("platform: no settings service")

// WindowPlacement is where the main window was left: the bounds it occupies when
// it is not maximised, plus whether it was maximised over them. The restored
// bounds are kept even while maximised so un-maximising lands back on the size
// the user picked.
type WindowPlacement struct {
	Width     int
	Height    int
	X         int
	Y         int
	Maximised bool
}

// HasSize reports whether the placement carries usable bounds. X and Y are only
// read alongside a size, because zero is a real coordinate and cannot itself
// distinguish "top-left" from "never recorded".
func (p WindowPlacement) HasSize() bool {
	return p.Width > 0 && p.Height > 0
}

// SavedWindowPlacement reads the placement out of app settings.
func SavedWindowPlacement(s AppSettings) WindowPlacement {
	return WindowPlacement{
		Width:     s.WindowWidth,
		Height:    s.WindowHeight,
		X:         s.WindowX,
		Y:         s.WindowY,
		Maximised: s.WindowMaximised,
	}
}

// SaveWindowPlacement records where the main window was left. It goes through
// the service's settings lock so a drag that ends while the user is toggling a
// setting cannot drop one of the two writes.
func SaveWindowPlacement(svc *PlatformService, p WindowPlacement) error {
	if svc == nil {
		return ErrNoSettingsService
	}
	return svc.withSettingsWrite(func(s *AppSettings) error {
		s.WindowWidth = p.Width
		s.WindowHeight = p.Height
		s.WindowX = p.X
		s.WindowY = p.Y
		s.WindowMaximised = p.Maximised
		return nil
	})
}
