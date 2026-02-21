package parser

import (
	"github.com/vailang/vai/internal/compiler/ast"
	"github.com/vailang/vai/internal/compiler/lexer"
)

// parsePrompt parses a prompt definition.
// Syntax: prompt name { body }
func (p *Parser) parsePrompt() *ast.PromptDecl {
	pos := tokenPos(p.current)
	p.expect(lexer.PROMPT)

	name := p.expect(lexer.IDENT)

	p.expect(lexer.LBRACE)
	body := p.parseBody()
	p.expect(lexer.RBRACE)

	return &ast.PromptDecl{
		Name: name.Val,
		Body: body,
		Pos:  pos,
	}
}

// parseConstraint parses a constraint block with an optional name and free text body.
// Syntax: constraint name { free text } or constraint { free text }
func (p *Parser) parseConstraint() *ast.ConstraintDecl {
	pos := tokenPos(p.current)
	p.expect(lexer.CONSTRAINT)

	var name string
	if p.current.Type != lexer.LBRACE {
		name = p.expect(lexer.IDENT).Val
	}

	p.expect(lexer.LBRACE)
	body := p.parseBody()
	p.expect(lexer.RBRACE)

	return &ast.ConstraintDecl{
		Name: name,
		Body: body,
		Pos:  pos,
	}
}

// parseInject parses a standalone inject statement.
// Syntax: inject name (name can be an identifier or keyword like "plan")
func (p *Parser) parseInject() *ast.InjectDecl {
	pos := tokenPos(p.current)
	p.expect(lexer.INJECT)

	name := p.expectName()

	return &ast.InjectDecl{
		Name: name.Val,
		Pos:  pos,
	}
}

// parseSpec parses a spec block (no name, free text body, only inside plan).
// Syntax: spec { free text }
func (p *Parser) parseSpec() *ast.SpecDecl {
	pos := tokenPos(p.current)
	p.expect(lexer.SPEC)

	p.expect(lexer.LBRACE)
	body := p.parseBody()
	p.expect(lexer.RBRACE)

	return &ast.SpecDecl{
		Body: body,
		Pos:  pos,
	}
}

// parseImpl parses an impl block with a signature string and body.
// Syntax: impl "signature" { body }
func (p *Parser) parseImpl() *ast.ImplDecl {
	pos := tokenPos(p.current)
	p.expect(lexer.IMPL)

	sig := unquote(p.expect(lexer.STRING).Val)

	p.expect(lexer.LBRACE)
	body := p.parseBody()
	p.expect(lexer.RBRACE)

	return &ast.ImplDecl{
		Signature: sig,
		Body:      body,
		Pos:       pos,
	}
}

// parseTarget parses a target path inside a plan block.
// Syntax: target "path/to/file"
func (p *Parser) parseTarget() string {
	p.expect(lexer.TARGET)

	if p.current.Type != lexer.STRING {
		p.errorf("expected string after 'target', got %s", p.current.Type)
		return ""
	}

	path := unquote(p.current.Val)
	p.advance()
	return path
}

// parsePlan parses a plan declaration containing structured declarations.
// Syntax: plan Name { declarations... }
func (p *Parser) parsePlan() *ast.PlanDecl {
	pos := tokenPos(p.current)
	p.expect(lexer.PLAN)

	name := p.expect(lexer.IDENT)

	if p.current.Type != lexer.LBRACE {
		p.errorf("plan declaration requires a body block")
		return &ast.PlanDecl{Name: name.Val, Pos: pos}
	}

	p.expect(lexer.LBRACE)

	decl := &ast.PlanDecl{Name: name.Val, Pos: pos}

	for p.current.Type != lexer.RBRACE && p.current.Type != lexer.EOF && p.current.Type != lexer.ILLEGAL {
		switch p.current.Type {
		case lexer.SPEC:
			decl.Specs = append(decl.Specs, p.parseSpec())
		case lexer.PROMPT:
			decl.Declarations = append(decl.Declarations, p.parsePrompt())
		case lexer.CONSTRAINT:
			decl.Constraints = append(decl.Constraints, p.parseConstraint())
		case lexer.IMPL:
			decl.Impls = append(decl.Impls, p.parseImpl())
		case lexer.INJECT:
			decl.Declarations = append(decl.Declarations, p.parseInject())
		case lexer.TARGET:
			decl.Targets = append(decl.Targets, p.parseTarget())
		default:
			p.errorf("unexpected token inside plan: %s (%q)", p.current.Type, p.current.Val)
			p.synchronize()
		}
	}

	p.expect(lexer.RBRACE)
	return decl
}
