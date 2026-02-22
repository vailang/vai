package lexer

// ---------------------------------------------------------------------------
// Normal mode state functions
// ---------------------------------------------------------------------------

// lexText is the initial state function, routing to other states.
func lexText(l *Lexer) stateFn {
	for {
		r := l.next()
		switch {
		case r == eof:
			l.emit(EOF)
			return nil
		case isSpace(r):
			l.ignore()
		case r == '/':
			next := l.peek()
			if next == '/' || next == '*' {
				l.backup()
				return lexComment
			}
			return l.errorf("unexpected character: /")
		case r == '"':
			l.backup()
			return lexString
		case r == '{':
			// Plan blocks use structured scope (declarations), not body mode
			if l.lastDeclKeyword == PLAN {
				l.emit(LBRACE)
				l.lastDeclKeyword = 0
				continue
			}
			l.lastDeclKeyword = 0
			// Check if this starts a body block
			if l.lastToken.IsBodyKeyword() || l.lastToken == PROMPT ||
				l.lastToken == IDENT || l.lastToken == STRING {
				l.emit(LBRACE)
				return lexBody
			}
			l.emit(LBRACE)
		case r == '}':
			l.emit(RBRACE)
		case r == '[':
			l.emit(LBRACK)
		case r == ']':
			l.emit(RBRACK)
		case r == '.':
			l.emit(DOT)
		case isAlphaNumeric(r) || r == '_':
			l.backup()
			return lexIdentifier
		default:
			return l.errorf("unexpected character: %c", r)
		}
	}
}

// lexIdentifier scans an identifier or keyword.
func lexIdentifier(l *Lexer) stateFn {
	for {
		r := l.next()
		if !isAlphaNumeric(r) && r != '_' {
			l.backup()
			break
		}
	}
	word := l.input[l.start:l.pos]
	tok := Lookup(word)
	l.emit(tok)
	return lexText
}

// lexString scans a regular string literal "...".
func lexString(l *Lexer) stateFn {
	l.next() // consume opening "
	for {
		r := l.next()
		if r == eof || r == '\n' {
			return l.errorf("unterminated string")
		}
		if r == '\\' {
			l.next()
			continue
		}
		if r == '"' {
			l.emit(STRING)
			return lexText
		}
	}
}

// lexComment scans a comment (single-line // or multi-line /* */).
func lexComment(l *Lexer) stateFn {
	l.next() // consume first /
	next := l.peek()

	switch next {
	case '/':

		l.next() // consume second /
		for {
			r := l.next()
			if r == eof {
				l.emit(COMMENT)
				return lexText
			}
			if r == '\n' {
				l.backup()
				l.emit(COMMENT)
				return lexText
			}
		}
	case '*':
		l.next() // consume *
		for {
			r := l.next()
			if r == eof {
				return l.errorf("unterminated comment")
			}
			if r == '*' && l.peek() == '/' {
				l.next() // consume /
				l.emit(COMMENT)
				return lexText
			}
		}
	}
	return l.errorf("unexpected character after /")
}
