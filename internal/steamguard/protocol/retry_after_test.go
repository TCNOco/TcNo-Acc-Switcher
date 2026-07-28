package protocol

import (
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		value string
		want  time.Duration
		ok    bool
	}{
		{name: "seconds", value: "120", want: 120 * time.Second, ok: true},
		{name: "HTTP date", value: "Wed, 22 Jul 2026 12:02:00 GMT", want: 2 * time.Minute, ok: true},
		{name: "past date", value: "Wed, 22 Jul 2026 11:59:00 GMT", want: 0, ok: true},
		{name: "negative", value: "-1", ok: false},
		{name: "invalid", value: "soon", ok: false},
		{name: "overflow", value: "999999999999999999999999", ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ParseRetryAfter(test.value, now)
			if ok != test.ok || got != test.want {
				t.Fatalf("ParseRetryAfter(%q) = (%s, %t), want (%s, %t)", test.value, got, ok, test.want, test.ok)
			}
		})
	}
}
