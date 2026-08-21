package tray

import "testing"

func TestTrayTextFallsBackWhenTranslationMissing(t *testing.T) {
	tests := []struct {
		name string
		tr   func(string, map[string]string) string
	}{
		{
			name: "translation key returned",
			tr:   func(key string, _ map[string]string) string { return key },
		},
		{
			name: "empty translation returned",
			tr:   func(string, map[string]string) string { return "" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trayText(tt.tr, "Tray_Switch", map[string]string{"account": "Marc"}, "Switch to: {account}")
			if got != "Switch to: Marc" {
				t.Fatalf("tray text = %q, want %q", got, "Switch to: Marc")
			}
		})
	}
}

func TestTrayTextUsesTranslation(t *testing.T) {
	tr := func(_ string, _ map[string]string) string { return "Cambiar a: Marc" }
	got := trayText(tr, "Tray_Switch", map[string]string{"account": "Marc"}, "Switch to: {account}")
	if got != "Cambiar a: Marc" {
		t.Fatalf("tray text = %q, want %q", got, "Cambiar a: Marc")
	}
}
