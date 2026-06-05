package graph

import "github.com/chaynes-simpleclouds/marco/internal/ast"

// Expr is a small expression AST used on the right-hand side of assignments
// and other value positions. The runtime evaluates it to a Value.
type Expr interface{ isExpr() }

// LiteralExpr is a primitive literal (text, number, boolean).
type LiteralExpr struct {
	Kind ast.PartKind // PartString | PartNumber | PartBoolean
	Text string       // raw value as written
	Pos  ast.Sentence
}

func (LiteralExpr) isExpr() {}

// RefExpr is a possessive reference chain like ["this", "State"] or
// ["that", "error"] or ["User", "Name"].
type RefExpr struct {
	Path []string
	Pos  ast.Sentence
}

func (RefExpr) isExpr() {}

// ErrorLiteralExpr is `error "<message>"` — produces an Err value.
type ErrorLiteralExpr struct {
	Message string
	Pos     ast.Sentence
}

func (ErrorLiteralExpr) isExpr() {}

// ConstructExpr is `a <Type>` or `a <Type> with <Field> <Expr>, ...` —
// produces a Set value of the named type.
type ConstructExpr struct {
	Type   *Node
	Fields []ConstructField
	Pos    ast.Sentence
}

func (ConstructExpr) isExpr() {}

type ConstructField struct {
	Name string
	Expr Expr
}

// BinaryExpr is `<L> + <R>` — text concatenation or numeric addition.
type BinaryExpr struct {
	Op string // "+"
	L  Expr
	R  Expr
}

func (BinaryExpr) isExpr() {}

// ListAtExpr is `<List> at <Index>` — indexed access on a list.
type ListAtExpr struct {
	List  Expr
	Index Expr
}

func (ListAtExpr) isExpr() {}

// ListExpr is `<a> and <b> [and <c>]*` — used in invocation `with` clauses
// to pass multiple positional arguments. Evaluates to a List value.
type ListExpr struct {
	Items []Expr
}

func (ListExpr) isExpr() {}

// CastExpr is `<expr> as a <Type>` — coerces or translates the source value
// into the target type. The runtime: returns the source unchanged when its
// declared type already matches; otherwise looks for a translator declared
// from the source type to Target (canonically `<Src>To<Target>`) and invokes
// it; otherwise errors.
//
// When Partial is true (`as a partial <Type>`), the result is a bridge value
// of Target type, with fields copied from the source where names match.
// Bridges allow incomplete construction; promotion to a full Target requires
// `<expr> as a <Type>` (Partial=false), which checks declared-required fields.
type CastExpr struct {
	Source  Expr
	Target  *Node
	Partial bool
	Pos     ast.Sentence
}

func (CastExpr) isExpr() {}
