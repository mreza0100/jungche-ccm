package artifact

import "testing"

func TestLaneMembershipParseAndCanonicalRender(t *testing.T) {
	parsed, err := ParseLaneMembership("z-map.md\tqa-orion-cortex\na-map.md\texplorer\n")
	if err != nil {
		t.Fatalf("ParseLaneMembership() error = %v", err)
	}
	want := "a-map.md\texplorer\nz-map.md\tqa-orion-cortex\n"
	if got := RenderLaneMembership(parsed); got != want {
		t.Fatalf("RenderLaneMembership() = %q, want %q", got, want)
	}
}

func TestLaneMembershipRejectsDuplicateAndUnsafeRows(t *testing.T) {
	_, err := ParseLaneMembership("map.md\texplorer\nmap.md\tqa\n../escape.md\texplorer\nother.md\t../../etc\n")
	assertErrorContains(t, err, "line 2: duplicate lane row for map.md")
	assertErrorContains(t, err, "line 3: invalid lane row")
	assertErrorContains(t, err, "line 4: invalid lane row")
}
