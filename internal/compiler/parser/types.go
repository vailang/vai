package parser

import (
	"fmt"

	"github.com/vailang/vai/internal/compiler/ast"
	"github.com/vailang/vai/internal/compiler/lexer"
)

// TokenStream is the interface for consuming tokens.
// The parser depends on this abstraction rather than directly on the lexer.
type TokenStream interface {
	NextToken() lexer.TokenInfo
}

// Error represents a parse error with position.
type Error struct {
	Msg string
	Pos ast.Position
}

func (e Error) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Pos.Line, e.Pos.Column, e.Msg)
}

// Parser is a recursive descent parser for Vai source files.
type Parser struct {
	streams   []TokenStream
	streamIdx int
	current   lexer.TokenInfo
	lookahead lexer.TokenInfo
	errors    []Error
}

// New creates a new Parser for the given token streams.
func New(streams ...TokenStream) *Parser {
	p := &Parser{
		streams: streams,
	}
	p.current = p.nextNonComment()
	p.lookahead = p.nextNonComment()
	return p
}

// nextNonComment reads the next token, skipping COMMENT tokens.
func (p *Parser) nextNonComment() lexer.TokenInfo {
	for {
		if p.streamIdx >= len(p.streams) {
			return lexer.TokenInfo{Type: lexer.EOF}
		}
		tok := p.streams[p.streamIdx].NextToken()
		if tok.Type == lexer.EOF {
			p.streamIdx++
			continue
		}
		if tok.Type != lexer.COMMENT {
			return tok
		}
	}
}

// advance moves to the next token (auto-skips comments).
func (p *Parser) advance() {
	p.current = p.lookahead
	p.lookahead = p.nextNonComment()
}

// expect consumes the current token if it matches, or records an error.
func (p *Parser) expect(t lexer.Token) lexer.TokenInfo {
	if p.current.Type != t {
		p.errorf("expected %s, got %s (%q)", t, p.current.Type, p.current.Val)
		return lexer.TokenInfo{}
	}
	tok := p.current
	p.advance()
	return tok
}



// errorf records a parse error at the current position.
func (p *Parser) errorf(format string, args ...any) {
	p.errors = append(p.errors, Error{
		Msg: fmt.Sprintf(format, args...),
		Pos: tokenPos(p.current),
	})
}

// synchronize skips tokens until a top-level keyword is found.
func (p *Parser) synchronize() {
	for p.current.Type != lexer.EOF && p.current.Type != lexer.ILLEGAL {
		switch p.current.Type {
		case lexer.PROMPT, lexer.PLAN, lexer.CONSTRAINT, lexer.INJECT, lexer.SPEC, lexer.IMPL, lexer.TARGET, lexer.REFERENCE:
			return
		}
		p.advance()
	}
}

// tokenPos converts a TokenInfo to an ast.Position.
func tokenPos(tok lexer.TokenInfo) ast.Position {
	return ast.Position{
		Line:   tok.Line,
		Column: tok.Col,
		Offset: tok.Pos,
	}
}

// expectName consumes an identifier or keyword token as a name.
// This allows keywords to be used as names in certain positions (e.g. inject).
func (p *Parser) expectName() lexer.TokenInfo {
	if p.current.Type == lexer.IDENT || p.current.Type.IsKeyword() {
		tok := p.current
		p.advance()
		return tok
	}
	p.errorf("expected name, got %s (%q)", p.current.Type, p.current.Val)
	return lexer.TokenInfo{}
}

// unquote removes surrounding double quotes from a string literal.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// ---------------------------------------------------------------------------
// ParseFile — main entry point
// ---------------------------------------------------------------------------

// ParseFile parses a complete Vai source and returns the AST and any errors.
func (p *Parser) ParseFile() (*ast.File, []Error) {
	file := &ast.File{}

	for p.current.Type != lexer.EOF && p.current.Type != lexer.ILLEGAL {
		switch p.current.Type {
		case lexer.CONSTRAINT:
			file.Declarations = append(file.Declarations, p.parseConstraint())
		case lexer.PROMPT:
			file.Declarations = append(file.Declarations, p.parsePrompt())
		case lexer.INJECT:
			file.Declarations = append(file.Declarations, p.parseInject())
		case lexer.PLAN:
			file.Declarations = append(file.Declarations, p.parsePlan())
		case lexer.SPEC, lexer.IMPL, lexer.TARGET, lexer.REFERENCE:
			p.errorf("%s is only valid inside a plan block", p.current.Type)
			p.advance()
			if p.current.Type == lexer.LBRACE {
				p.parseBody()
			}
		default:
			p.errorf("unexpected token: %s (%q)", p.current.Type, p.current.Val)
			p.synchronize()
		}
	}

	return file, p.errors
}
