// Package lexer turns Marco source text into a stream of tokens.
//
// Marco is line-oriented: every line is a sentence, and indentation under a
// `...` opener creates a phrase body. The lexer tracks indentation and emits
// virtual INDENT / DEDENT / NEWLINE tokens so the parser can consume a flat
// stream.
package lexer

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/chaynes-simpleclouds/marco/internal/token"
)

type Error struct {
	Pos token.Pos
	Msg string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Pos, e.Msg) }

// Lex tokenizes src. The returned slice always ends with an EOF token.
func Lex(src string) ([]token.Token, error) {
	l := &lex{
		src:     src,
		line:    1,
		col:     1,
		indents: []int{0},
	}
	if err := l.run(); err != nil {
		return nil, err
	}
	return l.out, nil
}

type lex struct {
	src     string
	pos     int
	line    int
	col     int
	out     []token.Token
	indents []int // stack of indent column widths; bottom is 0
}

func (l *lex) run() error {
	for l.pos < len(l.src) {
		if err := l.line_(); err != nil {
			return err
		}
	}
	// Close any open indents.
	for len(l.indents) > 1 {
		l.indents = l.indents[:len(l.indents)-1]
		l.emit(token.Dedent, "", l.curPos())
	}
	l.emit(token.EOF, "", l.curPos())
	return nil
}

func (l *lex) line_() error {
	// Measure indent.
	indent := 0
	startPos := l.curPos()
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == ' ' {
			indent++
			l.advance()
			continue
		}
		if c == '\t' {
			return &Error{Pos: l.curPos(), Msg: "tabs are not permitted; use spaces"}
		}
		break
	}

	// Blank or comment-only line: skip without emitting indent change.
	if l.atLineEnd() || l.atComment() {
		l.skipToLineEnd()
		l.consumeNewline()
		return nil
	}

	// Emit indent / dedent relative to stack.
	top := l.indents[len(l.indents)-1]
	switch {
	case indent > top:
		l.indents = append(l.indents, indent)
		l.emit(token.Indent, "", startPos)
	case indent < top:
		for len(l.indents) > 0 && l.indents[len(l.indents)-1] > indent {
			l.indents = l.indents[:len(l.indents)-1]
			l.emit(token.Dedent, "", startPos)
		}
		if l.indents[len(l.indents)-1] != indent {
			return &Error{Pos: startPos, Msg: "indentation does not match any enclosing level"}
		}
	}

	// Read tokens on the line.
	for l.pos < len(l.src) {
		if l.atComment() {
			l.skipToLineEnd()
			break
		}
		if l.atLineEnd() {
			break
		}
		c := l.src[l.pos]
		if c == ' ' {
			l.advance()
			continue
		}
		if err := l.readToken(); err != nil {
			return err
		}
	}

	l.emit(token.Newline, "", l.curPos())
	l.consumeNewline()
	return nil
}

func (l *lex) readToken() error {
	startPos := l.curPos()
	c := l.src[l.pos]

	switch {
	case isWordStart(c):
		word := l.readWord()
		switch word {
		case "true", "false":
			l.emitAt(token.Boolean, word, startPos)
		default:
			l.emitAt(token.Word, word, startPos)
		}
		// Possessive after a word: `'s` followed by non-word char.
		if l.pos+1 < len(l.src) && l.src[l.pos] == '\'' && l.src[l.pos+1] == 's' {
			if l.pos+2 >= len(l.src) || !isWordContinue(l.src[l.pos+2]) {
				possPos := l.curPos()
				l.advance() // '
				l.advance() // s
				l.emitAt(token.Possessive, "'s", possPos)
			}
		}
		return nil

	case c >= '0' && c <= '9':
		num := l.readNumber()
		l.emitAt(token.Number, num, startPos)
		return nil

	case c == '"':
		s, err := l.readString('"')
		if err != nil {
			return err
		}
		l.emitAt(token.String, s, startPos)
		return nil

	case c == '\'':
		// Possessive should already have been consumed inline after a word.
		// A bare apostrophe here opens a string literal.
		s, err := l.readString('\'')
		if err != nil {
			return err
		}
		l.emitAt(token.String, s, startPos)
		return nil

	case c == '.':
		if l.pos+2 < len(l.src) && l.src[l.pos+1] == '.' && l.src[l.pos+2] == '.' {
			l.advance()
			l.advance()
			l.advance()
			l.emitAt(token.Ellipsis, "...", startPos)
			return nil
		}
		l.advance()
		l.emitAt(token.Period, ".", startPos)
		return nil

	case c == '!':
		l.advance()
		l.emitAt(token.Bang, "!", startPos)
		return nil

	case c == '?':
		l.advance()
		l.emitAt(token.Question, "?", startPos)
		return nil

	case c == ',':
		l.advance()
		l.emitAt(token.Comma, ",", startPos)
		return nil

	case c == '+':
		l.advance()
		l.emitAt(token.Plus, "+", startPos)
		return nil
	}

	return &Error{Pos: startPos, Msg: fmt.Sprintf("unexpected character %q", string(c))}
}

