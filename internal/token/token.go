package token

import "fmt"

// Kind is the lexical category of a token.
type Kind int

const (
	Illegal Kind = iota
	EOF

	// Literal-bearing
	Word    // identifier or keyword (Marco distinguishes by Capitalization, not lex class)
	String  // "foo" or 'foo'
	Number  // 42, 3.14
	Boolean // true / false

	// Punctuation
	Period     // .
	Bang       // !
	Question   // ?
	Comma      // ,
	Ellipsis   // ...
	Possessive // 's after a word (e.g. User's, that's)
	Plus       // +

	// Layout
	Indent  // virtual: deeper indentation than previous line
	Dedent  // virtual: shallower indentation than previous line
	Newline // end of a line of source
)

func (k Kind) String() string {
	switch k {
	case Illegal:
		return "illegal"
	case EOF:
		return "eof"
	case Word:
		return "word"
	case String:
		return "string"
	case Number:
		return "number"
	case Boolean:
		return "boolean"
	case Period:
		return "."
	case Bang:
		return "!"
	case Question:
		return "?"
	case Comma:
		return ","
	case Ellipsis:
		return "..."
	case Possessive:
		return "'s"
	case Plus:
		return "+"
	case Indent:
		return "INDENT"
	case Dedent:
		return "DEDENT"
	case Newline:
		return "NL"
	default:
		return fmt.Sprintf("kind(%d)", int(k))
	}
}

// Pos is a 1-indexed line/column position in source.
type Pos struct {
	Line int
	Col  int
}

func (p Pos) String() string { return fmt.Sprintf("%d:%d", p.Line, p.Col) }

// Token is one lexed unit.
type Token struct {
	Kind  Kind
	Value string // raw text; for String, with quotes stripped and escapes unprocessed
	Pos   Pos
}

func (t Token) String() string {
	if t.Value == "" {
		return fmt.Sprintf("%s@%s", t.Kind, t.Pos)
	}
	return fmt.Sprintf("%s(%q)@%s", t.Kind, t.Value, t.Pos)
}
