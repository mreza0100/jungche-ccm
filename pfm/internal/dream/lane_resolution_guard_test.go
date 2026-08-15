package dream

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Night resolves an agent type to its lane through the lane profiles' Serves
// declarations (lane.FromAgentTypeIn). An entry point that resolves the same
// agent type alias-free (lane.FromAgentType) disagrees with what Night staged
// — Apply once rejected Night's own signed apply command this way ("staged
// lane.txt mismatch: got tracer, want explore"). Alias-free normalization is
// legitimate ONLY for syntactic validation of names that are already lane
// slugs; those sites are enumerated here and every other caller fails by name.
func TestLaneResolutionIsAliasAwareAtEveryEntryPoint(t *testing.T) {
	allowed := map[string]bool{
		// morning.go validates repos.list agent fields and lane profile
		// filenames syntactically; its real resolution runs FromAgentTypeIn.
		"morning.go": true,
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	inspected := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		inspected++
		body, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatal(err)
		}
		if allowed[name] {
			continue
		}
		if strings.Contains(string(body), "lane.FromAgentType(") {
			t.Errorf(
				"%s resolves an agent type alias-free (lane.FromAgentType); use lane.FromAgentTypeIn with the organ root so it agrees with what Night staged",
				name,
			)
		}
	}
	if inspected == 0 {
		t.Fatal("lane-resolution guard inspected no source files")
	}
	t.Logf("inspected %d dream source files", inspected)
}
