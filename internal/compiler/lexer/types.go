package lexer

// CodeSource is the input interface for the lexer.
// Implemented by the reader package's vaiSource and hostBlock types.
type CodeSource interface {
	GetCode() string
	GetOffset() uint
	IsVaiCode() bool
}

// Scanner is the public interface for lexical analysis.
// It tokenizes Vai source code into a stream of tokens.
type Scanner interface {
	// NextToken returns the next token from the input.
	// It blocks until a token is available.
	// Returns EOF token when input is exhausted.
	// Returns ILLEGAL token on lexical errors.
	NextToken() TokenInfo
}

// TokenInfo represents a lexical token with full position metadata.
// All position fields are populated for every token, enabling precise
// error messages that point to exact source locations.
type TokenInfo struct {
	Type    Token  // Token type (keyword, identifier, operator, etc.)
	Pos     int    // Starting byte position in source (0-indexed)
	Val     string // Literal value of the token
	Line    int    // Start line number (1-indexed)
	Col     int    // Start column number (1-indexed, rune-based)
	EndPos  int    // End byte position (exclusive)
	EndLine int    // End line number (1-indexed)
	EndCol  int    // End column number (1-indexed, rune-based)
}

// scanner implements the Scanner interface.
// It wraps the internal Lexer implementation.
type scanner struct {
	lexer *Lexer
}

// NewScanner creates a new Scanner from a CodeSource.
// The offset from CodeSource.GetOffset() is used as the starting line number.
// Offset 0 is treated as line 1 (default for .vai files).
func NewScanner(src CodeSource) Scanner {
	offset := int(src.GetOffset())
	if offset == 0 {
		offset = 1
	}
	return &scanner{
		lexer: NewAt(src.GetCode(), offset),
	}
}

// NextToken implements Scanner.NextToken.
func (s *scanner) NextToken() TokenInfo {
	item := s.lexer.NextToken()
	return TokenInfo{
		Type:    item.Type,
		Pos:     item.Pos,
		Val:     item.Val,
		Line:    item.Line,
		Col:     item.Col,
		EndPos:  item.EndPos,
		EndLine: item.EndLine,
		EndCol:  item.EndCol,
	}
}

// TokenType returns a human-readable string representation of a token type.
//
// Example:
//
//	typeStr := lexer.TokenType(lexer.FUNC)  // Returns "FUNC"
func TokenType(t Token) string {
	return t.String()
}

// IsKeyword checks if the given identifier string is a reserved keyword.
//
// Example:
//
//	lexer.IsKeyword("func")    // Returns true
//	lexer.IsKeyword("myVar")   // Returns false
func IsKeyword(ident string) bool {
	_, ok := keywords[ident]
	return ok
}
