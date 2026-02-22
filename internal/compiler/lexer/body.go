package lexer

import (
	"strings"
	"unicode"
)

// bodyContext holds the mutable state for body mode scanning.
type bodyContext struct {
	depth         int
	textBuf       strings.Builder
	textStart     int
	textStartLine int
	textStartCol  int
}

// flushText emits accumulated text as a TEXT token, trimming whitespace.
func (bc *bodyContext) flushText(l *Lexer) {
	if bc.textBuf.Len() > 0 {
		content := strings.TrimSpace(bc.textBuf.String())
		if content != "" {
			l.tokens <- TokenInfo{
				Type:    TEXT,
				Pos:     bc.textStart,
				Val:     content,
				Line:    bc.textStartLine,
				Col:     bc.textStartCol,
				EndPos:  l.pos,
				EndLine: l.line,
				EndCol:  l.col,
			}
			l.lastToken = TEXT
		}
		bc.textBuf.Reset()
	}
	bc.textStart = l.pos
	bc.textStartLine = l.line
	bc.textStartCol = l.col
}

// skipWhitespace consumes spaces and tabs.
func (bc *bodyContext) skipWhitespace(l *Lexer) {
	for l.peek() == ' ' || l.peek() == '\t' {
		l.next()
	}
}

// skipToClosingBracket advances past the closing ].
func (bc *bodyContext) skipToClosingBracket(l *Lexer) {
	for {
		pr := l.next()
		if pr == ']' || pr == eof {
			break
		}
	}
}

// readIdentWithDots reads an identifier that may contain dots (e.g. "Foo.bar").
func (bc *bodyContext) readIdentWithDots(l *Lexer) string {
	var ident strings.Builder
	for {
		pr := l.peek()
		if !unicode.IsLetter(pr) && !unicode.IsDigit(pr) && pr != '_' {
			break
		}
		ident.WriteRune(l.next())
	}
	for l.peek() == '.' {
		ident.WriteRune(l.next())
		for {
			pr := l.peek()
			if !unicode.IsLetter(pr) && !unicode.IsDigit(pr) && pr != '_' {
				break
			}
			ident.WriteRune(l.next())
		}
	}
	return ident.String()
}

// emitDirective emits a directive token and resets positions.
func (bc *bodyContext) emitDirective(l *Lexer, tok Token, val string) {
	if val != "" {
		l.tokens <- TokenInfo{
			Type:    tok,
			Pos:     l.start,
			Val:     val,
			Line:    l.startLine,
			Col:     l.startCol,
			EndPos:  l.pos,
			EndLine: l.line,
			EndCol:  l.col,
		}
		l.lastToken = tok
	}
	l.start = l.pos
	l.startLine = l.line
	l.startCol = l.col
	bc.textStart = l.pos
	bc.textStartLine = l.line
	bc.textStartCol = l.col
}

// emitStructuralBrace emits a LBRACE or RBRACE at the current position.
func (bc *bodyContext) emitStructuralBrace(l *Lexer, tok Token) {
	l.tokens <- TokenInfo{
		Type:    tok,
		Pos:     l.pos - 1,
		Val:     string(tok.braceRune()),
		Line:    l.line,
		Col:     l.col - 1,
		EndPos:  l.pos,
		EndLine: l.line,
		EndCol:  l.col,
	}
	l.lastToken = tok
	l.start = l.pos
	l.startLine = l.line
	l.startCol = l.col
	bc.textStart = l.pos
	bc.textStartLine = l.line
	bc.textStartCol = l.col
}

// ---------------------------------------------------------------------------
// Directive handlers
// ---------------------------------------------------------------------------

// lexUseDirective handles [use identifier] directives.
func (bc *bodyContext) lexUseDirective(l *Lexer) {
	bc.flushText(l)
	for range len("use") {
		l.next()
	}
	bc.skipWhitespace(l)
	val := bc.readIdentWithDots(l)
	bc.skipToClosingBracket(l)
	bc.emitDirective(l, USE_REF, val)
}

// lexInjectDirective handles [inject name] directives.
func (bc *bodyContext) lexInjectDirective(l *Lexer) {
	bc.flushText(l)
	for range len("inject") {
		l.next()
	}
	bc.skipWhitespace(l)
	val := bc.readIdentWithDots(l)
	bc.skipToClosingBracket(l)
	bc.emitDirective(l, INJECT_REF, val)
}

// lexTargetDirective handles [target "path"] directives.
func (bc *bodyContext) lexTargetDirective(l *Lexer) {
	bc.flushText(l)
	for range len("target") {
		l.next()
	}
	bc.skipWhitespace(l)
	// read quoted string
	var path strings.Builder
	if l.peek() == '"' {
		l.next() // consume opening "
		for {
			pr := l.next()
			if pr == '"' || pr == eof {
				break
			}
			path.WriteRune(pr)
		}
	}
	bc.skipToClosingBracket(l)
	bc.emitDirective(l, TARGET_REF, path.String())
}

