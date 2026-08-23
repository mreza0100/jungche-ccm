package engine

import (
	"strings"
	"testing"
)

func TestParseAcceptsShortAndLongSpellings(t *testing.T) {
	tests := map[string]ID{
		"cc":      Claude,
		"Claude":  Claude,
		" codex ": Codex,
		"OX":      Opencode,
	}
	for value, want := range tests {
		t.Run(value, func(t *testing.T) {
			got, err := Parse(value)
			if err != nil || got != want {
				t.Fatalf("Parse(%q) = %q, %v; want %q, nil", value, got, err, want)
			}
		})
	}
}

func TestParseRefusesEmptyAndUnknown(t *testing.T) {
	const acceptedSet = "cc/claude, cx/codex, ox/opencode"
	for _, value := range []string{"", "bogus"} {
		t.Run(value, func(t *testing.T) {
			_, err := Parse(value)
			if err == nil || !strings.Contains(err.Error(), acceptedSet) {
				t.Fatalf("Parse(%q) error = %v; want accepted set %q", value, err, acceptedSet)
			}
		})
	}
}

func TestEveryDescriptorIsComplete(t *testing.T) {
	for _, id := range All() {
		d := MustLookup(id)
		if d.ID == "" || d.Name == "" || d.Short == "" || d.LongName == "" ||
			d.Binary == "" || d.SocketPrefix == "" || d.RootEnv == "" {
			t.Errorf("descriptor %q is incomplete: %#v", id, d)
		}
		if d.DefaultRoots == nil || len(d.DefaultRoots("/h")) == 0 {
			t.Errorf("descriptor %q has no default roots", id)
		}
		if d.SocketPrefix != string(d.ID)+"-" {
			t.Errorf("descriptor %q socket prefix = %q, want %q", id, d.SocketPrefix, string(d.ID)+"-")
		}
		if d.LongName != strings.ToLower(d.LongName) {
			t.Errorf("descriptor %q long name = %q, want lowercase", id, d.LongName)
		}
	}
}

func TestRegisterRefusesADuplicate(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Register(existing descriptor) did not panic")
		}
	}()
	Register(MustLookup(Claude))
}

func TestFromSocketRecognizesEveryEngine(t *testing.T) {
	for _, id := range All() {
		name := MustLookup(id).SocketPrefix + "session"
		got, ok := FromSocket(name)
		if !ok || got != id {
			t.Fatalf("FromSocket(%q) = %q, %t; want %q, true", name, got, ok, id)
		}
	}
	if got, ok := FromSocket("zz-session"); ok || got != "" {
		t.Fatalf("FromSocket(unknown) = %q, %t; want empty, false", got, ok)
	}
}
