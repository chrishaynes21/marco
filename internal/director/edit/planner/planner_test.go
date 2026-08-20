package planner_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
	"github.com/chaynes-simpleclouds/marco/internal/director/edit/planner"
)

func TestParseEmitsOperationsRatherThanKeystrokes(t *testing.T) {
	cases := []struct {
		phrase string
		op     edit.OperationID
		text   string
		target string
		commit bool
	}{
		{phrase: "type hello into the search box", op: edit.OpSetText, text: "hello", target: "search box"},
		{phrase: "enter my name in the username field", op: edit.OpSetText, text: "my name", target: "username"},
		{phrase: `type "hello world" into the message box`, op: edit.OpSetText, text: "hello world", target: "message box"},
		{phrase: "clear the search box", op: edit.OpClearText, target: "search box"},
		{phrase: "empty the field", op: edit.OpClearText},
		{phrase: "add more text to the note", op: edit.OpAppendText, text: "more text", target: "note"},
		{phrase: "append world", op: edit.OpAppendText, text: "world"},
		{phrase: "replace that with goodbye", op: edit.OpReplaceSelection, text: "goodbye"},
		{phrase: "select all", op: edit.OpSelectAll},
		{phrase: "copy that", op: edit.OpCopySelection},
		{phrase: "paste", op: edit.OpPasteClipboard},
		{phrase: "undo that", op: edit.OpUndo},
		{phrase: "redo", op: edit.OpRedo},
		{phrase: "press enter", op: edit.OpPressEnter},
		{phrase: "type hello and press enter", op: edit.OpSetText, text: "hello", commit: true},
	}

	for _, c := range cases {
		t.Run(c.phrase, func(t *testing.T) {
			p, ok := planner.Parse(c.phrase)
			if !ok {
				t.Fatalf("%q was not recognised as an editing instruction", c.phrase)
			}
			if p.Operation.ID() != c.op {
				t.Fatalf("operation = %s, want %s", p.Operation.ID(), c.op)
			}
			if c.text != "" {
				got, _ := p.Operation.(edit.TextOperation)
				if got == nil || got.Text() != c.text {
					t.Fatalf("text = %q, want %q", textOf(p.Operation), c.text)
				}
			}
			if p.Target != c.target {
				t.Fatalf("target = %q, want %q", p.Target, c.target)
			}
			if p.Commit != c.commit {
				t.Fatalf("commit = %v, want %v", p.Commit, c.commit)
			}
		})
	}
}

func TestParseKeepsTheUsersOwnCapitalisation(t *testing.T) {
	// The text is the user's DATA. Lower-casing it to simplify parsing would be a
	// small, constant, entirely avoidable corruption of what they dictated.
	p, ok := planner.Parse(`type "Hello World" into the Message Box`)
	if !ok {
		t.Fatal("not recognised")
	}
	if got := textOf(p.Operation); got != "Hello World" {
		t.Fatalf("text = %q, want %q", got, "Hello World")
	}
}

func TestParseLeavesNonEditingPhrasesAlone(t *testing.T) {
	// Conservative on purpose: a phrase this package is not sure about must fall
	// through to the ordinary intent planner rather than be forced into an edit.
	for _, phrase := range []string{
		"open the file menu",
		"click the save button",
		"scroll down",
		"what is on my screen",
		"",
		"maximise discord",
	} {
		if p, ok := planner.Parse(phrase); ok {
			t.Fatalf("%q was wrongly read as the editing instruction %s", phrase, p.Operation.ID())
		}
	}
}

func TestParseTreatsAPronounAsTheFocusedControl(t *testing.T) {
	p, ok := planner.Parse("clear it")
	if !ok {
		t.Fatal("not recognised")
	}
	if p.Target != "" {
		t.Fatalf("target = %q — a pronoun is not a target phrase; it means the focused control", p.Target)
	}
}

func TestParseKeepsWholeTextWhenThereIsNoSeparator(t *testing.T) {
	// "type hello world" is ambiguous — two words of text, or a word and a target?
	// The honest answer is all of it, because guessing a split would silently drop
	// half of what the user dictated.
	p, ok := planner.Parse("type hello world")
	if !ok {
		t.Fatal("not recognised")
	}
	if got := textOf(p.Operation); got != "hello world" {
		t.Fatalf("text = %q, want the whole phrase kept", got)
	}
}

func textOf(op edit.Operation) string {
	if t, ok := op.(edit.TextOperation); ok {
		return t.Text()
	}
	return ""
}

func TestATrailingLocationWordIsNotTypedAsText(t *testing.T) {
	// "Type hello here" means type hello into the control I am pointing at. Keeping
	// "here" as text puts it in the user's document — a small silent corruption that is
	// hard to attribute later, because the request looked like it worked.
	for _, c := range []struct{ phrase, want string }{
		{"type hello here", "hello"},
		{"type hello in here", "hello"},
		{"append world here", "world"},
		{"put Alice into this field", "Alice"},
		{"type ${customer} here", "${customer}"},
	} {
		p, ok := planner.Parse(c.phrase)
		if !ok {
			t.Errorf("%q was not understood", c.phrase)
			continue
		}
		txt, has := p.Operation.(interface{ Text() string })
		if !has {
			t.Errorf("%q produced %s, which carries no text", c.phrase, p.Operation.ID())
			continue
		}
		if txt.Text() != c.want {
			t.Errorf("%q typed %q, want %q", c.phrase, txt.Text(), c.want)
		}
		// The location is gone rather than turned into a control to search for: an
		// empty target already means the focused one.
		if p.Target != "" {
			t.Errorf("%q targeted %q, want the focused control", c.phrase, p.Target)
		}
	}
	// A word that is genuinely part of the text survives.
	p, _ := planner.Parse(`type "meet me here"`)
	if txt, ok := p.Operation.(interface{ Text() string }); !ok || txt.Text() != "meet me here" {
		t.Errorf("quoted text lost its final word: %+v", p.Operation)
	}
}
