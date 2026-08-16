package inject

import "testing"

func TestSelectorLineExactChatShGuards(t *testing.T) {
	tests := []struct {
		name    string
		capture string
		want    string
	}{
		{
			name:    "claude numbered selector",
			capture: "Question\n  ❯ 1. Allow once\n    2. Deny\n",
			want:    "❯ 1. Allow once",
		},
		{
			name: "codex hint selector",
			capture: "Run command?\n› 1. Yes\n  2. No\n" +
				"Press enter to confirm or esc to cancel\n",
			want: "› 1. Yes",
		},
		{
			name:    "codex stacked selector",
			capture: "› 1. Yes\n  2. No\n",
			want:    "› 1. Yes",
		},
		{
			name:    "codex numbered draft allowed",
			capture: "conversation\n› 1. do this next\n",
		},
		{
			name:    "bare claude composer allowed",
			capture: "conversation\n❯ \n",
		},
		{
			name:    "sent codex echo allowed",
			capture: "› 1. already sent\nresponse\n› \n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SelectorLine(test.capture); got != test.want {
				t.Fatalf("SelectorLine() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestIsBusyExactMarkers(t *testing.T) {
	for _, capture := range []string{
		"esc to interrupt",
		"Working… (12s · ↓ 100 tokens)",
		"Working · 4s",
		"123 tokens",
	} {
		if !IsBusy(capture) {
			t.Errorf("IsBusy(%q) = false", capture)
		}
	}
	for _, capture := range []string{"✻ Worked for 12s", "❯", "idle"} {
		if IsBusy(capture) {
			t.Errorf("IsBusy(%q) = true", capture)
		}
	}
}

func TestHasDraftUsesPOSIXGraphNotUnicodeSpace(t *testing.T) {
	// Format controls are not POSIX graph characters. The ASCII space between
	// them must not turn an otherwise non-graph placeholder into a real draft.
	if hasDraft("❯ \u200b \u200b") {
		t.Fatal("ASCII space was counted as draft text")
	}
	if !hasDraft("❯ real draft") {
		t.Fatal("printable ASCII draft was missed")
	}
}

func FuzzSelectorLine(f *testing.F) {
	for _, seed := range []string{
		"❯ 1. allow\n2. deny",
		"› 1. draft",
		"› 1. allow\n2. deny\nenter to confirm",
		"\x00\xff❯",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, capture string) {
		_ = SelectorLine(capture)
	})
}
