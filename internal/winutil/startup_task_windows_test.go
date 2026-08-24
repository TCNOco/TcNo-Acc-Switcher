//go:build windows

package winutil

import (
	"strings"
	"testing"
)

func TestSanitizeTaskNameComponent(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// user.Current().Username is DOMAIN\user; an unescaped backslash would
		// register the task one folder deeper instead of naming it.
		{"domain account", `TCNO-PC\tcno`, "TCNO-PC_tcno"},
		{"plain account", "tcno", "tcno"},
		{"reserved characters", `a/b:c*d?e"f<g>h|i`, "a_b_c_d_e_f_g_h_i"},
		{"control characters dropped", "tc\x01no\x1f", "tcno"},
		{"trimmed", "  tcno  ", "tcno"},
		{"empty", "   ", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sanitizeTaskNameComponent(c.in); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestStartupTrayTaskNameStaysInOneFolder(t *testing.T) {
	name := startupTrayTaskName(`TCNO-PC\tcno`)
	if strings.Count(name, `\`) != 2 {
		t.Fatalf("task name %q should sit directly in %q", name, startupTaskFolder)
	}
}

func TestStartupTrayTaskXMLEscapesPaths(t *testing.T) {
	// & is legal in a Windows path and would otherwise make the XML malformed.
	xml := startupTrayTaskXML(`C:\Games & Apps\TcNo-Acc-Switcher.exe`, "S-1-5-21-1-2-3-1001")
	if !strings.Contains(xml, `<Command>C:\Games &amp; Apps\TcNo-Acc-Switcher.exe</Command>`) {
		t.Errorf("command not escaped: %s", xml)
	}
	if !strings.Contains(xml, `<WorkingDirectory>C:\Games &amp; Apps</WorkingDirectory>`) {
		t.Errorf("working directory not escaped: %s", xml)
	}
	if strings.Contains(xml, "& ") {
		t.Errorf("raw ampersand left in XML: %s", xml)
	}
}

func TestUTF16RoundTrip(t *testing.T) {
	// schtasks rejects the XML outright if the byte order or BOM is wrong, and
	// reads back its own tasks in the same encoding.
	const in = `<Command>C:\Ünïcode\app.exe</Command>`
	if got := decodeSchtasksOutput(utf16LEWithBOM(in)); got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

func TestDecodeSchtasksOutputPassesThroughSingleByteText(t *testing.T) {
	const in = "ERROR: The system cannot find the file specified."
	if got := decodeSchtasksOutput([]byte(in)); got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}
