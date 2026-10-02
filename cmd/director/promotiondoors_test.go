package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// EVERY DOOR INTO DURABLE MEMORY ASKS THE LICENCE, AND THE CHECK FINDS NEW DOORS BY ITSELF.
//
// # Why this exists, and the mutation that demanded it
//
// `TestObserveCannotMakeItsOwnEvidenceDurable` holds the licence boundary by CALLING each method
// on `promotion` and asserting the refusal. That is the right behavioural test and it has one
// failure mode: it names the doors. A door added later is a door that test does not know about, so
// the licence on it is unenforced and everything stays green.
//
// Measured. A `visit` method was added, its licence check was deleted on purpose, and the whole
// `cmd/director` suite passed — because nothing enumerated the type. This is the same shape as
// `internal/platform/navsource/pump_test.go`, which walks the tree for hook sites rather than
// listing them, and it exists for the same reason: a guard that has to be updated by hand is a
// guard that will not be.
//
// # What it asserts, and why it is structural rather than behavioural
//
// That every method on `promotion` reads `p.licence`. It cannot know what the right permission is —
// `establish` wants `EstablishPlaces` and `relate` wants `AcquireRouteEvidence`, and deciding that
// here would be a second copy of the policy. What it CAN say is that no method on the object
// writes without consulting the licence at all, which is the failure a forgotten door actually
// has.
//
// The behavioural half stays where it is. This is the half that notices a new door.
func TestEveryPromotionDoorIsLicensed(t *testing.T) {
	const file = "observepromote.go"
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing %s: %v", file, err)
	}
	source := readSource(t, file)

	// Methods that are deliberately not doors, each named with its reason. A method added
	// without a licence check and without a line here fails the test, which is the point: the
	// exemption is a decision somebody has to write down.
	notADoor := map[string]string{
		// errNotLicensed's own Error method, if it ever moves onto this type.
	}

	found := 0
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		if receiverName(fn.Recv.List[0].Type) != "promotion" {
			continue
		}
		found++
		name := fn.Name.Name
		if why, exempt := notADoor[name]; exempt {
			t.Logf("promotion.%s is exempt: %s", name, why)
			continue
		}
		body := sourceOf(source, fset, fn)
		if !strings.Contains(body, "p.licence.") {
			t.Errorf("promotion.%s is a method on the object that owns every durable "+
				"ambient write, and it never reads p.licence.\n\n"+
				"Ambient watching holds the ZERO licence for its whole life. A door "+
				"that does not ask is a door through which attention alone writes to "+
				"disk — and the behavioural licence test cannot catch it, because that "+
				"test names the doors it knows about.\n\n"+
				"Either add the licence check, or add %q to notADoor with the reason.",
				name, name)
		}
	}

	// AND THE WALK HAS TO HAVE FOUND SOMETHING. A parse that matched no methods would pass
	// this test while asserting nothing whatsoever — which is the way a tree-walking guard
	// fails silently.
	if found < 4 {
		t.Fatalf("the walk found %d method(s) on promotion. It is supposed to find every "+
			"door into durable memory, and there have never been fewer than four.", found)
	}
}

// receiverName is the type name of a method receiver, pointer or value.
func receiverName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return receiverName(t.X)
	}
	return ""
}

// sourceOf returns the text of one declaration, comments excluded.
//
// Read from the FILE rather than printed back from the AST, so what is checked is the bytes a
// reviewer sees. The comment on a method frequently mentions the licence; the comments are
// excluded by taking the span from the `func` keyword, so a method whose only mention of a licence
// is in its documentation does not pass.
func sourceOf(source string, fset *token.FileSet, fn *ast.FuncDecl) string {
	from := fset.Position(fn.Pos()).Offset
	to := fset.Position(fn.End()).Offset
	if from < 0 || to > len(source) || from >= to {
		return ""
	}
	return source[from:to]
}