func (l *lex) readWord() string {
	start := l.pos
	for l.pos < len(l.src) && isWordContinue(l.src[l.pos]) {
		l.advance()
	}
	return l.src[start:l.pos]
}

func (l *lex) readNumber() string {
	start := l.pos
	for l.pos < len(l.src) && (l.src[l.pos] >= '0' && l.src[l.pos] <= '9') {
		l.advance()
	}
	if l.pos < len(l.src) && l.src[l.pos] == '.' &&
		l.pos+1 < len(l.src) && l.src[l.pos+1] >= '0' && l.src[l.pos+1] <= '9' {
		l.advance() // .
		for l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '9' {
			l.advance()
		}
	}
	return l.src[start:l.pos]
}

func (l *lex) readString(quote byte) (string, error) {
	startPos := l.curPos()
	l.advance() // opening quote
	var b strings.Builder
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == quote {
			l.advance()
			return b.String(), nil
		}
		if c == '\n' {
			return "", &Error{Pos: startPos, Msg: "unterminated string literal"}
		}
		if c == '\\' {
			if l.pos+1 >= len(l.src) {
				return "", &Error{Pos: l.curPos(), Msg: "trailing backslash"}
			}
			l.advance()
			esc := l.src[l.pos]
			switch esc {
			case '"':
				b.WriteByte('"')
			case '\'':
				b.WriteByte('\'')
			case '\\':
				b.WriteByte('\\')
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			default:
				return "", &Error{Pos: l.curPos(), Msg: fmt.Sprintf("invalid escape \\%c", esc)}
			}
			l.advance()
			continue
		}
		b.WriteByte(c)
		l.advance()
	}
	return "", &Error{Pos: startPos, Msg: "unterminated string literal"}
}

func (l *lex) atComment() bool {
	return l.pos+1 < len(l.src) && l.src[l.pos] == '/' && l.src[l.pos+1] == '/'
}

func (l *lex) atLineEnd() bool {
	if l.pos >= len(l.src) {
		return true
	}
	c := l.src[l.pos]
	return c == '\n' || c == '\r'
}

func (l *lex) skipToLineEnd() {
	for l.pos < len(l.src) && l.src[l.pos] != '\n' && l.src[l.pos] != '\r' {
		l.advance()
	}
}

func (l *lex) consumeNewline() {
	if l.pos < len(l.src) && l.src[l.pos] == '\r' {
		l.advance()
	}
	if l.pos < len(l.src) && l.src[l.pos] == '\n' {
		l.pos++
		l.line++
		l.col = 1
	}
}

func (l *lex) advance() {
	l.pos++
	l.col++
}

func (l *lex) curPos() token.Pos {
	return token.Pos{Line: l.line, Col: l.col}
}

func (l *lex) emit(k token.Kind, val string, pos token.Pos) {
	l.out = append(l.out, token.Token{Kind: k, Value: val, Pos: pos})
}

func (l *lex) emitAt(k token.Kind, val string, pos token.Pos) {
	l.out = append(l.out, token.Token{Kind: k, Value: val, Pos: pos})
}

func isWordStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= 128 && unicode.IsLetter(rune(c)))
}

func isWordContinue(c byte) bool {
	return isWordStart(c) || (c >= '0' && c <= '9')
}
