package resources

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResourcesLayerPerFileAndPreservePriority(t *testing.T) {
	root := t.TempDir()
	development := filepath.Join(root, "development")
	organ := filepath.Join(root, "organ")
	for _, directory := range []string{
		filepath.Join(development, "lanes"),
		filepath.Join(organ, "lanes"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeResourceTest(t, filepath.Join(development, "lanes", "tracer.md"), "DEVELOPMENT\n")
	writeResourceTest(t, filepath.Join(development, "dreamer-distill.prompt.md"), "DISTILL OVERRIDE\n")
	writeResourceTest(t, filepath.Join(organ, "lanes", "tracer.md"), "ORGAN\n")
	writeResourceTest(t, filepath.Join(organ, "lanes", "reviewer.md"), "REVIEWER\n")

	resources := NewResources(development, organ)
	assertResourceBody(t, resources, "lanes/tracer.md", "DEVELOPMENT\n")
	assertResourceBody(t, resources, "lanes/reviewer.md", "REVIEWER\n")
	assertResourceBody(t, resources, "dreamer-distill.prompt.md", "DISTILL OVERRIDE\n")
	embedded, err := resources.ReadFile("dreamer-refiner.prompt.md")
	if err != nil || !strings.HasPrefix(string(embedded), "# Dreamer verify seat\n") {
		t.Fatalf("partial overlays lost embedded prompt: %q, %v", embedded, err)
	}

	if err := os.Remove(filepath.Join(development, "lanes", "tracer.md")); err != nil {
		t.Fatal(err)
	}
	assertResourceBody(t, resources, "lanes/tracer.md", "ORGAN\n")
}

func TestResourcesReadDirMergesAndFirstDeclarationWins(t *testing.T) {
	root := t.TempDir()
	override := filepath.Join(root, "override")
	if err := os.MkdirAll(filepath.Join(override, "lanes"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeResourceTest(t, filepath.Join(override, "lanes", "reviewer.md"), "REVIEWER\n")
	writeResourceTest(t, filepath.Join(override, "lanes", "tracer.md"), "OVERRIDE\n")

	resources := NewResources(override)
	entries, err := resources.ReadDir("lanes")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	want := []string{"explorer.md", "reviewer.md", "tracer.md"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("ReadDir(lanes) = %q, want %q", names, want)
	}
	assertResourceBody(t, resources, "lanes/tracer.md", "OVERRIDE\n")
}

func TestResourcesSkipMissingRootButPropagateRealDiskErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	resources := NewResources(missing)
	if _, err := resources.ReadFile("dreamer-distill.prompt.md"); err != nil {
		t.Fatalf("missing override blocked embedded file: %v", err)
	}

	notDirectory := filepath.Join(t.TempDir(), "resource-file")
	writeResourceTest(t, notDirectory, "not a root\n")
	resources = NewResources(notDirectory)
	_, err := resources.ReadFile("dreamer-distill.prompt.md")
	if err == nil || !strings.Contains(err.Error(), "root is not a directory") {
		t.Fatalf("real disk error = %v", err)
	}
}

func TestResourcesRejectEscapeAndSymlink(t *testing.T) {
	resources := NewResources()
	for _, name := range []string{"../secret", "/absolute", "lanes/../secret"} {
		if _, err := resources.ReadFile(name); !errors.Is(err, fs.ErrInvalid) {
			t.Fatalf("ReadFile(%q) error = %v, want fs.ErrInvalid", name, err)
		}
	}

	root := t.TempDir()
	target := filepath.Join(root, "target.md")
	writeResourceTest(t, target, "target\n")
	if err := os.Mkdir(filepath.Join(root, "lanes"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "lanes", "tracer.md")); err != nil {
		t.Fatal(err)
	}
	_, err := NewResources(root).ReadFile("lanes/tracer.md")
	if err == nil || !strings.Contains(err.Error(), "path is a symlink") {
		t.Fatalf("symlink resource error = %v", err)
	}
}

func TestResourcesMissingErrorNamesEveryLayer(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent")
	_, err := NewResources(root).ReadFile("lanes/absent.md")
	if !errors.Is(err, fs.ErrNotExist) ||
		!strings.Contains(err.Error(), filepath.Join(root, "lanes", "absent.md")) ||
		!strings.Contains(err.Error(), "embedded:lanes/absent.md") {
		t.Fatalf("missing resource error = %v", err)
	}
}

func assertResourceBody(t *testing.T, resources Resources, name, want string) {
	t.Helper()
	got, err := resources.ReadFile(name)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", name, err)
	}
	if string(got) != want {
		t.Fatalf("ReadFile(%s) = %q, want %q", name, got, want)
	}
}

func writeResourceTest(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
