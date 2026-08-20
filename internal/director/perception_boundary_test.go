package director

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The evidence/belief boundary, enforced.
//
//	Observations are EVIDENCE. World State is BELIEF.
//	The Director reasons only over belief.
//	Providers contribute only evidence.
//	Fusion is the sole component allowed to convert one into the other.
//
// That rule is worth exactly as much as its enforcement. It costs nothing to write in
// a document and one careless import to lose, and losing it is not obvious from any
// individual change: a planner that reaches for an observation "just to check the
// source" compiles, passes its tests, and quietly makes the whole perception layer
// unswappable. By the time OCR arrives, a dozen packages know that accessibility
// exists and every one of them has to be revisited.
//
// So it is a test. The point of this milestone is that adding a perception source
// touches the perception packages and nothing else; this is what keeps that true.

// perceptionPath is the only package tree allowed to know what an observation is.
const perceptionPath = modulePath + "/internal/director/perception"

// evidenceOnly are the packages whose types describe raw evidence. Importing one is a
// declaration that you intend to reason about where information CAME FROM, which is a
// perception concern and nobody else's.
var evidenceOnly = []string{
	perceptionPath + "/observation",
	perceptionPath + "/providers",
	// Explanations are a diagnostics layer and nothing else. If a planner, verifier,
	// policy engine, replay path or action graph node could consult one, explanations
	// would stop being a description of what the Director did and start being an input
	// to it — and the first time they disagreed, the description would be the one
	// nobody had tested.
	//
	// The service is NOT exempt. It transports explanations inside a diagnostics
	// payload without importing this package directly, which is exactly the
	// relationship intended: it is a pipe, and it has no opinion about what flows
	// through it.
	perceptionPath + "/explain",
	// Visual state, for the same reason as OCR and one more. Appearance is the weakest
	// evidence there is: a selection highlight and a hover look identical, and so do a
	// greyed-out control and an empty area. A planner that could read it directly would
	// be deciding from pixels what only structure may establish.
	perceptionPath + "/providers/visualstate",
}

// mayTouchEvidence are the packages exempt, each for a stated reason. The list is
// short on purpose: every entry is a place where the boundary is crossed on purpose,
// and a long list would mean there was no boundary.
var mayTouchEvidence = map[string]string{
	// The engine IS the conversion. It is the one thing that must see both sides.
	"perception/fusion": "the fusion engine converts evidence into belief — that is its job",
	// Rendering what perception did requires seeing what perception did.
	"perception/diagnostics": "diagnostics report on the pipeline, so they must see it",
	// Tests need a world from recorded evidence, and they get one by building the same
	// cycle a collector would and handing it to the same engine.
	"internal/recorded": "the recorded-evidence path for fixtures, which goes through the engine",
	// The service TRANSPORTS provider diagnostics — `director ocr` and `director
	// visual` run in a different process from the Director. It names the payload types
	// and interprets none of them: nothing it does changes because a diagnostic says
	// one thing rather than another, which is the property the rule actually protects.
	//
	// A narrow exemption, and slightly uncomfortable: it exists because the diagnostics
	// types live in the provider packages. Moving them to a shared payload package
	// would remove the exemption, and is a larger change than this milestone.
	"director/service": "transports provider diagnostics between processes without interpreting them",
}

