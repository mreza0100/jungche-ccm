package lane

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"hostops/pfm/internal/dream/artifact"
)

func TestSlugUsesSharedASCIIByteNormalization(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"qa_Orion/Cortex", "qa-orion-cortex"},
		{"a__b", "a--b"},
		{"xé", "x--"},
	}
	for _, test := range tests {
		got, err := Slug(test.input)
		if err != nil {
			t.Fatalf("Slug(%q) error = %v", test.input, err)
		}
		if got != test.want {
			t.Fatalf("Slug(%q) = %q, want %q", test.input, got, test.want)
		}
	}
	for _, input := range []string{"", " \t\r\n", "é"} {
		if _, err := Slug(input); err == nil {
			t.Fatalf("Slug(%q) error = nil", input)
		}
	}
}

func TestFromAgentTypeIsAliasFreeAndAliasesComeFromLaneProfiles(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Explore", "explore"},
		{"EXPLORE", "explore"},
		{"explore", "explore"},
		{"qa_Orion/Cortex", "qa-orion-cortex"},
	}
	for _, test := range tests {
		got, err := FromAgentType(test.input)
		if err != nil {
			t.Fatalf("FromAgentType(%q) error = %v", test.input, err)
		}
		if got != test.want {
			t.Fatalf("FromAgentType(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestResolveProfileIsOrganFirstAndNamesBothMissingPaths(t *testing.T) {
	root := t.TempDir()
	organ := filepath.Join(root, "organ")
	engine := filepath.Join(root, "engine")
	mustMkdir(t, filepath.Join(organ, "lanes"))
	mustMkdir(t, filepath.Join(engine, "lanes"))
	local := filepath.Join(organ, "lanes", "qa.md")
	global := filepath.Join(engine, "lanes", "qa.md")
	mustWrite(t, global, "GLOBAL\n")

	profile, err := ResolveProfile("QA", "qa", organ, engine)
	if err != nil {
		t.Fatalf("ResolveProfile(global) error = %v", err)
	}
	if profile.Path != global || profile.Body != "GLOBAL\n" || profile.AgentType != "QA" || profile.Lane != "qa" {
		t.Fatalf("global profile = %#v", profile)
	}
	mustWrite(t, local, "LOCAL\n")
	profile, err = ResolveProfile("QA", "qa", organ, engine)
	if err != nil {
		t.Fatalf("ResolveProfile(local) error = %v", err)
	}
	if profile.Path != local || profile.Body != "LOCAL\n" {
		t.Fatalf("local profile = %#v", profile)
	}
	_, err = ResolveProfile("Explore", "qa", organ, engine)
	assertErrorContains(t, err, "resolves to lane explore, not qa")

	err = os.Remove(local)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Remove(global)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ResolveProfile("QA", "qa", organ, engine)
	assertErrorContains(t, err, local)
	assertErrorContains(t, err, global)
}

func TestBuildMembershipBackfillsLegacyAndAssignsNewMaps(t *testing.T) {
	got, err := BuildMembership(MembershipRequest{
		FinalMaps:     []string{"new-map.md", "old-map.md"},
		PreviousMaps:  []string{"old-map.md"},
		LedgerPresent: false,
		CurrentLane:   "qa-orion-cortex",
	})
	if err != nil {
		t.Fatalf("BuildMembership() error = %v", err)
	}
	want := artifact.LaneMembership{"old-map.md": "explorer", "new-map.md": "qa-orion-cortex"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildMembership() = %#v, want %#v", got, want)
	}
}

func TestBuildMembershipPreservesLedgerAndFailsClosedOnOldHole(t *testing.T) {
	existing := artifact.LaneMembership{"old-map.md": "tracer", "new-map.md": "stale-lane"}
	got, err := BuildMembership(MembershipRequest{
		FinalMaps:     []string{"old-map.md", "new-map.md"},
		PreviousMaps:  []string{"old-map.md"},
		Existing:      existing,
		LedgerPresent: true,
		CurrentLane:   "qa",
	})
	if err != nil {
		t.Fatalf("BuildMembership() error = %v", err)
	}
	// A stale row for a filename that was not in the old pool cannot steer a
	// newly produced map into another lane.
	want := artifact.LaneMembership{"old-map.md": "tracer", "new-map.md": "qa"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildMembership() = %#v, want %#v", got, want)
	}

	_, err = BuildMembership(MembershipRequest{
		FinalMaps:     []string{"old-map.md"},
		PreviousMaps:  []string{"old-map.md"},
		Existing:      artifact.LaneMembership{},
		LedgerPresent: true,
		CurrentLane:   "qa",
	})
	assertErrorContains(t, err, "pre-existing map carries no lane row: old-map.md")

	_, err = BuildMembership(MembershipRequest{
		FinalMaps:     []string{"old-map.md"},
		PreviousMaps:  []string{"old-map.md"},
		Existing:      artifact.LaneMembership{"../old-map.md": "qa"},
		LedgerPresent: true,
		CurrentLane:   "qa",
	})
	assertErrorContains(t, err, "invalid lane membership")
}

func TestCachedTitlesCarryQuestionsAndSort(t *testing.T) {
	maps := t.TempDir()
	mustWrite(t, filepath.Join(maps, "z-map.md"), mapForDedup("Zulu", "What is zulu?"))
	mustWrite(t, filepath.Join(maps, "a-map.md"), mapForDedup("Alpha", "  What\tis alpha?  "))
	got, err := CachedTitles(maps)
	if err != nil {
		t.Fatalf("CachedTitles() error = %v", err)
	}
	want := []string{"Alpha — What is alpha?", "Zulu — What is zulu?"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CachedTitles() = %#v, want %#v", got, want)
	}

	mustWrite(t, filepath.Join(maps, "missing.md"), "# Missing\n\n## Answer\n\nanswer\n")
	_, err = CachedTitles(maps)
	assertErrorContains(t, err, "map lacks Question")
}

func mapForDedup(title, question string) string {
	return "# " + title + "\n\n## Question\n\n" + question + "\n\n## Answer\n\nanswer\n"
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want substring %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want substring %q", err, want)
	}
}

// An agent joins an existing lane by being named in that lane's profile, so a
// new agent type never requires an engine change. The alias lives in the organ.
func TestFromAgentTypeInReadsTheLaneProfileDeclaration(t *testing.T) {
	organ := t.TempDir()
	if err := os.MkdirAll(filepath.Join(organ, "lanes"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(organ, "lanes", "tracer.md"),
		[]byte("Serves: tracer, Explore, wave-tracer\n\nprofile\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, agentType := range []string{"tracer", "Explore", "wave-tracer"} {
		got, err := FromAgentTypeIn(agentType, organ, "")
		if err != nil || got != "tracer" {
			t.Fatalf("FromAgentTypeIn(%q) = %q, %v; want tracer", agentType, got, err)
		}
	}
	// An undeclared type still gets its own lane rather than being swallowed.
	got, err := FromAgentTypeIn("reviewer", organ, "")
	if err != nil || got != "reviewer" {
		t.Fatalf("FromAgentTypeIn(reviewer) = %q, %v; want reviewer", got, err)
	}
}
