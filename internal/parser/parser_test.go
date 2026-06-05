package parser

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/ast"
	"github.com/chaynes-simpleclouds/marco/internal/lexer"
)

func mustParse(t *testing.T, src string) *ast.File {
	t.Helper()
	toks, err := lexer.Lex(src)
	if err != nil {
		t.Fatalf("lex: %v", err)
	}
	f, err := Parse(toks)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return f
}

func TestSimpleDeclaration(t *testing.T) {
	f := mustParse(t, "the User is a set.\n")
	if len(f.Body) != 1 {
		t.Fatalf("want 1 sentence, got %d", len(f.Body))
	}
	s := f.Body[0]
	if s.Term != ast.TermPeriod {
		t.Fatalf("want period, got %v", s.Term)
	}
	if len(s.Parts) != 5 {
		t.Fatalf("want 5 parts, got %d", len(s.Parts))
	}
}

func TestPhraseBody(t *testing.T) {
	src := "this's Save does...\n    this is ok!\n"
	f := mustParse(t, src)
	if len(f.Body) != 1 {
		t.Fatalf("want 1 top sentence, got %d", len(f.Body))
	}
	s := f.Body[0]
	if s.Term != ast.TermEllipsis {
		t.Fatalf("want ellipsis terminator")
	}
	if len(s.Body) != 1 {
		t.Fatalf("want 1 body sentence, got %d", len(s.Body))
	}
	if s.Body[0].Term != ast.TermBang {
		t.Fatalf("want bang in body")
	}
}

func TestNestedBodies(t *testing.T) {
	src := "this's Save does...\n    when ok?\n        this is ok!\n    or?\n        this is failed!\n"
	f := mustParse(t, src)
	s := f.Body[0]
	if len(s.Body) != 2 {
		t.Fatalf("want 2 body sentences, got %d", len(s.Body))
	}
	if s.Body[0].Term != ast.TermQuestion {
		t.Fatalf("first child should be question")
	}
	if len(s.Body[0].Body) != 1 {
		t.Fatalf("first child should have 1 nested sentence")
	}
}
