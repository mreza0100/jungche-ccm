package artifact

import "testing"

func TestCensusRoundTrip(t *testing.T) {
	want := Census{
		WindowMetaCount:                  7,
		AgentMetaCount:                   5,
		PairedTranscriptCount:            4,
		SelectedPairedTranscriptCount:    3,
		OmittedPairedTranscriptCount:     1,
		CoverageGapCount:                 1,
		ExcludedOtherAgentOrInvalidCount: 2,
		InvalidMetaCount:                 1,
	}
	text := RenderCensus(want)
	got, err := ParseCensus(text)
	if err != nil {
		t.Fatalf("ParseCensus() error = %v", err)
	}
	if got != want {
		t.Fatalf("ParseCensus(RenderCensus()) = %#v, want %#v", got, want)
	}
}

func TestCensusRejectsMissingDuplicateUnknownAndNegative(t *testing.T) {
	text := RenderCensus(Census{}) +
		"window-meta-count\t2\n" +
		"foreign-count\t3\n" +
		"negative-count\t-1\n"
	_, err := ParseCensus(text)
	assertErrorContains(t, err, "duplicate census key window-meta-count")
	assertErrorContains(t, err, "unknown census key foreign-count")
	assertErrorContains(t, err, "invalid census row")

	_, err = ParseCensus("")
	assertErrorContains(t, err, "missing census key: window-meta-count")
}
