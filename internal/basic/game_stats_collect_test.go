package basic

import "testing"

func TestApplySelectFunc_SplitChain(t *testing.T) {
	in := `<img src="/core/level_badge/?level=337" style="width: 85px; height: 85px;" class="center-img">`
	got := applySelectFunc(in, `.split('?level=')[1].split('"')[0]`)
	if got != "337" {
		t.Fatalf("unexpected selectfunc output: got %q want %q", got, "337")
	}
}

func TestApplySelectFunc_OutOfRange(t *testing.T) {
	got := applySelectFunc("abc", `.split('-')[2]`)
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestApplySelectFunc_ChainedCommands_Extensible(t *testing.T) {
	in := `  level=337  `
	got := applySelectFunc(in, `.trim().replace('level=','').split('3')[0].trim()`)
	if got != "" {
		t.Fatalf("unexpected chained output: got %q want empty", got)
	}
	got2 := applySelectFunc(in, `.trim().replace('level=','').toUpper()`)
	if got2 != "337" {
		t.Fatalf("unexpected output: got %q want %q", got2, "337")
	}
}

func TestApplySelectFunc_Substring(t *testing.T) {
	in := "prefix-abcdef"
	got := applySelectFunc(in, `.substring(7:10)`)
	if got != "abc" {
		t.Fatalf("unexpected substring output: got %q want %q", got, "abc")
	}
}

