package parser

import (
	"github.com/vailang/vai/internal/compiler/ast"
	"github.com/vailang/vai/internal/compiler/lexer"
)

// parseBody parses body mode tokens between { and }.
func (p *Parser) parseBody() []ast.BodySegment {
	var segments []ast.BodySegment

	for p.current.Type != lexer.RBRACE && p.current.Type != lexer.EOF {
		switch p.current.Type {
		case lexer.TEXT:
			segments = append(segments, &ast.TextSegment{
				Content: p.current.Val,
				Pos:     tokenPos(p.current),
			})
			p.advance()

		case lexer.USE_REF:
			segments = append(segments, &ast.UseRefSegment{
				Name: p.current.Val,
				Pos:  tokenPos(p.current),
			})
			p.advance()

		case lexer.INJECT_REF:
			segments = append(segments, &ast.InjectRefSegment{
				Path: p.current.Val,
				Pos:  tokenPos(p.current),
			})
			p.advance()

		case lexer.TARGET_REF:
			segments = append(segments, &ast.TargetRefSegment{
				Name: p.current.Val,
				Pos:  tokenPos(p.current),
			})
			p.advance()

		case lexer.MATCH_REF:
			seg := p.parseMatchBlock()
			if seg != nil {
				segments = append(segments, seg)
			}

		default:
			p.errorf("unexpected token in body: %s (%q)", p.current.Type, p.current.Val)
			p.advance()
		}
	}

	return segments
}

// parseMatchBlock parses [match field] { [case "val"] { body } ... }.
// The current token is MATCH_REF.
func (p *Parser) parseMatchBlock() *ast.MatchSegment {
	seg := &ast.MatchSegment{
		Field: p.current.Val,
		Pos:   tokenPos(p.current),
	}
	p.advance() // consume MATCH_REF

	// Expect LBRACE to open match block
	if p.current.Type != lexer.LBRACE {
		p.errorf("expected '{' after [match %s], got %s", seg.Field, p.current.Type)
		return seg
	}
	p.advance() // consume {

	// Parse case clauses until closing RBRACE
	for p.current.Type != lexer.RBRACE && p.current.Type != lexer.EOF {
		if p.current.Type != lexer.CASE_REF {
			p.errorf("expected [case] inside match block, got %s (%q)", p.current.Type, p.current.Val)
			p.advance()
			continue
		}

		clause := &ast.CaseClause{
			Value: p.current.Val,
			Pos:   tokenPos(p.current),
		}
		p.advance() // consume CASE_REF

		// Expect LBRACE to open case body
		if p.current.Type != lexer.LBRACE {
			p.errorf("expected '{' after [case \"%s\"], got %s", clause.Value, p.current.Type)
			seg.Cases = append(seg.Cases, clause)
			continue
		}
		p.advance() // consume {

		// Parse case body segments until RBRACE
		clause.Body = p.parseBody()

		// Consume closing RBRACE of case
		if p.current.Type == lexer.RBRACE {
			p.advance()
		}

		seg.Cases = append(seg.Cases, clause)
	}

	// Consume closing RBRACE of match block
	if p.current.Type == lexer.RBRACE {
		p.advance()
	}

	return seg
}
