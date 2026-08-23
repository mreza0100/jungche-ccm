package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// jailTest pins PFM_HOME, but a test that reaches for the plain HOME variable
// gets whatever the operator's shell exported. The two names describe one
// concept, and while they were allowed to disagree a jailed test wrote fixture
// transcripts into a live ~/.claude/projects on a dev host. The jail has to
// pin both, and they have to agree.
func TestJailTestPinsPlainHOMEInsideTheJail(t *testing.T) {
	root := jailTest(t)

	plain, found := os.LookupEnv("HOME")
	if !found || plain == "" {
		t.Fatal("jailTest() left HOME unset; a test reading it would build paths from the empty string")
	}
	if jailed := os.Getenv("PFM_HOME"); plain != jailed {
		t.Fatalf("HOME=%q and PFM_HOME=%q disagree; one concept must carry one value", plain, jailed)
	}

	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", root, err)
	}
	canonicalHome, err := filepath.EvalSymlinks(plain)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", plain, err)
	}
	if !strings.HasPrefix(canonicalHome+string(os.PathSeparator), canonicalRoot+string(os.PathSeparator)) {
		t.Fatalf(
			"jailTest() left HOME=%q outside its jail %q;\n"+
				"a test writing under it lands in the operator's real account",
			canonicalHome, canonicalRoot,
		)
	}
}
