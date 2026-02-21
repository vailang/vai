package lexer

// Token represents a lexical token type in the Vai language.
type Token int

const (
	// Special tokens
	ILLEGAL Token = iota // Illegal/unknown character
	EOF                  // End of file
	COMMENT              // Comment: // or /* */

	// Literals (grouped by range markers)
	literal_beg
	IDENT  // Identifier: add, greet, MyFunc
	STRING // String literal: "text"
	literal_end

	// Body mode tokens
	body_beg
	TEXT       // Raw text inside body mode
	USE_REF    // [use identifier] inside body mode
	INJECT_REF // [inject name] inside body mode
	MATCH_REF  // [match field] inside body mode
	CASE_REF   // [case "value"] or [case _] inside body mode
	TARGET_REF // [target "path"] inside body mode
	body_end

	// Operators and delimiters (grouped by range markers)
	operator_beg
	LBRACE // {
	RBRACE // }
	LBRACK // [
	RBRACK // ]
	operator_end

	// Keywords (grouped by range markers)
	keyword_beg
	PROMPT     // prompt (reusable prompt definitions)
	INJECT     // inject (inject/print/execute)
	PLAN       // plan (structured scope with declarations)
	CONSTRAINT // constraint (inherited constraints)
	SPEC       // spec (natural language description inside plan)
	IMPL       // impl (implementation block inside plan)
	TARGET     // target (output file path inside plan)
	keyword_end
)

// keywords maps keyword strings to their Token type.
var keywords = map[string]Token{
	"prompt":     PROMPT,
	"inject":     INJECT,
	"plan":       PLAN,
	"constraint": CONSTRAINT,
	"spec":       SPEC,
	"impl":       IMPL,
	"target":     TARGET,
}

// tokens maps Token types to their string representation for debugging.
var tokens = [...]string{
	ILLEGAL: "ILLEGAL",
	EOF:     "EOF",
	COMMENT: "COMMENT",

	IDENT:  "IDENT",
	STRING: "STRING",

	TEXT:       "TEXT",
	USE_REF:    "USE_REF",
	INJECT_REF: "INJECT_REF",
	MATCH_REF:  "MATCH_REF",
	CASE_REF:   "CASE_REF",
	TARGET_REF: "TARGET_REF",

	LBRACE: "LBRACE",
	RBRACE: "RBRACE",
	LBRACK: "LBRACK",
	RBRACK: "RBRACK",

	PROMPT:     "PROMPT",
	INJECT:     "INJECT",
	PLAN:       "PLAN",
	CONSTRAINT: "CONSTRAINT",
	SPEC:       "SPEC",
	IMPL:       "IMPL",
	TARGET:     "TARGET",
}

// IsBodyKeyword returns true if the token is a keyword that expects a body block
// parsed in body mode (structured tokens: TEXT, USE_REF, INJECT_REF, etc.).
func (t Token) IsBodyKeyword() bool {
	return t == PROMPT || t == CONSTRAINT || t == SPEC
}

// String returns the string representation of the token.
func (t Token) String() string {
	if t >= 0 && int(t) < len(tokens) {
		return tokens[t]
	}
	return "UNKNOWN"
}

// IsLiteral returns true if the token is a literal (identifier, string).
func (t Token) IsLiteral() bool {
	return t > literal_beg && t < literal_end
}

// IsBodyToken returns true if the token is a body mode token.
func (t Token) IsBodyToken() bool {
	return t > body_beg && t < body_end
}

// IsOperator returns true if the token is an operator or delimiter.
func (t Token) IsOperator() bool {
	return t > operator_beg && t < operator_end
}

// IsKeyword returns true if the token is a keyword.
func (t Token) IsKeyword() bool {
	return t > keyword_beg && t < keyword_end
}

// Lookup checks if the identifier is a keyword and returns the appropriate token.
// If not a keyword, returns IDENT.
func Lookup(ident string) Token {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
