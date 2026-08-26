package controllerinput

import (
	"context"
	"testing"
)

type stubReader struct{}

func (stubReader) Snapshots() []snapshot { return nil }

func newTestService(t *testing.T) *Service {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	s := NewService()
	s.baseCtx = ctx
	s.reader = stubReader{}
	s.emit = func(Action) {}
	t.Cleanup(s.stop)
	return s
}

func (s *Service) hasLoop() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cancel != nil
}

func TestPollingWaitsForTheWindowToBeVisible(t *testing.T) {
	s := newTestService(t)

	s.SetWindowVisible(false)
	s.SetEnabled(true)
	if s.hasLoop() {
		t.Fatal("enabling controller support behind a hidden window must not start the poll")
	}

	s.SetWindowVisible(true)
	if !s.hasLoop() {
		t.Fatal("showing the window must start the poll")
	}
}

func TestHidingTheWindowStopsPolling(t *testing.T) {
	s := newTestService(t)
	s.SetEnabled(true)
	if !s.hasLoop() {
		t.Fatal("expected the poll to start for an enabled, visible window")
	}

	s.SetWindowVisible(false)
	if s.hasLoop() {
		t.Fatal("hiding the window must stop the poll")
	}

	s.SetWindowVisible(true)
	if !s.hasLoop() {
		t.Fatal("restoring the window must resume the poll")
	}
}

func TestLosingFocusStopsPolling(t *testing.T) {
	s := newTestService(t)
	s.SetEnabled(true)

	s.SetWindowFocused(false)
	if s.hasLoop() {
		t.Fatal("a window sitting behind another application must not poll")
	}

	s.SetWindowFocused(true)
	if !s.hasLoop() {
		t.Fatal("coming back to the front must resume the poll")
	}
}

func TestFocusDoesNotPollAHiddenWindow(t *testing.T) {
	s := newTestService(t)
	s.SetEnabled(true)
	s.SetWindowVisible(false)

	s.SetWindowFocused(false)
	s.SetWindowFocused(true)
	if s.hasLoop() {
		t.Fatal("focus must not override a hidden window")
	}
}

func TestShowingTheWindowDoesNotPollWhileControllerSupportIsOff(t *testing.T) {
	s := newTestService(t)
	s.SetEnabled(false)

	s.SetWindowVisible(false)
	s.SetWindowVisible(true)
	if s.hasLoop() {
		t.Fatal("window visibility must not override the controller support setting")
	}
}
