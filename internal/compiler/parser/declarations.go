package parser

import (
	"github.com/vailang/vai/internal/compiler/ast"
	"github.com/vailang/vai/internal/compiler/lexer"
)

// parsePrompt parses a prompt definition.
// Syntax: prompt name { reference "path" ... body }
func (p *Parser) parsePrompt() *ast.PromptDecl {
	pos := tokenPos(p.current)
	p.expect(lexer.PROMPT)

	name := p.expect(lexer.IDENT)

	// Collect reference declarations before the body.
	var refs []string
	for p.current.Type == lexer.REFERENCE {
		refs = append(refs, p.parseReference())
	}

	p.expect(lexer.LBRACE)
	body := p.parseBody()
	p.expect(lexer.RBRACE)

	return &ast.PromptDecl{
		Name:       name.Val,
		Body:       body,
		References: refs,
		Pos:        pos,
	}
}

// parseConstraint parses a constraint block with an optional name and free text body.
// Syntax: constraint name { body } or constraint { body }
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
// Syntax: inject name or inject plan.impl (name can be an identifier or keyword)
func (p *Parser) parseInject() *ast.InjectDecl {
	pos := tokenPos(p.current)
	p.expect(lexer.INJECT)

	name := p.expectName()
	fullName := name.Val

	// Check for dotted name: inject plan.impl
	if p.current.Type == lexer.DOT {
		p.advance() // consume .
		part := p.expectName()
		fullName = fullName + "." + part.Val
	}

	return &ast.InjectDecl{
		Name: fullName,
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

// parseImpl parses an impl block with a name identifier and body.
// Syntax: impl name { body } or impl Type.method { body }
func (p *Parser) parseImpl() *ast.ImplDecl {
	pos := tokenPos(p.current)
	p.expect(lexer.IMPL)

	if p.current.Type != lexer.IDENT {
		p.errorf("impl name must be a valid identifier, got %s (%q)", p.current.Type, p.current.Val)
		p.synchronize()
		return &ast.ImplDecl{Pos: pos}
	}
	name := p.expect(lexer.IDENT)
	fullName := name.Val

	// Support dotted names: impl Type.method { ... }
	if p.current.Type == lexer.DOT {
		p.advance() // consume .
		part := p.expect(lexer.IDENT)
		fullName = fullName + "." + part.Val
	}

	// If the opening brace is missing, skip to recovery point.
	if p.current.Type != lexer.LBRACE {
		p.errorf("expected '{' after impl name, got %s (%q)", p.current.Type, p.current.Val)
		p.synchronize()
		return &ast.ImplDecl{Name: fullName, Pos: pos}
	}

	p.expect(lexer.LBRACE)
	body := p.parseBody()
	p.expect(lexer.RBRACE)

	return &ast.ImplDecl{
		Name: fullName,
		Body: body,
		Pos:  pos,
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

// parseReference parses a reference path.
// Syntax: reference "path/to/file"
func (p *Parser) parseReference() string {
	p.expect(lexer.REFERENCE)

	if p.current.Type != lexer.STRING {
		p.errorf("expected string after 'reference', got %s", p.current.Type)
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
		case lexer.REFERENCE:
			decl.References = append(decl.References, p.parseReference())
		default:
			p.errorf("unexpected token inside plan: %s (%q)", p.current.Type, p.current.Val)
			p.synchronize()
		}
	}

	p.expect(lexer.RBRACE)
	return decl
}
