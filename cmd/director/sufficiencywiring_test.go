package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// ONE ASSESSMENT, AND PRODUCTION REACHES IT.
//
// # The failure this forecloses
//
// Four callers ask whether a reading is good enough — Observe, Learn, Perform and 36F
// recovery — and the cheapest way for each to answer is to look at what it has to hand.
// Element count is right there. So is the acquisition time, and the application's name, and
// whether memory recognised anything. Four plausible local answers, drifting apart, and each
// one correct about its own caller until the day a responsive layout or a slow tree makes them
// disagree.
//
// So there is one judgement — observe.ReachOfState, reached through observe.Place — and this
// holds the tree to it.

// The sentence a person is shown comes from the assessment, not from the call site.
//
// It was a hand-written string here, and perform.go had its own saying the same thing
// differently. Two callers of one judgement, each with a private idea of what it meant.
func TestTheUnreadableReasonComesFromTheAssessment(t *testing.T) {
	// A reading that got no further than the window frame.
	shell := observe.Place{
		Placed: true, Reach: observe.ReachShell,
		Vacancy: observe.Vacancy{
			Region: observe.Region{X: 0.17, Y: 0.10, Width: 0.82, Height: 0.88},
			Share:  0.72, Inside: 1, Structures: 13,
		},
	}
	want := observe.SufficiencyOf(shell).Describe()
	if want == "" {
		t.Fatal("the assessment describes a shell reading as nothing at all")
	}
	// The evidence has to survive into the sentence, or the caller may as well have kept
	// its hardcoded string.
	if !strings.Contains(want, "72%") {
		t.Errorf("the description %q does not carry how much of the window was empty.\n"+
			"That number is the whole reason the assessment keeps its evidence.", want)
	}

	src, err := os.ReadFile("showingnow.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "observe.SufficiencyOf(seen).Describe()") {
		t.Error("showingnow.go no longer takes its reason from the shared assessment.\n" +
			"A call site that writes its own sentence has a private opinion about what " +
			"the reading meant, and nothing keeps it in step with the judgement.")
	}
}

// No call site re-derives sufficiency from the raw evidence.
//
// The named judgement is observe.ReachOfState. Anything else in cmd/director that reaches a
// verdict about whether a reading is good enough — by counting elements, by timing the
// acquisition, by naming an application — is a second opinion.
//
// This walks the production source for the shapes that would mean somebody had started one.
func TestNoCallSiteInventsItsOwnSufficiencyRule(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			bin, ok := n.(*ast.BinaryExpr)
			if !ok {
				return true
			}
			switch bin.Op {
			case token.LSS, token.GTR, token.LEQ, token.GEQ:
			default:
				return true
			}
			// A comparison against a Place's own fields is the shape of a local rule.
			if s := exprText(fset, src, bin); strings.Contains(s, "Vacancy.Share") ||
				strings.Contains(s, "Vacancy.Inside") ||
				strings.Contains(s, "Vacancy.Structures") {
				t.Errorf("%s compares the vacancy evidence directly:\n    %s\n"+
					"The verdict is observe.ReachOfState's to reach. A caller that "+
					"re-derives one from the same evidence is a second classifier "+
					"with its own thresholds.", path, s)
			}
			return true
		})
	}
}

func exprText(fset *token.FileSet, src []byte, n ast.Node) string {
	start := fset.Position(n.Pos()).Offset
	end := fset.Position(n.End()).Offset
	if start < 0 || end > len(src) || start >= end {
		return ""
	}
	return string(src[start:end])
}
