package lexer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// stateFn represents the state of the scanner as a function that returns the next state.
type stateFn func(*Lexer) stateFn

// Lexer holds the state of the scanner.
type Lexer struct {
	input     string   // Input string being scanned
	start     int      // Start position of current token
	pos       int      // Current position in input
	width     int      // Width of last rune read
	tokens     chan TokenInfo // Channel of scanned items
	line      int      // Current line number (1-indexed)
	col       int      // Current column number (1-indexed, rune-based)
	startLine int      // Line number where current token started
	startCol  int      // Column number where current token started
	lastToken Token    // Last emitted token (for context-aware lexing)

	lastDeclKeyword Token // Tracks the last declaration keyword emitted (for plan/impl scope)

	// Match/case mode support
	matchDepth    int  // >0 when inside [match] blocks (supports nesting)
	inCaseBody    bool // true when scanning text inside a case body
	caseBodyDepth int  // brace depth for nested {} inside case body text
}

const eof = -1

// New creates a new lexer for the input string.
func New(input string) *Lexer {
	return NewAt(input, 1)
}

// NewAt creates a new lexer with a custom starting line number.
func NewAt(input string, startLine int) *Lexer {
	l := &Lexer{
		input:     input,
		tokens:     make(chan TokenInfo),
		line:      startLine,
		col:       1,
		startLine: startLine,
		startCol:  1,
	}
	go l.run()
	return l
}

// NextToken returns the next item from the input.
func (l *Lexer) NextToken() TokenInfo {
	return <-l.tokens
}

// run executes the state machine for the lexer.
func (l *Lexer) run() {
	for state := lexText; state != nil; {
		state = state(l)
	}
	close(l.tokens)
}

// ---------------------------------------------------------------------------
// Core operations
// ---------------------------------------------------------------------------

// next returns the next rune in the input.
func (l *Lexer) next() rune {
	if l.pos >= len(l.input) {
		l.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = w
	l.pos += l.width
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return r
}

// peek returns but does not consume the next rune in the input.
func (l *Lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune (can only be called once per call of next).
func (l *Lexer) backup() {
	l.pos -= l.width
	if l.width == 0 {
		return // EOF didn't advance pos or col, nothing to undo
	}
	if l.width == 1 && l.pos < len(l.input) && l.input[l.pos] == '\n' {
		l.line--
		// Recompute column: scan from start of this line
		l.col = l.computeCol(l.pos)
	} else {
		l.col--
	}
}

// computeCol computes the column number for a given byte position.
func (l *Lexer) computeCol(pos int) int {
	col := 1
	lineStart := strings.LastIndex(l.input[:pos], "\n")
	if lineStart == -1 {
		lineStart = 0
	} else {
		lineStart++
	}
	i := lineStart
	for i < pos {
		_, w := utf8.DecodeRuneInString(l.input[i:])
		i += w
		col++
	}
	return col
}

// ---------------------------------------------------------------------------
// Emission
// ---------------------------------------------------------------------------

// emit passes an item back to the client.
func (l *Lexer) emit(t Token) {
	l.tokens <- TokenInfo{
		Type:    t,
		Pos:     l.start,
		Val:     l.input[l.start:l.pos],
		Line:    l.startLine,
		Col:     l.startCol,
		EndPos:  l.pos,
		EndLine: l.line,
		EndCol:  l.col,
	}
	l.lastToken = t
	if t.IsKeyword() {
		l.lastDeclKeyword = t
	}
	l.start = l.pos
	l.startLine = l.line
	l.startCol = l.col
}


// ignore skips over the pending input before this point.
func (l *Lexer) ignore() {
	l.start = l.pos
	l.startLine = l.line
	l.startCol = l.col
}

// errorf returns an error token and terminates the scan.
func (l *Lexer) errorf(format string, args ...interface{}) stateFn {
	l.tokens <- TokenInfo{
		Type:    ILLEGAL,
		Pos:     l.start,
		Val:     fmt.Sprintf(format, args...),
		Line:    l.startLine,
		Col:     l.startCol,
		EndPos:  l.pos,
		EndLine: l.line,
		EndCol:  l.col,
	}
	return nil
}

// peekWord peeks ahead to read a word without consuming input.
func (l *Lexer) peekWord() string {
	i := l.pos
	for i < len(l.input) {
		r, w := utf8.DecodeRuneInString(l.input[i:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			break
		}
		i += w
	}
	return l.input[l.pos:i]
}

// ---------------------------------------------------------------------------
// Character classification helpers
// ---------------------------------------------------------------------------

// isSpace reports whether r is a space character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}



// isAlphaNumeric reports whether r is an alphabetic or digit.
func isAlphaNumeric(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
