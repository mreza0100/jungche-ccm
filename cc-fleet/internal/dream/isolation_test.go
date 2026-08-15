package dream

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	modulePath       = "hostops/cc-fleet"
	dreamPrefix      = modulePath + "/internal/dream"
	dreamSeatPackage = dreamPrefix + "/seat"
	storePackage     = modulePath + "/internal/store"
)

var allowedSeatImports = map[string]struct{}{
	modulePath + "/internal/action":     {},
	modulePath + "/internal/spawn":      {},
	modulePath + "/internal/headless":   {},
	modulePath + "/internal/transcript": {},
	modulePath + "/internal/paths":      {},
}

var forbiddenDreamImports = map[string]struct{}{
	storePackage:                     {},
	modulePath + "/internal/ui":      {},
	modulePath + "/internal/compose": {},
	modulePath + "/internal/hide":    {},
	modulePath + "/internal/mcpserv": {},
	modulePath + "/internal/gather":  {},
	modulePath + "/internal/check":   {},
}

type listedPackage struct {
	ImportPath   string
	Dir          string
	Imports      []string
	GoFiles      []string
	CgoFiles     []string
	TestGoFiles  []string
	XTestGoFiles []string
}

func TestDreamPackagesRespectDirectImportBoundary(t *testing.T) {
	root := moduleRoot(t)
	packages := goList(t, root, "-deps", "-json", "./internal/dream/...")

	var dreamPackages []listedPackage
	for _, pkg := range packages {
		if pkg.ImportPath == dreamPrefix || strings.HasPrefix(pkg.ImportPath, dreamPrefix+"/") {
			dreamPackages = append(dreamPackages, pkg)
		}
	}
	if len(dreamPackages) == 0 {
		t.Fatal("go list returned no internal/dream packages; refusing to treat an empty scan as isolation")
	}

	for _, pkg := range dreamPackages {
		for _, imported := range pkg.Imports {
			if _, forbidden := forbiddenDreamImports[imported]; forbidden {
				t.Errorf("%s directly imports forbidden host package %s", pkg.ImportPath, imported)
				continue
			}

			if !strings.HasPrefix(imported, modulePath+"/internal/") ||
				imported == dreamPrefix || strings.HasPrefix(imported, dreamPrefix+"/") {
				continue
			}
			if pkg.ImportPath != dreamSeatPackage {
				t.Errorf("%s directly imports host package %s; only %s may cross the host boundary", pkg.ImportPath, imported, dreamSeatPackage)
				continue
			}
			if _, allowed := allowedSeatImports[imported]; !allowed {
				t.Errorf("%s directly imports unapproved host package %s", pkg.ImportPath, imported)
			}
		}
	}
}

func TestDreamImportArrowIsOneWay(t *testing.T) {
	root := moduleRoot(t)
	packages := goList(t, root, "-json", "./...")
	if len(packages) == 0 {
		t.Fatal("go list returned no module packages; refusing to treat an empty source scan as isolation")
	}

	allowedFile := filepath.Join(root, "cmd", "cc-fleet", "dream_command.go")
	filesScanned := 0
	for _, pkg := range packages {
		if pkg.ImportPath == dreamPrefix || strings.HasPrefix(pkg.ImportPath, dreamPrefix+"/") {
			continue
		}
		for _, name := range packageFiles(pkg) {
			filesScanned++
			filename := filepath.Join(pkg.Dir, name)
			for _, imported := range fileImports(t, filename) {
				if imported != dreamPrefix && !strings.HasPrefix(imported, dreamPrefix+"/") {
					continue
				}
				if filepath.Clean(filename) != allowedFile {
					rel, err := filepath.Rel(root, filename)
					if err != nil {
						rel = filename
					}
					t.Errorf("%s imports %s; only cmd/cc-fleet/dream_command.go may import internal/dream", filepath.ToSlash(rel), imported)
				}
			}
		}
	}
	if filesScanned == 0 {
		t.Fatal("source scan visited no files; refusing to treat an empty scan as a one-way import arrow")
	}
}

func TestPermittedHostStoreReferencesAreConstantsOnly(t *testing.T) {
	// RULING-04 permits seat's transitive store dependency only while action and
	// spawn use store solely to name engines. This is the load-bearing pin for
	// that exception: a future store.Open, query, or handle reference fails here.
	root := moduleRoot(t)
	packages := goList(t, root, "-json", "./internal/action", "./internal/spawn")
	byImportPath := make(map[string]listedPackage, len(packages))
	for _, pkg := range packages {
		byImportPath[pkg.ImportPath] = pkg
	}

	allowedSelectors := map[string]struct{}{
		"ClaudeEngine": {},
		"CodexEngine":  {},
	}
	for _, importPath := range []string{
		modulePath + "/internal/action",
		modulePath + "/internal/spawn",
	} {
		pkg, ok := byImportPath[importPath]
		if !ok {
			t.Errorf("go list did not return required host package %s", importPath)
			continue
		}

		references := 0
		for _, name := range packageFiles(pkg) {
			filename := filepath.Join(pkg.Dir, name)
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, filename, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", filename, err)
			}

			aliases := storeImportAliases(t, filename, file)
			if len(aliases) == 0 {
				continue
			}
			ast.Inspect(file, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				ident, ok := selector.X.(*ast.Ident)
				if !ok {
					return true
				}
				if _, isStoreAlias := aliases[ident.Name]; !isStoreAlias {
					return true
				}

				references++
				if _, allowed := allowedSelectors[selector.Sel.Name]; !allowed {
					position := fset.Position(selector.Pos())
					t.Errorf("%s:%d references store.%s; permitted host edges may use store only for ClaudeEngine and CodexEngine", filename, position.Line, selector.Sel.Name)
				}
				return true
			})
		}
		if references == 0 {
			t.Errorf("found no store references in %s; refusing to treat an empty constants-only scan as proof", importPath)
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat go.mod in %s: %v", dir, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod above test working directory")
		}
		dir = parent
	}
}

func goList(t *testing.T, root string, args ...string) []listedPackage {
	t.Helper()

	command := exec.Command("go", append([]string{"list"}, args...)...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(append([]string{"list"}, args...), " "), err, output)
	}

	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var packages []listedPackage
	for {
		var pkg listedPackage
		err := decoder.Decode(&pkg)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode go list output: %v\n%s", err, output)
		}
		packages = append(packages, pkg)
	}
	return packages
}

func packageFiles(pkg listedPackage) []string {
	set := make(map[string]struct{})
	for _, files := range [][]string{pkg.GoFiles, pkg.CgoFiles, pkg.TestGoFiles, pkg.XTestGoFiles} {
		for _, file := range files {
			set[file] = struct{}{}
		}
	}

	result := make([]string, 0, len(set))
	for file := range set {
		result = append(result, file)
	}
	sort.Strings(result)
	return result
}

func fileImports(t *testing.T, filename string) []string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse imports in %s: %v", filename, err)
	}
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("unquote import %s in %s: %v", spec.Path.Value, filename, err)
		}
		imports = append(imports, path)
	}
	return imports
}

func storeImportAliases(t *testing.T, filename string, file *ast.File) map[string]struct{} {
	t.Helper()

	aliases := make(map[string]struct{})
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("unquote import %s in %s: %v", spec.Path.Value, filename, err)
		}
		if path != storePackage {
			continue
		}

		alias := "store"
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		if alias == "." || alias == "_" {
			t.Errorf("%s imports %s as %q; constants-only verification requires a named import", filename, storePackage, alias)
			continue
		}
		aliases[alias] = struct{}{}
	}
	return aliases
}
