package naming

import "testing"

func TestLabelHidden(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		label string
		want  bool
	}{
		{"exact prefix", "_HIDE", true},
		{"prefix with a payload", "_HIDE headless worker 3", true},
		{"lower case", "_hide worker", true},
		{"mixed case", "_HiDe worker", true},
		{"no underscore", "HIDE worker", false},
		{"prefix in the middle", "worker _HIDE", false},
		{"shorter than the prefix", "_HID", false},
		{"empty", "", false},
		{"underscore only", "_", false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := LabelHidden(testCase.label); got != testCase.want {
				t.Fatalf("LabelHidden(%q) = %t, want %t",
					testCase.label, got, testCase.want)
			}
		})
	}
}