func TestOnlyPerceptionKnowsWhatAnObservationIs(t *testing.T) {
	root := ".."
	checked, violations := 0, 0

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		rel := filepath.ToSlash(path)
		// Everything under perception/ is exempt from the import rule by definition:
		// the sub-packages are each other's collaborators. What is NOT exempt is the
		// reverse direction, checked below.
		if strings.Contains(rel, "/perception/") {
			return nil
		}
		if exempt(rel) {
			return nil
		}

		file, perr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if perr != nil {
			return nil // a file that does not parse is the compiler's problem, not this test's
		}
		checked++

		for _, imp := range file.Imports {
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr != nil {
				continue
			}
			for _, banned := range evidenceOnly {
				if p != banned {
					continue
				}
				violations++
				t.Errorf("%s imports %s\n"+
					"    Observations are evidence; this package reasons over belief.\n"+
					"    Everything outside internal/director/perception sees Elements, and must\n"+
					"    have no way to ask which source produced one — that is what lets OCR be\n"+
					"    added without touching it.\n"+
					"    If this is genuinely a perception concern, it belongs under perception/.",
					rel, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	if checked == 0 {
		t.Fatal("no files were checked — the walk is broken and this test is proving nothing")
	}
	if violations == 0 {
		t.Logf("checked %d files outside perception/", checked)
	}
}

// exempt reports whether a file is on the short list allowed to touch evidence.
func exempt(rel string) bool {
	for suffix := range mayTouchEvidence {
		if strings.Contains(rel, "/"+suffix+"/") {
			return true
		}
	}
	// cmd/director is the composition root: it constructs the providers and the engine
	// and wires them to the pipeline. It is the one place that is SUPPOSED to see the
	// whole shape, which is why the Director's wiring lives in exactly one small main
	// package.
	return strings.Contains(rel, "/cmd/director/")
}

// The other direction: the belief layer must not leak back into perception.
//
// Less obvious than the first rule and just as important. A fusion engine that could
// call the planner, or read the action graph, would be able to fuse differently
// depending on what the Director wanted to be true — and evidence that bends toward
// the conclusion is not evidence.
func TestPerceptionCannotReachIntoTheReasoningLayers(t *testing.T) {
	reasoning := []string{
		modulePath + "/internal/director/intent",
		modulePath + "/internal/director/plan",
		modulePath + "/internal/director/policy",
		modulePath + "/internal/director/target",
		modulePath + "/internal/director/verify",
		modulePath + "/internal/director/execute",
		modulePath + "/internal/director/actiongraph",
		modulePath + "/internal/director/service",
		modulePath + "/internal/director/memory",
		modulePath + "/internal/director/world",
	}

	checked := 0
	err := filepath.WalkDir("perception", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		file, perr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if perr != nil {
			return nil
		}
		checked++
		for _, imp := range file.Imports {
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr != nil {
				continue
			}
			for _, banned := range reasoning {
				if p == banned {
					t.Errorf("%s imports %s\n"+
						"    Perception must not depend on what the Director wants to be true.\n"+
						"    Evidence that could consult the planner is not evidence.",
						filepath.ToSlash(path), p)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking perception: %v", err)
	}
	if checked == 0 {
		t.Fatal("no perception files were checked — this test is proving nothing")
	}
}

// The OCR provider is a provider, and nothing downstream may know it exists.
//
// Checked separately from the evidence rule above because the failure it prevents is
// specific and tempting: a planner that reached for OCR text directly would be
// deciding, in a place with no structural evidence and no way to be overruled, that
// visible glyphs mean a control is there. That is precisely the judgement fusion exists
// to make and the reason OCR alone may not establish actionability.
func TestNothingOutsidePerceptionKnowsThatOCRExists(t *testing.T) {
	const ocrPkg = perceptionPath + "/providers/ocr"

	checked := 0
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel := filepath.ToSlash(path)
		// The perception tree owns it; cmd/director wires it; the platform packages
		// IMPLEMENT its interfaces, which is the direction that keeps the engine free
		// of an OCR dependency rather than the direction that leaks one.
		if strings.Contains(rel, "/perception/") ||
			strings.Contains(rel, "/cmd/director/") ||
			strings.Contains(rel, "/platform/ocrclient/") ||
			strings.Contains(rel, "/platform/wincapture/") ||
			strings.Contains(rel, "/director/service/") {
			return nil
		}

		file, perr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if perr != nil {
			return nil
		}
		checked++
		for _, imp := range file.Imports {
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr != nil || p != ocrPkg {
				continue
			}
			t.Errorf("%s imports %s\n"+
				"    OCR is a perception source. Everything downstream reasons over\n"+
				"    Elements and must not be able to ask whether a belief came from\n"+
				"    pixels — deciding that visible text means a control exists is\n"+
				"    fusion's judgement, and only fusion can be overruled on it.", rel, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	if checked == 0 {
		t.Fatal("no files were checked — this test is proving nothing")
	}
}
