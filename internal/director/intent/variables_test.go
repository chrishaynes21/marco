package intent_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

func TestVariablePhrasesParseToExplicitIntents(t *testing.T) {
	p := intent.New()
	for _, c := range []struct{ phrase, verb, name string }{
		// "remember this as save" is deliberately NOT here: a bare pronoun does not say
		// whether the control or its contents is meant, and it now asks. See
		// TestABarePronounDoesNotSaySwhichKindOfMemoryIsMeant.
		{"remember this button as save", intent.VerbRemember, "save"},
		{"remember the Save button as save", intent.VerbRemember, "save"},
		{"forget save", intent.VerbForget, "save"},
		{"forget variable save", intent.VerbForget, "save"},
		{"rename variable save to submit", intent.VerbRenameVariable, "save"},
		{"explain variable save", intent.VerbExplainVar, "save"},
		{"list variables", intent.VerbListVariables, ""},
	} {
		in := p.Parse(c.phrase)
		if in.Kind != directorapi.IntentAct {
			t.Errorf("%q kind = %s (%s)", c.phrase, in.Kind, in.Ambiguity)
			continue
		}
		if in.Verb != c.verb {
			t.Errorf("%q verb = %q, want %q", c.phrase, in.Verb, c.verb)
		}
		if c.name != "" {
			got, _ := in.Parameters[intent.VariableName].(string)
			if got != c.name {
				t.Errorf("%q name = %q, want %q", c.phrase, got, c.name)
			}
		}
	}
}

func TestAnInvalidNameIsNamedNotRewritten(t *testing.T) {
	in := intent.New().Parse("remember this button as 2save")
	if in.Kind == directorapi.IntentAct {
		t.Fatal("an invalid name was accepted")
	}
	if !contains(in.Ambiguity, "2save") {
		t.Fatalf("ambiguity = %q, want it to quote the rejected name", in.Ambiguity)
	}
}

func TestADollarReferenceIsATypedReferenceNotText(t *testing.T) {
	name, ok := intent.VariableTarget("$save")
	if !ok || name != "save" {
		t.Fatalf("VariableTarget = %q, %v", name, ok)
	}
	// A label that merely contains a dollar is not a reference.
	if _, ok := intent.VariableTarget("Save $5 now"); ok {
		t.Fatal("a multi-word phrase was read as a variable reference")
	}
	if _, ok := intent.VariableTarget("$2bad"); ok {
		t.Fatal("an invalid name was accepted as a reference")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
