package artifact

import "testing"

func TestVerdictsParseNormalizeAndRender(t *testing.T) {
	text := "CONFIRM\tmaps/valid-anchor.md\tclaims checked at pinned HEAD\n"
	parsed, err := ParseVerdicts(text)
	if err != nil {
		t.Fatalf("ParseVerdicts() error = %v", err)
	}
	normalized, err := NormalizeVerdicts([]string{"maps/valid-anchor.md"}, parsed)
	if err != nil {
		t.Fatalf("NormalizeVerdicts() error = %v", err)
	}
	if got := RenderNormalizedVerdicts(normalized); got != text {
		t.Fatalf("valid verdict changed during normalization: got %q, want %q", got, text)
	}
}

func TestMissingVerdictBecomesUnruled(t *testing.T) {
	parsed, err := ParseVerdicts("")
	if err != nil {
		t.Fatalf("ParseVerdicts(empty) error = %v", err)
	}
	normalized, err := NormalizeVerdicts([]string{"maps/valid-anchor.md"}, parsed)
	if err != nil {
		t.Fatalf("NormalizeVerdicts() error = %v", err)
	}
	want := "UNRULED\tmaps/valid-anchor.md\tno verifier verdict; not applied\n"
	if got := RenderNormalizedVerdicts(normalized); got != want {
		t.Fatalf("RenderNormalizedVerdicts() = %q, want %q", got, want)
	}
}

func TestVerdictsRejectMalformedDuplicateAndUnknownRows(t *testing.T) {
	_, err := ParseVerdicts("YES\tmaps/a.md\tunsupported verdict\n")
	assertErrorContains(t, err, "line 1: YES\tmaps/a.md\tunsupported verdict")

	parsed, err := ParseVerdicts("CONFIRM\tmaps/a.md\tone\nREFUTE\tmaps/a.md\ttwo\nAMEND\tmaps/unknown.md\tthree\n")
	if err != nil {
		t.Fatalf("ParseVerdicts() error = %v", err)
	}
	_, err = NormalizeVerdicts([]string{"maps/a.md"}, parsed)
	assertErrorContains(t, err, "DUPLICATE MAPS:\nmaps/a.md")
	assertErrorContains(t, err, "UNKNOWN MAPS:\nmaps/unknown.md")
}
