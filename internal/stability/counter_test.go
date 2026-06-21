package stability

import "testing"

func TestShouldPromptForCount(t *testing.T) {
	tests := []struct {
		count int
		want  bool
	}{
		{0, false},
		{1, true},
		{2, false},
		{9, false},
		{10, true},
		{11, false},
		{20, true},
		{21, false},
		{30, true},
	}
	for _, tc := range tests {
		got := ShouldPromptForCount(tc.count)
		if got != tc.want {
			t.Errorf("ShouldPromptForCount(%d) = %v, want %v", tc.count, got, tc.want)
		}
	}
}
