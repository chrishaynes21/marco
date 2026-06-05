// Package ast holds the sentence-tree shape that the parser produces.
//
// In Marco every line is a sentence. A sentence has an optional subject,
// a sequence of clause words / values, and a terminator that decides its
// effect (`.` `!` `?` `...`). A `...` terminator opens a phrase whose body
// is the indented block immediately below.
package ast

import "github.com/chaynes-simpleclouds/marco/internal/token"

// File is the parsed root of a single Marco source file.
type File struct {
	Body []Sentence
}

// Terminator is the punctuation that ends a sentence.
type Terminator int

const (
	TermPeriod   Terminator = iota // .
	TermBang                       // !
	TermQuestion                   // ?
	TermEllipsis                   // ...
)

// Sentence is one parsed line plus its phrase body (if it ended with `...`).
type Sentence struct {
	Parts []Part     // the word/value sequence in order
	Term  Terminator // sentence terminator
	Pos   token.Pos  // position of first token
	Body  []Sentence // populated when Term == TermEllipsis
}

// PartKind tags what kind of part appears in a sentence.
type PartKind int

const (
	PartWord       PartKind = iota // bare word; meaning resolved by parser/compiler
	PartString                     // string literal
	PartNumber                     // number literal
	PartBoolean                    // true / false
	PartPossessive                 // 's marker — connects previous and next parts
	PartComma                      // ,
	PartPlus                       // +
)

// Part is one cell in a sentence.
type Part struct {
	Kind  PartKind
	Value string
	Pos   token.Pos
}
