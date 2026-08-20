package values_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The boundaries this package's guarantees rest on.
//
// Each is one careless line away from being broken, and once broken the guarantee is
// gone silently — a persistence method would not fail any test, it would just quietly
// make values outlive their program.

func TestTheValuesPackageHasNoPersistenceAPI(t *testing.T) {
	// The ABSENCE of a Save is the design. A persisted value would be a fact about a
	// screen nobody is looking at, reused silently in a context it was never captured
	// for — so the absence is asserted rather than left to be noticed.
	forbidden := []string{
		"Save", "Load", "Persist", "Store", "WriteFile", "ReadFile",
		"Path", "Dir", "File", "Flush", "Marshal",
	}
	for _, name := range exportedNames(t, ".") {
		for _, f := range forbidden {
			if name == f || strings.HasPrefix(name, f) {
				t.Errorf("values exports %q — program-local values are deliberately not "+
					"persisted, and an API that could write them is how that stops being true",
					name)
			}
		}
	}
}

func TestTheValuesPackageImportsOnlyTheStandardLibrary(t *testing.T) {
	// No platform hosts, no accessibility bridge, no Marco. A value is DATA; the moment
	// this package could reach a desktop it would start being able to re-read one, and
	// "values never re-resolve" would become a convention rather than a fact.
	forEachImport(t, ".", func(file, imported string) {
		first, _, _ := strings.Cut(imported, "/")
		if strings.Contains(first, ".") {
			t.Errorf("%s imports %s — values holds data and reaches nothing", file, imported)
		}
	})
}

func TestMarcoDoesNotImportDirectorValues(t *testing.T) {
	// The other direction. Marco executes concrete deterministic effects and knows
	// nothing about the Director's data flow; an import here would mean the language
	// had grown value variables, which this milestone explicitly does not do.
	const values = "github.com/chaynes-simpleclouds/marco/internal/director/values"
	for _, dir := range []string{
		filepath.Join("..", "..", "runtime"),
		filepath.Join("..", "..", "compile"),
		filepath.Join("..", "..", "parser"),
		filepath.Join("..", "..", "ast"),
	} {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		forEachImport(t, dir, func(file, imported string) {
			if imported == values {
				t.Errorf("%s imports the Director's values package; Marco has no value "+
					"variables, and the Director's data flow is not the language's concern", file)
			}
		})
	}
}

// exportedNames lists every exported top-level name declared in a directory.
func exportedNames(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", dir, err)
	}
	var out []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, d := range file.Decls {
				switch decl := d.(type) {
				case *ast.FuncDecl:
					if decl.Name.IsExported() {
						out = append(out, decl.Name.Name)
					}
				case *ast.GenDecl:
					for _, spec := range decl.Specs {
						switch s := spec.(type) {
						case *ast.TypeSpec:
							if s.Name.IsExported() {
								out = append(out, s.Name.Name)
							}
						case *ast.ValueSpec:
							for _, n := range s.Names {
								if n.IsExported() {
									out = append(out, n.Name)
								}
							}
						}
					}
				}
			}
		}
	}
	if len(out) == 0 {
		t.Fatalf("no exported names found in %s — the check is not looking at anything", dir)
	}
	return out
}

// forEachImport walks every non-test Go file under dir and reports each import.
func forEachImport(t *testing.T, dir string, fn func(file, imported string)) {
	t.Helper()
	fset := token.NewFileSet()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") {
			return err
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return nil
		}
		for _, imp := range f.Imports {
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr == nil {
				fn(path, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
}
