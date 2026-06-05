// Package parser turns a token stream into a sentence tree.
//
// Marco's grammar at this layer is minimal: read parts until a terminator,
// then if the terminator is `...` collect the indented block as the body.
package parser

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/ast"
	"github.com/chaynes-simpleclouds/marco/internal/token"
)

type Error struct {
	Pos token.Pos
	Msg string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Pos, e.Msg) }

func Parse(toks []token.Token) (*ast.File, error) {
	p := &parser{toks: toks}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	if p.peek().Kind != token.EOF {
		return nil, &Error{Pos: p.peek().Pos, Msg: fmt.Sprintf("unexpected %s after top-level block", p.peek().Kind)}
	}
	return &ast.File{Body: body}, nil
}

type parser struct {
	toks []token.Token
	i    int
}

func (p *parser) peek() token.Token   { return p.toks[p.i] }
func (p *parser) peekAt(o int) token.Token {
	if p.i+o >= len(p.toks) {
		return token.Token{Kind: token.EOF}
	}
	return p.toks[p.i+o]
}
func (p *parser) advance() token.Token { t := p.toks[p.i]; p.i++; return t }

// parseBlock consumes sentences until DEDENT or EOF.
func (p *parser) parseBlock() ([]ast.Sentence, error) {
	var out []ast.Sentence
	for {
		// Skip stray newlines between sentences.
		for p.peek().Kind == token.Newline {
			p.advance()
		}
		switch p.peek().Kind {
		case token.EOF, token.Dedent:
			return out, nil
		}
		s, err := p.parseSentence()
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
}

func (p *parser) parseSentence() (ast.Sentence, error) {
	startPos := p.peek().Pos
	s := ast.Sentence{Pos: startPos}
	for {
		t := p.peek()
		switch t.Kind {
		case token.Word:
			s.Parts = append(s.Parts, ast.Part{Kind: ast.PartWord, Value: t.Value, Pos: t.Pos})
			p.advance()
		case token.String:
			s.Parts = append(s.Parts, ast.Part{Kind: ast.PartString, Value: t.Value, Pos: t.Pos})
			p.advance()
		case token.Number:
			s.Parts = append(s.Parts, ast.Part{Kind: ast.PartNumber, Value: t.Value, Pos: t.Pos})
			p.advance()
		case token.Boolean:
			s.Parts = append(s.Parts, ast.Part{Kind: ast.PartBoolean, Value: t.Value, Pos: t.Pos})
			p.advance()
		case token.Possessive:
			s.Parts = append(s.Parts, ast.Part{Kind: ast.PartPossessive, Value: "'s", Pos: t.Pos})
			p.advance()
		case token.Comma:
			s.Parts = append(s.Parts, ast.Part{Kind: ast.PartComma, Value: ",", Pos: t.Pos})
			p.advance()
		case token.Plus:
			s.Parts = append(s.Parts, ast.Part{Kind: ast.PartPlus, Value: "+", Pos: t.Pos})
			p.advance()
		case token.Period:
			p.advance()
			s.Term = ast.TermPeriod
			if err := p.endLine(); err != nil {
				return s, err
			}
			return s, nil
		case token.Bang:
			p.advance()
			s.Term = ast.TermBang
			if err := p.endLine(); err != nil {
				return s, err
			}
			return s, nil
		case token.Question:
			p.advance()
			s.Term = ast.TermQuestion
			if err := p.endLine(); err != nil {
				return s, err
			}
			// `?` may open an indented body (branch handler / listener).
			if p.peek().Kind == token.Indent {
				body, err := p.parseBody()
				if err != nil {
					return s, err
				}
				s.Body = body
			}
			return s, nil
		case token.Ellipsis:
			p.advance()
			s.Term = ast.TermEllipsis
			if err := p.endLine(); err != nil {
				return s, err
			}
			body, err := p.parseBody()
			if err != nil {
				return s, err
			}
			s.Body = body
			return s, nil
		default:
			return s, &Error{Pos: t.Pos, Msg: fmt.Sprintf("unexpected %s in sentence", t.Kind)}
		}
	}
}

func (p *parser) endLine() error {
	if p.peek().Kind != token.Newline {
		return &Error{Pos: p.peek().Pos, Msg: fmt.Sprintf("expected newline, got %s", p.peek().Kind)}
	}
	p.advance()
	return nil
}

func (p *parser) parseBody() ([]ast.Sentence, error) {
	if p.peek().Kind != token.Indent {
		return nil, &Error{Pos: p.peek().Pos, Msg: "expected indented block after `...`"}
	}
	p.advance()
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	if p.peek().Kind != token.Dedent {
		return nil, &Error{Pos: p.peek().Pos, Msg: fmt.Sprintf("expected DEDENT, got %s", p.peek().Kind)}
	}
	p.advance()
	return body, nil
}