// lexReferenceDirective handles [reference "path"] and [reference plan_name] directives.
func (bc *bodyContext) lexReferenceDirective(l *Lexer) {
	bc.flushText(l)
	for range len("reference") {
		l.next()
	}
	bc.skipWhitespace(l)
	var val string
	if l.peek() == '"' {
		// Quoted string: [reference "path/to/file"]
		var path strings.Builder
		l.next() // consume opening "
		for {
			pr := l.next()
			if pr == '"' || pr == eof {
				break
			}
			path.WriteRune(pr)
		}
		val = path.String()
	} else {
		// Bare identifier: [reference plan_name]
		val = bc.readIdentWithDots(l)
	}
	bc.skipToClosingBracket(l)
	bc.emitDirective(l, REFERENCE_REF, val)
}

// lexMatchDirective handles [match field] directives.
func (bc *bodyContext) lexMatchDirective(l *Lexer) {
	bc.flushText(l)
	for range len("match") {
		l.next()
	}
	bc.skipWhitespace(l)
	val := bc.readIdentWithDots(l)
	bc.skipToClosingBracket(l)
	bc.emitDirective(l, MATCH_REF, val)
	l.matchDepth++
}

// lexCaseDirective handles [case "value"] and [case _] directives.
func (bc *bodyContext) lexCaseDirective(l *Lexer) {
	bc.flushText(l)
	for range len("case") {
		l.next()
	}
	bc.skipWhitespace(l)
	// read value: either "quoted" or _ (wildcard)
	var val strings.Builder
	if l.peek() == '"' {
		l.next() // consume opening "
		for {
			pr := l.next()
			if pr == '"' || pr == eof {
				break
			}
			val.WriteRune(pr)
		}
	} else if l.peek() == '_' {
		val.WriteRune(l.next())
	}
	bc.skipToClosingBracket(l)
	// CASE_REF always emits, even with empty val
	l.tokens <- TokenInfo{
		Type:    CASE_REF,
		Pos:     l.start,
		Val:     val.String(),
		Line:    l.startLine,
		Col:     l.startCol,
		EndPos:  l.pos,
		EndLine: l.line,
		EndCol:  l.col,
	}
	l.lastToken = CASE_REF
	l.start = l.pos
	l.startLine = l.line
	l.startCol = l.col
	bc.textStart = l.pos
	bc.textStartLine = l.line
	bc.textStartCol = l.col
}

// ---------------------------------------------------------------------------
// Body mode main loop
// ---------------------------------------------------------------------------

// lexBody scans body content between { }, emitting structured body tokens.
// It recognizes: [use X], [inject X], [target "path"], [match X], [case "val"], [case _], and raw TEXT.
func lexBody(l *Lexer) stateFn {
	bc := &bodyContext{
		depth:         1,
		textStart:     l.pos,
		textStartLine: l.line,
		textStartCol:  l.col,
	}

	for {
		r := l.next()
		if r == eof {
			l.start = l.pos
			l.startLine = l.line
			l.startCol = l.col
			return l.errorf("unterminated body block")
		}

		// Backslash escape: \[ → literal [, \] → literal ]
		if r == '\\' {
			nr := l.peek()
			if nr == '[' || nr == ']' {
				l.next()
				bc.textBuf.WriteRune(nr)
				continue
			} else if nr == '\\' {
				l.next()
				if l.peek() == '[' {
					bc.textBuf.WriteRune('\\')
					continue
				}
				bc.textBuf.WriteRune('\\')
				bc.textBuf.WriteRune('\\')
				continue
			}
			bc.textBuf.WriteRune(r)
			continue
		}

		// Bracket directive detection
		if r == '[' {
			word := l.peekWord()
			switch word {
			case "use":
				bc.lexUseDirective(l)
				continue
			case "inject":
				bc.lexInjectDirective(l)
				continue
			case "target":
				bc.lexTargetDirective(l)
				continue
			case "reference":
				bc.lexReferenceDirective(l)
				continue
			case "match":
				bc.lexMatchDirective(l)
				continue
			case "case":
				bc.lexCaseDirective(l)
				continue
			}
			// Not a recognized bracket keyword — treat as text
			bc.textBuf.WriteRune(r)
			continue
		}

		// Brace depth tracking
		if r == '{' {
			if l.matchDepth > 0 && !l.inCaseBody {
				enterCase := l.lastToken == CASE_REF
				bc.flushText(l)
				bc.emitStructuralBrace(l, LBRACE)
				if enterCase {
					l.inCaseBody = true
					l.caseBodyDepth = 0
				}
				continue
			}
			if l.inCaseBody {
				l.caseBodyDepth++
				bc.textBuf.WriteRune(r)
				continue
			}
			bc.depth++
			bc.textBuf.WriteRune(r)
			continue
		}
		if r == '}' {
			if l.inCaseBody {
				if l.caseBodyDepth > 0 {
					l.caseBodyDepth--
					bc.textBuf.WriteRune(r)
					continue
				}
				// Close case body
				bc.flushText(l)
				bc.emitStructuralBrace(l, RBRACE)
				l.inCaseBody = false
				continue
			}
			if l.matchDepth > 0 {
				bc.flushText(l)
				bc.emitStructuralBrace(l, RBRACE)
				l.matchDepth--
				continue
			}
			bc.depth--
			if bc.depth == 0 {
				bc.flushText(l)
				bc.emitStructuralBrace(l, RBRACE)
				return lexText
			}
			bc.textBuf.WriteRune(r)
			continue
		}

		bc.textBuf.WriteRune(r)
	}
}

// braceRune returns the rune for a brace token.
func (t Token) braceRune() rune {
	if t == LBRACE {
		return '{'
	}
	return '}'
}
