package foreground

import "testing"

func TestCoversMonitor(t *testing.T) {
	mon := Rect{Left: 0, Top: 0, Right: 2560, Bottom: 1440}

	cases := []struct {
		name string
		win  Rect
		want bool
	}{
		{"exclusive fullscreen matches the monitor exactly", mon, true},
		{
			"borderless fullscreen sized to the monitor",
			Rect{Left: 0, Top: 0, Right: 2560, Bottom: 1440},
			true,
		},
		{
			// Some games sit a pixel outside the monitor; nothing shows through.
			"window spilling past every edge",
			Rect{Left: -1, Top: -1, Right: 2561, Bottom: 1441},
			true,
		},
		{
			// The taskbar strip is the usual reason a window is not quite tall enough.
			"maximised window stopping above the taskbar",
			Rect{Left: 0, Top: 0, Right: 2560, Bottom: 1392},
			false,
		},
		{
			"ordinary window",
			Rect{Left: 300, Top: 200, Right: 1500, Bottom: 1000},
			false,
		},
		{
			"minimised window reports an empty rect",
			Rect{Left: -32000, Top: -32000, Right: -31840, Bottom: -31972},
			false,
		},
		{
			"fullscreen on a second monitor to the right",
			Rect{Left: 2560, Top: 0, Right: 5120, Bottom: 1440},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CoversMonitor(tc.win, mon); got != tc.want {
				t.Fatalf("CoversMonitor(%+v, %+v) = %v, want %v", tc.win, mon, got, tc.want)
			}
		})
	}
}

func TestCoversMonitorSecondMonitorCoordinates(t *testing.T) {
	// A monitor to the right of the primary has a non-zero origin, so a window
	// covering it does not start at 0,0.
	mon := Rect{Left: 2560, Top: 0, Right: 5120, Bottom: 1440}
	win := Rect{Left: 2560, Top: 0, Right: 5120, Bottom: 1440}
	if !CoversMonitor(win, mon) {
		t.Fatal("fullscreen on a secondary monitor should count as covering it")
	}
}

func TestLooksFullscreen(t *testing.T) {
	mon := Rect{Left: 3440, Top: 0, Right: 5360, Bottom: 1080}
	const borderless = uint32(0)
	const ordinary = uint32(WSCaption | WSThickFrame)

	cases := []struct {
		name  string
		style uint32
		win   Rect
		want  bool
	}{
		{
			"borderless window sized to the monitor is a game",
			borderless,
			mon,
			true,
		},
		{
			// The case that caught this: a maximised browser on a second monitor.
			// Windows oversizes a maximised window by the invisible resize border,
			// so it overshoots the monitor exactly the way a game does, and with
			// the taskbar on another screen nothing trims it back.
			"maximised browser overshooting the monitor is not a game",
			ordinary | WSMaximize,
			Rect{Left: 3432, Top: -8, Right: 5368, Bottom: 1087},
			false,
		},
		{
			"framed window sized to the monitor but not maximised is not a game",
			ordinary,
			mon,
			false,
		},
		{
			"borderless but not covering the monitor is not a game",
			borderless,
			Rect{Left: 3440, Top: 0, Right: 4400, Bottom: 700},
			false,
		},
		{
			"borderless and maximised is still not a game",
			borderless | WSMaximize,
			mon,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LooksFullscreen(tc.style, tc.win, mon); got != tc.want {
				t.Fatalf("LooksFullscreen(%#x, %+v) = %v, want %v", tc.style, tc.win, got, tc.want)
			}
		})
	}
}

func TestEmptyRect(t *testing.T) {
	if !(Rect{}).Empty() {
		t.Fatal("zero rect should be empty")
	}
	if (Rect{Left: 0, Top: 0, Right: 10, Bottom: 10}).Empty() {
		t.Fatal("a rect with area should not be empty")
	}
	if !(Rect{Left: 10, Top: 0, Right: 10, Bottom: 10}).Empty() {
		t.Fatal("zero width should be empty")
	}
}

func TestIsShellClass(t *testing.T) {
	// The desktop is monitor-sized, so without this every "show desktop" would
	// read as a fullscreen application.
	for _, class := range []string{"Progman", "WorkerW", "Shell_TrayWnd"} {
		if !IsShellClass(class) {
			t.Fatalf("%q should be treated as shell", class)
		}
	}
	for _, class := range []string{"UnityWndClass", "CABINETWCLASS", ""} {
		if IsShellClass(class) {
			t.Fatalf("%q should not be treated as shell", class)
		}
	}
}
