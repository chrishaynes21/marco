package lexer

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/token"
)

func dump(toks []token.Token) string {
	var b strings.Builder
	for _, t := range toks {
		switch t.Kind {
		case token.Newline:
			b.WriteString("NL\n")
		case token.Indent:
			b.WriteString("INDENT\n")
		case token.Dedent:
			b.WriteString("DEDENT\n")
		case token.EOF:
			b.WriteString("EOF\n")
		default:
			b.WriteString(t.Kind.String())
			if t.Value != "" && t.Value != t.Kind.String() {
				b.WriteString("(")
				b.WriteString(t.Value)
				b.WriteString(")")
			}
			b.WriteString(" ")
		}
	}
	return b.String()
}

func TestSimpleSentence(t *testing.T) {
	got, err := Lex("the User is a set.\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "word(the) word(User) word(is) word(a) word(set) . NL\nEOF\n"
	if dump(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", dump(got), want)
	}
}

func TestPossessive(t *testing.T) {
	got, err := Lex("User's Name is \"Chris\".\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "word(User) 's word(Name) word(is) string(Chris) . NL\nEOF\n"
	if dump(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", dump(got), want)
	}
}

func TestStringSingleQuote(t *testing.T) {
	got, err := Lex("log 'hello'.\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "word(log) string(hello) . NL\nEOF\n"
	if dump(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", dump(got), want)
	}
}

func TestEscapes(t *testing.T) {
	got, err := Lex(`log "a\"b\\c\n".` + "\n")
	if err != nil {
		t.Fatal(err)
	}
	if got[1].Value != "a\"b\\c\n" {
		t.Fatalf("got %q", got[1].Value)
	}
}

func TestEllipsis(t *testing.T) {
	got, err := Lex("does...\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "word(does) ... NL\nEOF\n"
	if dump(got) != want {
		t.Fatalf("got:\n%s", dump(got))
	}
}

func TestIndentDedent(t *testing.T) {
	src := "this's Save does...\n    this is ok!\n"
	got, err := Lex(src)
	if err != nil {
		t.Fatal(err)
	}
	want := "word(this) 's word(Save) word(does) ... NL\nINDENT\nword(this) word(is) word(ok) ! NL\nDEDENT\nEOF\n"
	if dump(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", dump(got), want)
	}
}

func TestNestedIndent(t *testing.T) {
	src := "a does...\n    b does...\n        c.\n    d.\ne.\n"
	got, err := Lex(src)
	if err != nil {
		t.Fatal(err)
	}
	want := "word(a) word(does) ... NL\nINDENT\nword(b) word(does) ... NL\nINDENT\nword(c) . NL\nDEDENT\nword(d) . NL\nDEDENT\nword(e) . NL\nEOF\n"
	if dump(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", dump(got), want)
	}
}

func TestComments(t *testing.T) {
	src := "// header comment\nthe X is a set. // trailing\n"
	got, err := Lex(src)
	if err != nil {
		t.Fatal(err)
	}
	want := "word(the) word(X) word(is) word(a) word(set) . NL\nEOF\n"
	if dump(got) != want {
		t.Fatalf("got:\n%s", dump(got))
	}
}

func TestBoolean(t *testing.T) {
	got, err := Lex("its Active is true.\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "word(its) word(Active) word(is) boolean(true) . NL\nEOF\n"
	if dump(got) != want {
		t.Fatalf("got:\n%s", dump(got))
	}
}

func TestNumber(t *testing.T) {
	got, err := Lex("its Age is 30.\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "word(its) word(Age) word(is) number(30) . NL\nEOF\n"
	if dump(got) != want {
		t.Fatalf("got:\n%s", dump(got))
	}
}

func TestRejectsTabs(t *testing.T) {
	_, err := Lex("a\n\tb\n")
	if err == nil {
		t.Fatal("expected tab error")
	}
}
